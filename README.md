# Maritim Node — Mobile Node Service (Flutter)

Terminal **node kapal** untuk *Maritime LoRa Mesh Network Simulator*. Aplikasi
ini mengambil alih peran perangkat keras radio (ESP32/SX1276) dalam simulasi
*software-in-the-loop*: merakit payload menjadi **biner murni (MessagePack)**,
memelihara antrean **Store-and-Forward** di SQLite, menjalankan mesin
**ACK & Auto-Retry**, dan mengeksekusi **relay mesh** — semuanya lewat
WebSocket ke backend Go (*Virtual Ether*).

| Komponen | Peran | Branch |
| --- | --- | --- |
| `backend/` (Go) | Virtual Ether: fisika radio, routing mesh, Syahbandar | `backend` |
| `frontend/` (Next.js) | Dashboard kendali & monitoring peta real-time | `frontend` |
| **`mobile/` (Flutter)** | **Node kapal: sumber & relay paket LoRa virtual** | `mobile` |

- Versi: **1.0.0+1** (lihat [CHANGELOG.md](CHANGELOG.md))
- Kualitas: `flutter analyze` bersih · **40/40** unit+widget test ·
  **3/3** skenario integrasi live vs backend Docker (`test_live/`)

---

## 1. Arsitektur (Clean Architecture berlapis)

Mengikuti panduan `.skills/flutter-apply-architecture-best-practices`
(UI ↔ Domain ↔ Data) dengan **Riverpod** sebagai state management & DI
sesuai mandat PRD/SRS. Logika asinkron (Timer ACK, SQLite, socket) hidup
eksklusif di *repository* — widget tree tidak pernah menyentuhnya.

```
lib/
├── main.dart / app.dart          # entrypoint + MaterialApp
├── core/
│   ├── config/                   # AppConfig (preferensi) + NodeSessionConfig
│   └── utils/backoff.dart        # exponential backoff 1s→30s
├── domain/                       # model murni tanpa dependensi IO
│   ├── models/                   # EnvParams, SchemaField, RoutingInfo,
│   │                             # TransmitFrame, MeshAck, QueueEntry, …
│   ├── node_event.dart           # sealed union seluruh event kanal node
│   └── node_link_state.dart      # potret immutable untuk UI (LinkPhase, dst.)
├── data/
│   ├── services/                 # IO stateless:
│   │   ├── payload_codec.dart    #   MessagePack binary-first validator
│   │   ├── queue_database.dart   #   sqflite transmit_queue (transaksional)
│   │   ├── node_socket.dart      #   WebSocket + auto-reconnect backoff
│   │   ├── rest_client.dart      #   REST bootstrap (sesi, healthz, stats)
│   │   └── location_service.dart #   geolocator / koordinat manual + drift
│   └── repositories/
│       ├── node_link_repository.dart  # ★ MESIN DATA LINK (ACK/retry/relay)
│       └── config_repository.dart     # SharedPreferences
├── di/providers.dart             # graf dependensi Riverpod (overridable)
└── ui/
    ├── core/                     # tema (palet = frontend), AppLogo vektor
    └── features/
        ├── splash/               # splash animasi (logo CustomPaint)
        ├── setup/ (+view_models) # server, sesi aktif, Node ID, mode GPS
        └── node/  (+view_models) # HUD + tab Kirim / Antrean / Log
```

### Mesin Data Link (`NodeLinkRepository`)

1. **Originasi** — form dinamis → `PayloadCodec.pack()` (MessagePack) →
   ukuran diuji dari `bytes.length` murni terhadap `max_payload_bytes`
   (**dilarang** `jsonEncode().length`) → simpan `PENDING` → tembak
   `node:transmit` → `Timer` menunggu `mesh:ack` selama `ack_timeout_ms`
   (+500 ms toleransi, identik harness `nodesim` backend).
2. **Timeout** — transaksi `bumpRetryOrFail`: kirim ulang dengan
   **`packet_id` yang sama** (menguji dedup server) maksimal `max_retries`,
   lalu `FAILED (Link Lost)`.
