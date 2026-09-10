# Maritime LoRa Mesh Network Simulator — Backend

Backend *real-time* berbasis **Go** yang menyimulasikan topologi jaringan LoRa
*mesh* maritim: ratusan aplikasi mobile (Flutter) terhubung lewat WebSocket
sebagai *node* kapal, backend bertindak sebagai *Network Controller*, *Virtual
Edge* (Syahbandar) sekaligus *Virtual Ether* — menghitung rute *multi-hop* dari
jarak geospasial, menegakkan batas fisik LoRa (Spreading Factor), dan
menyuntikkan *artificial packet loss* probabilistik lengkap dengan mekanisme
*Data Link* (ACK, auto-retry, deduplikasi).

Dokumen sumber: [`docs/prd.md`](docs/prd.md) · [`docs/srs.md`](docs/srs.md) ·
[`docs/agent_instruction.md`](docs/agent_instruction.md)

| Komponen | Teknologi |
|---|---|
| Bahasa | Go 1.24+ (`net/http` + `ServeMux` routing, tanpa framework berat) |
| WebSocket | Gorilla WebSocket + worker pool + channel |
| Database relasional | PostgreSQL 16 (pgx/v5, migrasi otomatis saat startup) |
| In-memory & geospasial | Redis 7 (`GEOADD`/`GEOPOS`, `SETNX` dedup, hash routing/stats) |
| Payload biner | MessagePack (`vmihailenco/msgpack/v5`), Base64 di dalam envelope JSON |
| Observability | `log/slog` JSON ke stdout, pprof opsional |
| Deploy | Docker multi-stage (non-root, healthcheck) + docker compose |

---

## Quickstart

```bash
cp .env.example .env        # opsional — semua nilai punya default
docker compose up -d --build
curl http://localhost:8080/healthz
```

`docker compose up` otomatis memuat `docker-compose.override.yml` (mode dev):
pprof di `127.0.0.1:6060`, Postgres di `127.0.0.1:5433`, Redis di
`127.0.0.1:6380`. Server melakukan migrasi skema sendiri saat boot dan
men-seed satu Virtual Edge (`EDGE-PRATU-01`, Syahbandar Pelabuhan Ratu).

Uji cepat dengan armada kapal virtual (50 node, TX rendah supaya terbentuk
rute multi-hop):

```bash
make smoke
# atau manual:
go run ./cmd/nodesim -api http://localhost:8080 -nodes 50 -tx 10 -spread-km 8 -duration 60s
```

Target `make` lain: `make help`.

### Menjalankan tanpa Docker

```bash
make up-deps   # hanya Postgres + Redis dalam container
make dev       # go run ./cmd/server dengan env dari shell
```

---

## Arsitektur

```
cmd/
  server/        # entrypoint: wiring, graceful shutdown, pprof
  nodesim/       # armada kapal virtual: harness verifikasi + referensi protokol klien
internal/
  api/           # REST presentation layer (handler, middleware, router)
  ws/            # WebSocket gateway: hub, client pump, worker pool
  protocol/      # kontrak event WS yang dipakai bersama (backend ⇄ mobile ⇄ dashboard)
  service/       # application layer: MeshTopology, SimulationEngine,
                 #   EnvironmentalAnomaly, DynamicSchema, Session
  domain/        # invariant murni: batas SF, model packet loss, Haversine,
                 #   BFS topologi, codec skema — 100% unit-testable
  repository/    # data access: Redis (geo/dedup/route/stats) + PostgreSQL
  infra/         # pgxpool, go-redis, slog, migrator
  config/        # konfigurasi env (12-factor)
migrations/      # SQL (di-embed ke binary, dijalankan otomatis saat startup)
```

Alur satu paket telemetri (`node:transmit`):

