import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:uuid/uuid.dart';
import '../features/auth/domain/auth_models.dart';

abstract interface class SecureStore {
  Future<String?> read(String key);
  Future<void> write(String key, String value);
  Future<void> delete(String key);
}

class PlatformSecureStore implements SecureStore {
  PlatformSecureStore([FlutterSecureStorage? storage])
    : _storage = storage ?? const FlutterSecureStorage();
  final FlutterSecureStorage _storage;
  @override
  Future<String?> read(String key) => _storage.read(key: key);
  @override
  Future<void> write(String key, String value) =>
      _storage.write(key: key, value: value);
  @override
  Future<void> delete(String key) => _storage.delete(key: key);
}

class SessionVault {
  SessionVault(this.store);
  final SecureStore store;
  Future<TokenPair?> tokens() async {
    final raw = await store.read('mobile.tokens');
    if (raw == null) return null;
    try {
      return TokenPair.fromJson(jsonDecode(raw) as Map<String, dynamic>);
    } on FormatException {
      await clear();
      return null;
    } on TypeError {
      await clear();
      return null;
    }
  }

  // One value avoids a half-written pair during refresh rotation.
  Future<void> save(TokenPair pair) =>
      store.write('mobile.tokens', jsonEncode(pair.toJson()));
  Future<void> clear() => store.delete('mobile.tokens');
  Future<String> device() async {
    final stored = await store.read('mobile.device');
    if (stored != null) return stored;
    final value = const Uuid().v4();
    await store.write('mobile.device', value);
    return value;
  }
}
