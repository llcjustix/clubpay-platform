import 'dart:convert';
import 'package:dio/dio.dart';
import '../../../core/api_client.dart';
import '../domain/club_balance.dart';

class ProfileRepository {
  ProfileRepository(this.api, {this.playerId});
  final ApiClient api;
  final String? playerId;
  String get _cacheKey =>
      'mobile.balances.${Uri.encodeComponent(api.dio.options.baseUrl)}.$playerId';
  Future<List<ClubBalance>> balances() async {
    try {
      final json = await api.get('/api/mobile/balances');
      final rows = json['balances'] as List;
      final parsed = rows
          .map((e) => ClubBalance.fromJson(Map<String, dynamic>.from(e as Map)))
          .toList();
      if (playerId != null) {
        // A cache-write failure must not hide a successfully fetched balance.
        try {
          await api.vault.store.write(_cacheKey, jsonEncode(rows));
        } catch (_) {}
      }
      return parsed;
    } on DioException catch (e) {
      final code = e.response?.statusCode;
      if (playerId == null || (code != null && code < 500)) {
        rethrow;
      }
      final cached = await api.vault.store.read(_cacheKey);
      if (cached == null) rethrow;
      final rows = jsonDecode(cached) as List;
      if (rows.isEmpty) {
        rethrow;
      } // An unavailable service is not an empty balance.
      return rows
          .map(
            (e) => ClubBalance.fromJson(
              Map<String, dynamic>.from(e as Map),
              stale: true,
            ),
          )
          .toList();
    }
  }
}
