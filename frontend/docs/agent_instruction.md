# Agent Development Instruction - Frontend Service
## Maritime LoRa Mesh Network Simulator

---

## 1. Goal
Membangun aplikasi *Single Page Application* (SPA) berbasis **Next.js 14+ (App Router)** dan **React 18+** sebagai dasbor pemantauan *real-time* untuk jaringan LoRa Mesh maritim. Frontend bertugas memvisualisasikan data geospasial ratusan *node* (kapal) di atas peta interaktif, menggambar rute topologi dinamis, dan menyajikan metrik simulasi menggunakan state management berkinerja tinggi.

---

## 2. Requirements

### A. UI & Components Layer
* **Framework:** Gunakan Next.js 14+ (App Router) dengan TailwindCSS v3+ untuk *styling* berbasis *utility*.
* **Map Engine:** Integrasikan **React-Leaflet**. Wajib menggunakan `next/dynamic` dengan opsi `ssr: false` untuk membungkus komponen peta guna menghindari *error* objek `window` di lingkungan *Server-Side Rendering*.
* **Visual Invariants:** * *Node Marker:* Hijau (Terhubung), Kuning (*Retrying*), Merah (*Isolated*).
    * *Routing Line:* *Polyline* putus-putus dengan indikator arah dari *Node* ke *Parent*.
    * Klik pada *marker* wajib memunculkan *popup* yang berisi metrik *payload* terakhir.
* **Dashboard Layout:** Sediakan peta *fullscreen* di latar belakang dan *floating panel* (dengan efek *glassmorphism* / `backdrop-blur`) di sisi layar untuk kontrol simulasi dan metrik log.

### B. State Management Layer (Zustand)
* **TopologyStore:** Simpan sekumpulan *node* aktif beserta kordinat dan statusnya. Gunakan struktur data *Object/Dictionary* (tipe O(1) *lookup*) alih-alih *Array* untuk pembaruan *state* yang sangat cepat. Wajib memanfaatkan pola *selector* pada komponen UI agar mencegah *re-render* massal saat hanya satu *node* yang bergerak.
* **SimulationStore & MetricStore:** Kelola parameter konfigurasi simulasi (SF, TX Power) dan agregasi statistik jaringan (Total Paket, Drop, Retry).

### C. Data Access & Transport Layer
* **REST API (Axios):** Buat *service client* bersih untuk *endpoint* konfigurasi, seperti memulai sesi (`POST /api/v1/simulations`) dan injeksi badai (`PUT /api/v1/simulations/{session_id}/weather`).
* **WebSocket Client:** Implementasikan native HTML5 `WebSocket` API di dalam sebuah *custom hook* (`useAdminWebSocket`).
    * *Event Handling:* Tangkap pesan untuk pembaruan topologi (`mesh:routing_table`), pergerakan (*ping*), dan statistik paket (*drop/retry*).
    * *Lifecycle:* Wajib menerapkan logika *auto-reconnect* (*exponential backoff*) dan *cleanup* (`ws.close()`) saat komponen di-*unmount* untuk mencegah *memory leak*.

---

## 3. Constraints & Coding Standards
* **Clean & Performant Code:** Tulis kode modular dengan abstraksi komponen yang jelas. Berikan *inline comments* untuk menjelaskan bagian logika kompleks (seperti kalkulasi geospasial atau siklus WebSocket).
* **No Faked Delays:** Pergerakan kapal atau garis di peta murni disetir oleh penerimaan data WebSocket (berbasis *event*). Dilarang keras memalsukan pergerakan menggunakan `setInterval` statis di antarmuka.
* **Strict Type Safety:** Gunakan **TypeScript**. Definisikan *Interface* secara eksplisit (seperti `Node`, `RoutingPath`, `SimulationConfig`) agar selaras persis dengan *struct* Go di sisi *backend*. Jangan gunakan tipe `any`.
* **Trade-offs Documentation:** Jika terdapat beberapa opsi dalam menangani render peta berfrekuensi tinggi, tulis kompromi (*trade-offs*) teknisnya dalam komentar kode (misalnya, *rendering Canvas vs SVG pada Leaflet*).

---

## 4. Done When (Acceptance Criteria)
* [ ] Halaman Dasbor merender peta Leaflet tanpa *error* SSR dan mampu menangani pembaruan rute 50+ *node* secara *real-time*.
* [ ] Koneksi WebSocket tersambung ke `ws://[backend_host]/ws/admin/...` dan otomatis mencoba menghubungkan ulang jika *server backend* dimatikan sementara.
* [ ] Perubahan *Routing Table* dari *backend* memicu pembaruan letak dan warna *Polyline* dalam waktu < 500ms.
* [ ] Interaksi UI (contoh: *slider* cuaca badai) sukses menembak API transaksional Axios tanpa mengganggu koneksi WebSocket.

---

## 5. Strict Anti-Slop Enforcement
* **No Placeholders:** Agen wajib memproduksi kode fungsional yang utuh, tangguh untuk produksi, dan bebas dari komentar pengabaian seperti `// TODO: logic here` atau `// ... rest of code`.
* **No Backend Modification:** Fokus murni pada direktori repositori `frontend`. Jangan memanipulasi kode Go.