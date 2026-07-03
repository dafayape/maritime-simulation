# Product Requirement Document (PRD) - Mobile Node Service
## Maritime LoRa Mesh Network Simulator

---

## 1. Overview & Problem Statement
Dalam implementasi fisik jaringan LoRa maritim, perangkat keras (seperti ESP32/SX1276) di atas kapal bertugas membaca sensor, memampatkan data, dan memancarkan gelombang radio. Pada arsitektur simulator *software-in-the-loop* ini, **Aplikasi Mobile (Flutter)** mengambil alih peran *microcontroller* tersebut. 

Aplikasi ini tidak menggunakan HTTP standar untuk berkomunikasi, melainkan bertindak layaknya perangkat keras radio yang menembakkan *byte array* ke udara. Aplikasi wajib menyimulasikan komunikasi tingkat rendah (*Data Link Layer*), merakit *payload* menjadi biner murni, memelihara antrean *Store-and-Forward* menggunakan pangkalan data lokal, mengelola *timeout* untuk sinyal *Acknowledgement* (ACK), dan mengeksekusi transmisi ulang (*Auto-Retry*) menuju peladen Go via WebSocket.

---

## 2. Goals & Non-Goals

### A. Goals
* **True Node Simulation**: Menyimulasikan aplikasi seluler sebagai satu *node* radio *half-duplex* yang terhubung ke jaringan *mesh*.
* **Dynamic Binary Packing**: Mengonversi masukan JSON dinamis ke format biner (*MessagePack*) sebelum dikirim untuk mereplikasi batasan ukuran data LoRa di dunia nyata.
* **Store, Forward & Retry Protocol**: Membangun sistem antrean persisten lokal menggunakan SQLite (`sqflite`) untuk menahan paket jika belum menerima `node:ack` atau terputus dari jaringan, serta melakukan pengiriman ulang (maksimal 3 kali).
* **Location Tracking**: Menyiarkan posisi GPS perangkat secara akurat menggunakan `geolocator` setiap 3 detik via WebSocket sebagai dasar kalkulasi proksimitas dan jarak *mesh* di *backend*.
* **Dynamic Schema Consumption**: Merender formulir input secara dinamis berdasarkan definisi skema dari peladen (Syahbandar).

### B. Non-Goals
* Tidak memancarkan gelombang radio RF (*Bluetooth/WiFi Direct/P2P*). Semua transmisi antarkapal disimulasikan melalui WebSocket TCP/IP ke *Virtual Ether* (Backend Go).
* Tidak menghitung jarak atau menentukan *routing table* secara mandiri. *Node* beroperasi layaknya perangkat "buta" yang patuh pada perintah `mesh:routing_update`.

---

## 3. Detailed Architectural Layers & Tech Stack Breakdown

Arsitektur aplikasi dipisahkan secara ketat untuk memisahkan logika antarmuka (UI) dengan *background service* yang menangani antrean paket *mesh*.

| Nama Package Flutter | Peran dalam Simulasi | Alasan Pemilihan & Kompromi (Trade-Offs) |
| :--- | :--- | :--- |
| **`web_socket_channel`** | Simulasi Gelombang Radio | Mempertahankan koneksi dua arah yang persisten. Memungkinkan *backend* memaksa masuk (*push*) *routing table* baru kapan saja tanpa *polling*. |
| **`msgpack_dart`** | Dynamic Binary Packer | Skema *payload* bersifat dinamis (diubah dari web). MessagePack mengubah JSON menjadi format biner yang sangat padat, mereplikasi batasan ukuran data (SF limit) LoRa. |
| **`geolocator`** | Penentu Posisi (Node Tracking) | Mengambil koordinat lintang dan bujur absolut untuk dikirim ke *backend* sebagai variabel utama kalkulasi proksimitas *mesh routing*. |
| **`sqflite`** | Store and Forward Queue | Esensi jaringan *mesh*. Jika aplikasi bertindak sebagai *relay* paket dari HP lain tapi gagal terkirim (simulasi badai/putus koneksi), data diamankan di SQLite lokal dan dikirim ulang (*retry*) nanti. |
| **`flutter_riverpod`** | State Management | Sangat aman (*compile-safe*) untuk memisahkan logika UI (form dinamis, kamera) dengan logika *background* asinkron yang menangani antrean pengiriman pesan. |

---

## 4. Core Features & Logic

