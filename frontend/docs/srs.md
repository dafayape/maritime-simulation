# 📄 Software Requirements Specification (SRS) - Frontend Service
## Maritime LoRa Mesh Network Simulator

---

## 1. System Architecture & Tech Stack

Frontend Simulator adalah *Single Page Application* (SPA) berbasis dasbor pemantauan *real-time*. Sistem ini dioptimalkan untuk memproses ratusan pembaruan *state* per detik (koordinat dan rute) tanpa mengalami degradasi *frame rate* (FPS) pada antarmuka peta geospasial.

* **Core Framework:** Next.js 14+ (App Router) menggunakan React 18+.
* **State Management:** Zustand (dipilih karena arsitektur *flux-like* yang bebas *boilerplate* dan sangat optimal untuk *high-frequency renders* menggunakan *selectors*).
* **Map Engine:** React-Leaflet v4+ dengan integrasi *basemap* OpenStreetMap atau Mapbox. (Diharuskan menggunakan `next/dynamic` dengan `ssr: false` untuk menghindari *error* *Server-Side Rendering* terkait objek `window`).
* **Styling & UI:** TailwindCSS v3+ (pendekatan *utility-first* untuk meminimalisir ukuran *bundle* CSS).
* **WebSocket Client:** Native HTML5 `WebSocket` API (Wajib digunakan alih-alih Socket.io untuk memastikan kompatibilitas 100% dengan implementasi Gorilla WebSocket di sisi Go Backend).
* **HTTP Client:** Axios (untuk interaksi REST API transaksional).

---

## 2. Global State Stores (Zustand)

Manajemen *state* dipisahkan berdasarkan domain fungsionalitas agar pembaruan pada metrik simulasi tidak memicu *re-render* pada kanvas peta.

### A. `useTopologyStore`
Menyimpan posisi absolut kapal dan relasi *mesh routing* mereka.
* **State:** * `nodes`: *Record/Dictionary* berisikan objek `{ id, lat, lng, parentId, status, lastPayload }`. (Gunakan tipe data O(1) *lookup* seperti *Object/Map*, bukan *Array*, agar *update* cepat).
* **Actions:**
  * `updateNodePosition(id, lat, lng)`: Diperbarui setiap menerima kordinat baru.
  * `updateRoutingTable(routes)`: Menerima *array* rute dari *backend* dan memperbarui `parentId` pada masing-masing *node*.
  * `setNodeStatus(id, status)`: Status beruba `connected`, `retrying`, atau `isolated`.

### B. `useSimulationStore`
Menyimpan parameter sesi simulasi yang sedang berjalan (atau yang akan dibuat).
* **State:** * `sessionId`: UUID dari Go Backend.
  * `config`: `{ spreadingFactor: 7, txPower: 20, weatherSeverity: 1.0 }`.
* **Actions:**
  * `setSession(id, initialConfig)`
  * `updateWeather(severity)`

### C. `useMetricStore`
Agregasi data statistik jaringan di sisi peramban (*browser*).
* **State:**
  * `packetsSent`, `packetsDropped`, `packetsRetried`.
* **Actions:**
  * `incrementMetric(type, amount)`

---

## 3. Map Viewer & Domain Logic (React-Leaflet)

### A. Rendering Invariants (Aturan Visual)
1. **NodeMarker (Titik Kapal)**:
   * Merender komponen `<Marker>` untuk setiap *key* di dalam `TopologyStore.nodes`.
   * **Warna:**
     * Hijau (`#10B981`): Memiliki `parentId` yang valid (Terhubung ke *Mesh*).
     * Kuning (`#F59E0B`): Dalam status mencoba mengirim ulang (*Retrying* / menunggu ACK).
     * Merah (`#EF4444`): Tidak memiliki rute / kehilangan *parent* (*Isolated*).
   * **Interaksi:** *On-click* memunculkan `<Popup>` berisi `lastPayload` yang telah diurai (contoh: *Suhu: 28C, Jenis Ikan: Tuna*).

2. **MeshPolyline (Garis Rute)**:
   * Setiap kali `TopologyStore` mendeteksi relasi *Node* -> *Parent*, antarmuka menggambar `<Polyline>` dari kordinat pengirim ke kordinat penerima.
   * *Style:* Garis putus-putus (`dashArray`) berwarna biru muda dengan panah penunjuk arah (*arrowhead*) yang dinamis.

### B. Map Performance Rules
* Pembaruan komponen *marker* wajib dibungkus dengan `React.memo` agar titik kapal yang *diam* tidak ikut di-*render* ulang ketika titik kapal *lain* bergerak.

