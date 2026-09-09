import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/cupertino.dart';
import '../../../core/club_theme.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../domain/payment_state.dart';

class SessionScreen extends ConsumerStatefulWidget {
  const SessionScreen({super.key, this.invoice});
  final String? invoice;
  @override
  ConsumerState<SessionScreen> createState() => _SessionScreenState();
}

class _SessionScreenState extends ConsumerState<SessionScreen>
    with WidgetsBindingObserver {
  Timer? _timer;
  PendingOperation? _operation;
  SessionStatus? _status;
  String? _error;
  bool _busy = false, _slow = false;
  int _checks = 0;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _check();
  }

  @override
  void dispose() {
    _timer?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) _check(manual: true);
    if (state == AppLifecycleState.paused) _timer?.cancel();
  }

  Future<void> _check({bool manual = false}) async {
    if (_busy) return;
    _timer?.cancel();
    if (manual) {
      _checks = 0;
      _slow = false;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final repo = ref.read(paymentRepositoryProvider);
      var pending =
          _operation ??
          (widget.invoice != null
              ? PendingOperation(key: 'return', invoice: widget.invoice)
              : await repo.pending());
      if (pending == null) {
        if (mounted) context.go('/home');
        return;
      }
      pending = await repo.recover(pending);
      _operation = pending;
      final result = await repo.status(pending);
      if (!mounted) return;
      setState(() => _status = result);
      if (result?.terminal == true) {
        await repo.clear();
        ref.invalidate(balancesProvider);
      } else if (++_checks < 60) {
        _timer = Timer(const Duration(seconds: 3), _check);
      } else {
        setState(() => _slow = true);
      }
    } catch (e) {
      if (mounted) setState(() => _error = errorLabel(context, e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = context.l;
    final status = _status;
    final title = switch (status?.phase) {
      SessionPhase.waitingPayment => l.waitingPayment,
      SessionPhase.starting => l.startingSession,
      SessionPhase.active => l.sessionReady,
      SessionPhase.ended => l.sessionEnded,
      SessionPhase.paymentFailed => l.paymentFailed,
      SessionPhase.startFailed => l.startFailed,
      null => l.operationPending,
    };
    final success = status?.phase == SessionPhase.active;
    return AppPage(
      title: l.payment,
      children: [
        InfoCard(
          accent: true,
          children: [
            Icon(
              success
                  ? CupertinoIcons.check_mark_circled
                  : CupertinoIcons.hourglass,
              size: 56,
              color: ClubColors.blue,
            ),
            const SizedBox(height: 28),
            Semantics(
              liveRegion: true,
              child: Text(
                title,
                style: Theme.of(context).textTheme.headlineMedium,
              ),
            ),
            gap,
            if (status != null) ...[
              Text(
                status.pcLabel,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              gap,
              Text(timeLabel(context, status.seconds)),
            ],
            if (status?.phase == SessionPhase.starting) ...[
              gap,
              Text(l.startingHelp),
            ],
            if (status == null) ...[gap, Text(l.operationHelp)],
          ],
        ),
        if (status?.phase == SessionPhase.waitingPayment) ...[
          Text(l.returnHelp),
          if (status?.checkoutUrl != null)
            ActionButton(
              label: l.openPayment,
              icon: Icons.open_in_new,
              onPressed: () async {
                try {
                  await openExternal(status!.checkoutUrl!);
                } catch (e) {
                  if (context.mounted) showFailure(context, e);
                }
              },
            ),
        ],
        if (_error != null) ...[
          gap,
          Text(
            _error!,
            style: TextStyle(color: Theme.of(context).colorScheme.error),
          ),
        ],
        if (_slow) ...[gap, Text(l.checkSlow)],
        ActionButton(
          label: l.checkStatus,
          icon: Icons.refresh,
          secondary: true,
          busy: _busy,
          onPressed: () => _check(manual: true),
        ),
        ActionButton(
          label: l.home,
          icon: Icons.home_outlined,
          secondary: true,
          onPressed: () => context.go('/home'),
        ),
      ],
    );
  }
}
