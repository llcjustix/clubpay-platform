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
    this.favorite = false,
  });
  final String id, name, address;
  final bool online, favorite;
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
        favorite: json['favorite'] == true,
      );
}

class ClubComputer {
  const ClubComputer({
    required this.id,
    required this.label,
    required this.number,
    required this.status,
    required this.token,
    this.reservationStartsAt,
    this.reservationEndsAt,
    this.reservedByMe = false,
  });
  final String id, label, status, token;
  final int number;
  final DateTime? reservationStartsAt, reservationEndsAt;
  final bool reservedByMe;
  bool get selectable =>
      token.isNotEmpty && (status == 'available' || status == 'sleeping');
  bool get wakeable => status == 'sleeping' || status == 'offline';
  factory ClubComputer.fromJson(Map<String, dynamic> json) => ClubComputer(
    id: json['id'] as String,
    label: json['label'] as String,
    number: (json['number'] as num).toInt(),
    status: json['status'] as String,
    token: json['qr_token'] as String? ?? '',
    reservationStartsAt: json['reservation_starts_at'] == null
        ? null
        : DateTime.parse(json['reservation_starts_at'] as String).toLocal(),
    reservationEndsAt: json['reservation_ends_at'] == null
        ? null
        : DateTime.parse(json['reservation_ends_at'] as String).toLocal(),
    reservedByMe: json['reserved_by_me'] == true,
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
    this.favorite = false,
  });
  final String id, name, address;
  final bool online, favorite;
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
      favorite: club['favorite'] == true,
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
    required this.pcID,
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
  final String id, pcID, status, clubName, zoneName, pcLabel;
  final DateTime startsAt, endsAt, heldFrom, checkinDeadline;
  final int durationHours;
  factory MobileReservation.fromJson(Map<String, dynamic> json) =>
      MobileReservation(
        id: json['id'] as String,
        pcID: json['pc_id'] as String? ?? '',
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
