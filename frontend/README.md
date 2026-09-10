# Maritime LoRa Mesh Network Simulator — Frontend Dashboard

Dasbor pemantauan *real-time* untuk [backend simulator Go](../backend): memvisualisasikan
posisi armada kapal nelayan di peta interaktif, menggambar rute mesh multi-hop
(Kapal → Kapal → … → Syahbandar), dan menyediakan panel kendali untuk menginjeksi anomali
cuaca serta mengubah parameter LoRa (Spreading Factor / TX Power) saat simulasi berjalan.

**Stack:** Next.js 14 (App Router) · React 18 · TypeScript (strict, tanpa `any`) ·
Zustand · React-Leaflet v4 · TailwindCSS v3 · Axios · native HTML5 WebSocket.

---

## Fitur

| Halaman | Isi |
|---|---|
| `/` | Landing + indikator kesehatan backend |
| `/setup` | Form sesi baru (nama, slider SF 7–12, TX 2–20 dBm, cuaca awal), **Schema Builder** payload dinamis dengan estimasi byte live vs plafon SF, master data Virtual Edge, daftar sesi tersimpan |
| `/dashboard?session=<uuid>` | Peta fullscreen (basemap gelap CARTO) + panel overlay *glassmorphism*: metrik jaringan, slider badai, pengubah SF/TX runtime, log stream auto-scroll, daftar armada (klik → kamera terbang ke kapal) |
| `/reports?session=<uuid>` | Laporan historis: statistik sesi (live / snapshot beku) + tabel telemetri yang mendarat di Syahbandar, berpaginasi |

Invarian visual (SRS §3): marker **hijau** `#10B981` terhubung, **kuning** `#F59E0B`
*retrying* (menunggu ACK), **merah** `#EF4444` terisolasi; garis rute putus-putus biru
muda dengan panah arah node → parent; klik marker memunculkan popup berisi payload
terakhir yang berhasil sampai Edge.

---

## Arsitektur

Empat layer sesuai PRD §3, dipetakan ke direktori `src/`:

```
src/
├── app/                    # 1. Presentation — halaman App Router
│   ├── page.tsx            #    landing
│   ├── setup/page.tsx      #    konfigurasi pra-simulasi
│   ├── dashboard/page.tsx  #    live mesh monitor (peta dimuat dynamic ssr:false)
│   ├── reports/page.tsx    #    laporan historis
│   └── api/health/route.ts #    liveness probe container
├── components/
│   ├── map/                #    MapViewer, ShipMarker, RouteLine, EdgeMarker, controller
│   ├── dashboard/          #    StatusBar, MetricsPanel, WeatherControl, RadioParams…
│   ├── setup/              #    SessionForm, SchemaBuilder, EdgePanel, SessionList
│   └── ui/                 #    GlassCard
├── stores/                 # 2. State — Zustand per domain
│   ├── topologyStore.ts    #    Record<id, ShipNode> O(1) + edges (+ unit test)
│   ├── simulationStore.ts  #    sesi, env params, status WS
│   ├── metricStore.ts      #    counter kanonik backend (+ unit test)
│   ├── logStore.ts         #    ring buffer 200 baris
│   └── uiStore.ts          #    fokus kamera peta
├── hooks/
│   └── useMonitorSocket.ts # 4. Transport — siklus hidup WS + reconnect + re-hidrasi
├── lib/
│   ├── api.ts              # 4. Transport — klien Axios + pembongkar envelope
│   ├── events.ts           #    router event WS -> store (pure, teruji)
│   ├── lora.ts             # 3. Domain mirror — batas SF/TX/byte, estimasi msgpack
│   ├── backoff.ts          #    jadwal reconnect eksponensial (teruji)
│   ├── geo.ts              #    bearing panah rute (render-only)
│   ├── config.ts           #    resolusi NEXT_PUBLIC_API_URL / WS
│   └── format.ts           #    label log, waktu, persen
└── types/backend.ts        #    mirror presisi struct Go (REST + event WS)
```

**Aturan performa yang dipegang** (PRD §3.5, SRS §3B):

