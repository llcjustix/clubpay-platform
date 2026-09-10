import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../features/auth/data/auth_repository.dart';
import '../features/auth/domain/auth_models.dart';
import '../features/profile/data/profile_repository.dart';
import '../features/profile/domain/club_balance.dart';
import '../features/qr/data/qr_repository.dart';
import '../features/payment/data/payment_repository.dart';
import 'api_client.dart';
import 'secure_store.dart';

final secureStoreProvider = Provider<SecureStore>(
  (ref) => PlatformSecureStore(),
);
final vaultProvider = Provider(
  (ref) => SessionVault(ref.watch(secureStoreProvider)),
);
final apiProvider = Provider((ref) => ApiClient(ref.watch(vaultProvider)));
final authRepositoryProvider = Provider(
  (ref) => AuthRepository(ref.watch(apiProvider)),
);
final authProvider = AsyncNotifierProvider<AuthController, Player?>(
  AuthController.new,
);

class AuthController extends AsyncNotifier<Player?> {
  @override
  Future<Player?> build() async {
    final api = ref.watch(apiProvider);
    api.onUnauthorized = () => state = const AsyncData(null);
    try {
      return await ref.watch(authRepositoryProvider).restore();
    } catch (error, stack) {
      if (await api.vault.tokens() == null) return null;
      Error.throwWithStackTrace(error, stack);
    }
  }

  Future<void> verify(AuthChallenge challenge, String otp) async {
    final player = await ref
        .read(authRepositoryProvider)
        .verify(challenge, otp);
    ref.invalidate(balancesProvider);
    state = AsyncData(player);
  }

  Future<void> logout() async {
    await ref.read(authRepositoryProvider).logout();
    ref.invalidate(balancesProvider);
    state = const AsyncData(null);
  }
}

final balancesProvider = FutureProvider<List<ClubBalance>>((ref) async {
  final player = ref.watch(authProvider).asData?.value;
  if (player == null) return [];
  return ProfileRepository(
    ref.watch(apiProvider),
    playerId: player.id,
  ).balances();
});
final localeProvider = NotifierProvider<LocaleController, Locale>(
  LocaleController.new,
);

class LocaleController extends Notifier<Locale> {
  @override
  Locale build() {
    ref
        .read(secureStoreProvider)
        .read('mobile.locale')
        .then((value) {
          if (ref.mounted && value != null) {
            state = Locale(value == 'uz' ? 'uz' : 'ru');
          }
        })
        .catchError((_) {});
    return const Locale('ru');
  }

  Future<void> select(String code) async {
    await ref.read(secureStoreProvider).write('mobile.locale', code);
    state = Locale(code);
  }
}

final qrRepositoryProvider = Provider(
  (ref) => QrRepository(ref.watch(apiProvider)),
);
final paymentRepositoryProvider = Provider(
  (ref) => PaymentRepository(ref.watch(apiProvider)),
);
