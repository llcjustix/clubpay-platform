class Player {
  const Player({
    required this.id,
    required this.phone,
    required this.firstName,
  });
  final String id, phone, firstName;
  String get displayName => firstName.isEmpty ? phone : firstName;
  factory Player.fromJson(Map<String, dynamic> json) => Player(
    id: json['id'] as String,
    phone: json['phone'] as String,
    firstName: json['first_name'] as String? ?? '',
  );
}

class TokenPair {
  const TokenPair(this.access, this.refresh);
  final String access, refresh;
  factory TokenPair.fromJson(Map<String, dynamic> json) => TokenPair(
    json['access_token'] as String,
    json['refresh_token'] as String,
  );
  Map<String, dynamic> toJson() => {
    'access_token': access,
    'refresh_token': refresh,
  };
}

class AuthChallenge {
  const AuthChallenge(
    this.token,
    this.telegramLink,
    this.expiresAt,
  );
  final String token, telegramLink;
  final DateTime expiresAt;
  factory AuthChallenge.fromJson(Map<String, dynamic> json) => AuthChallenge(
    json['challenge'] as String,
    json['telegram_link'] as String,
    DateTime.parse(json['expires_at'] as String),
  );
}

String? normalizeUzPhone(String value) {
  final digits = value.replaceAll(RegExp(r'[^0-9]'), '');
  final phone = digits.length == 9 ? '+998$digits' : '+$digits';
  return RegExp(r'^\+998[0-9]{9}$').hasMatch(phone) ? phone : null;
}
