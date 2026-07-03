import 'package:flutter_test/flutter_test.dart';
import 'package:maritim_node/core/utils/backoff.dart';

void main() {
  group('ExponentialBackoff (SRS §5.3)', () {
    test('deret 1s→2s→4s→8s→16s→30s dengan plafon 30s', () {
      final backoff = ExponentialBackoff();
      final delays = [for (var i = 0; i < 7; i++) backoff.next().inSeconds];
      expect(delays, [1, 2, 4, 8, 16, 30, 30]);
      expect(backoff.attempt, 7);
    });

    test('reset mengembalikan deret ke awal', () {
      final backoff = ExponentialBackoff();
      backoff
        ..next()
        ..next()
        ..reset();
      expect(backoff.attempt, 0);
      expect(backoff.next().inSeconds, 1);
    });
  });
}