- Store node memakai `Record` (lookup O(1)); aksi hanya mengganti objek node yang
  benar-benar berubah, komponen marker ber-subscribe **per-node** via selector — kapal
  lain bergerak ⇒ marker ini tidak render ulang (ada unit test identitas referensi).
- `MapContainer preferCanvas`: 50+ CircleMarker digambar di satu `<canvas>`, bukan 50+
  elemen DOM/SVG (trade-off dikomentari di `MapViewer.tsx` dan `ShipMarker.tsx`).
- Event WS diproses lewat `store.getState()` di luar siklus render React.
- Log stream dibatasi ring buffer 200 baris.
- Pergerakan kapal 100% *event-driven* dari WebSocket — tidak ada `setInterval` palsu.

---

## Kontrak integrasi dengan backend

Sumber kebenaran: `../backend/README.md` + `backend/internal/protocol/events.go`.
Semua tipe di `src/types/backend.ts` memetakan struct Go 1:1 (termasuk slice `nil` Go
yang menjadi `null` JSON — seluruh array dari backend bertipe `T[] | null`).

### REST (Axios, `lib/api.ts`)

| Method & Path | Dipakai oleh |
|---|---|
| `POST /api/v1/simulations` | SessionForm |
| `GET /api/v1/simulations` · `GET /{id}` · `POST /{id}/stop` | SessionList, hidrasi dasbor, StatusBar |
| `DELETE /api/v1/simulations/{id}` | SessionList (hapus riwayat sesi; `409` bila sesi masih aktif) |
| `PUT /{id}/weather` · `PUT /{id}/params` | WeatherControl, RadioParamsControl |
| `POST /{id}/schema` · `GET /{id}/schema` | SchemaBuilder, hidrasi |
| `GET /{id}/topology` · `GET /{id}/stats` · `GET /{id}/telemetry` | hidrasi peta, MetricsPanel, Reports (kolom Edge: `edge_code` + badge `edge_active`), popup marker |
| `GET /api/v1/edges` · `POST /api/v1/edges` · `PUT /api/v1/edges/{code}` · `DELETE /api/v1/edges/{code}` | EdgePanel (daftar, tambah, **sunting**, hapus) |
| `GET /healthz` | BackendStatus |

Semua respons dibungkus envelope `{status, message, data}`; `lib/api.ts` melempar
`ApiError` berisi pesan validasi asli backend sehingga form menampilkan alasan persis.

### WebSocket monitor (read-only)

```
ws://<backend>/ws/monitor?session_id=<uuid>
```

| Event | Reaksi frontend |
|---|---|
| `env:sync_params` (priming + tiap perubahan) | chip SF/TX/jangkauan/ACK/cuaca di StatusBar |
| `mesh:routing_update` (tabel penuh) | gambar ulang Polyline + status routed/isolated |
| `node:ping` | geser marker kapal |
| `packet:event` (`transmit/forward/deliver/drop/duplicate/ack/ack_drop`) | counter metrik + log stream + derivasi status kuning *retrying* |
| `node:status` | dim marker offline + baris log |
| `schema:sync` | info skema aktif |
| `session:ended` | banner sesi berakhir (lihat *Temuan* di bawah) |

Reconnect otomatis: backoff eksponensial 1s→30s + jitter (teruji), dengan pengecekan
REST `is_active` sebelum tiap percobaan agar loop berhenti permanen saat sesi memang
sudah usai; setiap koneksi ulang melakukan re-hidrasi penuh (sesi + statistik +
snapshot topologi) karena event yang terlewat tidak diputar ulang backend.

---

## Menjalankan

Prasyarat: Node.js ≥ 18.17 dan backend hidup di `:8080`
(`cd ../backend && docker compose up -d`).

```bash
npm install
cp .env.example .env.local        # NEXT_PUBLIC_API_URL=http://localhost:8080
npm run dev                       # dev server di :3000

npm test                          # unit test vitest (30 test: stores, events, lora, backoff)
npm run lint                      # ESLint (termasuk larangan `any`)
npm run build && npm start        # build + serve produksi
```

### Docker

```bash
docker compose up -d --build      # dev lokal; browser -> backend di localhost:8080
```

