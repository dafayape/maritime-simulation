# Agent Development Instruction - Backend Service
## Maritime LoRa Mesh Network Simulator

---

## 1. Goal
Membangun backend *real-time* berbasis **Go (Golang) 1.22+** untuk menyimulasikan topologi jaringan LoRa *Mesh* maritim. Sistem ini memadukan REST API untuk konfigurasi, arsitektur WebSocket berkinerja tinggi untuk emulasi gelombang radio, kalkulasi *Geospatial* menggunakan **Redis**, serta penyimpanan state dan log menggunakan **PostgreSQL 16+**.

---

## 2. Requirements

### A. API / Presentation Layer
* **REST API**: Buat *endpoint* untuk inisialisasi simulasi (`/api/v1/simulations`), manajemen skema *payload* dinamis (`/schema`), dan pemicu anomali cuaca buatan (`/weather`).
* **WebSocket Gateway**: Implementasikan *Gorilla WebSocket* dengan pola *Worker Pool* dan *Channel* untuk menangani *event* dua arah: `node:ping`, `node:transmit`, `node:ack`, `mesh:routing_update`, dan `env:sync_params`.

### B. Application & Domain Layer
* **`MeshTopologyService`**: Logika kalkulasi jarak geospasial setiap 5 detik. Gunakan *Directed Acyclic Graph* (DAG) untuk menentukan rute terpendek (*hop*) ke *Virtual Edge*, lalu *broadcast* tabel rute (`mesh:routing_update`) ke klien terkait.
* **`SimulationEngineService`**: Mengatur lalu lintas data *node*. Lakukan validasi ukuran *byte* berdasarkan batas Spreading Factor (SF). Fasilitasi mekanisme *Store and Forward*, serta *Data Link layer* (*ACK* dan *Auto-Retry*).
* **`EnvironmentalAnomalyService`**: Terapkan logika probabilistik untuk membuang paket (*artificial packet loss*) secara acak berdasarkan kalkulasi *Haversine distance* (pengirim-penerima) yang dikalikan dengan faktor keparahan badai (*weather severity*).
* **`DynamicSchemaService`**: Injeksi konfigurasi batas batas *byte* (SF) dan struktur JSON *payload* ke memori *client* secara dinamis.

### C. Data Access & Persistence Layer
* **Redis (In-Memory & Routing)**: 
    * Gunakan tipe data *Geospatial* (`GEOADD`, `GEORADIUS`) untuk menyimpan koordinat *node* secara absolut.
    * Gunakan perintah `SETNX` (dengan TTL 30 detik) pada ID Paket untuk memastikan deduplikasi paket yang dikirim ulang (*retry mechanism*).
* **PostgreSQL (State & Logging)**: Tulis migrasi skema untuk tabel `users`, `virtual_edges`, `simulation_sessions`, `dynamic_schemas`, dan `telemetry_logs`. Simpan rekam jejak paket yang sukses mencapai target akhir (*Edge*).

### D. Infrastructure & Cross-Cutting Layer
* **Graceful Shutdown & Context**: Terapkan `context.Context` pada seluruh eksekusi Goroutine. Pastikan koneksi DB, Redis, dan WebSocket ditutup secara anggun (*graceful*) saat server dimatikan.
* **Concurrency Safety**: Gunakan `sync.Mutex` atau *Redis Distributed Lock* ketika memperbarui jumlah *hop* atau status *node* untuk mencegah *race condition* akibat serbuan paket bersamaan (*broadcast storm*).

---

## 3. Constraints
* **No Frontend Code**: Jangan menulis atau mengubah berkas *client-side* (Next.js/Flutter). Fokus murni pada arsitektur direktori Go (misal: `/cmd`, `/internal/api`, `/internal/service`, `/internal/repository`).
* **Binary-First Mindset**: Data `binary_payload_b64` dari *client* tidak boleh di-*cast* menjadi tipe `String` biasa untuk menghitung batas ukuran. *Decode* secara absolut ke `[]byte` dan gunakan `len()` untuk menguji batas SF LoRa.
* **Strict Anti-Slop Enforcement**: Dilarang keras menggunakan komentar *placeholder* seperti `// TODO: implement logic`, `// logic here`, atau `// ... rest of the code`. Tulis fungsi secara utuh, bersih, *production-ready*, dan aman.
* **No Execution Before Approval**: Jangan menulis kode fungsional `.go` apa pun sampai persetujuan arsitektur awal ini dikonfirmasi oleh pengguna (USER).

---

## 4. Done When (Acceptance Criteria)
* [ ] Migrasi skema PostgreSQL dan struktur *key-value* Redis terinisiasi sempurna tanpa *error*.
* [ ] Server WebSocket mampu menahan >50 koneksi *node* simultan tanpa mendeteksi *Goroutine leak* (dibuktikan via *pprof* atau *logging* internal).
* [ ] Algoritma `EnvironmentalAnomalyService` sukses men-*drop* pesan secara probabilistik ketika simulasi badai dipicu via REST API.
* [ ] Fitur ACK dan *Auto-Retry* terbukti bekerja; *backend* Go mampu menyaring duplikat paket dengan ID yang sama menggunakan Redis `SETNX`.
* [ ] *Payload* biner sukses diurai (*unpack*) berdasar skema dinamis di tahap akhir saat paket mencapai titik *Virtual Edge*.