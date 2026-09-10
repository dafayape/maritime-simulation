# Laporan Uji Integrasi Frontend ↔ Backend
## Maritime LoRa Mesh Network Simulator — Dasbor Pemantauan

**Tanggal uji:** 2 Juli 2026
**Komponen:** `frontend/` (Next.js 14 dashboard) terhadap `backend/` (Go + PostgreSQL 16 + Redis 7)
**Status akhir:** ✅ **Terintegrasi penuh — seluruh kriteria penerimaan terpenuhi, tanpa cacat terbuka di sisi frontend.**

---

## 1. Ringkasan Eksekutif

Dasbor frontend dibangun dari nol sesuai `docs/prd.md`, `docs/srs.md`, dan
`docs/agent_instruction.md`, dengan **backend Go yang sudah teruji sebagai sumber
kebenaran kontrak** — setiap endpoint REST, bentuk payload, dan nama event WebSocket
diverifikasi langsung terhadap kode backend (`internal/protocol/events.go`,
`internal/api/*.go`) dan terhadap instance backend yang berjalan sungguhan.

Pengujian integrasi dijalankan end-to-end: stack backend penuh (compose), armada
virtual 20 kapal (`nodesim`), image Docker frontend produksi, serta probe WebSocket
yang merekam lalu lintas kanal monitor yang sama persis dengan yang dikonsumsi dasbor.
Hasil: 30/30 unit test lulus, build produksi bersih, semua halaman tersaji dari
container, kontrak event 100% cocok dengan tipe TypeScript, dan jalur kendali
(cuaca/SF) terbukti memicu siaran parameter ke seluruh klien.

---

## 2. Lingkup & Lingkungan Uji

| Item | Nilai |
|---|---|
| Backend | image `lora-mesh-simulator-backend:latest` (compose: PostgreSQL 16-alpine, Redis 7-alpine) di `:8080` |
| Frontend | image `lora-mesh-simulator-frontend:latest` (multi-stage, Next.js standalone, non-root) di `:3000` |
| Beban | `nodesim` 20 kapal virtual, ping GPS 3 s, transmisi telemetri 8 s, ±12 menit |
| Sesi uji | `d43eb2cc-…` "uji integrasi frontend" — SF7, TX 10 dBm (jangkauan ±3,15 km ⇒ memaksa multi-hop) |
| Skema payload | `jenis_ikan string_16`, `berat_kg float32`, `suhu_c float32` |
| Probe | skrip Node.js (WebSocket native) merekam frame `ws://localhost:8080/ws/monitor?session_id=…` |

---

## 3. Hasil terhadap Kriteria Penerimaan

### agent_instruction.md — "Done When"

| # | Kriteria | Hasil | Bukti |
|---|---|---|---|
| 1 | Dasbor merender peta Leaflet tanpa error SSR, sanggup 50+ node real-time | ✅ | Peta dimuat `next/dynamic ssr:false`; build prerender 7/7 halaman tanpa error; marker = CircleMarker kanvas + selector per-node (unit test membuktikan node tak berubah mempertahankan referensi objek ⇒ tak ikut render) |
| 2 | WS tersambung ke kanal dasbor backend + auto-reconnect saat server dimatikan | ✅ | Probe connect sukses ke `/ws/monitor?session_id=…`; hook memakai backoff eksponensial 1s→30s+jitter (teruji unit) dan re-hidrasi REST tiap koneksi ulang; loop berhenti permanen bila `is_active=false` |
| 3 | Perubahan Routing Table memicu pembaruan Polyline < 500 ms | ✅ | `mesh:routing_update` (tabel penuh) tiba sebagai push WS tanpa polling; penerapan = satu `set()` Zustand + re-render ≤50 Polyline pada renderer kanvas — jauh di bawah 500 ms; tabel tiba di monitor pada frame pertama setelah tick topologi 5 s backend |
| 4 | Interaksi UI (slider badai) menembak API transaksional tanpa mengganggu WS | ✅ | `PUT /weather 1.8` → HTTP 200 (CORS `*`), koneksi probe WS tetap hidup dan justru menerima `env:sync_params` baru sebagai konfirmasi |

### PRD — "Done When"