```
kapal C ──ws──▶ worker pool ──▶ SimulationEngineService
  1. validasi envelope + decode Base64 → []byte
  2. len(payload) ≤ batas byte SF?          ── tidak → drop (tanpa ACK)
  3. hop_count ≤ 5?                         ── tidak → drop
  4. resolve target: kapal lain / Virtual Edge
  5. dadu EnvironmentalAnomalyService (jarak × cuaca) ── kalah → drop (sender timeout → retry)
  6. dedup Redis SETNX (packet_id + penerima) ── duplikat → jangan kirim ulang, re-ACK sender
  7a. target kapal  → mesh:receive_rf ke target, catat pengirim utk relay ACK
  7b. target Edge   → unpack MessagePack sesuai skema dinamis → telemetry_logs
                      → ACK kembali ke pengirim (kena dadu juga)
```

`MeshTopologyService` berjalan tiap 5 detik per sesi aktif: tarik seluruh
posisi dari Redis GEO, susun DAG dengan BFS berlapis dari Edge keluar
(level 1 = kapal dalam jangkauan Edge, dst. sampai 5 hop), simpan ke Redis,
lalu kirim `mesh:routing_update` **hanya ke node yang rutenya berubah** dan
tabel penuh ke dashboard monitor.

---

## Konfigurasi (environment)

| Variabel | Default | Keterangan |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Listen address REST + WS |
| `POSTGRES_DSN` | — | Menimpa `POSTGRES_HOST/PORT/USER/PASSWORD/DB/SSLMODE` |
| `REDIS_ADDR` / `REDIS_PASSWORD` / `REDIS_DB` | `localhost:6379` / kosong / `0` | |
| `LOG_LEVEL` | `info` | `debug` menampilkan event per-paket (drop air_loss, forward, dedup) |
| `TOPOLOGY_INTERVAL_SECONDS` | `5` | Kadens kalkulasi ulang topologi (SRS §2B.3) |
| `PING_MIN_INTERVAL_SECONDS` | `3` | Rate-limit tulis GPS per node (PRD §3.5) |
| `WORKER_POOL_SIZE` | `0` (auto = 4×CPU) | Worker event WebSocket |
| `WS_SEND_BUFFER` | `256` | Buffer outbound per koneksi |
| `PPROF_ADDR` | kosong (off) | Contoh `127.0.0.1:6060`; bukti bebas *goroutine leak* |
| `CORS_ALLOWED_ORIGINS` | `*` | Isi daftar origin dashboard di produksi |
| `SHUTDOWN_TIMEOUT_SECONDS` | `10` | Batas graceful shutdown |

---

## REST API (dashboard → backend)

Semua respons memakai envelope `{"status":"success"|"error","message":"…","data":…}`.

| Method & Path | Fungsi |
|---|---|
| `POST /api/v1/simulations` | Buat sesi. Body: `{"session_name","spreading_factor":7,"tx_power_dbm":20,"weather_severity":1.0}` → `201` + `data.session_id` |
| `GET /api/v1/simulations?limit=` | Daftar sesi terbaru |
| `GET /api/v1/simulations/{id}` | Detail sesi + `live_stats` (counter Redis) |
| `POST /api/v1/simulations/{id}/stop` | Hentikan sesi: snapshot statistik → PG, putuskan semua socket, bersihkan state Redis |
| `DELETE /api/v1/simulations/{id}` | Hapus riwayat sesi (cascade skema + telemetri). `409` bila sesi masih aktif — hentikan dulu |
| `PUT /api/v1/simulations/{id}/weather` | `{"weather_severity":1.8}` — pengali packet-loss 0.0–2.0, langsung di-broadcast |
| `PUT /api/v1/simulations/{id}/params` | `{"spreading_factor":10,"tx_power_dbm":14}` (salah satu/keduanya) — batas byte & jangkauan berubah *real-time* |
| `POST /api/v1/simulations/{id}/schema` | `{"fields":[{"name":"berat_kg","type":"uint16"},…]}` — simpan + push `schema:sync` ke semua node |
| `GET /api/v1/simulations/{id}/schema` | Skema aktif |
| `GET /api/v1/simulations/{id}/topology` | Snapshot: posisi node (+online), tabel rute, daftar edge — untuk render peta |
| `GET /api/v1/simulations/{id}/telemetry?limit=&offset=` | Riwayat paket yang tiba di Syahbandar (payload sudah di-decode); tiap baris menyertakan `edge_code` (snapshot, bertahan meski edge dihapus) + `edge_active` (masih terdaftar hari ini?) |
| `GET /api/v1/simulations/{id}/stats` | Counter langsung (lihat daftar di bawah) |
| `POST /api/v1/edges` · `GET /api/v1/edges` | Master data Virtual Edge |
| `PUT /api/v1/edges/{code}` | Ubah nama/koordinat edge (`edge_code` tetap — rute aktif tidak terganggu) |
| `DELETE /api/v1/edges/{code}` | Hapus edge. Riwayat telemetrinya tetap tampil (nama dari snapshot), ditandai `edge_active:false` |
| `GET /healthz` · `GET /readyz` | Liveness / readiness (ping PG + Redis) |

