class ZoneBalance {
  const ZoneBalance(this.id, this.name, this.seconds, this.hourlyPriceTiyin);
  final String id, name;
  final int seconds, hourlyPriceTiyin;
  factory ZoneBalance.fromJson(Map<String, dynamic> j) => ZoneBalance(
    j['id'] as String,
    j['name'] as String,
    (j['seconds_available'] as num).toInt(),
    (j['hourly_price_tiyin'] as num).toInt(),
  );
}

class ClubBalance {
  const ClubBalance(
    this.clubId,
    this.clubName,
    this.seconds, {
    this.zones = const [],
    this.updatedAt,
    this.online = true,
    this.stale = false,
  });
  final String clubId, clubName;
  final int seconds;
  final List<ZoneBalance> zones;
  final DateTime? updatedAt;
  final bool online, stale;
  int secondsForZone(String name) =>
      zones.where((z) => z.name == name).firstOrNull?.seconds ??
      (zones.isEmpty ? seconds : 0);
  factory ClubBalance.fromJson(
    Map<String, dynamic> json, {
    bool stale = false,
  }) => ClubBalance(
    json['club_id'] as String,
    json['club_name'] as String,
    (json['seconds_balance'] as num).toInt(),
    zones: ((json['zones'] as List?) ?? [])
        .map((z) => ZoneBalance.fromJson(Map<String, dynamic>.from(z)))
        .toList(),
    updatedAt: DateTime.tryParse(json['updated_at'] as String? ?? ''),
    online: json['club_online'] != false,
    stale: stale,
  );
}

class TimeLedgerEntry {
  const TimeLedgerEntry({
    required this.id,
    required this.clubId,
    required this.secondsDelta,
    required this.kind,
    required this.createdAt,
  });
  final String id, clubId, kind;
  final int secondsDelta;
  final DateTime createdAt;
}