### 4.1. Setup & Dashboard (Presentation Layer)
* **Setup Screen**: Antarmuka untuk memasukkan `Node ID` (misal: KPL-001) dan mengunci koneksi WebSocket ke *Virtual Edge*. Opsi *toggle* untuk menggunakan GPS Asli (`geolocator`) atau simulasi manual.
* **Main HUD (Heads-Up Display)**:
  * Indikator status koneksi WebSocket (Hijau/Merah).
  * Indikator *Next Hop Target* (Nama kapal *parent* yang didikte oleh *backend*).
  * *Byte Counter* *real-time* (misal: `45 / 51 bytes (SF10)`). Tombol "Kirim" akan terkunci otomatis jika ukuran biner melampaui batas yang diizinkan peladen.

### 4.2. ACK & Retry Machine (Application Layer)
* Saat `node:transmit` dikirim, aplikasi merekam entri di `sqflite` dengan status `PENDING` dan menjalankan `Timer` Dart asinkron.
* Waktu *timeout* ($T$) bersifat dinamis mengikuti nilai Spreading Factor (SF tinggi = $T$ lebih lama).
* Jika `node:ack` diterima untuk *packet ID* yang bersangkutan, ubah status SQLite menjadi `ACKED` dan batalkan *Timer*.
* Jika *Timer* habis, naikkan *counter retry* dan kirim ulang (maksimal 3x). Jika gagal, setel status ke `FAILED`.

### 4.3. Mesh Relay Logic (Hopping)
Jika *node* menerima *event* `mesh:receive_rf` (yang berarti aplikasi ini sedang dijadikan jembatan oleh kapal lain):
1. Segera tembakkan `node:ack` ke pengirim asli.
2. Masukkan *payload* biner asing tersebut ke dalam tabel antrean `sqflite` lokal.
3. Eksekusi transmisi ulang ke target `parent` milik *node* ini.

---

## 5. WebSocket Event Contracts (Client Perspective)

| Arah | Event Name | Payload Intent | Reaksi Klien (Flutter) |
| :--- | :--- | :--- | :--- |
| **OUT** | `node:ping` | `{ lat, lng, node_id }` | Dikirim setiap 3 detik menggunakan data `geolocator`. |
| **OUT** | `node:transmit` | `{ target, packet_id, payload_b64, hop_count }` | Mengirim data ke antrean lokal dan memicu *Timer* ACK. |
| **OUT** | `node:ack` | `{ packet_id, status: "received" }` | Dikirim seketika saat klien menerima paket lintas-udara (*relay*). |
| **IN** | `mesh:routing_update` | `{ parent_target }` | Memperbarui state `Riverpod`. Semua transmisi berikutnya diarahkan ke target ini. |
| **IN** | `mesh:receive_rf` | (Identik dengan `node:transmit`) | Menyimpan paket asing ke `sqflite`, membalas ACK, dan menaruh di antrean *forward*. |
| **IN** | `env:sync_params` | `{ sf, max_payload_bytes, schema }` | Memodifikasi limit UI *byte counter* dan menyusun ulang form input dinamis. |

---

## 6. Acceptance Criteria (Done When)
* [ ] Komponen form dinamis berhasil mencegah pengiriman data apabila kompresi biner dari `msgpack_dart` mengukur ukuran melebihi batas Spreading Factor saat ini.
* [ ] Posisi GPS berhasil disiarkan setiap 3 detik via WebSocket secara asinkron tanpa memblokir perenderan antarmuka Flutter.
* [ ] **Mekanisme *Store, Forward & Retry* terbukti beroperasi**: Saat koneksi memburuk atau *backend* membuang paket, aplikasi mendeteksi *timeout*, menyimpan state di `sqflite`, dan mengeksekusi transmisi ulang otomatis.
* [ ] Aplikasi secara sukses meneruskan paket (*relay*) ketika ditunjuk oleh peladen sebagai jembatan *hop* perantara.

---

## 7. Strict Implementation Standards
* **No String Manipulation for Bytes:** Dilarang keras menggunakan fungsi `jsonEncode().length` untuk menguji batas ukuran LoRa. Kompresi `msgpack_dart` harus dieksekusi terlebih dahulu, lalu dihitung panjang *byte array*-nya (`length`).
* **Safe State Injection:** Pemanfaatan `flutter_riverpod` harus dilakukan untuk mengisolasi logika *timer* ACK dan operasi `sqflite` dari *Widget tree*, guna mencegah *memory leak* dan kesalahan re-render.
* **Graceful Disconnect:** WebSocket harus merespons putusnya koneksi jaringan dengan logika *exponential backoff reconnect*. Selama koneksi terputus, data pengguna yang disubmit wajib diamankan di SQLite lokal.