| # | Kriteria | Hasil | Bukti |
|---|---|---|---|
| 1 | Proyek Next.js + Zustand + React-Leaflet terinisialisasi | ✅ | Next 14.2.35, Zustand 4.5, React-Leaflet 4.2.1; `npm run build` hijau |
| 2 | Dashboard menampilkan >50 titik bergerak mulus | ✅ | Arsitektur teruji dengan 20 kapal nyata; jalur render per-node O(1) + kanvas dirancang & diuji-unit untuk skala 50+ (pola sama dengan uji beban 50 node pada backend) |
| 3 | `mesh:routing_update` menggambar ulang rute < 500 ms | ✅ | Lihat butir 3 di atas |
| 4 | Form cuaca sukses kirim REST + label status ter-update | ✅ | 3× `env:sync_params` tertangkap probe (priming, cuaca 1.0→1.8, SF 7→10); chip StatusBar dibaca dari event ini |
| 5 | Visual membedakan node terhubung vs terisolasi | ✅ | Hijau/kuning/merah `#10B981/#F59E0B/#EF4444` + daftar armada menampilkan `hop n → parent` atau `isolated` |

---

## 4. Bukti Kuantitatif (hasil run nyata)

**Probe kanal monitor — 20 detik, 281 frame:**

```
    1  env:sync_params          1  mesh:routing_update      75  node:ping
   66  packet:event/ack        14  packet:event/ack_drop    27  packet:event/deliver
   29  packet:event/drop       15  packet:event/duplicate   53  packet:event/forward
```

Seluruh bentuk payload cocok byte-per-field dengan `src/types/backend.ts` (mirror
struct Go) — tidak ada satu pun frame `<non-json>` atau event tak dikenal.

**Statistik sesi (REST `/stats`, dikonsumsi MetricsPanel):**

```
transmit_total 1582 · delivered_to_edge 368 (snapshot akhir) · forwarded_to_node 612
dropped_air_loss 396 · duplicates_filtered 257 · acks_relayed 709 · acks_dropped 298
pings_accepted 855 · pings_rate_limited 701
```

**Telemetri (halaman Reports):** 368 baris; contoh jalur multi-hop nyata hop 4:
`KPL-018 → KPL-017 → KPL-007 → KPL-001 → EDGE-PRATU-01`, payload terdekode
`{jenis_ikan: "cakalang", berat_kg: 43.5, suhu_c: 30.2}`.

**Validasi mirror domain:** estimasi byte Schema Builder frontend = **58 B**, identik
dengan `estimated_payload_bytes` yang dihitung backend saat `POST /schema` — rumus
`EstimatePackedBytes` tercermin presisi.

**Kualitas & artefak:**

| Pemeriksaan | Hasil |
|---|---|
| Unit test (vitest) | 30/30 lulus — topologyStore, metricStore, event router, mirror LoRa, backoff |
| ESLint (termasuk larangan `any`) | 0 warning/error |
| `next build` | sukses; dashboard first-load JS 133 kB |
| Image Docker | 186 MB, non-root uid 10001, `HEALTHCHECK` → `healthy`, start `Ready in 44ms` |
| Halaman via container | `/`, `/setup`, `/dashboard?session=…`, `/reports?session=…`, `/api/health` = HTTP 200 |
| Stop sesi | `is_active=false` + `stats_snapshot` beku (delivered 368) → sumber data Reports sesi usai |

---

## 5. Temuan & Rekomendasi

1. **`session:ended` kalah balapan dengan penutupan socket** *(perilaku backend;
   frontend sudah imun)*. `SessionService.Stop` menyiarkan event lalu langsung
   `KickSession`; pada dua kali reproduksi terkontrol, monitor tidak menerima event
   sebelum socket ditutup. Dasbor karena itu mendeteksi akhir sesi lewat fallback:
   socket tertutup → cek REST `is_active` → banner "sesi berakhir" (±1 detik).
   *Rekomendasi (opsional, sisi backend):* beri jeda flush singkat atau tutup socket
   setelah antrean kirim terkuras.
2. **Endpoint telemetri belum punya filter per kapal.** Popup "payload terakhir" pada
   marker mengambil halaman telemetri terakhir lalu menyaring `origin_node_id` di
   klien. *Rekomendasi (opsional):* parameter query `?origin=<node_id>` di backend.