3. **Relay** (`mesh:receive_rf`) — balas `node:ack` **seketika** (duplikat
   pun di-ACK), dedup via keunikan `packet_id`, simpan dengan **`origin_node`
   asli** (identitas pembuat tidak pernah ditimpa), teruskan ke parent dengan
   `hop_count+1` dan `routing_path` diperpanjang identitas node ini.
4. **Isolated / offline** — paket ditahan `PENDING` di SQLite; saat
   `mesh:routing_update` memulihkan rute (atau kanal tersambung ulang),
   antrean **dipompa ulang** dengan retarget ke parent baru.
5. **GPS ping** — `node:ping` tiap 3 detik (GPS asli `geolocator` atau manual
   + drift random-walk ala `nodesim.move`).
6. **Ketahanan** — reconnect exponential backoff (1→30 s); bila dial gagal
   3× beruntun, verifikasi REST `is_active` (event `session:ended` bisa kalah
   balapan dengan penutupan socket di server).

---

## 2. Kontrak protokol (sumber kebenaran: backend)

Kontrak yang diimplementasikan adalah **`backend/internal/protocol/events.go`**
— bukan contoh payload di SRS mobile — karena backend + `cmd/nodesim` adalah
implementasi acuan yang sudah teruji. WS node: `GET /ws/nodes?session_id&node_id`.

**Keluar (klien → server)**

| Event | Payload inti |
| --- | --- |
| `node:ping` | `node_id, session_id, lat, lng` — tiap 3 s |
| `node:transmit` | `packet_id, origin_node, target_parent, hop_count, routing_path[], binary_payload_b64` |
| `node:ack` | `packet_id, receiver_node, status:"received"` — balasan frame relay |

**Masuk (server → klien)**

| Event | Reaksi klien |
| --- | --- |
| `env:sync_params` | `sf, max_payload_bytes, ack_timeout_ms, max_retries, …` → limit byte counter + parameter mesin retry |
| `schema:sync` | `fields[], estimated_packed_bytes` → susun ulang form dinamis |
| `mesh:routing_update` | `parent_target, parent_is_edge, hop_level, status` → next-hop; `isolated` mengunci Kirim, antrean ditahan |
| `mesh:receive_rf` | frame asing → ACK + simpan + forward (relay) |
| `mesh:ack` | kaki balik ACK → batalkan Timer, tandai `ACKED` |
| `session:ended` | hentikan mesin permanen (tanpa reconnect) |
| `error` | frame kita ditolak (mis. `transmitter_mismatch`) → jurnal |

### Deviasi SRS mobile → kontrak backend nyata (yang dipakai)

| SRS mobile (docs/srs.md) | Backend nyata (diimplementasikan) |
| --- | --- |
| field `target` | `target_parent` |
| field `payload_b64` | `binary_payload_b64` |
| field `origin_node_id` di frame | `origin_node` |
| ACK balik tak bernama | event `mesh:ack` (+`receiver_node`, `duplicate`) |
| skema ikut `env:sync_params.schema` | event terpisah `schema:sync` |
| timeout ACK dihitung klien dari SF | `ack_timeout_ms` & `max_retries` didikte server |
| — (tersirat) | `routing_path` **wajib diakhiri pemancar** (jika tidak: reject `transmitter_mismatch`) |
| tabel queue tanpa hop/jalur | + kolom `hop_count`, `routing_path` (wajib ikut saat retry/relay) |

Skema SQLite `transmit_queue` selebihnya persis SRS §2 (`packet_id` UNIQUE,
`origin_node_id`, `target_parent`, `payload_b64`, `status`, `retry_count`).

---

## 3. Menjalankan & menguji

Prasyarat: Flutter ≥ 3.44 (Dart ≥ 3.12), dan untuk uji live: stack backend
(branch `backend`) berjalan via Docker.

```bash
flutter pub get

# jalankan di perangkat/emulator Android
flutter run

# gerbang mutu
flutter analyze          # 0 issue
flutter test             # 40 unit + widget test

# uji integrasi LIVE vs backend nyata (di luar test/ agar CI unit bebas Docker)
cd ../backend && docker compose up -d && cd ../mobile
flutter test test_live   # 3 skenario: originasi→telemetri, relay multi-hop
                         # origin terjaga, timeout→retry 3x→FAILED
```

