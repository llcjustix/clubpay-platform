import 'dart:convert';
import 'dart:typed_data';
import 'package:dio/dio.dart';
import 'package:clubpay_client/core/secure_store.dart';

class MemoryStore implements SecureStore {
  final values = <String, String>{};
  bool failWrites = false;
  @override
  Future<String?> read(String key) async => values[key];
  @override
  Future<void> write(String key, String value) async {
    if (failWrites) throw StateError('storage unavailable');
    values[key] = value;
  }

  @override
  Future<void> delete(String key) async {
    values.remove(key);
  }
}

class StubAdapter implements HttpClientAdapter {
  StubAdapter(this.handle);
  final Future<(int, Map<String, dynamic>)> Function(RequestOptions) handle;
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final (status, data) = await handle(options);
    return ResponseBody.fromString(
      jsonEncode(data),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio stubDio(
  Future<(int, Map<String, dynamic>)> Function(RequestOptions) handle,
) =>
    Dio(BaseOptions(baseUrl: 'http://localhost:8080'))
      ..httpClientAdapter = StubAdapter(handle);

// Widget tests run in FakeAsync. Resolve typed fixtures before transport so
// browser stream scheduling does not interfere with frame-based assertions.
Dio widgetDio(
  Future<(int, Map<String, dynamic>)> Function(RequestOptions) handle,
) {
  final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8080'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) async {
        final (status, data) = await handle(options);
        handler.resolve(
          Response(requestOptions: options, statusCode: status, data: data),
        );
      },
    ),
  );
  return dio;
}
