# 📄 Software Requirements Specification (SRS) - Backend Service
## Maritime LoRa Mesh Network Simulator

---

## 1. System Architecture & Tech Stack

Backend Simulator dirancang sebagai mesin orkestrasi *real-time* yang sangat konkuren. Sistem ini bertugas menghitung spasial (*geospatial computing*), memfasilitasi komunikasi WebSocket *Full-Duplex*, dan menyimulasikan probabilitas jaringan fisik (LoRa MAC/Data Link Layer) tanpa menggunakan mesin fisika (*physics engine*).

* **Core Language:** Go (Golang) 1.22+
* **Web Framework / Router:** Fiber v3 atau standar `net/http` dengan arsitektur *clean architecture*.
* **WebSocket Engine:** Gorilla WebSocket (dioptimalkan dengan *Worker Pools* & *Channel* untuk menangani *broadcast storm*).
* **Relational Database:** PostgreSQL 16+ (menyimpan konfigurasi, skema dinamis, dan log hasil).
* **In-Memory & Spatial Database:** Redis 7.x (Pub/Sub, deduplikasi paket, dan komputasi `GEOADD`/`GEORADIUS`).
* **Binary Packer (Payload):** MessagePack (diimplementasikan di *client* dan diurai di *Edge/Backend*).

---

## 2. Persistence Layer Specs

### A. PostgreSQL Schema (Konfigurasi & Log Historis)

##### 1. `users` (Dashboard Syahbandar Admin)
* `id` (BIGINT UNSIGNED, PRIMARY KEY, AUTO_INCREMENT)
* `name` (VARCHAR(150), NOT NULL)
* `email` (VARCHAR(150), UNIQUE, NOT NULL)
* `password_hash` (VARCHAR(255), NOT NULL)
* `created_at`, `updated_at` (TIMESTAMP)

##### 2. `virtual_edges` (Titik Akhir Syahbandar)
* `id` (BIGINT UNSIGNED, PRIMARY KEY, AUTO_INCREMENT)
* `edge_code` (VARCHAR(50), UNIQUE, NOT NULL)
* `name` (VARCHAR(150), NOT NULL)
* `latitude` (DECIMAL(11, 8), NOT NULL)
* `longitude` (DECIMAL(11, 8), NOT NULL)
* `created_at`, `updated_at` (TIMESTAMP)

##### 3. `simulation_sessions` (Sesi Simulasi Aktif)
* `id` (UUID, PRIMARY KEY)
* `session_name` (VARCHAR(150), NOT NULL)
* `spreading_factor` (INT, DEFAULT 7) - *Mempengaruhi batas byte dan timeout ACK.*
* `tx_power_dbm` (INT, DEFAULT 20) - *Mempengaruhi jangkauan maksimal radio.*
* `weather_severity` (DECIMAL(3,2), DEFAULT 1.00) - *Multiplier untuk probabilitas packet loss.*
* `is_active` (BOOLEAN, DEFAULT TRUE)
* `created_at`, `ended_at` (TIMESTAMP)

##### 4. `dynamic_schemas` (Aturan Struktur Payload)
* `id` (BIGINT UNSIGNED, PRIMARY KEY, AUTO_INCREMENT)
* `session_id` (UUID, FOREIGN KEY -> `simulation_sessions.id`, CASCADE)
* `schema_definition` (JSONB) - *Menyimpan array field: name, tipe data, dan ukuran byte.*
* `created_at` (TIMESTAMP)

##### 5. `telemetry_logs` (Penyimpanan Paket Sukses ke Syahbandar)
* `id` (BIGINT UNSIGNED, PRIMARY KEY, AUTO_INCREMENT)
* `session_id` (UUID, FOREIGN KEY -> `simulation_sessions.id`, CASCADE)
* `origin_node_id` (VARCHAR(100), NOT NULL)
* `edge_id` (BIGINT UNSIGNED, FOREIGN KEY -> `virtual_edges.id`, SET NULL)
* `edge_code_snapshot` (VARCHAR(50), NULLABLE) — **ditambahkan migrasi `0002_edge_lifecycle.sql`.**
  Kode Edge dibekukan (*snapshot*) tepat saat paket mendarat, independen dari
  siklus hidup `edge_id`. Alasan: `edge_id` ber-`ON DELETE SET NULL` — tanpa
  kolom ini, menghapus sebuah Edge akan membuat *seluruh* riwayat
  pengirimannya kehilangan nama ("Edge" tampil `—` di dasbor) meski paket
  itu benar-benar pernah sampai. Kolom ini membuat nama Edge **permanen**
  pada baris telemetrinya; status aktif/tidaknya Edge tersebut *hari ini*
  dihitung terpisah saat `SELECT` (lihat `edge_active` di §3.4).
