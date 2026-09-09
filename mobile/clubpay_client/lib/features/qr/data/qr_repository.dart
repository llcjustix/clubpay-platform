import '../../../core/api_client.dart';
import '../domain/qr_models.dart';

class QrRepository {
  QrRepository(this.api);
  final ApiClient api;
  Future<QrComputer> resolve(String input) async {
    final token = parseQrToken(input);
    if (token == null) throw const FormatException('invalid_qr');
    return QrComputer.fromJson(
      token,
      await api.request('GET', '/api/qr/${Uri.encodeComponent(token)}'),
    );
  }
}
