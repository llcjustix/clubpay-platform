import '../../../core/api_client.dart';
import '../domain/auth_models.dart';

class AuthRepository {
  AuthRepository(this.api);
  final ApiClient api;
  Future<Player?> restore() async {
    if (await api.vault.tokens() == null) return null;
    return Player.fromJson(await api.get('/api/mobile/me'));
  }

  Future<AuthChallenge> challenge(String phone) async => AuthChallenge.fromJson(
    await api.post('/api/mobile/auth/challenge', {
      'phone': phone,
      'device_id': await api.vault.device(),
    }, auth: false),
  );
  Future<Player> verify(AuthChallenge challenge, String otp) async {
    final pair = TokenPair.fromJson(
      await api.post('/api/mobile/auth/verify', {
        'challenge': challenge.token,
        'otp': otp,
        'device_id': await api.vault.device(),
      }, auth: false),
    );
    try {
      await api.vault.save(pair);
    } catch (_) {
      try {
        await api.post('/api/mobile/auth/logout', {
          'refresh_token': pair.refresh,
          'device_id': await api.vault.device(),
        }, auth: false);
      } catch (_) {}
      rethrow;
    }
    return Player.fromJson(await api.get('/api/mobile/me'));
  }

  Future<void> logout() async {
    final pair = await api.vault.tokens();
    if (pair != null) {
      // Preserve the credential on network failure so logout can be retried.
      await api.post('/api/mobile/auth/logout', {
        'refresh_token': pair.refresh,
        'device_id': await api.vault.device(),
      }, auth: false);
    }
    await api.vault.clear();
    await api.vault.store.delete('mobile.pending');
  }
}
