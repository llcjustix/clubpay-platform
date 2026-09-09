import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/club_theme.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import 'scanner_view.dart';

class QrScreen extends ConsumerStatefulWidget {
  const QrScreen({super.key, this.intent});
  final String? intent;
  @override
  ConsumerState<QrScreen> createState() => _QrScreenState();
}

class _QrScreenState extends ConsumerState<QrScreen> {
  bool _busy = false, _camera = true;
  String? _error;
  String? _lastInput;

  Future<void> _resolve(String value) async {
    if (_busy || !mounted) return;
    setState(() {
      _busy = true;
      _lastInput = value;
      _error = null;
      _camera = false;
    });
    try {
      final pc = await ref.read(qrRepositoryProvider).resolve(value);
      if (!mounted) return;
      ref.read(selectedQrProvider.notifier).select(pc);
      ref.invalidate(balancesProvider);
      // Keep camera unmounted while the computer/payment route is on top.
      await context.push('/computer/${Uri.encodeComponent(pc.token)}');
      if (mounted) setState(() => _camera = true);
    } catch (e) {
      if (mounted) setState(() => _error = errorLabel(context, e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = context.l;
    return Scaffold(
      backgroundColor: ClubColors.background,
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
              child: Row(
                children: [
                  IconButton(
                    tooltip: l.close,
                    onPressed: () =>
                        context.canPop() ? context.pop() : context.go('/home'),
                    style: IconButton.styleFrom(
                      backgroundColor: ClubColors.surface,
                    ),
                    icon: const Icon(CupertinoIcons.xmark, size: 20),
                  ),
                  Expanded(
                    child: Text(
                      l.scan,
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                  ),
                  const SizedBox(width: 48),
                ],
              ),
            ),
            Expanded(
              child: ClipRRect(
                borderRadius: BorderRadius.circular(26),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    if (_camera && !_busy && _error == null)
                      ref.watch(scannerBuilderProvider)(_resolve)
                    else
                      ColoredBox(
                        color: ClubColors.surface,
                        child: Center(
                          child: Padding(
                            padding: const EdgeInsets.all(28),
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                if (_busy) ...[
                                  const CupertinoActivityIndicator(
                                    color: ClubColors.blue,
                                  ),
                                  gap,
                                  Text(l.checkingQr),
                                ] else if (_error != null) ...[
                                  const Icon(
                                    CupertinoIcons.exclamationmark_circle,
                                    size: 38,
                                    color: ClubColors.muted,
                                  ),
                                  gap,
                                  Semantics(
                                    liveRegion: true,
                                    child: Text(
                                      _error!,
                                      textAlign: TextAlign.center,
                                    ),
                                  ),
                                  gap,
                                  if (_lastInput != null)
                                    FilledButton(
                                      key: const ValueKey('retry-qr'),
                                      onPressed: () => _resolve(_lastInput!),
                                      child: Text(l.retryQr),
                                    ),
                                  TextButton(
                                    onPressed: () => setState(() {
                                      _error = null;
                                      _camera = true;
                                    }),
                                    child: Text(l.scanAgain),
                                  ),
                                ],
                              ],
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              ),
            ),
            Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 480),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(24, 22, 24, 18),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        l.pointCamera,
                        textAlign: TextAlign.center,
                        style: Theme.of(context).textTheme.titleMedium,
                      ),
                      const SizedBox(height: 6),
                      Text(
                        l.scanAutomatic,
                        textAlign: TextAlign.center,
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
