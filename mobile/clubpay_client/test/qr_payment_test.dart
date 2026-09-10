import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/auth/domain/auth_models.dart';
import 'package:clubpay_client/features/qr/data/qr_repository.dart';
import 'package:clubpay_client/features/qr/domain/qr_models.dart';
import 'package:clubpay_client/features/payment/data/payment_repository.dart';
import 'package:clubpay_client/features/payment/domain/payment_state.dart';
import 'helpers.dart';

void main() {
  test('QR URLs only yield tokens, never network destinations', () {
    expect(
      parseQrToken('https://clubpay.uz/qr/pc_opaque?player_auth_token=old'),
      'pc_opaque',
    );
    expect(parseQrToken('http://192.168.1.2/qr/pc_test'), 'pc_test');
    expect(
      parseQrToken('https://t.me/ClubPayBot?startapp=pc_opaque'),
      'pc_opaque',
    );
    expect(parseQrToken('javascript:alert(1)'), isNull);
    expect(parseQrToken('https://host/admin'), isNull);
    expect(parseQrToken('../admin'), isNull);
  });
  test('QR resolution calls configured public API path only', () async {
    final api = ApiClient(
      SessionVault(MemoryStore()),
      dio: stubDio((r) async {
        expect(r.uri.host, 'localhost');
        expect(r.path, '/api/qr/pc_test');
        return (
          200,
          {
            'club': {'id': 'club', 'name': 'Club'},
            'pc': {'label': 'PC 1', 'status': 'available'},
            'zone': {'name': 'Standard', 'hourly_price_uzs': 10000},
            'qr_type': 'static_pc',
            'tariffs': null,
            'payment_providers': [],
          },
        );
      }),
    );
    final pc = await QrRepository(api).resolve('http://192.168.1.2/qr/pc_test');
    expect(pc.canStart, isTrue);
    expect(pc.tariffs, isEmpty);
  });
  test('Paid order without accepted grant remains starting', () {
    expect(
      SessionStatus.fromJson({
        'status': 'paid',
        'grant_status': 'pending',
      }).phase,
      SessionPhase.starting,
    );
    expect(
      SessionStatus.fromJson({
        'status': 'payment_pending',
        'grant_status': 'pending',
      }).phase,
      SessionPhase.waitingPayment,
    );
    expect(
      SessionStatus.fromJson({
        'status': 'paid',
        'grant_status': 'accepted',
      }).phase,
      SessionPhase.active,
    );
    expect(
      SessionStatus.fromJson({
        'status': 'paid',
        'grant_status': 'start_failed',
      }).phase,
      SessionPhase.startFailed,
    );
    expect(
      SessionStatus.fromJson({
        'status': 'refunded',
        'grant_status': 'accepted',
      }).phase,
      SessionPhase.paymentFailed,
    );
    expect(
      SessionStatus.fromJson({'grant_status': 'ended'}).phase,
      SessionPhase.ended,
    );
  });
  test('Lost checkout response is persisted and never duplicated', () async {
    final vault = SessionVault(MemoryStore());
    var posts = 0;
    final api = ApiClient(
      vault,
      dio: stubDio((r) async {
        posts++;
        expect(r.headers['Idempotency-Key'], isNotEmpty);
        expect(await vault.store.read('mobile.pending'), isNotNull);
        throw StateError('connection lost');
      }),
    );
    final repo = PaymentRepository(api);
    await expectLater(
      repo.begin(token: 'pc_test', tariff: 'tariff', provider: 'click'),
      throwsA(anything),
    );
    final pending = await repo.begin(
      token: 'pc_test',
      tariff: 'tariff',
      provider: 'click',
    );
    expect(pending.key, isNotEmpty);
    expect(posts, 1);
  });
  test(
    'Recovery retries only the saved request with the original key',
    () async {
      final vault = SessionVault(MemoryStore());
      final keys = <String>[];
      final api = ApiClient(
        vault,
        dio: stubDio((r) async {
          if (r.method == 'GET') return (404, {'error': 'operation_not_found'});
          keys.add(r.headers['Idempotency-Key'] as String);
          if (keys.length == 1) {
            throw StateError('request never reached server');
          }
          return (
            201,
            {
              'order': {'invoice_id': 'cp_recovered'},
            },
          );
        }),
      );
      final repo = PaymentRepository(api);
      await expectLater(
        repo.begin(token: 'pc_test', tariff: 'one', provider: 'click'),
        throwsA(anything),
      );
      final recovered = await repo.recover((await repo.pending())!);
      expect(recovered.invoice, 'cp_recovered');
      expect(keys.length, 2);
      expect(keys.toSet().length, 1);
    },
  );
  test('Test payment confirms only the mobile order after checkout', () async {
    final vault = SessionVault(MemoryStore());
    await vault.save(const TokenPair('mob_a_test', 'mob_r_test'));
    var checkoutCalls = 0;
    var confirmationCalls = 0;
    final api = ApiClient(
      vault,
      dio: stubDio((request) async {
        if (request.path == '/api/checkouts') {
          checkoutCalls++;
          expect(request.data['payment_provider'], 'mock');
          return (
            201,
            {
              'order': {'invoice_id': 'cp_test_payment'},
            },
          );
        }
        if (request.path ==
            '/api/mobile/payments/test/success/cp_test_payment') {
          confirmationCalls++;
          expect(request.headers['Authorization'], 'Bearer mob_a_test');
          return (200, {'success': true, 'grant_id': 'grant_test_payment'});
        }
        fail('Unexpected request ${request.path}');
      }),
    );
    final pending = await PaymentRepository(api).begin(
      token: 'pc_test',
      tariff: 'tariff_test',
      provider: 'mock',
      testPayment: true,
    );
    expect(pending.invoice, 'cp_test_payment');
    expect(checkoutCalls, 1);
    expect(confirmationCalls, 1);
  });
}
