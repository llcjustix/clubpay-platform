import 'dart:async';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/club_navigation.dart';
import '../../../core/club_theme.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../domain/club_balance.dart';

class BalanceList extends StatelessWidget {
  const BalanceList({super.key, required this.balances});
  final List<ClubBalance> balances;
  @override
  Widget build(BuildContext context) {
    if (balances.isEmpty) {
      return SettingsGroup(
        children: [
          SettingsRow(
            icon: CupertinoIcons.clock_fill,
            color: ClubColors.orange,
            title: context.l.emptyBalance,
            subtitle: context.l.timeReturnsHere,
          ),
        ],
      );
    }
    return Column(
      children: [
        for (final balance in balances)
          Padding(
            padding: const EdgeInsets.only(bottom: 24),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SettingsGroup(
                  children: [
                    SettingsRow(
                      icon: CupertinoIcons.game_controller_solid,
                      color: ClubColors.purple,
                      title: balance.clubName,
                      subtitle: balance.stale
                          ? context.l.balanceStale
                          : (!balance.online ? context.l.clubOffline : null),
                    ),
                    SettingsRow(
                      icon: CupertinoIcons.clock_fill,
                      color: ClubColors.orange,
                      title: context.l.clubGameBalance,
                      subtitle: context.l.clubBalanceHelp,
                      trailing: Text(moneyLabel(context, balance.balanceUzs)),
                    ),
                  ],
                ),
                if (balance.updatedAt != null)
                  Padding(
                    padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
                    child: Text(
                      context.l.balanceUpdated(
                        '${MaterialLocalizations.of(context).formatShortDate(balance.updatedAt!.toLocal())} ${MaterialLocalizations.of(context).formatTimeOfDay(TimeOfDay.fromDateTime(balance.updatedAt!.toLocal()), alwaysUse24HourFormat: true)}',
                      ),
                      style: const TextStyle(
                        fontSize: 12,
                        color: ClubColors.muted,
                      ),
                    ),
                  ),
              ],
            ),
          ),
      ],
    );
  }
}

