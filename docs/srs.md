# 📄 Software Requirements Specification (SRS) - Mobile Node Service
## Maritime LoRa Mesh Network Simulator

---

## 1. System Architecture & Tech Stack

Aplikasi seluler ini beroperasi sebagai terminal *Edge / Node* yang menyimulasikan cara kerja perangkat keras radio LoRa secara murni di lapisan *Data Link* (MAC Layer). Arsitektur difokuskan pada manajemen antrean (*Queue Management*) yang persisten agar simulasi pengiriman paket tidak terganggu oleh hilangnya konektivitas jaringan sementara.

* **Core Framework:** Flutter (Dart 3+).
* **State Management & DI:** Riverpod (`flutter_riverpod`). Dipilih karena aman terhadap kompilasi (*compile-safe*) untuk memisahkan logika antarmuka UI dengan *background thread* yang memonitor siklus WebSocket.
* **WebSocket Client:** `web_socket_channel` (mendukung WSS murni dengan interval ping/pong bawaan untuk mencegah *drop* koneksi yang tidak disengaja).
* **Local Persistence:** SQLite (`sqflite`). Berfungsi sebagai memori *Store-and-Forward* yang tahan banting ketika aplikasi di-*minimize* atau koneksi terputus.
* **Binary Serialization:** `msgpack_dart` (konversi *Map/JSON* ke *byte array* biner untuk mereplikasi kompresi fisik LoRa).
* **Location Tracking:** `geolocator` (mendapatkan lat/lng secara berkala).

---

## 2. Persistence Layer Specs (SQLite Schema)

Tabel ini bertindak sebagai tulang punggung antrean transmisi (*Store-and-Forward*).

**Tabel: `transmit_queue`**
```sql
CREATE TABLE transmit_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    packet_id TEXT UNIQUE NOT NULL, -- UUID v4 dari paket
    origin_node_id TEXT NOT NULL,   -- Kritis: Pencatat identitas pembuat asli paket (untuk backend logs)
    target_parent TEXT NOT NULL,    -- Contoh: "KPL-002"
    payload_b64 TEXT NOT NULL,      -- Base64 dari MessagePack byte array
    status TEXT NOT NULL,           -- Enum/String: 'PENDING', 'ACKED', 'FAILED'
    retry_count INTEGER DEFAULT 0,  -- Maksimal 3
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

```

---

## 3. Domain Rules & Core Algorithms

### A. Strict LoRa Payload Constraint (MessagePack Validation)

Setiap input pengguna dari form dinamis **wajib** dikompresi menjadi biner terlebih dahulu sebelum diukur ukurannya. Jika ukuran biner melampaui batas *Spreading Factor* (SF) yang disetel oleh Backend, sistem menolak eksekusi `sqflite` dan transmisi.

```dart
// Contoh Implementasi Validator Strict Binary dengan penanganan error
import 'package:msgpack_dart/msgpack_dart.dart';
import 'dart:convert';

/// Mengonversi Map JSON ke Base64 MessagePack dan memvalidasi ukuran bit fisik
String encodeAndValidate(Map<String, dynamic> rawData, int maxBytesAllowed) {
  try {
    // 1. Serialize ke MessagePack byte array
    final List<int> binaryPayload = serialize(rawData);
    
    // 2. Evaluasi ukuran byte array murni BUKAN string JSON!
    if (binaryPayload.length > maxBytesAllowed) {
      // Edge case: Payload terlalu besar, batalkan transmisi
      throw FormatException('Payload melampaui limit fisik LoRa: ${binaryPayload.length} / $maxBytesAllowed bytes');
    }
    
    // 3. Encode ke Base64 agar dapat melintasi WebSocket (JSON Envelope)
    return base64Encode(binaryPayload);
  } catch (e) {
    // Tangkap error serialisasi (misal tipe data tidak didukung msgpack)
    rethrow;
  }
}

```

### B. Auto-Retry & ACK Machine (*Data Link Simulation*)

Mekanisme ini meniru transmisi radio fisik yang menunggu kepastian balasan (*Acknowledgement*):

1. **Transmit:** Saat menembakkan `node:transmit`, simpan paket ke `transmit_queue` (Status: `PENDING`), lalu picu `Timer` asinkron. Waktu *timeout* ($T$) ditentukan oleh *Spreading Factor* aktif.
2. **ACK Received:** Jika *listener* WebSocket menerima `node:ack` dengan `packet_id` yang cocok sebelum *timeout*: batalkan `Timer` menggunakan `timer.cancel()`, dan perbarui baris di `sqflite` menjadi `ACKED`.
3. **Timeout Triggered:** Jika *Timer* habis:
* Lakukan operasi baca `retry_count` di `sqflite`.
* Jika `< 3`: Increment nilai, *update* `sqflite`, dan tembakkan ulang *payload* via WebSocket.
* Jika `>= 3`: Set status menjadi `FAILED (Link Lost)`.



