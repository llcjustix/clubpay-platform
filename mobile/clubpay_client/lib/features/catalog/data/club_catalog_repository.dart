import '../../../core/api_client.dart';
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

  Future<ClubCatalog> club(String id) async => ClubCatalog.fromJson(
    await api.get('/api/mobile/clubs/${Uri.encodeComponent(id)}'),
  );
}
