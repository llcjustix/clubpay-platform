import 'dart:convert';
import 'package:dio/dio.dart';
import 'package:uuid/uuid.dart';
import '../../../core/api_client.dart';
import '../domain/payment_state.dart';

class PaymentRepository {
  PaymentRepository(this.api);
  final ApiClient api;
  Future<void> _storeQueue = Future.value();
  Future<void> _beginQueue = Future.value();
  final _inFlightRequests = <String, Future<PendingOperation>>{};

  Future<T> _withStore<T>(Future<T> Function() action) {
    final result = _storeQueue.then((_) => action());
    _storeQueue = result.then<void>(
      (_) {},
      onError: (Object _, StackTrace _) {},
    );
    return result;
  }

  Future<List<PendingOperation>> _readOperations() async {
    final raw = await api.vault.store.read('mobile.pending');
    if (raw == null) return [];
    final json = jsonDecode(raw) as Map<String, dynamic>;
    return [
      for (final old in (json['other_operations'] as List?) ?? [])
        PendingOperation.fromJson(Map<String, dynamic>.from(old as Map)),
      PendingOperation.fromJson(json),
    ];
  }

  Future<void> _writeOperations(List<PendingOperation> operations) async {
    if (operations.isEmpty) {
      await api.vault.store.delete('mobile.pending');
      return;
    }
    // Retain the legacy top-level shape for existing installations. Other
    // in-flight operations remain recoverable instead of being overwritten.
    await api.vault.store.write(
      'mobile.pending',
      jsonEncode({
        ...operations.last.toJson(),
        if (operations.length > 1)
          'other_operations': [
            for (final p in operations.take(operations.length - 1)) p.toJson(),
          ],
      }),
    );
  }

  Future<List<PendingOperation>> pendingOperations() =>
      _withStore(_readOperations);

  Future<PendingOperation?> pending() async =>
      (await pendingOperations()).lastOrNull;

  Future<void> remember(PendingOperation p, {bool makeCurrent = true}) =>
      _withStore(() async {
        final operations = await _readOperations();
        final index = operations.indexWhere((old) => old.key == p.key);
        if (index >= 0 && !makeCurrent) {
          operations[index] = p;
        } else {
          operations.removeWhere((old) => old.key == p.key);
          operations.add(p);
        }
        await _writeOperations(operations);
      });

  Future<void> clear([String? key]) => _withStore(() async {
    final operations = await _readOperations();
    final removedKey = key ?? operations.lastOrNull?.key;
    operations.removeWhere((p) => p.key == removedKey);
    await _writeOperations(operations);
  });

  Future<PendingOperation> begin({
    required String token,
    String? tariff,
    int? amount,
    String? provider,
    bool redeem = false,
    bool testPayment = false,
    bool isExtension = false,
    String? pcId,
    String? pcLabel,
    String? clubId,
  }) {
    final operation = PendingOperation.request(
      key: const Uuid().v4(),
      token: token,
      tariff: tariff,
      amount: amount,
      provider: provider,
      redeem: redeem,
      isExtension: isExtension,
      pcId: pcId,
      pcLabel: pcLabel,
      clubId: clubId,
    );
    final identity = jsonEncode([
      operation.path,
      operation.body,
      operation.isExtension,
    ]);
    final inFlight = _inFlightRequests[identity];
    if (inFlight != null) return inFlight;
    // Serialize double taps across screens: identical in-flight requests must
    // share their original idempotency key, even before an HTTP reply arrives.
    final result = _beginQueue.then((_) => _begin(operation, testPayment));
    _inFlightRequests[identity] = result;
    _beginQueue = result.then<void>(
      (_) {
        _inFlightRequests.remove(identity);
      },
      onError: (Object _, StackTrace _) {
        _inFlightRequests.remove(identity);
      },
    );
    return result;
  }

  Future<PendingOperation> _begin(
    PendingOperation operation,
    bool testPayment,
  ) async {
    for (final old in (await pendingOperations()).reversed) {
      if (!old.matchesRequest(operation)) continue;
      final oldStatus = await status(old);
      if (oldStatus?.terminal == true) {
        await clear(old.key);
      } else {
        await remember(old);
        if (testPayment && old.invoice != null) {
          await _completeTestPayment(old.invoice!);
        }
        return old;
      }
    }
    // Save the exact operation before submission, including its immutable key.
    await remember(operation);
    final submitted = await _submit(operation);
    if (testPayment && submitted.invoice != null) {
      await _completeTestPayment(submitted.invoice!);
    }
    return submitted;
  }

  Future<void> _completeTestPayment(String invoice) async {
    await api.post(
      '/api/mobile/payments/test/success/${Uri.encodeComponent(invoice)}',
      {},
    );
  }

  Future<PendingOperation> _submit(
    PendingOperation operation, {
    bool makeCurrent = true,
  }) async {
    final json = await api.post(
      operation.path!,
      operation.body!,
      key: operation.key,
    );
    final saved = operation.withResult(
      invoice: json['order']?['invoice_id'] as String?,
      grant: json['grant_id'] as String?,
      isExtension:
          (json['order']?['is_extension'] == true) || operation.isExtension,
    );
    await remember(saved, makeCurrent: makeCurrent);
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
        return _submit(p, makeCurrent: false);
      }
      rethrow;
    }
    final invoice = j['invoice_id'] as String?;
    final grant = j['grant_id'] as String?;
    final result = PendingOperation(
      key: p.key,
      path: p.path,
      body: p.body,
      isExtension: p.isExtension,
      invoice: invoice?.isEmpty == false ? invoice : null,
      grant: grant?.isEmpty == false ? grant : null,
      pcId: p.pcId,
      pcLabel: p.pcLabel,
      clubId: p.clubId,
    );
    if ((j['http_status'] as num? ?? 0) >= 400 &&
        result.invoice == null &&
        result.grant == null) {
      // Older Cloud releases preserve a rejected idempotency key.  Keeping it
      // makes every later status check replay that historical error forever,
      // even though the request did not create an order or a game-access
      // grant.  In that narrow, safe case start a fresh operation immediately.
      // A key is never replaced once an invoice or grant exists, so this cannot
      // duplicate a payment or playing time.
      final retry = PendingOperation(
        key: const Uuid().v4(),
        path: p.path,
        body: p.body,
        isExtension: p.isExtension,
        pcId: p.pcId,
        pcLabel: p.pcLabel,
        clubId: p.clubId,
      );
      await remember(retry, makeCurrent: false);
      await clear(p.key);
      return _submit(retry, makeCurrent: false);
    }
    await remember(result, makeCurrent: false);
    return result;
  }

  Future<SessionStatus?> status(PendingOperation p) async {
    SessionStatus? result;
    if (p.invoice != null) {
      result = SessionStatus.fromJson(
        await api.get('/api/mobile/orders/${Uri.encodeComponent(p.invoice!)}'),
      );
    } else if (p.grant != null) {
      result = SessionStatus.fromJson(
        await api.get('/api/mobile/sessions/${Uri.encodeComponent(p.grant!)}'),
      );
    }
    if ((p.pcId != null && result?.pcId != null && p.pcId != result!.pcId) ||
        (p.clubId != null &&
            result?.clubId != null &&
            p.clubId != result!.clubId)) {
      throw const FormatException('operation_failed');
    }
    return result;
  }
}
