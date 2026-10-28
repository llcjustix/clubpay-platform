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
    'A finished extension cannot replace a new reservation balance start',
    () async {
      final vault = SessionVault(MemoryStore());
      await vault.store.write(
        'mobile.pending',
        '{"key":"old-extension","grant":"grant_old","is_extension":true}',
      );
      final api = ApiClient(
        vault,
        dio: stubDio((request) async {
          if (request.path == '/api/mobile/sessions/grant_old') {
            return (
              200,
              {
                'grant_status': 'ended',
                'pc_label': 'Pilot PC #01',
                'session_seconds': 1200,
              },
            );
          }
          if (request.path == '/api/player-balance/redeem') {
            expect(request.data['qr_token'], 'reservation_pc');
            return (200, {'grant_id': 'grant_new'});
          }
          fail('Unexpected request ${request.path}');
        }),
      );

      final started = await PaymentRepository(
        api,
      ).begin(token: 'reservation_pc', redeem: true, isExtension: false);

      expect(started.grant, 'grant_new');
      expect(started.isExtension, isFalse);
    },
  );
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

  test(
    'A legacy 11-hour payment cannot replace 35 hours on another PC',
    () async {
      final vault = SessionVault(MemoryStore());
      await vault.store.write(
        'mobile.pending',
        '{"key":"legacy-next-pc","invoice":"cp_old"}',
      );
      final api = ApiClient(
        vault,
        dio: stubDio((r) async {
          if (r.path == '/api/player-balance/redeem') {
            expect(r.data['qr_token'], 'justix_pc_1');
            return (200, {'grant_id': 'justix_new'});
          }
          if (r.path == '/api/mobile/sessions/justix_new') {
            return (
              200,
              {
                'grant_status': 'pending',
                'pc_label': '1',
                'pc_id': 'pc_1',
                'club_id': 'justix',
                'session_seconds': 127129,
              },
            );
          }
          fail('Another operation was opened: ${r.path}');
        }),
      );
      final repo = PaymentRepository(api);
      final result = await repo.begin(
        token: 'justix_pc_1',
        redeem: true,
        pcId: 'pc_1',
        clubId: 'justix',
      );
      final status = await repo.status(result);
      expect(status!.pcLabel, '1');
      expect(status.seconds, 35 * 3600 + 18 * 60 + 49);
      expect(result.body!['qr_token'], 'justix_pc_1');
      final retained = await repo.pendingOperations();
      expect(retained.length, 2);
      expect(retained.first.invoice, 'cp_old');
    },
  );

  for (final change in [
    'pc',
    'tariff',
    'amount',
    'provider',
    'balance',
    'extension',
  ]) {
    test(
      'Changing $change cannot reuse or confirm a different payment',
      () async {
        final vault = SessionVault(MemoryStore());
        final old = PendingOperation.request(
          key: 'original-operation',
          token: 'pc_a',
          tariff: 'hour',
          provider: 'click',
        ).withResult(invoice: 'cp_old');
        final api = ApiClient(
          vault,
          dio: stubDio((r) async {
            if (r.path == '/api/checkouts') {
              expect(r.headers['Idempotency-Key'], isNot(old.key));
              return (
                201,
                {
                  'order': {'invoice_id': 'cp_new'},
                },
              );
            }
            if (r.path == '/api/player-balance/redeem') {
              return (200, {'grant_id': 'grant_new'});
            }
            if (change == 'provider' &&
                r.path == '/api/mobile/payments/test/success/cp_new') {
              return (200, {'success': true});
            }
            fail('An unrelated payment was reused: ${r.path}');
          }),
        );
        final repo = PaymentRepository(api);
        await repo.remember(old);
        final result = await repo.begin(
          token: change == 'pc' ? 'pc_b' : 'pc_a',
          tariff: change == 'tariff' ? 'two_hours' : 'hour',
          amount: change == 'amount' ? 2000 : null,
          provider: change == 'provider' ? 'mock' : 'click',
          testPayment: change == 'provider',
          redeem: change == 'balance',
          isExtension: change == 'extension',
        );
        expect(result.key, isNot(old.key));
        expect((await repo.pendingOperations()).first.key, old.key);
        expect((await repo.pending())!.key, result.key);
      },
    );
  }

  for (final phase in ['pending', 'accepted']) {
    test(
      'Concurrent identical submissions reuse one key when $phase',
      () async {
        var posts = 0;
        final api = ApiClient(
          SessionVault(MemoryStore()),
          dio: stubDio((r) async {
            if (r.method == 'POST') {
              posts++;
              return (
                201,
                {
                  'order': {'invoice_id': 'cp_one'},
                },
              );
            }
            return (200, {'status': 'paid', 'grant_status': phase});
          }),
        );
        final repo = PaymentRepository(api);
        final results = await Future.wait([
          repo.begin(token: 'pc_a', tariff: 'hour', provider: 'click'),
          repo.begin(token: 'pc_a', tariff: 'hour', provider: 'click'),
        ]);
        expect(posts, 1);
        expect(results.first.key, results.last.key);
        expect(results.last.body!['tariff_block_id'], 'hour');
      },
    );
  }

  test(
    'Finishing an older operation does not clear the new selection',
    () async {
      final repo = PaymentRepository(ApiClient(SessionVault(MemoryStore())));
      const old = PendingOperation(key: 'old', invoice: 'cp_old');
      const newer = PendingOperation(key: 'new', grant: 'new_grant');
      await repo.remember(old);
      await repo.remember(newer);
      await repo.clear(old.key);
      expect((await repo.pending())!.key, newer.key);
      expect((await repo.pendingOperations()).length, 1);
    },
  );

  test(
    'Recovering an older lost response cannot select it over a new one',
    () async {
      final api = ApiClient(
        SessionVault(MemoryStore()),
        dio: stubDio((r) async {
          if (r.method == 'GET') return (404, {'error': 'operation_not_found'});
          expect(r.headers['Idempotency-Key'], 'old-key');
          expect(r.data['qr_token'], 'old_pc');
          return (
            201,
            {
              'order': {'invoice_id': 'cp_recovered_old'},
            },
          );
        }),
      );
      final repo = PaymentRepository(api);
      final old = PendingOperation.request(
        key: 'old-key',
        token: 'old_pc',
        tariff: 'hour',
        provider: 'click',
      );
      await repo.remember(old);
      await repo.remember(
        const PendingOperation(key: 'new-key', grant: 'new_grant'),
      );
      final recovered = await repo.recover(old);
      expect(recovered.invoice, 'cp_recovered_old');
      expect((await repo.pending())!.key, 'new-key');
      expect(
        (await repo.pendingOperations()).first.body!['qr_token'],
        'old_pc',
      );
    },
  );

  test('Status rejects a response bound to a different PC or club', () async {
    final api = ApiClient(
      SessionVault(MemoryStore()),
      dio: stubDio(
        (r) async => (
          200,
          {
            'pc_id': 'other_pc',
            'club_id': 'other_club',
            'session_seconds': 40000,
          },
        ),
      ),
    );
    await expectLater(
      PaymentRepository(api).status(
        const PendingOperation(
          key: 'one',
          grant: 'one',
          pcId: 'pc_1',
          clubId: 'justix',
        ),
      ),
      throwsFormatException,
    );
  });
}
