import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/app_logo.dart';
import '../../core/app_theme.dart';
import '../../core/widgets.dart';
import '../node/node_screen.dart';
import 'view_models/setup_view_model.dart';

/// Layar Setup (PRD §4.1): alamat server, pilihan sesi aktif, Node ID, dan
/// sumber posisi (GPS asli vs koordinat manual + drift kapal).
class SetupScreen extends ConsumerStatefulWidget {
  const SetupScreen({super.key});

  @override
  ConsumerState<SetupScreen> createState() => _SetupScreenState();
}

class _SetupScreenState extends ConsumerState<SetupScreen> {
  final _serverCtrl = TextEditingController();
  final _nodeIdCtrl = TextEditingController();
  final _latCtrl = TextEditingController();
  final _lngCtrl = TextEditingController();
  bool _useRealGps = false;
  bool _drift = true;
  bool _seeded = false;

  @override
  void dispose() {
    _serverCtrl.dispose();
    _nodeIdCtrl.dispose();
    _latCtrl.dispose();
    _lngCtrl.dispose();
    super.dispose();
  }

  void _seedFromConfig(SetupState s) {
    if (_seeded) return;
    _seeded = true;
    _serverCtrl.text = s.config.serverUrl;
    _nodeIdCtrl.text = s.config.nodeId;
    _latCtrl.text = s.config.manualLat.toStringAsFixed(5);
    _lngCtrl.text = s.config.manualLng.toStringAsFixed(5);
    _useRealGps = s.config.useRealGps;
    _drift = s.config.driftEnabled;
  }

  Future<void> _connect() async {
    final vm = ref.read(setupViewModelProvider.notifier);
    final error = await vm.connect(
      serverUrl: _serverCtrl.text,
      nodeId: _nodeIdCtrl.text.trim(),
      useRealGps: _useRealGps,
      manualLat: double.tryParse(_latCtrl.text.trim()) ?? double.nan,
      manualLng: double.tryParse(_lngCtrl.text.trim()) ?? double.nan,
      driftEnabled: _drift,
    );
    if (!mounted) return;
    if (error != null) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(
        content: Text(error),
        backgroundColor: AppColors.rose.withValues(alpha: 0.18),
      ));
      return;
    }
    Navigator.of(context)
        .push(MaterialPageRoute(builder: (_) => const NodeScreen()));
  }

  @override
  Widget build(BuildContext context) {
    final async = ref.watch(setupViewModelProvider);

    return Scaffold(
      body: SafeArea(
        child: async.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => Center(
            child: Text('Gagal memuat preferensi: $e',
                style: const TextStyle(color: AppColors.rose)),
          ),
          data: (s) {
            _seedFromConfig(s);
            return _buildForm(context, s);
          },
        ),
      ),
    );
  }

  Widget _buildForm(BuildContext context, SetupState s) {
    final vm = ref.read(setupViewModelProvider.notifier);

    return ListView(
      padding: const EdgeInsets.fromLTRB(18, 18, 18, 28),
      children: [
        Row(
          children: [
            const AppLogo(size: 46),
            const SizedBox(width: 12),
            const Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Maritim Node',
                      style: TextStyle(
                          fontSize: 20, fontWeight: FontWeight.w800)),
                  Text('Terminal node kapal — simulasi radio LoRa',
                      style: TextStyle(
                          fontSize: 12, color: AppColors.textMuted)),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 22),

        SectionCard(
          title: 'Server simulasi',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              TextField(
                controller: _serverCtrl,
                keyboardType: TextInputType.url,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Alamat backend',
                  hintText: 'http://192.168.1.10:8080',
                  helperText:
                      'Docker lokal: http://IP-laptop:8080 · VPS: https://domain-anda',
                ),
              ),
              const SizedBox(height: 12),
              OutlinedButton.icon(
                onPressed:
                    s.probing ? null : () => vm.probeServer(_serverCtrl.text),
                icon: s.probing
                    ? const SizedBox(
                        width: 15,
                        height: 15,
                        child: CircularProgressIndicator(strokeWidth: 2))
                    : const Icon(Icons.wifi_tethering, size: 17),
                label: Text(
                    s.probing ? 'Menguji koneksi…' : 'Tes koneksi & muat sesi'),
              ),
              if (s.message != null) ...[
                const SizedBox(height: 10),
                Text(
                  s.message!,
                  style: TextStyle(
                    fontSize: 12.5,
                    color: s.messageIsError ? AppColors.rose : AppColors.emerald,
                  ),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 14),

        SectionCard(
          title: 'Sesi & identitas node',
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              DropdownButtonFormField<String>(
                initialValue: s.selectedSessionId,
                items: [
                  for (final session in s.sessions)
                    DropdownMenuItem(
                      value: session.id,
                      child: Text(
                        '${session.sessionName} · SF${session.spreadingFactor} '
                        '· ${session.txPowerDbm} dBm',
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 13.5),
                      ),
                    ),
                ],
                onChanged: s.sessions.isEmpty ? null : vm.selectSession,
                decoration: InputDecoration(
                  labelText: 'Sesi simulasi aktif',
                  helperText: s.serverOk
                      ? null
                      : 'Jalankan “Tes koneksi” untuk memuat daftar sesi.',
                ),
                dropdownColor: AppColors.surfaceHi,
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _nodeIdCtrl,
                autocorrect: false,
                decoration: const InputDecoration(
                  labelText: 'Node ID kapal',
                  hintText: 'KPL-001',
                  helperText:
                      'Huruf/angka/titik/strip/garis bawah, maks 100 karakter.',
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 14),

        SectionCard(
          title: 'Sumber posisi (node:ping tiap 3 detik)',
          child: Column(
            children: [
              SwitchListTile(
                value: _useRealGps,
                onChanged: (v) => setState(() => _useRealGps = v),
                contentPadding: EdgeInsets.zero,
                title: const Text('Gunakan GPS asli perangkat',
                    style: TextStyle(fontSize: 14.5)),
                subtitle: const Text(
                  'Nonaktif = koordinat manual (cocok untuk pengujian di darat).',
                  style: TextStyle(fontSize: 12, color: AppColors.textMuted),
                ),
              ),
              if (!_useRealGps) ...[
                const SizedBox(height: 6),
                Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: _latCtrl,
                        keyboardType: const TextInputType.numberWithOptions(
                            decimal: true, signed: true),
                        decoration:
                            const InputDecoration(labelText: 'Latitude'),
                      ),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: TextField(
                        controller: _lngCtrl,
                        keyboardType: const TextInputType.numberWithOptions(
                            decimal: true, signed: true),
                        decoration:
                            const InputDecoration(labelText: 'Longitude'),
                      ),
                    ),
                  ],
                ),
                SwitchListTile(
                  value: _drift,
                  onChanged: (v) => setState(() => _drift = v),
                  contentPadding: EdgeInsets.zero,
                  title: const Text('Simulasikan kapal bergerak (drift)',
                      style: TextStyle(fontSize: 14.5)),
                  subtitle: const Text(
                    'Random-walk ±10 km/jam seperti armada nodesim.',
                    style: TextStyle(fontSize: 12, color: AppColors.textMuted),
                  ),
                ),
              ],
            ],
          ),
        ),
        const SizedBox(height: 20),

        FilledButton.icon(
          onPressed: (s.connecting || s.selectedSessionId == null)
              ? null
              : _connect,
          icon: s.connecting
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                      strokeWidth: 2, color: AppColors.textMuted))
              : const Icon(Icons.sailing, size: 19),
          label:
              Text(s.connecting ? 'Menyambungkan…' : 'Hubungkan & berlayar'),
        ),
      ],
    );
  }
}