Catatan penting: `NEXT_PUBLIC_*` di-*inline* saat **build** (perilaku Next.js), dan
alamat tersebut adalah alamat backend **dilihat dari browser pengguna** — container
frontend sendiri tidak pernah memanggil backend. Ganti alamat ⇒ build ulang image.

### Deploy produksi (VPS)

```bash
NEXT_PUBLIC_API_URL="" docker compose \
  -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

`NEXT_PUBLIC_API_URL` kosong = **mode same-origin**: browser memakai origin domain
sendiri, dan nginx/Caddy di depan mem-proxy jalur API/WS ke container backend — satu
domain, TLS otomatis menjadikan `wss://`. Contoh nginx:

```nginx
server {
  server_name simulator.example.com;
  location / {
    proxy_pass http://127.0.0.1:3000;      # frontend (prod overlay bind localhost)
  }
  location ~ ^/(api/v1|healthz|readyz) {
    proxy_pass http://127.0.0.1:8080;      # backend REST
  }
  location /ws/ {
    proxy_pass http://127.0.0.1:8080;      # backend WebSocket
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 1h;
  }
}
```

### Variabel lingkungan

| Variabel | Default | Keterangan |
|---|---|---|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` (dev) / kosong (prod) | Base URL backend dari sudut pandang browser; kosong = same-origin |
| `NEXT_PUBLIC_WS_URL` | *(kosong)* | Override WS; default diturunkan dari API URL (`http→ws`, `https→wss`) |
| `FRONTEND_PORT` | `3000` | Port publikasi container |

---

## Keputusan desain & deviasi terdokumentasi dari SRS

Frontend **beradaptasi ke backend riil** (instruksi proyek); saat dokumen SRS frontend
dan implementasi backend berbeda, backend yang menang:

| Topik | SRS/instruksi frontend | Implementasi (mengikuti backend) |
|---|---|---|
| URL kanal dasbor | `ws://…/ws/admin/{session_id}` | `GET /ws/monitor?session_id=<uuid>` — path `/ws/admin` tidak ada di backend |
| Event pergerakan | `node:telemetry_ping` | `node:ping` (relay verbatim ke monitor) |
| Event statistik | `sim:packet_dropped`, `sim:packet_retry` | `packet:event` tunggal ber-`type` (`drop`, `duplicate`, `ack_drop`, …) + `reason` |
| Nama event routing | `mesh:routing_table` (agent_instruction) | `mesh:routing_update`; varian monitor membawa tabel penuh `routes[]` |
| Rentang slider cuaca | 1.0–2.0 | 0.0–2.0 sesuai `domain.MinWeatherSeverity/MaxWeatherSeverity` |
| Status *Retrying* (kuning) | disebut sebagai status node | backend tidak menyimpan status retrying eksplisit — **diderivasi klien** dari aliran `packet:event` (drop/duplicate/ack_drop ⇒ kuning; forward/deliver/ack ⇒ pulih; tabel routing baru me-reset) |
| `mesh:ack` | — | hanya dikirim ke node transmitter, bukan ke monitor; dasbor memakai `packet:event type=ack` |
| Marker kapal | `<Marker>` + `React.memo` | `CircleMarker` renderer Canvas + memo + selector per-node — alasan performa dikomentari di kode |
| Panah arah rute | plugin decorator | divIcon CSS dirotasi bearing (plugin pihak ketiga tak terawat; trade-off di `RouteLine.tsx`) |
| Estimasi byte skema | — | mirror `EstimatePackedBytes` Go (worst-case); terverifikasi identik dengan backend (58 B untuk skema contoh) |
| `transmit_total` / "Frame Kirim" | diasumsikan ada `packet:event` live | backend **tak menyiarkan** tipe `transmit` (konstanta `PktTransmit` tak terpakai) — counter disegarkan lewat **polling `/stats` tiap 4 s** (`STATS_POLL_MS`), bukan stream WS |
| Warna status kapal | 3 warna SRS §3A (hijau/kuning/merah) | +`offline` abu-abu (`#64748B`) agar kapal terputus terbaca "pergi", bukan "error merah" isolated |
| Kapal `node:status offline` | hanya tandai flag offline | rute digugurkan (status→`isolated`, parent→null) agar garis rute hantu hilang; "Terisolasi" hanya hitung kapal **online tanpa rute** |
| Ikon UI | — | set SVG inline self-hosted (`ui/icons.tsx`, tanpa dependensi) demi image Docker hermetis; tak ada emoji sebagai ikon, tak ada tautan-teks untuk aksi perintah |

