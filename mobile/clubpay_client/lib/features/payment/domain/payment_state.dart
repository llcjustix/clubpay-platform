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
  });
  final SessionPhase phase;
  final String pcLabel;
  final int seconds;
  final String? checkoutUrl;
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
    } else if (grant == 'accepted') {
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
  });
  final String key;
  final String? invoice, grant, path;
  final Map<String, dynamic>? body;
  Map<String, dynamic> toJson() => {
    'key': key,
    'invoice': invoice,
    'grant': grant,
    'path': path,
    'body': body,
  };
  factory PendingOperation.fromJson(Map<String, dynamic> j) => PendingOperation(
    key: j['key'] as String,
    invoice: j['invoice'] as String?,
    grant: j['grant'] as String?,
    path: j['path'] as String?,
    body: j['body'] == null
        ? null
        : Map<String, dynamic>.from(j['body'] as Map),
  );
}
