import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/data/services/payload_codec.dart';
import 'package:maritim_node/domain/models/schema_field.dart';

void main() {
  const codec = PayloadCodec();

  group('PayloadCodec — Binary-First (SRS §3A)', () {
    test('float32 ter-encode 4 byte data (header 0xca), bukan float64', () {
      const schema = [SchemaField(name: 'lat', type: 'float32')];
      final out = codec.pack({'lat': 1.5}, schema);
      expect(out, isA<PackSuccess>());
      final bytes = (out as PackSuccess).bytes;
      // fixmap(1)=1 + fixstr "lat"=4 + (0xca + 4 byte)=5 → total 10 byte.
      expect(bytes.length, 10);
      expect(bytes.contains(0xca), isTrue);
    });

    test('float64 lebih besar dari float32 untuk nilai sama', () {
      final f32 = codec.pack(
          {'lat': 1.5}, const [SchemaField(name: 'lat', type: 'float32')]);
      final f64 = codec.pack(
          {'lat': 1.5}, const [SchemaField(name: 'lat', type: 'float64')]);
      expect((f64 as PackSuccess).size, (f32 as PackSuccess).size + 4);
    });

    test('ukuran diambil dari byte array murni dan konsisten dengan base64',
        () {
      const schema = [
        SchemaField(name: 'berat_kg', type: 'uint16'),
        SchemaField(name: 'fresh', type: 'bool'),
      ];
      final out =
          codec.pack({'berat_kg': 300, 'fresh': true}, schema) as PackSuccess;
      expect(out.size, out.bytes.length);
      expect(out.base64Payload, isNotEmpty);
    });

    test('string_N menghitung BYTE UTF-8, bukan jumlah karakter', () {
      const schema = [SchemaField(name: 'jenis', type: 'string_8')];
      // "ikan🐟" = 4 byte ASCII + 4 byte emoji = 8 byte UTF-8 (6 karakter).
      final pas = codec.pack({'jenis': 'ikan🐟'}, schema);
      expect(pas, isA<PackSuccess>());

      const sempit = [SchemaField(name: 'jenis', type: 'string_7')];
      final gagal = codec.pack({'jenis': 'ikan🐟'}, sempit);
      expect(gagal, isA<PackFailure>());
      expect((gagal as PackFailure).message, contains('jenis'));
      expect(gagal.message, contains('7'));
    });

    test('integer di luar rentang lebar registernya ditolak', () {
      const schema = [SchemaField(name: 'n', type: 'uint8')];
      expect(codec.pack({'n': 255}, schema), isA<PackSuccess>());
      expect(codec.pack({'n': 256}, schema), isA<PackFailure>());
      expect(codec.pack({'n': -1}, schema), isA<PackFailure>());

      const i16 = [SchemaField(name: 'n', type: 'int16')];
      expect(codec.pack({'n': -32768}, i16), isA<PackSuccess>());
      expect(codec.pack({'n': -32769}, i16), isA<PackFailure>());
    });

    test('tipe tidak cocok dan tipe tak dikenal ditolak dengan pesan jelas',
        () {
      expect(
        codec.pack(
            {'fresh': 'ya'}, const [SchemaField(name: 'fresh', type: 'bool')]),
        isA<PackFailure>(),
      );
      expect(
        codec.pack(
            {'x': 1}, const [SchemaField(name: 'x', type: 'blob_128')]),
        isA<PackFailure>(),
      );
    });

    test('field kosong dilewati; skema kosong = passthrough', () {
      const schema = [
        SchemaField(name: 'a', type: 'uint8'),
        SchemaField(name: 'b', type: 'uint8'),
      ];
      final sebagian = codec.pack({'a': 1}, schema) as PackSuccess;
      final penuh = codec.pack({'a': 1, 'b': 2}, schema) as PackSuccess;
      expect(sebagian.size, lessThan(penuh.size));

      expect(codec.pack({'bebas': 42}, const []), isA<PackSuccess>());
    });

    test('unpack membaca kembali payload hasil pack (round-trip)', () {
      const schema = [
        SchemaField(name: 'lat', type: 'float32'),
        SchemaField(name: 'jenis', type: 'string_10'),
        SchemaField(name: 'fresh', type: 'bool'),
      ];
      final out = codec
              .pack({'lat': 1.5, 'jenis': 'tuna', 'fresh': true}, schema)
          as PackSuccess;
      final back = codec.unpack(out.base64Payload);
      expect(back, isNotNull);
      expect(back!['jenis'], 'tuna');
      expect(back['fresh'], true);
      expect((back['lat'] as num).toDouble(), 1.5);
    });
  });
}
