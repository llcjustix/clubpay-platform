import 'package:dio/dio.dart';
import '../features/auth/domain/auth_models.dart';
import 'secure_store.dart';

class ApiClient {
  ApiClient(this.vault, {Dio? dio})
    : dio =
          dio ??
          Dio(
            BaseOptions(
              baseUrl: const String.fromEnvironment(
                'API_BASE_URL',
                defaultValue: 'https://api-clubpay.justix.uz',
              ),
              connectTimeout: const Duration(seconds: 15),
              receiveTimeout: const Duration(seconds: 95),
              followRedirects: false,
            ),
          );
  final Dio dio;
  final SessionVault vault;
  Future<void>? _refreshing;
  void Function()? onUnauthorized;

  Future<Map<String, dynamic>> get(String path) => request('GET', path);
  Future<Map<String, dynamic>> post(
    String path,
    Map<String, dynamic> data, {
    bool auth = true,
    String? key,
  }) => request('POST', path, data: data, auth: auth, key: key);
  Future<Map<String, dynamic>> request(
    String method,
    String path, {
    Map<String, dynamic>? data,
    bool auth = true,
    String? key,
  }) async {
    final tokens = auth ? await vault.tokens() : null;
    Future<Response<dynamic>> send(String? access) => dio.request(
      path,
      data: data,
      options: Options(
        method: method,
        headers: {
          if (access != null) 'Authorization': 'Bearer $access',
          'Idempotency-Key': ?key,
        },
      ),
    );
    Response<dynamic> result;
    try {
      result = await send(tokens?.access);
    } on DioException catch (error) {
      if (!auth || tokens == null || error.response?.statusCode != 401) rethrow;
      // Another concurrent request may already have rotated the stored pair.
      final latest = await vault.tokens();
      if (latest?.access == tokens.access) {
        _refreshing ??= _refresh().whenComplete(() => _refreshing = null);
        await _refreshing;
      }
      final refreshed = await vault.tokens();
      if (refreshed == null) rethrow;
      try {
        result = await send(refreshed.access);
      } on DioException catch (retry) {
        if (retry.response?.statusCode == 401) {
          await vault.clear();
          onUnauthorized?.call();
        }
        rethrow;
      }
    }
    return result.data == null || result.data == ''
        ? {}
        : Map<String, dynamic>.from(result.data as Map);
  }

  Future<void> _refresh() async {
    final pair = await vault.tokens();
    if (pair == null) return;
    try {
      final response = await dio.post<Map<String, dynamic>>(
        '/api/mobile/auth/refresh',
        data: {
          'refresh_token': pair.refresh,
          'device_id': await vault.device(),
        },
      );
      final rotated = TokenPair.fromJson(response.data!);
      try {
        await vault.save(rotated);
      } catch (_) {
        // Revoke the newly issued pair if local persistence fails.
        try {
          await dio.post(
            '/api/mobile/auth/logout',
            data: {
              'refresh_token': rotated.refresh,
              'device_id': await vault.device(),
            },
          );
        } catch (_) {}
        await vault.clear();
        onUnauthorized?.call();
        rethrow;
      }
    } on DioException catch (error) {
      if (error.response?.statusCode == 401) {
        await vault.clear();
        onUnauthorized?.call();
      }
      rethrow;
    }
  }
}