class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({super.key, this.profile = false});
  final bool profile;
  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen>
    with WidgetsBindingObserver {
  Timer? _balanceTimer;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _watchBalances();
  }

  void _watchBalances() {
    _balanceTimer?.cancel();
    _balanceTimer = Timer.periodic(const Duration(seconds: 20), (_) {
      if (mounted) ref.invalidate(balancesProvider);
    });
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      ref.invalidate(balancesProvider);
      _watchBalances();
    } else {
      _balanceTimer?.cancel();
    }
  }

  @override
  void dispose() {
    _balanceTimer?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  bool _loggingOut = false;
  late final _pending = ref.read(paymentRepositoryProvider).pending();

  @override
  Widget build(BuildContext context) {
    final l = context.l;
    final player = ref.watch(authProvider).asData?.value;
    if (player == null) {
      return const Scaffold(body: Center(child: CupertinoActivityIndicator()));
    }
    final balances = ref.watch(balancesProvider);
    final name = player.firstName.isEmpty ? l.yourAccount : player.firstName;
    return AppPage(
      title: widget.profile ? l.profile : l.appName,
      largeTitle: !widget.profile,
      bottom: ClubNavigation(profile: widget.profile),
      children: [
        if (widget.profile) ...[
          const SizedBox(height: 12),
          const Center(child: _AccountAvatar(size: 96)),
          const SizedBox(height: 16),
          Text(
            name,
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.headlineMedium,
          ),
          const SizedBox(height: 5),
          Text(
            player.phone,
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 17, color: ClubColors.muted),
          ),
          const SizedBox(height: 36),
          SettingsGroup(
            children: [
              SettingsRow(
                icon: CupertinoIcons.globe,
                color: ClubColors.blue,
                title: l.language,
                trailing: Text(
                  ref.watch(localeProvider).languageCode == 'ru'
                      ? l.russian
                      : l.uzbek,
                ),
                onTap: () => _showLanguage(context),
              ),
              SettingsRow(
                icon: CupertinoIcons.clock_fill,
                color: ClubColors.orange,
                title: l.gameTime,
                onTap: () => context.go('/home'),
              ),
            ],
          ),
          const SizedBox(height: 32),
          SettingsGroup(
            children: [
              SettingsRow(
                icon: CupertinoIcons.info,
                color: const Color(0xff8e8e93),
                title: l.aboutApp,
                trailing: Text(l.appName),
              ),
            ],
          ),
          const SizedBox(height: 32),
          SettingsGroup(
            inset: 16,
            children: [
              SizedBox(
                width: double.infinity,
                child: CupertinoButton(
                  onPressed: _loggingOut ? null : _logout,
                  child: _loggingOut
                      ? const CupertinoActivityIndicator()
                      : Text(
                          l.logout,
                          style: const TextStyle(
                            color: ClubColors.red,
                            fontSize: 17,
                          ),
                        ),
                ),
              ),
            ],
          ),
        ] else ...[
          SettingsGroup(
            children: [
              SettingsRow(
                icon: CupertinoIcons.question,
                color: ClubColors.blue,
                title: l.howItWorks,
                onTap: () => showClubGuide(context),
              ),
            ],
          ),
          FutureBuilder(
            future: _pending,
            builder: (context, snapshot) => snapshot.data == null
                ? const SizedBox.shrink()
                : Padding(
                    padding: const EdgeInsets.only(top: 24),
                    child: SettingsGroup(
                      children: [
                        SettingsRow(
                          icon: CupertinoIcons.play_fill,
                          color: ClubColors.green,
                          title: l.resumePayment,
                          onTap: () => context.push('/session'),
                        ),
                      ],
                    ),
                  ),
          ),
          const SizedBox(height: 32),
          Row(
            children: [
              Expanded(child: SectionCaption(l.myClubs)),
              CupertinoButton(
                padding: const EdgeInsets.only(right: 16, bottom: 8),
                minimumSize: const Size(44, 32),
                onPressed: () => ref.invalidate(balancesProvider),
                child: Icon(
                  CupertinoIcons.arrow_clockwise,
                  size: 17,
                  semanticLabel: l.refresh,
                ),
              ),
            ],
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
            child: Text(
              l.clubBalanceHomeHelp,
              style: const TextStyle(fontSize: 13, color: ClubColors.muted),
            ),
          ),
          balances.when(
            data: (items) => BalanceList(balances: items),
            loading: () => const Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: CupertinoActivityIndicator()),
            ),
            error: (e, _) => SettingsGroup(
              children: [
                SettingsRow(
                  icon: CupertinoIcons.exclamationmark_triangle,
                  color: ClubColors.orange,
                  title: l.balanceUnknown,
                  onTap: () => ref.invalidate(balancesProvider),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Future<void> _logout() async {
    setState(() => _loggingOut = true);
    try {
      await ref.read(authProvider.notifier).logout();
    } catch (e) {
      if (mounted) showFailure(context, e);
    } finally {
      if (mounted) setState(() => _loggingOut = false);
    }
  }

  Future<void> _showLanguage(BuildContext context) =>
      showCupertinoModalPopup<void>(
        context: context,
        builder: (sheetContext) => CupertinoActionSheet(
          title: Text(context.l.language),
          actions: [
            for (final locale in ['ru', 'uz'])
              CupertinoActionSheetAction(
                onPressed: () async {
                  Navigator.pop(sheetContext);
                  try {
                    await ref.read(localeProvider.notifier).select(locale);
                  } catch (e) {
                    if (context.mounted) showFailure(context, e);
                  }
                },
                child: Text(
                  locale == 'ru' ? context.l.russian : context.l.uzbek,
                ),
              ),
          ],
          cancelButton: CupertinoActionSheetAction(
            onPressed: () => Navigator.pop(sheetContext),
            child: Text(context.l.close),
          ),
        ),
      );
}

class _AccountAvatar extends StatelessWidget {
  const _AccountAvatar({required this.size});
  final double size;
  @override
  Widget build(BuildContext context) => Container(
    width: size,
    height: size,
    decoration: const BoxDecoration(
      shape: BoxShape.circle,
      color: Color(0xff636366),
    ),
    child: Icon(
      CupertinoIcons.person_fill,
      size: size * .58,
      color: Colors.white,
    ),
  );
}

Future<void> showClubGuide(BuildContext context) => showModalBottomSheet<void>(
  context: context,
  isScrollControlled: true,
  useSafeArea: true,
  builder: (context) => SingleChildScrollView(
    child: Padding(
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 24),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            context.l.howItWorks,
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 24),
          for (final step in [
            (
              CupertinoIcons.qrcode,
              ClubColors.blue,
              context.l.guideScan,
              context.l.guideScanHelp,
            ),
            (
              CupertinoIcons.timer,
              ClubColors.orange,
              context.l.guideChoose,
              context.l.guideChooseHelp,
            ),
            (
              CupertinoIcons.game_controller_solid,
              ClubColors.green,
              context.l.guidePlay,
              context.l.guidePlayHelp,
            ),
          ])
            Padding(
              padding: const EdgeInsets.only(bottom: 24),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SettingsIcon(step.$1, color: step.$2),
                  const SizedBox(width: 15),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          step.$3,
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        const SizedBox(height: 5),
                        Text(
                          step.$4,
                          style: const TextStyle(
                            fontSize: 15,
                            height: 1.35,
                            color: ClubColors.muted,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          SizedBox(
            width: double.infinity,
            child: FilledButton(
              onPressed: () => Navigator.pop(context),
              child: Text(context.l.gotIt),
            ),
          ),
        ],
      ),
    ),
  ),
);
