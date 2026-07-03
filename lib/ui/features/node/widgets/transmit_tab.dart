import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../data/services/payload_codec.dart';
import '../../../../di/providers.dart';
import '../../../../domain/models/schema_field.dart';
import '../../../../domain/node_link_state.dart';
import '../../../core/app_theme.dart';
import '../../../core/widgets.dart';
import '../view_models/transmit_view_model.dart';

/// Skema cadangan saat server belum menyuntikkan skema dinamis — meniru
/// payload fallback harness `nodesim` backend (lat/lng/berat_kg).
const _fallbackSchema = [
  SchemaField(name: 'lat', type: 'float32'),
  SchemaField(name: 'lng', type: 'float32'),
  SchemaField(name: 'berat_kg', type: 'uint16'),
];

const _sampleFish = ['tuna', 'layur', 'tongkol', 'cakalang', 'kembung'];

/// Form dinamis (PRD §4.1): field disusun dari `schema:sync`, byte counter
/// dihitung dari hasil kompresi MessagePack sesungguhnya, dan tombol Kirim
/// terkunci reaktif saat melampaui limit SF / terisolasi / offline.
class TransmitTab extends ConsumerStatefulWidget {
  const TransmitTab({super.key, required this.state});

  final NodeLinkState state;

  @override
  ConsumerState<TransmitTab> createState() => _TransmitTabState();
}

class _TransmitTabState extends ConsumerState<TransmitTab> {
  final Map<String, TextEditingController> _textCtrls = {};
  final Map<String, bool> _boolValues = {};
  String _schemaSig = '';
  PackOutcome? _measured;
  final _random = math.Random();

  List<SchemaField> get _schema =>
      widget.state.schema.isEmpty ? _fallbackSchema : widget.state.schema;

  bool get _usingFallback => widget.state.schema.isEmpty;

  @override
  void didUpdateWidget(covariant TransmitTab oldWidget) {
    super.didUpdateWidget(oldWidget);
    _syncControllers();
  }

  @override
  void initState() {
    super.initState();
    _syncControllers();
  }

  @override
  void dispose() {
    for (final c in _textCtrls.values) {
      c.dispose();
    }
    super.dispose();
  }

  /// Menyusun ulang controller ketika komposisi skema berubah dari server.
  void _syncControllers() {
    final sig = _schema.map((f) => '${f.name}:${f.type}').join('|');
    if (sig == _schemaSig) return;
    _schemaSig = sig;

    final valid = {for (final f in _schema) f.name};
    _textCtrls.removeWhere((name, ctrl) {
      if (!valid.contains(name)) {
        ctrl.dispose();
        return true;
      }
      return false;
    });
    _boolValues.removeWhere((name, _) => !valid.contains(name));

    for (final field in _schema) {
      if (field.isBool) {
        _boolValues.putIfAbsent(field.name, () => false);
      } else {
        _textCtrls.putIfAbsent(field.name, TextEditingController.new);
      }
    }
    _remeasure();
  }

  Map<String, dynamic> _values() {
    final values = <String, dynamic>{};
    for (final field in _schema) {
      if (field.isBool) {
        values[field.name] = _boolValues[field.name] ?? false;
        continue;
      }
      final raw = _textCtrls[field.name]?.text.trim() ?? '';
      if (raw.isEmpty) continue;
      if (field.isFloat) {
        values[field.name] = double.tryParse(raw) ?? raw;
      } else if (field.isInteger) {
        values[field.name] = int.tryParse(raw) ?? raw;
      } else {
        values[field.name] = raw;
      }
    }
    return values;
  }

  void _remeasure() {
    final repo = ref.read(nodeLinkRepositoryProvider);
    setState(() => _measured = repo.codec.pack(_values(), _schema));
  }

  void _fillSample() {
    final pos = widget.state.position;
    for (final field in _schema) {
      if (field.isBool) {
        _boolValues[field.name] = _random.nextBool();
        continue;
      }
      final ctrl = _textCtrls[field.name];
      if (ctrl == null) continue;
      if (field.isFloat && field.name == 'lat' && pos != null) {
        ctrl.text = pos.lat.toStringAsFixed(5);
      } else if (field.isFloat && field.name == 'lng' && pos != null) {
        ctrl.text = pos.lng.toStringAsFixed(5);
      } else if (field.isFloat) {
        ctrl.text = (_random.nextDouble() * 100).toStringAsFixed(1);
      } else if (field.isInteger) {
        ctrl.text = '${_random.nextInt(300) + 1}';
      } else if (field.isString) {
        var name = _sampleFish[_random.nextInt(_sampleFish.length)];
        final cap = field.maxStringBytes ?? name.length;
        if (name.length > cap) name = name.substring(0, cap);
        ctrl.text = name;
      }
    }
    _remeasure();
  }

  Future<void> _submit() async {
    await ref.read(transmitViewModelProvider.notifier).submit(_values());
  }