**Temuan integrasi (perilaku backend, bukan bug frontend):** saat `POST /stop`,
backend menyiarkan `session:ended` lalu langsung menutup semua socket
(`KickSession`) — pada praktiknya penutupan menang balapan sehingga monitor hampir
tidak pernah menerima event tersebut (direproduksi konsisten). Dasbor karena itu
**tidak bergantung** pada event ini: saat socket tertutup, hook reconnect mengecek
`GET /simulations/{id}` dan menandai sesi berakhir dari `is_active=false`. Banner
"sesi berakhir" muncul ±1 detik setelah stop.

---

## Bukti uji integrasi (hasil run nyata)

Diuji terhadap stack backend compose (PostgreSQL 16 + Redis 7 + backend Go) dan armada
`nodesim` 20 kapal (interval ping 3 s, transmit 8 s), 2026-07-02:

| Pengujian | Hasil |
|---|---|
| `npm test` | 30/30 lulus (stores, event router, mirror LoRa, backoff) |
| `npm run lint` / `next build` | 0 error; dashboard first-load JS 133 kB |
| Probe WS monitor 20 s | 281 frame: 75 `node:ping`, tabel `mesh:routing_update`, 6 jenis `packet:event` — seluruh bentuk payload cocok dengan `types/backend.ts` |
| `PUT /weather 1.8` + `PUT /params SF10→SF7` | tiap perubahan memicu broadcast `env:sync_params` (3× tertangkap probe) → chip StatusBar |
| CORS dari origin dasbor | `Access-Control-Allow-Origin: *` (default backend) |
| Telemetri | 368 baris mendarat, multi-hop sampai hop 4 (`KPL-018→KPL-017→KPL-007→KPL-001→EDGE-PRATU-01`), payload terdekode `{jenis_ikan, berat_kg, suhu_c}` |
| Stop sesi | `stats_snapshot` terbekukan (delivered 368) → dipakai halaman Reports untuk sesi usai |
| Image Docker | multi-stage, 186 MB, non-root uid 10001, healthcheck `healthy`, start `✓ Ready in 44ms` |
| Halaman via container | `/`, `/setup`, `/dashboard`, `/reports`, `/api/health` semua HTTP 200 |

### Putaran audit UI/UX + perbaikan (3 Juli 2026)

Audit lanjutan (skill *design-intelligence*) + uji ulang langsung terhadap stack Docker:

| Pengujian | Hasil |
|---|---|
| `npm test` | **31/31 lulus** (+1: `setOnline` menggugurkan rute jadi isolated) |
| `npm run lint` / `next build` | 0 error; dashboard first-load 134 kB (< budget 150 kB) |
| Fix "Frame Kirim" beku (polling `/stats`) | `transmit_total` hidup **24→40→57→68→84** (armada SF9/TX8) |
| Fix rute hantu kapal offline | probe: **4 `node:status online` → 4 `offline`**; topologi kosong pasca-bubar |
| Relay multi-hop (TX8/spread 8 km) | `forwarded_to_node` **8→16→24→28→33** (relay antar-kapal aktif) |
| Perapian UI | ikon SVG (bukan emoji/underline), kontras `slate-400/500`, tooltip anti-terpotong, `tabular-nums`, `:focus-visible`, `prefers-reduced-motion` |

Rincian lengkap + cara reproduksi: [`docs/laporan_integrasi_id.md`](docs/laporan_integrasi_id.md) (§8 adendum).

### Putaran perapian lanjutan (3 Juli 2026)

Audit visual kedua atas dasbor yang berjalan di Docker:

