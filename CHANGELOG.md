# Changelog — Maritim Node (Mobile)

Semua perubahan penting aplikasi ini dicatat di berkas ini.
Format mengikuti [Keep a Changelog](https://keepachangelog.com/id-ID/1.1.0/)
dan penomoran versi mengikuti [Semantic Versioning](https://semver.org/lang/id/).
Versi aplikasi = `version:` pada `pubspec.yaml` (mis. `1.0.0+1` →
versionName `1.0.0`, versionCode `1`); nama artefak build mengikutinya:
`maritim-node-v1.0.0-build1-release.apk`.

## [1.0.0+1] - 2026-07-03

### Added
- Terminal node kapal (simulasi radio LoRa half-duplex) di atas WebSocket
  `/ws/nodes` backend Go — kontrak event mengikuti
  `backend/internal/protocol/events.go`.
- Binary packing **MessagePack** dengan validasi Binary-First: ukuran diuji
  dari panjang byte array murni terhadap `max_payload_bytes` (limit SF), bukan
  panjang string JSON; `float32` di-encode 4 byte via `msgpack_dart.Float`.
- Antrean **Store-and-Forward** persisten SQLite (`transmit_queue`, sesuai SRS
  §2 + kolom `hop_count`/`routing_path` yang diwajibkan kontrak transmisi
  backend), semua mutasi dibungkus `db.transaction()` anti-race.
- Mesin **ACK & Auto-Retry**: timeout `ack_timeout_ms` dari server, kirim
  ulang dengan `packet_id` sama maksimal `max_retries` (3), lalu status
  `FAILED (Link Lost)`.
- **Mesh relay**: `mesh:receive_rf` dibalas `node:ack` seketika, dedup via
  keunikan `packet_id`, `origin_node` pembuat asli dipertahankan, frame
  diteruskan ke parent dengan `hop_count+1` dan jalur diperpanjang.
- Antrean ditahan saat **terisolasi** dan dipompa ulang otomatis (dengan
  retarget parent baru) saat `mesh:routing_update` memulihkan rute.
- **GPS ping 3 detik** (`node:ping`): GPS asli via `geolocator` atau koordinat
  manual + drift random-walk ala harness `nodesim`.
- Auto-reconnect **exponential backoff** (1s→30s) + fallback verifikasi REST
  `is_active` ketika event `session:ended` kalah balapan dengan penutupan
  socket.
- UI: splash animasi + Setup (URL server, sesi aktif, Node ID, mode GPS) +
  HUD (status kanal, next-hop, SF/limit, penghitung Data Link) + form dinamis
  `schema:sync` dengan byte counter real-time + tab Antrean + tab Log.
- Branding identik frontend: ikon launcher & splash dihasilkan dari geometri
  `frontend/src/app/icon.svg` (`tool/generate_branding.py`), tema gelap
  slate/sky/emerald.
- Pengujian: 40 unit/widget test + 3 skenario integrasi live terhadap backend
  Docker (`test_live/`), `flutter analyze` bersih.

[1.0.0+1]: https://github.com/dafayape/maritime-simulation/tree/mobile