* `hop_count` (INT, NOT NULL) - *Berapa kali paket melompat sebelum sampai.*
* `routing_path` (VARCHAR(255)) - *Contoh: "NodeC->NodeA->NodeB->Edge1".*
* `decoded_payload` (JSONB) - *Data biner yang sudah diurai (*unpack*) berdasarkan `dynamic_schemas`.*
* `arrived_at` (TIMESTAMP, INDEXED)

### B. Redis Schema (In-Memory State & Spatial Routing)

##### 1. Geospatial Tracking (Per-Sesi)
* **Key:** `sim:{session_id}:geo` (Redis Type: ZSET / Geospatial)
* **Member:** `node_id` (String)
* **Score:** (Longitude, Latitude) diupdate secara konstan oleh WebSocket.

##### 2. Paket Deduplication Cache
* **Key:** `sim:{session_id}:dedup:{packet_id}` (Redis Type: String)
* **Value:** `1`
* **TTL:** 30 Detik. (Digunakan agar jika Node A menerima paket hasil *retry* dari Node C padahal paket pertama sudah diteruskan, Node A akan mengabaikannya).

##### 3. Routing Table Cache
* **Key:** `sim:{session_id}:routes` (Redis Type: Hash)
* **Field:** `node_id` | **Value:** `parent_node_id`
* *Dihitung ulang setiap 5 detik oleh MeshTopologyService dan di-broadcast ke klien.*

---

## 3. REST API Endpoint Registry (Dashboard to Go)

Semua *endpoint* mengembalikan struktur JSON standar:
`{ "status": "success" | "error", "message": "string", "data": object | array | null }`

### 1. Inisialisasi Sesi Simulasi
* **Endpoint:** `POST /api/v1/simulations`
* **Payload (`application/json`):**
    ```json
    {
      "session_name": "Simulasi Badai Selatan",
      "spreading_factor": 7,
      "tx_power_dbm": 20,
      "weather_severity": 1.0
    }
    ```
* **Response (201 Created):** Mengembalikan `session_id` (UUID) yang akan dipakai oleh aplikasi *mobile* Flutter untuk *login* ke WebSocket.

### 2. Konfigurasi Skema Dinamis
* **Endpoint:** `POST /api/v1/simulations/{session_id}/schema`
* **Payload (`application/json`):**
    ```json
    {
      "fields": [
        {"name": "lat", "type": "float32"},
        {"name": "lng", "type": "float32"},
        {"name": "berat_kg", "type": "uint16"},
        {"name": "jenis_ikan", "type": "string_10"}
      ]
    }
    ```
* **Response (200 OK):** Skema disimpan di PostgreSQL dan secara otomatis di- *push* ke seluruh *node* yang aktif di sesi tersebut via WebSocket.

### 3. Injeksi Cuaca (Artificial Packet Loss Trigger)
* **Endpoint:** `PUT /api/v1/simulations/{session_id}/weather`
* **Payload (`application/json`):**
    ```json
    {
      "weather_severity": 1.8 
    }
    ```
* **Response (200 OK):** *Backend* segera memperbarui pengali probabilitas, membuat transmisi radio berikutnya lebih rentan terhadap kegagalan.

### 4. Master Data Virtual Edge (Syahbandar)
*Endpoint* pengelolaan gateway Syahbandar (`virtual_edges`) — titik akhir yang wajib dijangkau paket agar sebuah rute dianggap `routed`.

* **`POST /api/v1/edges`** — Registrasi Edge baru.
    ```json
    { "edge_code": "EDGE-PRATU-01", "name": "Syahbandar Pelabuhan Ratu", "latitude": -6.9875, "longitude": 106.5504 }
    ```
    Validasi: `edge_code` wajib (≤50 char, unik → 400 bila duplikat), `name` wajib (≤150 char), `latitude` -90..90, `longitude` -180..180. **Response 201 Created**.