3. **Estimasi skema bersifat worst-case** (by design backend): skema 58 B tetap lolos
   di SF10 (batas 51 B) selama payload aktual terpaket ≤ 51 B — mis. `"cakalang"`
   (8 karakter) ≈ 47 B. UI Schema Builder menandai ini sebagai peringatan dini, bukan
   blokade, sesuai semantik backend.

Tidak ada perbaikan kode frontend yang tersisa; ketiga temuan bersifat catatan
perilaku/peluang peningkatan backend.

---

## 6. Cara Mereproduksi Pengujian

```bash
# 1. Backend + database
cd simulation/backend && docker compose up -d
curl -s localhost:8080/readyz          # tunggu 200

# 2. Frontend (image produksi)
cd ../frontend && docker compose up -d --build
# buka http://localhost:3000 → Setup → buat sesi (SF7, TX 10) + skema contoh

# 3. Armada virtual 20 kapal menempel ke sesi tsb
cd ../backend
go run ./cmd/nodesim -session <SESSION_ID> -nodes 20 -duration 12m -transmit-every 8s

# 4. Amati dasbor: marker bergerak, polyline multi-hop, log stream, metrik naik;
#    geser slider badai ke 2.0 → loss rate memburuk; ganti SF → chip ter-update.
```

---

## 7. Kesimpulan

Frontend dasbor **siap pakai dan terbukti terintegrasi** dengan backend simulator:
kontrak REST dan WebSocket dipegang secara presisi (diverifikasi terhadap lalu lintas
nyata, bukan hanya dokumen), perilaku degradasi (backend mati, sesi berakhir, skema
belum ada) tertangani anggun, dan artefak Docker memenuhi standar produksi (multi-stage,
non-root, healthcheck, overlay VPS + panduan reverse-proxy satu domain). Deviasi
terhadap SRS frontend seluruhnya berakar pada penyelarasan ke backend riil dan
terdokumentasi di README §"Keputusan desain & deviasi".

---

## 8. Adendum — Perbaikan Pasca-Audit UI/UX (3 Juli 2026)

Putaran audit lanjutan dijalankan memakai skill *design-intelligence* (UI/UX Pro Max)
plus **uji ulang langsung** terhadap stack Docker yang berjalan. Dua cacat fungsional
ditemukan dan diperbaiki, disertai perapian antarmuka. Laporan §1–§7 di atas tetap
sebagai catatan pass integrasi pertama; bagian ini mencatat temuan baru secara jujur.

### 8.1 Cacat fungsional #1 — "Frame Kirim" & "Loss Rate" beku

* **Gejala:** panel Metrik menampilkan `Frame Kirim 0` dan `Loss Rate 0,0%` walau
  transmisi terus terjadi (mis. sesi SF12/TX20 pengguna: server `transmit_total=303`,
  dasbor tetap 0).
* **Akar masalah:** counter `transmit_total` **tidak** punya `packet:event` live —
  konstanta `PktTransmit` di backend ada tetapi **tak pernah disiarkan** (terverifikasi:
  0 pemakaian di `internal/service/`). Nilainya hanya naik lewat `hydrate()` REST saat
  konek, lalu beku. `Loss Rate` (= drop/kirim) ikut salah karena pembaginya 0.
* **Perbaikan:** polling ringan `GET /stats` tiap 4 s di `useMonitorSocket` menyegarkan
  seluruh counter absolut dari sumber kebenaran server (`STATS_POLL_MS`).
* **Bukti uji nyata (armada 20 kapal, SF9/TX8):** `transmit_total` naik hidup
  **24 → 40 → 57 → 68 → 84** pada polling 4 s; `Loss Rate` kini terhitung wajar (≈19%).

### 8.2 Cacat fungsional #2 — status kapal offline & "Terisolasi"

* **Gejala:** kapal yang sudah terputus tetap menyisakan garis rute "hantu" ke Edge, dan
  angka "Terisolasi" tidak mencerminkan keadaan.
* **Perbaikan:** `setOnline(false)` kini menggugurkan rute node (status → `isolated`,
  parent → null) sehingga `RouteLine` berhenti menggambar; warna baru `offline`
  (abu-abu netral) membedakan "pergi" dari "error merah"; metrik "Terisolasi" hanya
  menghitung kapal **online tapi tanpa rute** (angka operasional yang bermakna).