---

## 4. HTTP Client & API Integration (REST)

Konfigurasi Axios dasar menggunakan *Base URL* dari `.env.local` (`NEXT_PUBLIC_API_URL`).

### A. Memulai Sesi Simulasi Baru
* **Trigger:** Form "Create Session" di halaman `/setup`.
* **Action:** `POST /api/v1/simulations`
* **Payload:** `{ "session_name", "spreading_factor", "tx_power_dbm" }`
* **Response:** Menyimpan `session_id` ke dalam `SimulationStore` dan mengalihkan (*redirect*) pengguna ke `/dashboard`.

### B. Menginjeksi Skema Dinamis
* **Trigger:** Input form *drag-and-drop* pembangun JSON di `/setup`.
* **Action:** `POST /api/v1/simulations/{session_id}/schema`
* **Payload:** `{ "fields": [{ "name": "...", "type": "..." }] }`

### C. Injeksi Anomali Cuaca (Live Dashboard)
* **Trigger:** Slider *Weather Severity* (1.0 - 2.0) di komponen `ControlWidget`.
* **Action:** `PUT /api/v1/simulations/{session_id}/weather`
* **Payload:** `{ "weather_severity": 1.5 }`

---

## 5. WebSocket Event Consumption (Dashboard <-> Backend)

Karena ini adalah sisi Admin/Dasbor, `WebSocket` API digunakan untuk "mendengarkan" lalu lintas jaringan yang sedang disimulasikan secara *real-time*.

### A. Lifecycle Hook (`useAdminWebSocket`)
* Saat masuk ke `/dashboard`, buat instansiasi: `new WebSocket('ws://{BACKEND_HOST}/ws/admin/{session_id}')`.
* Wajib mengimplementasikan logika *Reconnection* otomatis dengan *exponential backoff* jika koneksi terputus dari *backend* Go.
* Wajib memanggil `ws.close()` pada siklus `useEffect` *cleanup* (saat komponen *unmount*).

### B. Inbound Events (Dari Go ke Frontend)
* **`mesh:routing_update`**: Menerima daftar topologi utuh atau parsial. 
  * *Handler:* Memetakan `parent_target` ke `TopologyStore`.
* **`node:telemetry_ping`** (Jika difasilitasi backend):
  * *Handler:* Mengubah parameter `lat` dan `lng` di dalam `TopologyStore.nodes` agar kapal bergerak di atas peta.
* **`sim:packet_dropped`** & **`sim:packet_retry`**:
  * *Handler:* Memicu penambahan nilai pada `MetricStore` (dikonsumsi oleh komponen *chart*/*counter*).

---

## 6. Layout & Views Configuration

1. **`/setup` (Konfigurasi Pra-Simulasi)**:
   * Menggunakan tata letak kartu yang berada di tengah (*centered card*).
   * Memiliki slider untuk Spreading Factor (7-12) dan input daya TX (dBm).
2. **`/dashboard` (Live Mesh Monitor)**:
   * **Kiri/Background**: Kanvas `MapViewer` *fullscreen* yang memenuhi 100% tinggi jendela (*Viewport Height*).
   * **Kanan (Floating Overlay)**: Panel tembus pandang bergaya *glassmorphism* (mendukung Tailwind `backdrop-blur`) berisi:
     * Metrik Total Paket, Kehilangan Paket (%), dan Jumlah Kapal Aktif.
     * Slider Cuaca (Badai) interaktif.
     * Tabel *log stream* kecil yang bergulir (*auto-scroll*) ketika data dari kapal sukses mendarat ke Syahbandar.

---

## 7. Anti-Slop Strict Implementation Standards
1. **Dynamic Import Strictness:** Modul Leaflet tidak boleh di-*import* langsung menggunakan standar ES6 `import { MapContainer }` pada komponen sisi-server. Wajib dibungkus dengan *dynamic import* Next.js agar tidak menyebabkan peramban (*browser*) *crash* saat perenderan HTML statis.
2. **No Faked Delays in UI:** Kapal di UI tidak boleh berpindah tempat menggunakan `setInterval` statis. Pergerakan murni dikendalikan sepenuhnya oleh masuknya data melalui jalur WebSocket.
3. **Type Safety (TypeScript):** Semua objek *payload* (Node, Route, Metrics) wajib diikat oleh *Interface* TypeScript yang merepresentasikan struktur data dari *Backend Go* secara presisi. Tidak ada toleransi untuk penggunaan tipe `any`.