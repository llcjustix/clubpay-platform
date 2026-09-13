import 'api_client.dart';

/// Best-effort product counters. A failed counter must never block gameplay.
class MobileAnalytics {
  MobileAnalytics(this._api);
  final ApiClient _api;

  Future<void> track(String eventName, {String screen = ''}) async {
    try {
      await _api.post('/api/mobile/events', {
        'event_name': eventName,
        'screen': screen,
      });
    } catch (_) {}
  }
}