Tipe field skema: `float32|float64|int8..int64|uint8..uint64|bool|string_N`
(`string_10` = maks 10 byte). Respons `POST /schema` menyertakan
`estimated_payload_bytes` dan `warning` bila estimasi melebihi jendela SF aktif.

Counter statistik: `transmit_total`, `delivered_to_edge`, `forwarded_to_node`,
`dropped_sf_limit`, `dropped_max_hops`, `dropped_air_loss`,
`dropped_out_of_range`, `dropped_target_offline`, `dropped_no_position`,
`dropped_invalid`, `duplicates_filtered`, `acks_relayed`, `acks_dropped`,
`acks_stale`, `pings_accepted`, `pings_rate_limited`.

---

## WebSocket

### Kanal node (kapal / aplikasi Flutter)

```
ws://HOST:8080/ws/nodes?session_id=<uuid>&node_id=KPL-001
```

Koneksi ditolak (`404`) bila sesi tidak aktif. Begitu tersambung, server
langsung mengirim `env:sync_params`, `schema:sync` (bila ada) dan
`mesh:routing_update` (bila rute sudah dihitung).

**Client → server**

| Event | Payload inti | Catatan |
|---|---|---|
| `node:ping` | `lat`, `lng` | Tiap 3 detik; identitas diambil dari koneksi, di-rate-limit server |
| `node:transmit` | `packet_id`, `origin_node`, `target_parent`, `hop_count`, `routing_path[]`, `binary_payload_b64` | Elemen terakhir `routing_path` **wajib** = node pengirim |
| `node:ack` | `packet_id`, `receiver_node`, `status` | Dikirim penerima setiap menerima `mesh:receive_rf` |

**Server → node**

| Event | Payload | Kapan |
|---|---|---|
| `env:sync_params` | `sf`, `max_payload_bytes`, `ack_timeout_ms`, `max_retries`, `tx_power_dbm`, `max_range_km`, `weather_severity` | Saat connect + setiap parameter berubah |
| `schema:sync` | `fields[]`, `estimated_packed_bytes` | Saat connect + setiap skema di-set |
| `mesh:routing_update` | `parent_target`, `parent_is_edge`, `distance_to_parent_km`, `hop_level`, `status` (`routed`/`isolated`) | Hanya saat rute node ini berubah |
| `mesh:receive_rf` | identik `node:transmit` | Frame tiba di jendela RX; balas `node:ack`, lalu teruskan ke parent-mu |
| `mesh:ack` | `packet_id`, `receiver_node`, `status`, `duplicate?` | Relay ACK ke pengirim — penutup loop retry |
| `session:ended` / `error` | — | Sesi dihentikan / frame ditolak |

