enum SessionPhase {
  waitingPayment,
  starting,
  active,
  ended,
  paymentFailed,
  startFailed,
}

class SessionStatus {
  const SessionStatus(
    this.phase,
    this.pcLabel,
    this.seconds, {
    this.checkoutUrl,
    this.isExtension = false,
    this.pcId,
    this.clubId,
  });
  final SessionPhase phase;
  final String pcLabel;
  final int seconds;
  final String? checkoutUrl;
  final bool isExtension;
  final String? pcId, clubId;
  factory SessionStatus.fromJson(Map<String, dynamic> j) {
    final payment = j['status'] as String?;
    final grant = j['grant_status'] as String?;
    final SessionPhase phase;
    if ([
      'failed',
      'refunded',
      'cancelled',
      'canceled',
      'expired',
    ].contains(payment)) {
      phase = SessionPhase.paymentFailed;
    } else if (['start_failed', 'failed', 'rejected'].contains(grant)) {
      phase = SessionPhase.startFailed;
    } else if (['accepted', 'extended'].contains(grant)) {
      phase = SessionPhase.active;
    } else if (['ended', 'completed', 'expired'].contains(grant)) {
      phase = SessionPhase.ended;
    } else if (payment != null && payment != 'paid') {
      phase = SessionPhase.waitingPayment;
    } else {
      phase = SessionPhase.starting;
    }
    return SessionStatus(
      phase,
      j['pc_label'] as String? ?? '',
      (j['session_seconds'] as num? ?? j['duration_seconds'] as num? ?? 0)
          .toInt(),
      checkoutUrl: j['checkout_url'] as String?,
      isExtension: j['is_extension'] == true,
      pcId: j['pc_id'] as String?,
      clubId: j['club_id'] as String?,
    );
  }
  bool get terminal => [
    SessionPhase.active,
    SessionPhase.ended,
    SessionPhase.paymentFailed,
    SessionPhase.startFailed,
  ].contains(phase);
}

class PendingOperation {
  const PendingOperation({
    required this.key,
    this.invoice,
    this.grant,
    this.path,
    this.body,
    this.isExtension = false,
    this.pcId,
    this.pcLabel,
    this.clubId,
  });
  final String key;
  final String? invoice, grant, path;
  final Map<String, dynamic>? body;
  final bool isExtension;
  final String? pcId, pcLabel, clubId;

  factory PendingOperation.request({
    String key = '',
    required String token,
    String? tariff,
    int? amount,
    String? provider,
    bool redeem = false,
    bool isExtension = false,
    String? pcId,
    String? pcLabel,
    String? clubId,
  }) => PendingOperation(
    key: key,
    path: redeem ? '/api/player-balance/redeem' : '/api/checkouts',
    body: {
      'qr_token': token,
      if (!redeem) 'payment_provider': provider,
      if (!redeem && amount != null) 'amount_uzs': amount,
      if (!redeem && amount == null) 'tariff_block_id': tariff,
    },
    isExtension: isExtension,
    pcId: pcId,
    pcLabel: pcLabel,
    clubId: clubId,
  );

  bool matchesRequest(PendingOperation other) =>
      path != null &&
      path == other.path &&
      body != null &&
      other.body != null &&
      isExtension == other.isExtension &&
      body!.length == other.body!.length &&
      body!.entries.every((entry) => other.body![entry.key] == entry.value);

  PendingOperation withResult({
    String? invoice,
    String? grant,
    bool? isExtension,
  }) => PendingOperation(
    key: key,
    invoice: invoice ?? this.invoice,
    grant: grant ?? this.grant,
    path: path,
    body: body,
    isExtension: isExtension ?? this.isExtension,
    pcId: pcId,
    pcLabel: pcLabel,
    clubId: clubId,
  );
  Map<String, dynamic> toJson() => {
    'key': key,
    'invoice': invoice,
    'grant': grant,
    'path': path,
    'body': body,
    'is_extension': isExtension,
    'pc_id': pcId,
    'pc_label': pcLabel,
    'club_id': clubId,
  };
  factory PendingOperation.fromJson(Map<String, dynamic> j) => PendingOperation(
    key: j['key'] as String,
    invoice: j['invoice'] as String?,
    grant: j['grant'] as String?,
    path: j['path'] as String?,
    body: j['body'] == null
        ? null
        : Map<String, dynamic>.from(j['body'] as Map),
    isExtension: j['is_extension'] == true,
    pcId: j['pc_id'] as String?,
    pcLabel: j['pc_label'] as String?,
    clubId: j['club_id'] as String?,
  );
}
