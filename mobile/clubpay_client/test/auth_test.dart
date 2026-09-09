import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/auth/data/auth_repository.dart';
import 'package:clubpay_client/features/auth/domain/auth_models.dart';
import 'helpers.dart';

void main() {
  test('Uzbek phone normalization and validation', () {
    expect(normalizeUzPhone('90 123 45 67'), '+998901234567');
    expect(normalizeUzPhone('+998 (90) 123-45-67'), '+998901234567');
    expect(normalizeUzPhone('+7 9012345678'), isNull);
    expect(normalizeUzPhone(''), isNull);
  });
  test('Token pair is saved atomically and device survives logout', () async {
    final store = MemoryStore();
    final vault = SessionVault(store);
    final device = await vault.device();
    await vault.save(const TokenPair('access', 'refresh'));
    expect((await vault.tokens())!.refresh, 'refresh');
    expect(store.values.containsKey('mobile.tokens'), isTrue);
    await vault.clear();
    expect(await vault.tokens(), isNull);
    expect(await vault.device(), device);
  });
  test('Restore without credentials never requests a fake profile', () async {
    final api = ApiClient(
      SessionVault(MemoryStore()),
      dio: stubDio((_) async {
        fail('unexpected request');
      }),
    );
    expect(await AuthRepository(api).restore(), isNull);
  });
  test(
    'Verification persists mobile credentials and loads real profile',
    () async {
      final vault = SessionVault(MemoryStore());
      final api = ApiClient(
        vault,
        dio: stubDio((r) async {
          if (r.path.endsWith('/verify')) {
            expect(r.data['otp'], '123456');
            expect(r.data['device_id'], isNotEmpty);
            return (
              200,
              {'access_token': 'mob_a_1', 'refresh_token': 'mob_r_1'},
            );
          }
          expect(r.headers['Authorization'], 'Bearer mob_a_1');
          return (
            200,
            {'id': 'player', 'phone': '+998901234567', 'first_name': 'Ali'},
          );
        }),
      );
      final player = await AuthRepository(api).verify(
        AuthChallenge('challenge', 'https://t.me/clubpay', DateTime.now()),
        '123456',
      );
      expect(player.firstName, 'Ali');
      expect((await vault.tokens())!.refresh, 'mob_r_1');
    },
  );
  test('Concurrent 401 responses rotate refresh exactly once', () async {
    final vault = SessionVault(MemoryStore());
    await vault.save(const TokenPair('old', 'refresh'));
    var rotations = 0;
    final api = ApiClient(
      vault,
      dio: stubDio((r) async {
        if (r.path.endsWith('/refresh')) {
          rotations++;
          await Future<void>.delayed(const Duration(milliseconds: 10));
          return (200, {'access_token': 'new', 'refresh_token': 'rotated'});
        }
        if (r.headers['Authorization'] == 'Bearer old') {
          return (401, {'error': 'expired'});
        }
        return (200, {'ok': true});
      }),
    );
    await Future.wait([
      api.get('/api/mobile/me'),
      api.get('/api/mobile/balances'),
    ]);
    expect(rotations, 1);
    expect((await vault.tokens())!.refresh, 'rotated');
  });
  test('Refresh rejection clears credentials and signals login', () async {
    final vault = SessionVault(MemoryStore());
    await vault.save(const TokenPair('old', 'refresh'));
    var unauthorized = false;
    final api =
        ApiClient(
            vault,
            dio: stubDio((_) async => (401, {'error': 'refresh_reused'})),
          )
          ..onUnauthorized = () {
            unauthorized = true;
          };
    await expectLater(api.get('/api/mobile/me'), throwsA(anything));
    expect(await vault.tokens(), isNull);
    expect(unauthorized, isTrue);
  });
  test('Failed storage after OTP revokes issued refresh token', () async {
    final store = MemoryStore();
    final vault = SessionVault(store);
    await vault.device();
    store.failWrites = true;
    var revoked = false;
    final api = ApiClient(
      vault,
      dio: stubDio((r) async {
        if (r.path.endsWith('/logout')) {
          revoked = true;
          return (204, <String, dynamic>{});
        }
        return (200, {'access_token': 'a', 'refresh_token': 'r'});
      }),
    );
    await expectLater(
      AuthRepository(api).verify(
        AuthChallenge('c', 'https://t.me/bot', DateTime.now()),
        '123456',
      ),
      throwsA(anything),
    );
    expect(revoked, isTrue);
    expect(await vault.tokens(), isNull);
  });
}
