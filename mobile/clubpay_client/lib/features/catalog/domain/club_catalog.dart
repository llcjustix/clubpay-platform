class ClubSearchResult {
  const ClubSearchResult({
    required this.id,
    required this.name,
    required this.address,
    required this.online,
    required this.availablePCs,
  });
  final String id, name, address;
  final bool online;
  final int availablePCs;
  factory ClubSearchResult.fromJson(Map<String, dynamic> json) =>
      ClubSearchResult(
        id: json['club_id'] as String,
        name: json['club_name'] as String,
        address: json['address'] as String? ?? '',
        online: json['club_online'] != false,
        availablePCs: (json['available_pcs'] as num?)?.toInt() ?? 0,
      );
}

class ClubComputer {
  const ClubComputer({
    required this.id,
    required this.label,
    required this.number,
    required this.status,
    required this.token,
  });
  final String id, label, status, token;
  final int number;
  bool get selectable =>
      token.isNotEmpty && (status == 'available' || status == 'sleeping');
  factory ClubComputer.fromJson(Map<String, dynamic> json) => ClubComputer(
    id: json['id'] as String,
    label: json['label'] as String,
    number: (json['number'] as num).toInt(),
    status: json['status'] as String,
    token: json['qr_token'] as String? ?? '',
  );
}

class ClubZone {
  const ClubZone({
    required this.id,
    required this.name,
    required this.hourlyPrice,
    required this.computers,
  });
  final String id, name;
  final int hourlyPrice;
  final List<ClubComputer> computers;
  factory ClubZone.fromJson(Map<String, dynamic> json) => ClubZone(
    id: json['zone_id'] as String,
    name: json['zone_name'] as String,
    hourlyPrice: (json['hourly_price_uzs'] as num?)?.toInt() ?? 0,
    computers: ((json['pcs'] as List?) ?? [])
        .map(
          (item) =>
              ClubComputer.fromJson(Map<String, dynamic>.from(item as Map)),
        )
        .toList(),
  );
}

class ClubCatalog {
  const ClubCatalog({
    required this.id,
    required this.name,
    required this.address,
    required this.online,
    required this.zones,
  });
  final String id, name, address;
  final bool online;
  final List<ClubZone> zones;
  factory ClubCatalog.fromJson(Map<String, dynamic> json) {
    final club = Map<String, dynamic>.from(json['club'] as Map);
    return ClubCatalog(
      id: club['club_id'] as String,
      name: club['club_name'] as String,
      address: club['address'] as String? ?? '',
      online: club['club_online'] != false,
      zones: ((club['zones'] as List?) ?? [])
          .map(
            (item) => ClubZone.fromJson(Map<String, dynamic>.from(item as Map)),
          )
          .toList(),
    );
  }
}
