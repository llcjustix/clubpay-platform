import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/club_theme.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../../catalog/data/club_catalog_repository.dart';
import '../../catalog/domain/club_catalog.dart';

class ActiveSessionScreen extends ConsumerStatefulWidget {
  const ActiveSessionScreen({super.key, required this.session});
  final MobileActiveSession session;

  @override
  ConsumerState<ActiveSessionScreen> createState() =>
      _ActiveSessionScreenState();
}

class _ActiveSessionScreenState extends ConsumerState<ActiveSessionScreen> {
  bool _ending = false;
  bool _refreshing = false;
  late MobileActiveSession _session;
  Timer? _clock;
  Timer? _serverRefresh;

  @override
  void initState() {
    super.initState();
    _session = widget.session;
    _clock = Timer.periodic(const Duration(seconds: 1), (_) {
      if (mounted) setState(() {});
    });
    _serverRefresh = Timer.periodic(
      const Duration(seconds: 3),
      (_) => unawaited(_refreshSession()),
    );
  }

  @override
  void dispose() {
    _clock?.cancel();
    _serverRefresh?.cancel();
    super.dispose();
  }

  Future<void> _refreshSession() async {
    if (_refreshing) return;
    _refreshing = true;
    try {
      final session = await ClubCatalogRepository(
        ref.read(apiProvider),
      ).activeSession();
      if (!mounted) return;
      if (session == null) {
        context.go('/home');
        return;
      }
      setState(() => _session = session);
    } catch (_) {
      // The on-screen countdown remains useful while a periodic sync retries.
    } finally {
      _refreshing = false;
    }
  }

  Future<void> _endSession() async {
    final approved = await showCupertinoDialog<bool>(
      context: context,
      builder: (dialog) => CupertinoAlertDialog(
        title: Text(context.l.endSessionConfirmTitle),
        content: Text(context.l.endSessionConfirmBody),
        actions: [
          CupertinoDialogAction(
            onPressed: () => Navigator.pop(dialog, false),
            child: Text(context.l.cancel),
          ),
          CupertinoDialogAction(
            isDestructiveAction: true,
            onPressed: () => Navigator.pop(dialog, true),
            child: Text(context.l.end),
          ),
        ],
      ),
    );
    if (approved != true || _ending) return;
    setState(() => _ending = true);
    try {
      await ClubCatalogRepository(ref.read(apiProvider)).endActiveSession();
      ref
          .read(analyticsProvider)
          .track('session_ended_from_mobile', screen: 'active_session');
      ref.read(catalogRevisionProvider.notifier).bump();
      ref.invalidate(balancesProvider);
      if (mounted) context.go('/home');
    } catch (error) {
      if (mounted) showFailure(context, error);
    } finally {
      if (mounted) setState(() => _ending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final session = _session;
    return AppPage(
      title: context.l.activeSession,
      children: [
        InfoCard(
          accent: true,
          children: [
            const Icon(
              CupertinoIcons.play_circle_fill,
              size: 46,
              color: ClubColors.green,
            ),
            const SizedBox(height: 16),
            Text(
              session.clubName,
              style: Theme.of(context).textTheme.titleLarge,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 6),
            Text(
              '${session.pcLabel} · ${session.zoneName}',
              style: const TextStyle(color: ClubColors.muted),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 20),
            Text(
              timeLabel(context, session.currentRemainingSeconds),
              style: Theme.of(context).textTheme.headlineMedium,
            ),
            const SizedBox(height: 4),
            Text(
              context.l.remaining,
              style: const TextStyle(color: ClubColors.muted),
            ),
          ],
        ),
        const SizedBox(height: 24),
        ActionButton(
          label: context.l.extendSession,
          icon: CupertinoIcons.add_circled,
          onPressed: session.extendToken.isEmpty
              ? null
              : () {
                  ref
                      .read(analyticsProvider)
                      .track(
                        'session_extension_opened',
                        screen: 'active_session',
                      );
                  context.push('/computer/${session.extendToken}?mode=extend');
                },
        ),
        const SizedBox(height: 10),
        ActionButton(
          label: context.l.endSession,
          icon: CupertinoIcons.stop_circle,
          secondary: true,
          busy: _ending,
          onPressed: _ending ? null : _endSession,
        ),
      ],
    );
  }
}
