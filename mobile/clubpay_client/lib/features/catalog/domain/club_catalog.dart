class ClubSearchResult {
  const ClubSearchResult({
    required this.id,
    required this.name,
    required this.address,
    required this.online,
    required this.availablePCs,
    required this.totalPCs,
    this.latitude,
    this.longitude,
  });
  final String id, name, address;
  final bool online;
  final int availablePCs, totalPCs;
  final double? latitude, longitude;
  factory ClubSearchResult.fromJson(Map<String, dynamic> json) =>
      ClubSearchResult(
        id: json['club_id'] as String,
        name: json['club_name'] as String,
        address: json['address'] as String? ?? '',
        online: json['club_online'] != false,
        availablePCs: (json['available_pcs'] as num?)?.toInt() ?? 0,
        totalPCs: (json['total_pcs'] as num?)?.toInt() ?? 0,
        latitude: (json['latitude'] as num?)?.toDouble(),
        longitude: (json['longitude'] as num?)?.toDouble(),
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
  bool get wakeable => status == 'sleeping' || status == 'offline';
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
    this.latitude,
    this.longitude,
  });
  final String id, name, address;
  final bool online;
  final List<ClubZone> zones;
  final double? latitude, longitude;
  factory ClubCatalog.fromJson(Map<String, dynamic> json) {
    final club = Map<String, dynamic>.from(json['club'] as Map);
    return ClubCatalog(
      id: club['club_id'] as String,
      name: club['club_name'] as String,
      address: club['address'] as String? ?? '',
      online: club['club_online'] != false,
      latitude: (club['latitude'] as num?)?.toDouble(),
      longitude: (club['longitude'] as num?)?.toDouble(),
      zones: ((club['zones'] as List?) ?? [])
          .map(
            (item) => ClubZone.fromJson(Map<String, dynamic>.from(item as Map)),
          )
          .toList(),
    );
  }
}

class MobileReservation {
  const MobileReservation({
    required this.id,
    required this.status,
    required this.clubName,
    required this.zoneName,
    required this.pcLabel,
    required this.startsAt,
    required this.endsAt,
    required this.heldFrom,
    required this.checkinDeadline,
    required this.durationHours,
  });
  final String id, status, clubName, zoneName, pcLabel;
  final DateTime startsAt, endsAt, heldFrom, checkinDeadline;
  final int durationHours;
  factory MobileReservation.fromJson(Map<String, dynamic> json) =>
      MobileReservation(
        id: json['id'] as String,
        status: json['status'] as String,
        clubName: json['club_name'] as String,
        zoneName: json['zone_name'] as String,
        pcLabel: json['pc_label'] as String,
        startsAt: DateTime.parse(json['starts_at'] as String).toLocal(),
        endsAt: DateTime.parse(json['ends_at'] as String).toLocal(),
        heldFrom: DateTime.parse(json['held_from'] as String).toLocal(),
        checkinDeadline: DateTime.parse(
          json['checkin_deadline'] as String,
        ).toLocal(),
        durationHours: (json['duration_hours'] as num).toInt(),
      );
}
