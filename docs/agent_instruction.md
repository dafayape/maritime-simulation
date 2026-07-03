# Agent Development Instruction - Mobile Node Service (Flutter)
## Maritime LoRa Mesh Network Simulator

---

## 1. Goal
Membangun aplikasi **Flutter (Dart 3+)** yang bertindak sebagai terminal *Edge / Node* dalam simulasi jaringan LoRa maritim. Aplikasi ini berfungsi sebagai perangkat radio *half-duplex* yang mengubah input dinamis menjadi biner murni, memelihara antrean *Store-and-Forward* menggunakan pangkalan data lokal, dan mengelola pengiriman paket (*routing relay*) secara asinkron melalui WebSocket ke *Backend* Go.

---

## 2. Requirements & Architecture

### A. Presentation & UI Layer
* **Framework & State:** Gunakan Flutter dengan **Riverpod** (`flutter_riverpod`) untuk injeksi dependensi dan reaktivitas *state* yang bersih (*clean architecture*).
* **Dynamic Form & HUD:** * Susun antarmuka formulir secara dinamis berdasarkan skema JSON yang diterima dari event `env:sync_params`.
  * Tampilkan *Heads-Up Display* (HUD) yang memuat status WebSocket, nama *Next Hop Target* (didapat dari `mesh:routing_update`), dan *Byte Counter* *real-time*.
  * Tombol "Kirim" harus dinonaktifkan secara reaktif jika batas *Spreading Factor* (SF) terlampaui.

### B. Domain & Application Layer (Core Logic)
* **Strict Binary Packing (`msgpack_dart`):** Saat pengguna menekan kirim, data wajib diserialisasi ke format *MessagePack*. Hitung batas ukuran murni dari `length` *byte array* biner tersebut. Jika valid, *encode* ke Base64 sebelum dibungkus ke dalam JSON payload WebSocket.
* **ACK & Auto-Retry Machine:**
  * Saat `node:transmit` dikirim, simpan data ke tabel SQLite (`sqflite`) dengan status `PENDING` dan inisialisasi `Timer` asinkron.
  * Jika `node:ack` diterima, perbarui status menjadi `ACKED` dan batalkan `Timer`.
  * Jika `Timer` habis (*timeout*), baca `retry_count` dari SQLite. Jika $< 3$, inkremen nilai dan kirim ulang. Jika $\ge 3$, set status menjadi `FAILED`.
* **Mesh Hopping Relay:** Tangani event `mesh:receive_rf` dengan membalas `node:ack` ke pengirim asli, menyimpan *payload* ke SQLite, dan mengeksekusi *forwarding* ke *parent* node ini.

### C. Data Access & Transport Layer
* **Local Queue (`sqflite`):** Buat skema tabel `transmit_queue` (dengan kolom `packet_id`, `target_parent`, `payload_b64`, `status`, `retry_count`). Pastikan operasi tulis/baca dibungkus dengan `db.transaction()` untuk menangani *race condition* saat banyak paket diproses serentak.
* **WebSocket Client:** Gunakan `web_socket_channel` untuk komunikasi *Full-Duplex*. Wajib menerapkan *Exponential Backoff* untuk percobaan koneksi ulang (*auto-reconnect*) jika server terputus.
* **Location Service:** Integrasikan `geolocator` untuk menyiarkan *event* `node:ping` (lat/lng) setiap 3 detik.

---

## 3. Constraints & Coding Standards
* **Binary-First Validation:** Dilarang keras memvalidasi ukuran LoRa menggunakan fungsi panjang *String JSON* seperti `jsonEncode().length`. Gunakan murni ukuran *byte array*.
* **Riverpod Async Isolation:** Logika *Timer* ACK dan akses SQLite wajib dipisahkan ke dalam *Service/Repository* dan dikelola oleh `AsyncNotifierProvider`. Jangan mencemari *Widget tree* (seperti `onPressed`) dengan logika asinkron yang bisa menyebabkan *memory leak* jika UI di-*unmount*.
* **Error Handling:** Bungkus semua operasi jaringan dan pangkalan data dengan blok *try-catch* yang solid. Jangan biarkan aplikasi mengalami *crash* saat putus koneksi; biarkan data mengantre aman di SQLite.

---

## 4. Done When (Acceptance Criteria)
* [ ] Aplikasi sukses merender form dinamis dan memblokir pengiriman jika kompresi *MessagePack* melampaui batas SF *Backend*.
* [ ] Posisi GPS berhasil disiarkan setiap 3 detik via WebSocket tanpa mengganggu FPS UI (60fps).
* [ ] Sistem *Store, Forward, & Retry* tervalidasi: Aplikasi mendeteksi *timeout* paket yang dibuang *Backend*, menyimpannya di SQLite, dan mengirim ulang maksimal 3 kali.
* [ ] Aplikasi mampu merespons `mesh:receive_rf` dan meneruskan paket tersebut (*relay*) secara otomatis di latar belakang.

---

## 5. Strict Anti-Slop Enforcement
* **Production-Ready Code:** Hasilkan kode Flutter yang modular, bersih, dan fungsional penuh. Dilarang keras meninggalkan komentar *placeholder* seperti `// TODO: implement retry timer` atau `// logic here`. 
* **Scope Discipline:** Fokus murni pada direktori repositori seluler (Flutter). Jangan mengubah arsitektur Go atau Next.js.