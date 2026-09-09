import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:uuid/uuid.dart';
import '../../../core/api_client.dart';
import '../domain/payment_state.dart';

class PaymentRepository {
  PaymentRepository(this.api);
  final ApiClient api;
  Future<PendingOperation?> pending() async {
    final raw = await api.vault.store.read('mobile.pending');
    return raw == null
        ? null
        : PendingOperation.fromJson(jsonDecode(raw) as Map<String, dynamic>);
  }

  Future<void> remember(PendingOperation p) =>
      api.vault.store.write('mobile.pending', jsonEncode(p.toJson()));
  Future<void> clear() => api.vault.store.delete('mobile.pending');
  Future<PendingOperation> begin({
    required String token,
    String? tariff,
    int? amount,
    String? provider,
    bool redeem = false,
    bool testPayment = false,
  }) async {
    final old = await pending();
    if (old != null) {
      if (testPayment && old.invoice != null) await _completeTestPayment(old.invoice!);
      return old;
    }
    final operation = PendingOperation(
      key: const Uuid().v4(),
      path: redeem ? '/api/player-balance/redeem' : '/api/checkouts',
      body: {
        'qr_token': token,
        if (!redeem) 'payment_provider': provider,
        if (!redeem && amount != null) 'amount_uzs': amount,
        if (!redeem && amount == null) 'tariff_block_id': tariff,
      },
    );
    // Save the exact operation before submission, including its immutable key.
    await remember(operation);
    final submitted = await _submit(operation);
    if (testPayment && submitted.invoice != null) {
      await _completeTestPayment(submitted.invoice!);
    }
    return submitted;
  }

  Future<void> _completeTestPayment(String invoice) async {
    await api.post('/api/mobile/payments/test/success/${Uri.encodeComponent(invoice)}', {});
  }

  Future<PendingOperation> _submit(PendingOperation operation) async {
    final json = await api.post(
      operation.path!,
      operation.body!,
      key: operation.key,
    );
    final saved = PendingOperation(
      key: operation.key,
      invoice: json['order']?['invoice_id'] as String?,
      grant: json['grant_id'] as String?,
    );
    await remember(saved);
    return saved;
  }

  Future<PendingOperation> recover(PendingOperation p) async {
    if (p.invoice != null || p.grant != null) return p;
    Map<String, dynamic> j;
    try {
      j = await api.get('/api/mobile/operations/${Uri.encodeComponent(p.key)}');
    } on DioException catch (error) {
      // No reservation exists: retransmit the same request with the same key.
      // Even if an earlier request arrives later, the server executes it once.
      if (error.response?.statusCode == 404 &&
          p.body != null &&
          ['/api/checkouts', '/api/player-balance/redeem'].contains(p.path)) {
        return _submit(p);
      }
      rethrow;
    }
    final invoice = j['invoice_id'] as String?;
    final grant = j['grant_id'] as String?;
    final result = PendingOperation(
      key: p.key,
      path: p.path,
      body: p.body,
      invoice: invoice?.isEmpty == false ? invoice : null,
      grant: grant?.isEmpty == false ? grant : null,
    );
    if ((j['http_status'] as num? ?? 0) >= 400 &&
        result.invoice == null &&
        result.grant == null) {
      await clear();
      throw const FormatException('operation_failed');
    }
    await remember(result);
    return result;
  }

  Future<SessionStatus?> status(PendingOperation p) async {
    if (p.invoice != null) {
      return SessionStatus.fromJson(
        await api.get('/api/mobile/orders/${Uri.encodeComponent(p.invoice!)}'),
      );
    }
    if (p.grant != null) {
      return SessionStatus.fromJson(
        await api.get('/api/mobile/sessions/${Uri.encodeComponent(p.grant!)}'),
      );
    }
    return null;
  }
}
