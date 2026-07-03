import 'dart:convert';
import 'dart:typed_data';

import 'package:msgpack_dart/msgpack_dart.dart' as msgpack;

import '../../domain/models/schema_field.dart';

/// Hasil kompresi biner payload dinamis.
sealed class PackOutcome {
  const PackOutcome();
}

class PackSuccess extends PackOutcome {
  const PackSuccess(this.bytes);

  final Uint8List bytes;

  /// Ukuran fisik frame — SATU-SATUNYA angka yang sah untuk diuji terhadap
  /// batas Spreading Factor (Binary-First, SRS §3A).
  int get size => bytes.length;

  String get base64Payload => base64Encode(bytes);
}

class PackFailure extends PackOutcome {
  const PackFailure(this.message);

  final String message;
}

/// Codec MessagePack payload dinamis (SRS §3A — Strict LoRa Payload
/// Constraint). Nilai form dikonversi mengikuti tipe skema dari server:
///
///  - `float32` dibungkus [msgpack.Float] agar ter-encode 4 byte (0xca),
///    identik dengan perilaku firmware & harness `nodesim` backend;
///  - `float64` dibiarkan sebagai `double` (8 byte);
///  - integer memakai representasi terpadat MessagePack setelah divalidasi
///    terhadap rentang lebar registernya;
///  - `string_N` divalidasi terhadap kapasitas **byte UTF-8** (bukan jumlah
///    karakter) sebelum dikirim.
///
/// DILARANG mengukur ukuran lewat `jsonEncode(...).length` — pengukuran
/// selalu dari panjang byte array hasil serialisasi.
class PayloadCodec {
  const PayloadCodec();

  /// Serialisasi + validasi tipe. Tidak memeriksa batas SF — pemanggil
  /// membandingkan [PackSuccess.size] dengan `max_payload_bytes` miliknya
  /// (byte counter UI memakai fungsi yang sama agar angkanya identik).
  PackOutcome pack(Map<String, dynamic> values, List<SchemaField> schema) {
    try {
      final wire = <String, dynamic>{};
      if (schema.isEmpty) {
        wire.addAll(values);
      } else {
        for (final field in schema) {
          if (!values.containsKey(field.name)) continue;
          final converted = _convert(field, values[field.name]);
          if (converted is PackFailure) return converted;
          wire[field.name] = converted;
        }
      }
      return PackSuccess(msgpack.serialize(wire));
    } on Object catch (e) {
      return PackFailure('Serialisasi MessagePack gagal: $e');
    }
  }

  /// Dekode payload biner (dipakai untuk pratinjau isi frame relay).
  Map<String, dynamic>? unpack(String payloadB64) {
    try {
      final decoded = msgpack.deserialize(base64Decode(payloadB64));
      if (decoded is Map) {
        return decoded.map((k, v) => MapEntry(k.toString(), v));
      }
      return null;
    } on Object {
      return null;
    }
  }

  Object? _convert(SchemaField field, dynamic raw) {
    if (raw == null) return null;

    if (field.isBool) {
      if (raw is bool) return raw;
      return PackFailure('Field ${field.name} harus boolean.');
    }

    if (field.isFloat) {
      final v = raw is num ? raw.toDouble() : double.tryParse('$raw');
      if (v == null) return PackFailure('Field ${field.name} harus angka.');
      return field.isFloat32 ? msgpack.Float(v) : v;
    }

    if (field.isInteger) {
      final v = raw is int ? raw : int.tryParse('$raw');
      if (v == null) {
        return PackFailure('Field ${field.name} harus bilangan bulat.');
      }
      final range = field.integerRange;
      if (range != null) {
        final big = BigInt.from(v);
        if (big < range.$1 || big > range.$2) {
          return PackFailure(
            'Field ${field.name} di luar rentang ${field.type} '
            '(${range.$1}..${range.$2}).',
          );
        }
      }
      return v;
    }

    if (field.isString) {
      final text = '$raw';
      final capacity = field.maxStringBytes ?? 0;
      final byteLen = utf8.encode(text).length;
      if (capacity > 0 && byteLen > capacity) {
        return PackFailure(
          'Field ${field.name} $byteLen B melebihi kapasitas '
          '${field.type} ($capacity B UTF-8).',
        );
      }
      return text;
    }

    return PackFailure('Tipe skema tidak dikenal: ${field.type}.');
  }
}
