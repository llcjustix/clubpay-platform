import '../../../core/api_client.dart';
import 'package:uuid/uuid.dart';
import '../domain/club_catalog.dart';

class ClubCatalogRepository {
  ClubCatalogRepository(this.api);
  final ApiClient api;

  Future<List<ClubSearchResult>> search(String query) async {
    final suffix = query.trim().isEmpty
        ? ''
        : '?q=${Uri.encodeQueryComponent(query.trim())}';
    final data = await api.get('/api/mobile/clubs$suffix');
    return ((data['clubs'] as List?) ?? [])
        .map(
          (item) =>
              ClubSearchResult.fromJson(Map<String, dynamic>.from(item as Map)),
        )
        .toList();
  }

  Future<String> mapAPIKey() async {
    final data = await api.get('/api/mobile/map-config');
    return (data['yandex_maps_api_key'] as String? ?? '').trim();
  }

  Future<ClubCatalog> club(String id) async => ClubCatalog.fromJson(
    await api.get('/api/mobile/clubs/${Uri.encodeComponent(id)}'),
  );

  Future<List<ClubSearchResult>> favorites() async {
    final data = await api.get('/api/mobile/favorites');
    return ((data['clubs'] as List?) ?? [])
        .map(
          (item) =>
              ClubSearchResult.fromJson(Map<String, dynamic>.from(item as Map)),
        )
        .toList();
  }

  Future<bool> toggleFavorite(String clubID, {required bool favorite}) async {
    final suffix = favorite ? 'remove' : '';
    final path =
        '/api/mobile/favorites/${Uri.encodeComponent(clubID)}${suffix.isEmpty ? '' : '/$suffix'}';
    final data = await api.post(
      path,
      {},
      key: const Uuid().v4().replaceAll('-', ''),
    );
    return data['favorite'] == true;
  }

  Future<void> wake(String pcID) async {
    await api.post('/api/mobile/pcs/${Uri.encodeComponent(pcID)}/wake', {});
  }

  Future<MobileActiveSession?> activeSession() async {
    final data = await api.get('/api/mobile/active-session');
    final session = data['session'];
    if (session is! Map) return null;
    return MobileActiveSession.fromJson(Map<String, dynamic>.from(session));
  }

  Future<void> endActiveSession() => api.post(
    '/api/mobile/active-session/end',
    const {},
    key: const Uuid().v4().replaceAll('-', ''),
  );

  Future<List<MobileReservation>> reservations() async {
    final data = await api.get('/api/mobile/reservations');
    return ((data['reservations'] as List?) ?? [])
        .map(
          (item) => MobileReservation.fromJson(
            Map<String, dynamic>.from(item as Map),
          ),
        )
        .toList();
  }

  Future<MobileReservation> createReservation({
    required String pcID,
    required DateTime startsAt,
    required int durationHours,
  }) async {
    final data = await api.post('/api/mobile/reservations', {
      'pc_id': pcID,
      'starts_at': startsAt.toUtc().toIso8601String(),
      'duration_hours': durationHours,
    }, key: const Uuid().v4().replaceAll('-', ''));
    return MobileReservation.fromJson(
      Map<String, dynamic>.from(data['reservation'] as Map),
    );
  }

  Future<String> startReservation({required String id}) async {
    final data = await api.post(
      '/api/mobile/reservations/${Uri.encodeComponent(id)}/start',
      const {},
      key: const Uuid().v4().replaceAll('-', ''),
    );
    return data['pc_token'] as String;
  }

  Future<void> cancelReservation(String id) async {
    await api.post(
      '/api/mobile/reservations/${Uri.encodeComponent(id)}/cancel',
      {},
      key: const Uuid().v4().replaceAll('-', ''),
    );
  }

  Future<MobileReservation> rescheduleReservation({
    required String id,
    required DateTime startsAt,
    required int durationHours,
  }) async {
    final data = await api.post(
      '/api/mobile/reservations/${Uri.encodeComponent(id)}/reschedule',
      {
        'starts_at': startsAt.toUtc().toIso8601String(),
        'duration_hours': durationHours,
      },
      key: const Uuid().v4().replaceAll('-', ''),
    );
    return MobileReservation.fromJson(
      Map<String, dynamic>.from(data['reservation'] as Map),
    );
  }
}