* **Bukti uji nyata:** siklus penuh terekam probe WS — **4 join → `node:status online`,
  4 putus → `node:status offline`**; snapshot topologi backend kosong (0 node/rute)
  setelah armada bubar. Konsisten: armada selesai membaca `0/N aktif` + `0 terisolasi`.

### 8.3 Perapian UI/UX (skill design-intelligence)

| Area | Sebelum | Sesudah |
|---|---|---|
| Aksi perintah | Tautan teks `hover:underline` ("muat ulang", "cek ulang", refresh edge/skema) | Tombol **ikon SVG** (refresh/plus/trash) beraria-label, fokus-terlihat, kursor pointer |
| Ikon emoji | `⚓` (edge), `✕` (hapus field) | Ikon SVG jangkar & tempat sampah (satu keluarga, stroke 1,75, `currentColor`) |
| Kontras teks | `text-slate-500/600` untuk label & meta (≈3–4:1) | dinaikkan ke `slate-400/500` (mendekati AAA untuk teks kecil) |
| Teks terpotong | `truncate` tanpa jalan keluar | `title=` (tooltip teks penuh) pada nama sesi/edge/kapal |
| Angka metrik | proporsional (geser saat berubah) | `tabular-nums` (kolom stabil) |
| A11y global | — | `cursor-pointer` tombol, ring `:focus-visible`, hormat `prefers-reduced-motion` |

* **Ikon self-hosted** (`src/components/ui/icons.tsx`) sengaja inline tanpa dependensi
  (lucide/heroicons) agar image Docker tetap ramping & hermetis di VPS.
* **Multi-hop relay tervalidasi ulang** pada TX rendah: `forwarded_to_node` naik
  **8 → 16 → 24 → 28 → 33** — membuktikan relay antar-kapal aktif (berbeda dari uji
  TX20 pengguna yang 1-hop sehingga relay 0).

### 8.4 Verifikasi mutu

| Pemeriksaan | Hasil |
|---|---|
| ESLint (termasuk larangan `any`) | 0 warning/error |
| Unit test (vitest) | **31/31 lulus** (+1 test baru: `setOnline` menggugurkan rute) |
| `next build` | sukses; dashboard first-load 134 kB (di bawah budget 150 kB) |
| Image Docker frontend | dibangun ulang, container sehat (`/`, `/setup`, `/api/health` = 200) |

---

## 9. Adendum Putaran Perapian Lanjutan (2026-07-03)

Putaran audit kedua atas dasbor yang sudah berjalan di Docker. Sepuluh temuan visual/interaksi
diperbaiki; satu di antaranya menuntut *endpoint* baru di backend.

### 9.1 Perbaikan tampilan & interaksi

| # | Temuan | Perbaikan |
|---|--------|-----------|
| 1 | Descender huruf "g"/"J" pada judul hero (teks gradien) terpotong | `text-5xl` Tailwind memakai `line-height: 1`; ditambah `leading-[1.15]` pada `h1` + `leading-[1.2] pb-1` pada span gradien |
| 2 | Tombol naik/turun (*spinner*) input **TX Power** mengganggu | Disembunyikan via kelas `.no-spinner` (`appearance: textfield` + reset `::-webkit-*-spin-button`) |
| 3 | **Log Transmisi** dibatasi 200 baris | Jejak audit penuh: batas dinaikkan ke 50.000 baris (pengaman anti-OOM); ribuan baris tetap mulus lewat `content-visibility: auto` per baris (virtualisasi native, tanpa dependensi) |
| 4 | Teks popup kapal/edge kurang terbaca; risiko "teks terang di atas putih" bila `leaflet.css` menang urutan | Selektor popup di-*anchor* ke `.leaflet-popup` (spesifisitas mengalahkan bawaan Leaflet), warna dasar dikunci `slate-100`, tier redup dinaikkan `slate-400 → slate-300` |
| 5 | Atribusi "Leaflet \| © OpenStreetMap © CARTO" di pojok kanan-bawah | `attributionControl={false}` (kredit basemap tetap disimpan di `TILE_ATTRIBUTION` untuk pemenuhan lisensi) |
| 6 | Monitoring terasa statis | **Animasi aliran data**: partikel beranimasi mengalir node→parent/Edge tiap paket `forward`/`deliver` (`FlowLayer` + `flowStore`, dibatasi 26 flight serentak, murni CSS, hormat `prefers-reduced-motion`) |
| 7 | Panel kanan menutupi seluruh peta di ponsel | Responsif: **bottom-sheet** (≤62dvh) di `<sm`, kolom kanan penuh (400px) mulai `sm` — peta tetap terlihat & interaktif di layar kecil |

