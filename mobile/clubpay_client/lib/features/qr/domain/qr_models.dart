String? parseQrToken(String input) {
  final value = input.trim();
  if (value.isEmpty || value.length > 2048) return null;
  String? token;
  final uri = Uri.tryParse(value);
  if (uri != null && uri.hasScheme) {
    if (!['https', 'http'].contains(uri.scheme)) return null;
    // Extract only a token. Never navigate to, or make requests to, the QR host.
    if (uri.host == 't.me' &&
        uri.pathSegments.length == 1 &&
        uri.queryParameters.containsKey('startapp')) {
      token = uri.queryParameters['startapp'];
    }
    if (uri.pathSegments.length == 2 && uri.pathSegments.first == 'qr') {
      token = uri.pathSegments.last;
    }
  } else {
    token = value;
  }
  return token != null && RegExp(r'^[A-Za-z0-9_-]{4,256}$').hasMatch(token)
      ? token
      : null;
}

class Tariff {
  const Tariff(this.id, this.name, this.seconds, this.price);
  final String id, name;
  final int seconds, price;
  factory Tariff.fromJson(Map<String, dynamic> j) => Tariff(
    j['id'] as String,
    j['name'] as String,
    (j['duration_minutes'] as num).toInt() * 60,
    (j['price_uzs'] as num).toInt(),
  );
}

class PaymentProviderOption {
  const PaymentProviderOption(this.id, this.available);
  final String id;
  final bool available;
}

class QrComputer {
  const QrComputer({
    required this.token,
    required this.clubId,
    required this.clubName,
    required this.label,
    required this.status,
    required this.zone,
    required this.hourlyPrice,
    required this.type,
    required this.tariffs,
    required this.providers,
    this.previewOnly = false,
  });
  final String token, clubId, clubName, label, status, zone, type;
  final int hourlyPrice;
  final List<Tariff> tariffs;
  final List<PaymentProviderOption> providers;
  final bool previewOnly;
  bool get canStart =>
      !previewOnly &&
      (['available', 'sleeping'].contains(status) ||
          (['occupied', 'frozen'].contains(status) &&
              type == 'session_extend'));
  factory QrComputer.fromJson(String token, Map<String, dynamic> j) =>
      QrComputer(
        token: token,
        clubId: j['club']['id'] as String,
        clubName: j['club']['name'] as String,
        label: j['pc']['label'] as String,
        status: j['pc']['status'] as String,
        zone: j['zone']['name'] as String,
        hourlyPrice: (j['zone']['hourly_price_uzs'] as num).toInt(),
        type: j['qr_type'] as String,
        previewOnly: j['development_catalog_preview'] == true,
        tariffs: ((j['tariffs'] as List?) ?? [])
            .map((e) => Tariff.fromJson(Map<String, dynamic>.from(e as Map)))
            .toList(),
        providers: ((j['payment_providers'] as List?) ?? [])
            .where((e) => ['click', 'payme', 'mock'].contains(e['provider']))
            .map(
              (e) => PaymentProviderOption(
                e['provider'] as String,
                e['configured'] == true,
              ),
            )
            .toList(),
      );
}