**Regenerasi branding** (bila logo frontend berubah):

```bash
python3 tool/generate_branding.py      # rasterisasi ulang ikon+splash
dart run flutter_launcher_icons        # ikon launcher Android/iOS
dart run flutter_native_splash:create  # splash native (#020617)
```

## 4. Build APK & penamaan artefak

```bash
tool/build_apk.sh
# → dist/maritim-node-v1.0.0-build1-release.apk (+ .sha256)
```

Versi tunggal bersumber dari `version:` di `pubspec.yaml` (`1.0.0+1` →
versionName `1.0.0` / versionCode `1`). Setiap rilis: naikkan versi, catat di
[CHANGELOG.md](CHANGELOG.md) — nama artefak otomatis mengikuti sehingga jejak
perubahan mudah dilacak. CI (`.github/workflows/ci-cd.yml`, branch `mobile`)
menjalankan analyze + test lalu mengunggah artefak APK bernama versi.

> APK ditandatangani debug key (cukup untuk pengujian internal). Untuk
> distribusi luas, ganti `signingConfig` di `android/app/build.gradle.kts`
> dengan keystore rilis.

## 5. Menghubungkan APK ke backend — dua opsi

Alamat server **dikonfigurasi runtime** di layar Setup (tersimpan di
preferensi), jadi **satu APK yang sama** berlaku untuk kedua skenario tanpa
build ulang.

### Opsi A — Backend & frontend Docker di laptop (jaringan lokal)

1. Pastikan HP dan laptop berada di **Wi-Fi yang sama**.
2. Di laptop: `cd simulation/backend && docker compose up -d`
   (port `8080` sudah dipublikasikan ke `0.0.0.0`).
3. Cari IP LAN laptop: `ip addr` → mis. `192.168.1.10`; pastikan firewall
   mengizinkan: `sudo firewall-cmd --add-port=8080/tcp` (Fedora).
4. Di aplikasi → Setup → Alamat backend: **`http://192.168.1.10:8080`** →
   *Tes koneksi & muat sesi* → pilih sesi → Hubungkan.
   (Lalu lintas HTTP polos diizinkan lewat `usesCleartextTraffic` — khusus
   skenario lab ini.)
5. Buat/monitor sesi dari dashboard: `http://192.168.1.10:3000`.

### Opsi B — Backend & frontend di VPS (produksi)

1. Ikuti tutorial lengkap: **`simulation/docs/tutorial_deploy_vps_id.md`**
   (provisioning → Docker → Nginx → TLS → domain).
2. Di aplikasi → Setup → Alamat backend: **`https://api.domain-anda.id`** —
   aplikasi otomatis memakai **WSS** untuk kanal WebSocket ketika skema HTTPS.
3. Tidak ada konfigurasi lain: sesi & skema tetap didorong server.

## 6. Keputusan teknis

- **Riverpod 2.6.1** (bukan 3.x): API `AsyncNotifier` yang stabil dan teruji;
  seluruh graf DI di `di/providers.dart` sehingga mudah di-override saat test.
- **Navigator biasa** (bukan `go_router`): hanya 3 layar linear tanpa deep
  link — sesuai kriteria skill `flutter-setup-declarative-routing` yang
  menyarankan go_router hanya bila butuh URL/deep-link.
- **Skema fallback** `lat/lng/berat_kg` ketika server belum kirim skema —
  meniru payload cadangan `nodesim`, node tetap bisa beroperasi.
- **Logo vektor CustomPaint** (`ui/core/app_logo.dart`) dengan geometri
  identik `frontend/src/app/icon.svg` — tajam di semua ukuran; PNG hanya
  untuk aset native (launcher/splash) via `tool/generate_branding.py`.
- **`test_live/` terpisah dari `test/`** — unit test selalu hijau di CI tanpa
  Docker; uji live dijalankan manual sebelum rilis.