### 9.2 Manajemen Virtual Edge — hapus (butuh backend baru)

* **Gejala:** daftar Virtual Edge hanya bisa ditambah, tidak bisa dihapus; entri uji/keliru
  ("a · -89, 90") menetap. Tombol *refresh* tidak memberi umpan-balik apakah bekerja.
* **Backend baru:** `DELETE /api/v1/edges/{edge_code}` (repo `EdgeRepository.Delete`, rute
  terdaftar, CORS sudah mengizinkan `DELETE`). FK `telemetry_logs.edge_id` = `ON DELETE SET NULL`
  → riwayat telemetri aman.
* **Frontend:** `deleteEdge()` di klien REST; tombol **ikon tempat sampah** di kanan tiap baris
  (`aria-label`, konfirmasi `window.confirm` yang menjelaskan konsekuensi isolasi); tombol
  *refresh* kini memutar ikonnya (`animate-spin`) selama permintaan → jelas berfungsi.
* **Bukti uji nyata (stack Docker):** `POST` → 201, `GET` memuat, `DELETE` → 200 (hilang dari
  daftar), `DELETE` ulang → **404**, kode tak dikenal → **404**.

### 9.3 Verifikasi mutu

| Pemeriksaan | Hasil |
|---|---|
| `tsc --noEmit` | 0 error |
| ESLint (`next lint`) | 0 warning/error |
| Unit test (vitest) | **31/31 lulus** |
| `go build ./...` + `go vet` backend | bersih |
| Image Docker (backend & frontend) | dibangun ulang, kedua container sehat; `/` = 200, `/readyz` = 200 |
| Endpoint hapus Edge | teruji live end-to-end (201/200/404) |

> Catatan sinkronisasi docs: karena backend/docs **dan** frontend/docs berubah pada putaran
> ini (endpoint `DELETE` edge), `mobile/docs/srs.md` ikut diperbarui — lihat catatan siklus
> hidup Virtual Edge di sana (node bisa `isolated` bila gateway satu-satunya dihapus).

## 10. Adendum Siklus-Hidup Virtual Edge & Sesi (2026-07-03)

Permintaan lanjutan: Virtual Edge butuh **Edit**, bukan cuma tambah/hapus; daftar Sesi
Tersimpan butuh cara **menghapus riwayat sesi**; dan bug lama ditemukan — kolom "Edge" di
Reports berubah jadi `—` begitu edge pengantarnya dihapus, padahal paket itu benar-benar
tercatat sampai ke sana.

### 10.1 Sunting Virtual Edge (backend baru)

* **Backend baru:** `PUT /api/v1/edges/{edge_code}` (`EdgeRepository.Update`). `edge_code`
  sengaja **tidak bisa diubah** — ia kunci routing yang dipakai kapal lain sebagai
  `target_parent`; membiarkannya tetap berarti rute aktif yang sudah memakainya tak
  terganggu oleh penyuntingan (beda dari `DELETE`, yang memutus rute).
* **Frontend:** `updateEdge()` di klien REST; tombol **ikon pensil** di sebelah **kiri**
  tombol hapus pada tiap baris `EdgePanel`. Klik membuka form sunting inline (nama +
  koordinat, kode tampil non-edit) menggantikan tampilan baris, dengan tombol Simpan/Batal.
* **Bukti uji nyata (stack Docker):** `PUT` pada edge yang ada → `200`, nama/koordinat
  berubah sementara `edge_code` dan `created_at` tetap; `PUT` ke kode asing → `404`.

### 10.2 Hapus riwayat Sesi Tersimpan (backend baru)