### C. Mesh Relay Logic (Hopping Relay)

Jika *node* menerima *event* `mesh:receive_rf` (yang berarti *Backend* menugaskan aplikasi ini sebagai jembatan/perantara rute *mesh*):

1. Cegat ID Paket tersebut, lalu langsung tembakkan balasan `node:ack` ke pengirim asli (agar *Timer* pengirim berhenti).
2. Tulis *payload* tersebut ke dalam tabel `transmit_queue` lokal. **Penting:** Pastikan kolom `origin_node_id` diisi dengan identitas pengirim awal, bukan identitas *node relay* ini.
3. Reposisi *forwarding*: Transmisikan paket tersebut ke `target_parent` milik *node* ini saat ini, lengkap dengan penanganan *Timer* ACK baru.

---

## 4. WebSocket Event Registry (Mobile <-> Backend)

Semua event dikemas dalam *envelope* JSON standar. Autentikasi dan *Session ID* divalidasi pada saat inisiasi koneksi.

### A. Outbound (Dari Klien ke Go Backend)

* **`node:ping`**: Menyiarkan koordinat GPS asli (dari `geolocator`) atau simulasi setiap 3 detik.
* *Payload*: `{ "event": "node:ping", "node_id": "KPL-001", "lat": -7.1, "lng": 106.4 }`


* **`node:transmit`**: Menyiarkan paket baru atau paket hasil *relay*.
* *Payload*: `{ "event": "node:transmit", "origin_node_id": "KPL-001", "target": "KPL-002", "packet_id": "uuid...", "payload_b64": "...", "hop_count": 1 }`


* **`node:ack`**: Mengonfirmasi bahwa paket *relay* berhasil dicatat ke dalam `sqflite`.
* *Payload*: `{ "event": "node:ack", "packet_id": "uuid...", "status": "received" }`



### B. Inbound (Dari Go Backend ke Klien)

* **`mesh:routing_update`**:
* *Payload*: `{ "event": "mesh:routing_update", "parent_target": "KPL-002" }`
* *Action*: Memperbarui *State* Riverpod untuk menunjuk *Next Hop*. Jika `null`, matikan tombol "Kirim" (*Status: Isolated*).
* *Catatan interkoneksi (rute bisa gugur kapan saja):* `parent_target` dapat berubah menjadi `null` **bukan hanya** karena kapal bergerak keluar jangkauan, tetapi juga bila **Virtual Edge (Syahbandar) yang menjadi gateway satu-satunya dihapus** dari master data lewat `DELETE /api/v1/edges/{edge_code}` (lihat backend SRS §3.4). Karena itu aplikasi WAJIB memperlakukan rute sebagai *ephemeral* — selalu bereaksi terhadap tiap `mesh:routing_update` (termasuk transisi mendadak `routed → isolated`), tidak pernah meng-*cache* parent sebagai permanen. Paket yang sedang di antrean *Store-and-Forward* tetap ditahan hingga rute pulih.


* **`mesh:receive_rf`**:
* *Payload*: Identik dengan transmisi `node:transmit`, tetapi ditujukan agar *node* ini menjadi *relay*.


* **`env:sync_params`**:
* *Payload*: `{ "event": "env:sync_params", "sf": 10, "max_payload_bytes": 51, "schema": [...] }`
* *Action*: Merender ulang form masukan dinamis di UI dan mengubah batas validasi variabel *byte array*.



---

## 5. Anti-Slop Strict Implementation Standards

1. **Riverpod Provider Scoping:** Logika inisiasi *Timer* dan penanganan `sqflite` TIDAK BOLEH berada di dalam kode presentasi (seperti di dalam metode `onPressed` milik *Button Widget*). Wajib disuntikkan (*injected*) menggunakan `AsyncNotifierProvider` untuk mencegah *Memory Leak* jika pengguna berpindah halaman saat antrean sedang berjalan.
2. **Database Transaction Safety:** Karena operasi baca/tulis `sqflite` dalam skenario *Retry* terjadi secara asinkron dengan frekuensi tinggi, pastikan pengubahan `retry_count` dan `status` dibungkus menggunakan blok `db.transaction()` agar data tidak bertabrakan saat paket ganda memicu *timeout* bersamaan (mencegah *race conditions*).
3. **Graceful Auto-Reconnect:** Wajib menangani kode *Error* WSS dari kelas `web_socket_channel` menggunakan *Exponential Backoff Reconnection* (misal jeda 1s, 2s, 4s, 8s). Ketika status *Offline*, jangan hilangkan data UI; biarkan paket bertumpuk dengan aman di status `PENDING` pada `sqflite`.