**Kontrak Data Link untuk klien** (diimplementasikan persis di
[`cmd/nodesim`](cmd/nodesim/main.go) sebagai referensi):

1. Kirim `node:transmit`, tunggu `mesh:ack` dengan `packet_id` sama selama
   `ack_timeout_ms` (bergantung SF — SF7 1 s … SF12 10 s).
2. Timeout → kirim ulang dengan **packet_id yang sama**, maksimal **3×**.
3. Tiga kali gagal → deklarasikan link mati; tunggu `mesh:routing_update`
   berikutnya (topologi dihitung ulang tiap 5 detik).
4. Saat menerima `mesh:receive_rf`: selalu balas `node:ack`; abaikan
   `packet_id` yang sudah pernah diproses; bila punya rute, teruskan frame ke
   parent dengan `hop_count+1` dan namamu ditambahkan ke `routing_path`.

### Kanal monitor (dashboard Next.js — read-only)

```
ws://HOST:8080/ws/monitor?session_id=<uuid>
```

Menerima: relay `node:ping` (gerakkan marker), `node:status`
(online/offline), `mesh:routing_update` **versi tabel penuh**
(`routes: [{node,parent,parent_is_edge,distance_km,hop_level,status}]`),
`packet:event` (feed log langsung: `transmit|forward|deliver|drop|duplicate|ack|ack_drop`
+ jarak + probabilitas loss), `env:sync_params`, `schema:sync`,
`session:ended`. Snapshot tabel rute dikirim saat monitor baru tersambung.

---

## Aturan domain (invariant)

| Aturan | Nilai |
|---|---|
| Batas payload per SF | SF 7–8: **242 B** · SF 9: **115 B** · SF 10–12: **51 B** — dilanggar = drop tanpa ACK |
| Batas hop (TTL) | **5** — lebih = drop |
| ACK timeout per SF | 7→1000 ms · 8→1500 · 9→2500 · 10→4000 · 11→6500 · 12→10000 |
| Auto-retry | maks **3×**, packet_id sama |
| Jangkauan radio | `5 km × 2^((tx_dbm − 14)/6)` — +6 dB ≈ 2× jarak (20 dBm ≈ 10 km, 10 dBm ≈ 3.1 km) |
| Base packet loss | ≤2 km: **5%** · 2–5 km: **20%** · >5 km: **60%** · > jangkauan: **100%** |
| Cuaca | loss final = base × `weather_severity` (0.0–2.0), cap 100%; ACK kena dadu yang sama |
| Dedup | Redis `SETNX` TTL **30 s** per (`packet_id`, penerima) |
| Rate-limit GPS | 1 tulis / 3 s / node |

---

## Verifikasi

```bash
go test -race ./...   # unit test domain (SF, loss, Haversine, BFS mesh, codec msgpack) + service
make vet              # go vet + gofmt
make smoke            # 50 kapal end-to-end (perlu stack compose hidup)
make goroutines       # jumlah goroutine via pprof (dev)
```

### Bukti uji penerimaan (hasil run nyata)

Armada `nodesim` **50 node**, TX 10 dBm (jangkauan ≈ 3,1 km), sebaran 8 km,
75 detik, cuaca 1.0:

| Kriteria (PRD/agent_instruction) | Bukti terukur |
|---|---|
| >50 koneksi WS simultan tanpa *goroutine leak* | pprof: **57** goroutine baseline → **208** saat 50 kapal tersambung (3/koneksi, sesuai desain) → **kembali 57** setelah semua diputus |
| Rute multi-hop dari proksimitas jarak | Tabel rute live: hop1 7 · hop2 15 · hop3 17 · hop4 9 · **hop5 2**; contoh jalur tercatat `KPL-009->KPL-010->KPL-008->KPL-017->EDGE-PRATU-01` |
| Paket biner utuh & ter-unpack di Edge | **273 baris** `telemetry_logs`, payload MessagePack ter-decode penuh (`{"berat_kg":148,"jenis_ikan":"tongkol",…}`), distribusi hop_count 1→5 |
| ACK & auto-retry | 1030 frame terkirim termasuk **308 retry**; 706 ACK diterima klien; 6 link dideklarasi mati setelah 3× gagal |
| Deduplikasi Redis `SETNX` | **136 duplikat retry** tersaring server (klien melihat 0 duplikat) |
| Injeksi cuaca menaikkan *drop rate* | Sesi cuaca 1.0: drop udara 17,3% (178/1030) → sesi cuaca **2.0**: **31,9%** (52/163) ≈ ×1,85 |
| Perubahan SF runtime memicu validasi byte | `PUT params {"spreading_factor":10}` di tengah sesi → `env:sync_params` tersebar (terlihat di socket monitor), `dropped_sf_limit` naik dari 0 → 15+ dan pengiriman ke Edge membeku (payload 58 B > jendela 51 B) |
| Migrasi PG + skema Redis terinisiasi | `migration applied version=0001_init.sql` saat boot; seluruh key `sim:{id}:*` terbentuk/terhapus mengikuti lifecycle sesi |
| Statistik dipersistenkan saat stop | `POST /stop` → `stats_snapshot` beku di PostgreSQL; stop kedua = 409; WS ke sesi mati = 404 |
| **Edit edge (`PUT /edges/{code}`)** | Nama/koordinat berubah, `edge_code`/`created_at` tetap; target kode asing = **404** |
| **Riwayat telemetri bertahan setelah edge dihapus** | Armada `nodesim` terkirim ke edge X → `edge_code`/`edge_active:true` pada baris telemetri → `DELETE /edges/X` → baris yang sama kini `edge_code` **tetap tampil**, `edge_active` berubah **`false`** (bukan `—` kosong seperti sebelum migrasi `0002`) |
| **Hapus riwayat sesi (`DELETE /simulations/{id}`)** | Sesi aktif → **409**; `POST /stop` → **200**; `DELETE` lagi → **200**; `GET` sesudahnya → **404**; `DELETE` kedua kali → **404** (siklus penuh 409→200→200→404→404 diverifikasi langsung) |

---

## Production deployment (VPS)