* **`GET /api/v1/edges`** — Daftar seluruh Edge (diurut `id`). **Response 200 OK**.
* **`PUT /api/v1/edges/{edge_code}`** — Ubah `name`/`latitude`/`longitude` Edge yang sudah ada.
    ```json
    { "name": "Syahbandar Pelabuhan Ratu (Direlokasi)", "latitude": -6.90, "longitude": 106.60 }
    ```
    * `edge_code` **tidak bisa diubah** — ia adalah kunci *routing* yang dipakai
      kapal lain sebagai `target_parent`; membiarkannya tetap berarti rute
      aktif yang sudah memakainya sama sekali tidak terganggu oleh
      penyuntingan (berbeda dari `DELETE`, yang memutus rute).
    * Validasi identik `POST` (minus `edge_code`). **Response 200 OK** berisi
      baris Edge terbaru; **404 Not Found** bila kode tidak ada.
* **`DELETE /api/v1/edges/{edge_code}`** — Hapus satu Edge berdasarkan kode.
    * **Response 200 OK:** `data: { "edge_code": "..." }`.
    * **Response 404 Not Found:** bila kode tidak ada.
    * **Efek data:** FK `telemetry_logs.edge_id` memakai `ON DELETE SET NULL`, tapi
      `edge_code_snapshot` (§2A.5) **tetap mempertahankan namanya** — dasbor
      menampilkan kode itu dengan badge **merah** ("edge sudah tidak aktif")
      alih-alih `—`, badge **hijau** untuk paket yang diantar Edge yang masih
      terdaftar (`edge_active`, dihitung saat `GET .../telemetry` — *endpoint*
      ini di luar cakupan SRS, lihat tabel deviasi di README §"Endpoint
      tambahan"). Dampak operasional:
      *node* yang menjadikannya satu-satunya gateway dalam jangkauan akan
      `isolated` pada tick topologi berikutnya (≤5 s) sampai ada Edge lain
      yang terjangkau.

### 5. Hapus Riwayat Sesi Simulasi

* **`DELETE /api/v1/simulations/{session_id}`** — Menghapus permanen baris
  sesi beserta seluruh `dynamic_schemas` dan `telemetry_logs` terkait
  (*cascade* lewat FK `ON DELETE CASCADE`, §2A — satu-satunya titik
  penghapusan, tidak ada langkah manual terpisah per tabel).
    * **Response 200 OK:** `data: { "session_id": "..." }`.
    * **Response 409 Conflict:** bila sesi **masih aktif**
      (`"simulation session is still active; stop it before deleting its history"`)
      — hentikan lebih dulu via `POST .../stop`. Aturan ini mencegah riwayat
      sebuah simulasi yang sedang berjalan lenyap begitu saja dari bawah
      klien yang masih tersambung.
    * **Response 404 Not Found:** bila `session_id` tidak ada (termasuk saat
      dipanggil dua kali berturut-turut).

---

## 4. WebSocket Event Registry (Mobile Node <-> Go Backend)

Komunikasi menggunakan koneksi `ws://` atau `wss://`. Semua pesan dikemas dalam *envelope* JSON untuk *routing command*, namun data spesifik (*payload*) menggunakan array biner (Base64 di JSON atau Binary Frame murni).

### A. Client-to-Server Events (Dikirim dari Flutter)

#### 1. Registrasi & Posisi (Ping)
Dikirim setiap 3 detik untuk memperbarui lokasi geospasial *node* di Redis.
* **Event:** `node:ping`
* **Payload:**
    ```json
    {
      "event": "node:ping",
      "node_id": "KPL-001",
      "session_id": "uuid-v4",
      "lat": -7.1853,
      "lng": 106.4521
    }
    ```

#### 2. Transmisi Telemetri (*Store and Forward*)
* **Event:** `node:transmit`
* **Keterangan:** Jika ukuran `binary_payload_b64` melampaui batas *Spreading Factor* yang disepakati, *backend* Go akan langsung mem- *drop* pesan tanpa ampun.
* **Payload:**
    ```json
    {
      "event": "node:transmit",
      "packet_id": "pkt-9912A",
      "origin_node": "KPL-001",
      "target_parent": "KPL-002",
      "hop_count": 1,
      "routing_path": ["KPL-001"],
      "binary_payload_b64": "v2sH..." 
    }
    ```

#### 3. Acknowledgement (ACK)
Dikirim saat *node* menerima paket dari tetangganya.
* **Event:** `node:ack`
* **Payload:**
    ```json
    {
      "event": "node:ack",
      "packet_id": "pkt-9912A",
      "receiver_node": "KPL-002",
      "status": "received"
    }
    ```

### B. Server-to-Client Events (Diterima oleh Flutter)

#### 1. Distribusi Routing Table
Dikirim otomatis oleh `MeshTopologyService` ketika ada perubahan topologi (misal kapal bergerak menjauh).
* **Event:** `mesh:routing_update`
* **Payload:**
    ```json
    {
      "event": "mesh:routing_update",
      "parent_target": "KPL-002", 
      "distance_to_parent_km": 1.2
    }
    ```
    *Catatan: Setiap node hanya menerima informasi siapa target selanjutnya (parent), bukan seluruh peta jaringan, mereplikasi memori ESP32 yang terbatas.*

#### 2. Simulasi RF Receive (RX Window)
Go *backend* meneruskan paket ke *node* yang dituju (sebagai simulasi rambatan udara).
* **Event:** `mesh:receive_rf`
* **Payload:** Identik dengan format `node:transmit`, diterima oleh target. Target akan menyimpan, memvalidasi paket, lalu membalas dengan `node:ack`.

#### 3. Pembaruan Parameter Lingkungan
Membatalkan batasan *hardware* di sisi *client*.
* **Event:** `env:sync_params`
* **Payload:** `{ "sf": 10, "max_payload_bytes": 51 }`

---

## 5. Domain Rules & Core Algorithms

### A. LoRa Payload Constraint Table (Berdasarkan Spreading Factor)
Setiap paket yang masuk dievaluasi oleh *middleware* `SimulationEngineService`.
* **SF 7 - 8:** Ukuran `binary_payload` <= 242 bytes.
* **SF 9:** Ukuran `binary_payload` <= 115 bytes.
* **SF 10 - 12:** Ukuran `binary_payload` <= 51 bytes.
* *Jika melanggar aturan: Paket dihentikan (Drop). Node pengirim tidak akan menerima ACK dan akan terkena Timeout.*

### B. Environmental Anomaly Algorithm (Artificial Packet Loss)
Setiap `node:transmit` yang melewati *backend* Go akan dikenakan algoritma dadu probabilistik (*pseudo-random*).
1.  **Hitung Jarak Udara ($D$):** Menggunakan formula Haversine antara koordinat Pengirim dan Target (diambil dari Redis Geo).
2.  **Base Packet Loss ($P_b$):**
    * $D \le 2\text{ km} \rightarrow P_b = 5\%$
    * $2\text{ km} < D \le 5\text{ km} \rightarrow P_b = 20\%$
    * $D > 5\text{ km} \rightarrow P_b = 60\%$
    * $D > \text{Max Range (berdasarkan TX Power)} \rightarrow P_b = 100\%$ (Out of Range)
3.  **Final Packet Loss ($P_f$):** $P_f = P_b \times \text{Weather Severity (1.0 hingga 2.0)}$
4.  **Eksekusi:** Go menghasilkan angka acak 1-100. Jika angka acak $\le P_f$, paket di-*drop* (tidak diteruskan ke Target). Sinyal ACK dari target juga terkena probabilitas yang sama saat kembali.

### C. Mesh Routing Algorithm
`MeshTopologyService` berjalan dalam sebuah Goroutine setiap 5 detik:
1.  Menarik semua titik dari `geo:nodes`.
2.  Menentukan *Node* yang berada dalam jangkauan langsung (*Line of Sight*) dari *Virtual Edge*. Ini disebut **Hop 0**.
3.  Menghitung jarak terdekat *Node* sisa ke *Node Hop 0*. Mereka menjadi **Hop 1**, dan seterusnya.
4.  Jika suatu *Node* melebihi batas **Hop 5** atau tidak memiliki rute, statusnya diset menjadi *Isolated* (Pencarian rute putus).

---

## 6. Anti-Slop Strict Implementation Standards
1.  **Goroutine Leaks Prevention:** Semua koneksi WebSocket wajib di- *defer* `conn.Close()` dan fungsi pembacaannya (`ReadPump`) wajib memonitor siklus `context.Done()` dari manajemen sesi global.
2.  **Mutex Lock / Redis Transaction:** Pengubahan `hop_count` dan *packet deduplication* wajib bersifat *thread-safe* menggunakan Redis `SETNX` (Set if Not Exists) untuk mencegah *Race Condition* di antara Goroutine yang melayani koneksi WebSocket yang berbeda.
3.  **No String Manipulation for Bytes:** Data *binary payload* tidak boleh di- *cast* menjadi String standar untuk kalkulasi batas byte. Gunakan `len(byte_slice)` dari hasil *decode* Base64 murni.