| Perbaikan | Ringkas |
|---|---|
| Judul hero terpotong | `leading` + `pb` pada teks gradien (`text-5xl` line-height 1) |
| Spinner TX Power | disembunyikan (`.no-spinner`) |
| Log Transmisi | tak dibatasi 200 baris (audit penuh, `content-visibility` untuk ribuan baris) |
| Kontras popup kapal/edge | selektor `.leaflet-popup` menang atas Leaflet, warna dasar `slate-100` |
| Atribusi peta kanan-bawah | dihapus (`attributionControl={false}`; kredit lisensi tetap di `TILE_ATTRIBUTION`) |
| Animasi aliran data | partikel node→parent/Edge tiap hop sukses (`FlowLayer`, dibatasi 26, hormat reduced-motion) |
| Responsif | panel jadi *bottom-sheet* di ponsel; kolom kanan 400px mulai `sm` |
| Hapus Virtual Edge | tombol ikon tempat sampah + `DELETE /api/v1/edges/{code}` baru (teruji live 201/200/404) |

Rincian: [`docs/laporan_integrasi_id.md`](docs/laporan_integrasi_id.md) (§9). Karena endpoint
backend baru menyentuh kontrak, `backend/docs/srs.md` (§3.4) & `mobile/docs/srs.md` ikut diperbarui.

### Putaran siklus-hidup Virtual Edge & Sesi (3 Juli 2026)

Menambah **sunting Edge** (EdgePanel, tombol pensil di sebelah kiri tombol hapus) dan
**hapus riwayat sesi** (SessionList, tombol tempat sampah di sebelah kanan
Laporan/Buka Dasbor) + memperbaiki kolom "Edge" Reports yang sebelumnya menampilkan
`—` begitu edge pengantarnya dihapus, padahal paket itu benar-benar pernah sampai.

| Pengujian | Hasil |
|---|---|
| `npx tsc --noEmit` / `npm run lint` / `next build` | 0 error; halaman `/reports` 2.98 kB, `/setup` 4.29 kB |
| `npm test` | **31/31 lulus** (tak berubah — perubahan murni UI + REST baru) |
| `PUT /api/v1/edges/{code}` live | nama/koordinat berubah, `edge_code`/`created_at` tetap; kode asing → `404` |
| Snapshot nama edge bertahan setelah dihapus | armada `nodesim` kirim ke edge X → baris telemetri `edge_code=X, edge_active=true` → `DELETE /edges/X` → baris **sama** kini `edge_code=X` (tetap tampil, bukan `—`), `edge_active=false` — badge Reports otomatis merah |
| `DELETE /api/v1/simulations/{id}` live | sesi aktif → `409`; `stop` → `200`; `delete` → `200`; `GET` sesudahnya → `404`; `delete` kedua → `404` (siklus 409→200→200→404→404 diverifikasi) |
| Tombol hapus sesi terkunci saat aktif | `disabled` + tooltip "hentikan sesi terlebih dahulu" pada baris `is_active` |

Karena kontrak backend berubah (endpoint baru + field `edge_active` pada
`TelemetryLog`), `backend/docs/srs.md`, `backend/README.md`, `mobile/docs/srs.md`,
dan `simulation/docs/maritime_lora_simulator_documentation.md` ikut diperbarui.

---

## Skrip npm

| Perintah | Fungsi |
|---|---|
| `npm run dev` | dev server (hot reload) di :3000 |
| `npm run build` / `npm start` | build produksi (standalone) / serve |
| `npm test` / `npm run test:watch` | unit test vitest |
| `npm run lint` | ESLint `src/` |

Pintasan `make` tersedia (`make up`, `make prod-up`, `make test`, …).

---

## Lisensi & Kepatuhan Standar ISO

- **ISO/IEC 25010**: Functional Suitability, Usability, & Real-Time Monitoring Performance.
- **ISO/IEC 27001**: Data Protection & Centralized Vulnerability Disclosure ([SECURITY.md](SECURITY.md)).
- **ISO/IEC 12207**: Software Life Cycle Processes & Conventional Commits.
- **Lisensi**: MIT License. Lihat [LICENSE](LICENSE).

## Author

**Daffa Jaya Perkasa**  
Full-Stack | Mobile | AI | DevOps Engineer