```bash
# di VPS (Docker + compose plugin terpasang)
git clone <repo> && cd backend
cp .env.example .env
$EDITOR .env                       # WAJIB: ganti POSTGRES_PASSWORD, set CORS_ALLOWED_ORIGINS
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

Overlay produksi mengikat API ke `127.0.0.1:8080` (database & Redis tidak
pernah terekspos), menyalakan `restart: always` dan rotasi log JSON. Pasang
reverse proxy untuk TLS supaya dashboard memakai `https://` dan kapal memakai
`wss://` — contoh Caddy (otomatis Let's Encrypt):

```
# /etc/caddy/Caddyfile
simulator.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

Caddy/Nginx meneruskan upgrade WebSocket secara default (`/ws/nodes`,
`/ws/monitor`). Update versi: `git pull && docker compose -f docker-compose.yml
-f docker-compose.prod.yml up -d --build` — migrasi berjalan otomatis, sesi
yang masih aktif dipulihkan ke scheduler saat boot. Backup: `docker compose
exec postgres pg_dump -U simulator lora_simulator > backup.sql` (state Redis
sengaja volatile). Kapasitas: worker pool & buffer dapat dinaikkan lewat env
(`WORKER_POOL_SIZE`, `WS_SEND_BUFFER`) tanpa rebuild.

---

## Keputusan desain & deviasi terdokumentasi dari SRS

| Topik | Keputusan | Alasan |
|---|---|---|
| Router | `net/http` ServeMux (Go 1.22+) alih-alih Fiber v3 | SRS mengizinkan keduanya; Gorilla WebSocket butuh `net/http` asli (Fiber berbasis fasthttp), dependensi lebih ramping |
| Kunci dedup | `sim:{id}:dedup:{packet_id}:{receiver}` (SRS: tanpa receiver) | packet_id yang sama **sah** melintasi banyak hop (C→A lalu A→B); tanpa scope penerima, forward hop kedua akan salah ditandai duplikat |
| Duplikat retry | Disaring **dan di-re-ACK** ke pengirim (lewat dadu loss) | Receiver LoRa nyata me-re-ACK duplikat; diam total membuat pengirim retry-loop untuk paket yang sebenarnya sudah sampai |
| `stats_snapshot JSONB` di `simulation_sessions` | Kolom tambahan di luar SRS | PRD mewajibkan persistensi statistik loss/retry; skema SRS tidak memberi tempatnya |
| Endpoint tambahan | `GET` sesi/topology/telemetry/stats/schema/edges, `POST stop`, `PUT params`, `POST edges`, `PUT edges/{code}`, `DELETE simulations/{id}` | Dashboard (PRD frontend) membutuhkan data ini; kriteria PRD menuntut perubahan SF runtime; edge harus bisa didaftarkan, disunting, dan riwayat sesi lama harus bisa dibersihkan dari daftar |
| `telemetry_logs.edge_code_snapshot` (migrasi `0002`) | Kolom tambahan di luar SRS | `edge_id` ber-`ON DELETE SET NULL`; tanpa snapshot terpisah, menghapus sebuah Edge membutakan nama Edge pada **seluruh** riwayat pengiriman lamanya, bukan cuma tautannya |
| Kanal monitor WS | `/ws/monitor` dengan relay `node:ping`, `packet:event`, tabel rute penuh | PRD frontend mendengarkan `node:ping` & `mesh:routing_update`; node hanya menerima parent-nya sendiri (memori ESP32), dashboard butuh peta penuh |
| Nama event baru | `schema:sync`, `mesh:ack`, `node:status`, `packet:event`, `session:ended` | SRS mewajibkan push skema & loop ACK tetapi tidak menamai eventnya |
| Tabel `users` | Dimigrasikan sesuai SRS, **tanpa** endpoint auth | Registry REST SRS tidak mendefinisikan endpoint auth; membangun auth setengah jadi = slop. Tambahkan bersama dashboard |
| Tipe PostgreSQL | `BIGSERIAL`/`TIMESTAMPTZ`/`JSONB` menggantikan idiom MySQL (`BIGINT UNSIGNED`) | SRS ditulis dengan tipe MySQL; PostgreSQL 16 adalah DB yang dimandatkan |
| Seed edge | `EDGE-PRATU-01` dibuat via migrasi (idempoten) | Simulator langsung bisa dipakai tanpa langkah manual |
| Sesi & edge | Edge bersifat global; semua edge aktif untuk semua sesi | SRS tidak punya tabel relasi sesi↔edge; topologi memilih edge terdekat |

## Batasan yang diketahui

- **Single-instance**: registry koneksi WebSocket hidup di memori proses.
  Skala horizontal membutuhkan sticky routing per sesi atau Redis Pub/Sub
  antar instance (di luar lingkup SRS saat ini).
- **Tanpa autentikasi**: sesuai registry SRS. Jangan mengekspos API mentah ke
  internet publik tanpa reverse proxy + pembatasan akses.

---

## Lisensi & Kepatuhan Standar ISO

- **ISO/IEC 25010**: Functional Suitability, Reliability, & Zero Goroutine Leaks.
- **ISO/IEC 27001**: Data Integrity, Secure LoRa Framing, & Vulnerability Reporting ([SECURITY.md](SECURITY.md)).
- **ISO/IEC 12207**: Software Life Cycle Processes & Conventional Commits.
- **Lisensi**: MIT License. Lihat [LICENSE](LICENSE).

## Author

**Daffa Jaya Perkasa**  
Full-Stack | Mobile | AI | DevOps Engineer
