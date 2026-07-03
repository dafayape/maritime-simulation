/// Satu atribut skema payload dinamis yang didorong dashboard lewat event
/// `schema:sync` (backend `internal/domain/schema.go`, struct `SchemaField`).
///
/// Tipe yang didukung backend: primitif lebar-tetap (`float32`, `float64`,
/// `int8..int64`, `uint8..uint64`, `bool`) dan string terbatas byte
/// (`string_10` = maksimal 10 byte UTF-8).
class SchemaField {
  const SchemaField({required this.name, required this.type});

  factory SchemaField.fromJson(Map<String, dynamic> json) => SchemaField(
        name: json['name'] as String? ?? '',
        type: json['type'] as String? ?? '',
      );

  final String name;
  final String type;

  Map<String, dynamic> toJson() => {'name': name, 'type': type};

  bool get isBool => type == 'bool';

  bool get isFloat32 => type == 'float32';

  bool get isFloat => type == 'float32' || type == 'float64';

  bool get isInteger => type.startsWith('int') || type.startsWith('uint');

  bool get isString => type.startsWith('string_');

  /// Kapasitas byte UTF-8 untuk tipe `string_N`; null untuk tipe lain.
  int? get maxStringBytes {
    if (!isString) return null;
    return int.tryParse(type.substring('string_'.length));
  }

  /// Rentang nilai valid untuk tipe integer (meniru lebar register fisik).
  /// `uint64` dipangkas ke maksimum int 64-bit bertanda milik Dart — cukup
  /// untuk kebutuhan simulasi.
  (BigInt min, BigInt max)? get integerRange {
    switch (type) {
      case 'int8':
        return (BigInt.from(-128), BigInt.from(127));
      case 'int16':
        return (BigInt.from(-32768), BigInt.from(32767));
      case 'int32':
        return (BigInt.from(-2147483648), BigInt.from(2147483647));
      case 'int64':
      case 'uint64':
        final max = BigInt.parse('9223372036854775807');
        return (type == 'int64' ? -max - BigInt.one : BigInt.zero, max);
      case 'uint8':
        return (BigInt.zero, BigInt.from(255));
      case 'uint16':
        return (BigInt.zero, BigInt.from(65535));
      case 'uint32':
        return (BigInt.zero, BigInt.from(4294967295));
      default:
        return null;
    }
  }

  @override
  String toString() => 'SchemaField($name: $type)';
}
