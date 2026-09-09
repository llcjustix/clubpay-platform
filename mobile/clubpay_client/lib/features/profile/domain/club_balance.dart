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
    this.seconds,
    this.balanceUzs, {
    this.zones = const [],
    this.updatedAt,
    this.online = true,
    this.stale = false,
  });
  final String clubId, clubName;
  final int seconds, balanceUzs;
  final List<ZoneBalance> zones;
  final DateTime? updatedAt;
  final bool online, stale;
  int secondsForZone(String name) =>
      zones.where((z) => z.name == name).firstOrNull?.seconds ??
      (zones.isEmpty ? seconds : 0);

  factory ClubBalance.fromJson(
    Map<String, dynamic> json, {
    bool stale = false,
  }) {
    final seconds = (json['seconds_balance'] as num).toInt();
    final zones = ((json['zones'] as List?) ?? [])
        .map((z) => ZoneBalance.fromJson(Map<String, dynamic>.from(z)))
        .toList();
    return ClubBalance(
      json['club_id'] as String,
      json['club_name'] as String,
      seconds,
      (json['balance_uzs'] as num?)?.toInt() ??
          _estimateBalanceUzs(seconds, zones),
      zones: zones,
      updatedAt: DateTime.tryParse(json['updated_at'] as String? ?? ''),
      online: json['club_online'] != false,
      stale: stale,
    );
  }

  static int _estimateBalanceUzs(int seconds, List<ZoneBalance> zones) {
    if (seconds <= 0 || zones.isEmpty) return 0;
    final referencePrice = zones
        .map((zone) => zone.hourlyPriceTiyin)
        .reduce((lowest, price) => price < lowest ? price : lowest);
    return (seconds * referencePrice / 360000).round();
  }
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