  @override
  Widget build(BuildContext context) {
    final state = widget.state;
    final env = state.env;
    final limit = env?.maxPayloadBytes ?? 0;

    ref.listen(transmitViewModelProvider, (_, next) {
      final outcome = next.valueOrNull;
      if (outcome == null || !next.hasValue) return;
      ScaffoldMessenger.of(context)
        ..clearSnackBars()
        ..showSnackBar(SnackBar(
          content: Text(outcome.message),
          backgroundColor: outcome.accepted
              ? AppColors.emerald.withValues(alpha: 0.16)
              : AppColors.rose.withValues(alpha: 0.16),
        ));
    });
    final busy = ref.watch(transmitViewModelProvider).isLoading;

    final measured = _measured;
    final size = measured is PackSuccess ? measured.size : null;
    final packError = measured is PackFailure ? measured.message : null;
    final overLimit = size != null && limit > 0 && size > limit;
    final canSend = state.canTransmit &&
        env != null &&
        size != null &&
        !overLimit &&
        !busy;

    final meterColor = overLimit
        ? AppColors.rose
        : (size != null && limit > 0 && size > limit * 0.7)
            ? AppColors.amber
            : AppColors.emerald;

    return ListView(
      padding: const EdgeInsets.fromLTRB(14, 6, 14, 24),
      children: [
        SectionCard(
          title: _usingFallback
              ? 'Payload (skema bawaan — server belum kirim skema)'
              : 'Payload dinamis (skema dari Syahbandar)',
          trailing: TextButton.icon(
            onPressed: _fillSample,
            icon: const Icon(Icons.auto_fix_high, size: 15),
            label: const Text('Isi contoh', style: TextStyle(fontSize: 12.5)),
          ),
          child: Column(
            children: [
              for (final field in _schema) ...[
                _buildField(field),
                const SizedBox(height: 10),
              ],
            ],
          ),
        ),
        const SizedBox(height: 14),
        SectionCard(
          title: 'Byte counter (biner MessagePack)',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(999),
                child: LinearProgressIndicator(
                  minHeight: 8,
                  value: (size != null && limit > 0)
                      ? (size / limit).clamp(0.0, 1.0)
                      : 0,
                  color: meterColor,
                ),
              ),
              const SizedBox(height: 8),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  TabularText(
                    env == null
                        ? 'Menunggu parameter radio…'
                        : size == null
                            ? '— / $limit B'
                            : '$size / $limit B (SF${env.spreadingFactor})',
                    color: meterColor,
                    size: 13.5,
                  ),
                  if (state.estimatedPackedBytes > 0)
                    TabularText(
                      'estimasi skema ~${state.estimatedPackedBytes} B',
                      color: AppColors.textMuted,
                      size: 11.5,
                    ),
                ],
              ),
              if (packError != null) ...[
                const SizedBox(height: 6),
                Text(packError,
                    style:
                        const TextStyle(color: AppColors.rose, fontSize: 12)),
              ],
              if (overLimit)
                const Padding(
                  padding: EdgeInsets.only(top: 6),
                  child: Text(
                    'Payload melampaui limit fisik LoRa — kurangi isi data '
                    'atau minta dashboard menaikkan Spreading Factor.',
                    style: TextStyle(color: AppColors.rose, fontSize: 12),
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(height: 18),
        FilledButton.icon(
          onPressed: canSend ? _submit : null,
          icon: busy
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                      strokeWidth: 2, color: AppColors.textMuted))
              : const Icon(Icons.podcasts, size: 19),
          label: Text(busy
              ? 'Mengudara…'
              : !state.isOnline
                  ? 'Kanal terputus'
                  : !state.route.isRouted
                      ? 'Terisolasi — tanpa rute'
                      : 'Kirim frame'),
        ),
      ],
    );
  }

  Widget _buildField(SchemaField field) {
    if (field.isBool) {
      return SwitchListTile(
        value: _boolValues[field.name] ?? false,
        onChanged: (v) {
          _boolValues[field.name] = v;
          _remeasure();
        },
        contentPadding: EdgeInsets.zero,
        title: Text(field.name, style: const TextStyle(fontSize: 14.5)),
        subtitle: Text(field.type,
            style: const TextStyle(fontSize: 11, color: AppColors.textMuted)),
      );
    }

    final range = field.integerRange;
    return TextField(
      controller: _textCtrls[field.name],
      onChanged: (_) => _remeasure(),
      keyboardType: field.isString
          ? TextInputType.text
          : TextInputType.numberWithOptions(
              decimal: field.isFloat,
              signed: !field.type.startsWith('uint'),
            ),
      maxLength: field.maxStringBytes,
      decoration: InputDecoration(
        labelText: field.name,
        counterText: '',
        helperText: field.isString
            ? '${field.type} · maks ${field.maxStringBytes} B UTF-8'
            : range != null
                ? '${field.type} · ${range.$1}..${range.$2}'
                : field.type,
      ),
    );
  }
}