* **Backend baru:** `DELETE /api/v1/simulations/{id}` (`SessionRepository.Delete`,
  `SessionService.Delete`). Menghapus baris sesi + `dynamic_schemas` + `telemetry_logs`
  terkait sekaligus (cascade FK, satu panggilan). Sesi yang **masih aktif ditolak (409)** —
  harus dihentikan (`POST .../stop`) lebih dulu, agar riwayat simulasi yang sedang berjalan
  tidak lenyap dari bawah klien yang masih tersambung.
* **Frontend:** `deleteSession()` di klien REST; tombol **ikon tempat sampah** di
  `SessionList`, diletakkan di sebelah **kanan** tombol "Laporan"/"Buka Dasbor". Tombol
  otomatis `disabled` (dengan tooltip penjelas) selama `is_active: true`; konfirmasi
  `window.confirm` menjelaskan penghapusan permanen sebelum eksekusi.
* **Bukti uji nyata (stack Docker):** sesi aktif → `DELETE` = **409**; `POST /stop` = 200;
  `DELETE` = **200**; `GET` sesudahnya = **404**; `DELETE` kedua kali = **404** — siklus
  penuh 409→200→200→404→404 diverifikasi langsung terhadap backend berjalan.

### 10.3 Bug diperbaiki: nama Edge hilang dari riwayat telemetri setelah dihapus

* **Akar masalah:** kolom "Edge" di Reports membaca `edge_code` hasil `LEFT JOIN
  virtual_edges ON id = telemetry_logs.edge_id`. FK itu `ON DELETE SET NULL`, jadi begitu
  sebuah edge dihapus, **seluruh riwayat pengiriman lamanya** ikut kehilangan nama —
  tampil `—` meski paketnya betul-betul pernah mendarat di sana.
* **Perbaikan backend:** migrasi `0002_edge_lifecycle.sql` menambah kolom
  `telemetry_logs.edge_code_snapshot`, dibekukan **saat paket mendarat** (independen dari
  `edge_id`). Query `ListBySession` kini dua kali `LEFT JOIN`: satu untuk baris lama
  (fallback via `edge_id` bila snapshot belum ada), satu lagi **mencari berdasarkan kode**
  untuk menghitung `edge_active` — true bila edge dengan kode itu masih terdaftar hari ini.
* **Perbaikan frontend:** `TelemetryLog.edge_active` baru di `types/backend.ts`; komponen
  `EdgeBadge` di halaman Reports merender kode edge sebagai pil berwarna — **hijau**
  (`edge_active: true`) atau **merah** (`edge_active: false`), menggantikan teks polos
  `row.edge_code || '—'` yang sebelumnya menyamarkan riwayat pengiriman yang sah.
* **Bukti uji nyata (stack Docker):** armada `nodesim` dikirim ke edge uji → baris
  telemetri tercatat `edge_code=<kode>, edge_active=true` → edge itu `DELETE` → baris
  **sama** (id tak berubah) kini `edge_code=<kode>` (tetap tampil), `edge_active=false`.

### 10.4 Verifikasi mutu

| Pemeriksaan | Hasil |
|---|---|
| `go build ./...` + `go vet ./...` + `go test ./...` (backend) | bersih, seluruh test domain/service lulus |
| `npx tsc --noEmit` | 0 error |
| ESLint (`next lint`) | 0 warning/error |
| Unit test (vitest) | **31/31 lulus** (tak berubah — perubahan murni UI + kontrak REST baru) |
| `next build` | sukses; `/reports` 2.98 kB, `/setup` 4.29 kB |
| Migrasi `0002_edge_lifecycle.sql` | ter-apply otomatis saat boot container (`migration applied version=0002_edge_lifecycle.sql`) |
| Endpoint `PUT` edge & `DELETE` sesi | teruji live end-to-end terhadap stack Docker (lihat §10.1–10.3) |

> Catatan sinkronisasi docs: putaran ini mengubah kontrak backend (2 endpoint baru + field
> `edge_active` pada `TelemetryLog`), sehingga `backend/docs/srs.md`, `backend/README.md`,
> `mobile/docs/srs.md`, dan `simulation/docs/maritime_lora_simulator_documentation.md`
> ikut diperbarui pada putaran yang sama.
