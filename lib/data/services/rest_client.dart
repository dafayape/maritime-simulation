import 'dart:convert';

import 'package:http/http.dart' as http;

import '../../domain/models/session_summary.dart';

class RestException implements Exception {
  const RestException(this.message, {this.statusCode});

  final String message;
  final int? statusCode;

  @override
  String toString() => message;
}

/// Klien REST bootstrap ke backend Go (amplop `{status, message, data}`).
/// WebSocket tetap menjadi kanal utama; REST hanya dipakai untuk memilih
/// sesi, uji koneksi, dan fallback pengecekan `is_active` ketika event
/// `session:ended` kalah balapan dengan penutupan socket di sisi server.
class RestClient {
  RestClient(String baseUrl, {http.Client? client})
      : baseUrl = normalizeBaseUrl(baseUrl),
        _http = client ?? http.Client();

  final String baseUrl;
  final http.Client _http;

  static const _timeout = Duration(seconds: 8);

  /// Merapikan input pengguna: memangkas spasi & garis miring akhir dan
  /// menambahkan skema `http://` bila lupa ditulis.
  static String normalizeBaseUrl(String raw) {
    var url = raw.trim();
    if (url.isEmpty) return url;
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      url = 'http://$url';
    }
    while (url.endsWith('/')) {
      url = url.substring(0, url.length - 1);
    }
    return url;
  }

  /// URL kanal node: `http(s)` → `ws(s)` + `/ws/nodes?session_id&node_id`.
  static Uri wsNodesUri(String baseUrl, String sessionId, String nodeId) {
    final base = Uri.parse(normalizeBaseUrl(baseUrl));
    return base.replace(
      scheme: base.scheme == 'https' ? 'wss' : 'ws',
      path: '/ws/nodes',
      queryParameters: {'session_id': sessionId, 'node_id': nodeId},
    );
  }

  Future<void> health() async {
    final resp = await _get('/healthz');
    if (resp.statusCode != 200) {
      throw RestException(
        'Server menjawab ${resp.statusCode} pada /healthz.',
        statusCode: resp.statusCode,
      );
    }
  }

  Future<List<SessionSummary>> listSessions({int limit = 50}) async {
    final data = await _dataOf(await _get('/api/v1/simulations?limit=$limit'));
    return [
      for (final row in (data as List<dynamic>? ?? []))
        SessionSummary.fromJson(row as Map<String, dynamic>),
    ];
  }

  Future<SessionSummary?> getSession(String id) async {
    final resp = await _get('/api/v1/simulations/$id');
    if (resp.statusCode == 404) return null;
    final data = await _dataOf(resp) as Map<String, dynamic>? ?? {};
    final session = data['session'] as Map<String, dynamic>?;
    return session == null ? null : SessionSummary.fromJson(session);
  }

  /// Statistik server-side truth (dipakai uji integrasi & diagnosa).
  Future<Map<String, dynamic>> stats(String id) async {
    final data = await _dataOf(await _get('/api/v1/simulations/$id/stats'));
    return (data as Map<String, dynamic>?) ?? {};
  }

  Future<http.Response> _get(String path) async {
    try {
      return await _http.get(Uri.parse('$baseUrl$path')).timeout(_timeout);
    } on RestException {
      rethrow;
    } on Object catch (e) {
      throw RestException('Tidak dapat menghubungi $baseUrl: $e');
    }
  }

  Future<dynamic> _dataOf(http.Response resp) async {
    Map<String, dynamic> body;
    try {
      body = jsonDecode(resp.body) as Map<String, dynamic>;
    } on Object {
      throw RestException(
        'Respons bukan JSON amplop backend (HTTP ${resp.statusCode}).',
        statusCode: resp.statusCode,
      );
    }
    if (resp.statusCode >= 300) {
      throw RestException(
        body['message'] as String? ?? 'HTTP ${resp.statusCode}',
        statusCode: resp.statusCode,
      );
    }
    return body['data'];
  }

  void dispose() => _http.close();
}
