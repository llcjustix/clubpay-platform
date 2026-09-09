import 'package:flutter/material.dart';
import 'package:flutter/cupertino.dart';
import '../../../core/club_theme.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../domain/qr_models.dart';

class ComputerScreen extends ConsumerStatefulWidget {
  const ComputerScreen({super.key, required this.token});
  final String token;
  @override
  ConsumerState<ComputerScreen> createState() => _ComputerScreenState();
}

class _ComputerScreenState extends ConsumerState<ComputerScreen> {
  late Future<QrComputer> _future;
  final _amount = TextEditingController();
  String? _tariff, _provider;
  bool _busy = false;
  @override
  void initState() {
    super.initState();
    _load();
  }

  void _load() {
    _future = ref.read(qrRepositoryProvider).resolve(widget.token);
  }

  @override
  void dispose() {
    _amount.dispose();
    super.dispose();
  }

  Future<void> _submit(QrComputer pc, bool redeem) async {
    if (pc.previewOnly) return;
    final amount = _amount.text.isEmpty ? null : int.tryParse(_amount.text);
    if (!redeem &&
        _amount.text.isNotEmpty &&
        (amount == null || amount <= 0 || amount > 100000000)) {
      showFailure(context, const FormatException('operation_failed'));
      return;
    }
    setState(() => _busy = true);
    final repo = ref.read(paymentRepositoryProvider);
    final provider =
        _provider ?? pc.providers.where((p) => p.available).firstOrNull?.id;
    try {
      await repo.begin(
        token: pc.token,
        tariff: _tariff ?? (pc.tariffs.isEmpty ? null : pc.tariffs.first.id),
        amount: amount,
        provider: provider,
        redeem: redeem,
        testPayment: !redeem && provider == 'mock',
      );
      if (mounted) context.push('/session');
    } catch (e) {
      // Once submission was persisted, only recovery/status is offered.
      final pending = await repo.pending();
      if (mounted) {
        if (pending != null) {
          context.push('/session');
        } else {
          showFailure(context, e);
        }
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = context.l;
    return FutureBuilder(
      future: _future,
      builder: (context, snapshot) {
        if (!snapshot.hasData) {
          return AppPage(
            title: l.pc,
            children: [
              if (snapshot.hasError) ...[
                Text(errorLabel(context, snapshot.error!)),
                ActionButton(
                  label: l.refresh,
                  onPressed: () => setState(_load),
                ),
              ] else
                const Center(child: CircularProgressIndicator()),
            ],
          );
        }
        final pc = snapshot.data!;
        final available = pc.providers.where((p) => p.available).toList();
        final selectedProvider = _provider ?? available.firstOrNull?.id;
        final selectedTariff = _tariff ?? pc.tariffs.firstOrNull?.id;
        final balances = ref.watch(balancesProvider);
        final club = balances.asData?.value
            .where((b) => b.clubId == pc.clubId)
            .firstOrNull;
        final seconds = club?.secondsForZone(pc.zone) ?? 0;
        final balanceUsable = club != null && club.online && !club.stale;
        String providerName(String id) => switch (id) {
          'click' => l.click,
          'payme' => l.payme,
          _ => l.mock,
        };
        final status = switch (pc.status) {
          'available' => l.available,
          'sleeping' => l.sleeping,
          'occupied' || 'frozen' => l.occupied,
          _ => l.maintenance,
        };
        return AppPage(
          title: pc.clubName,
          leading: Padding(
            padding: const EdgeInsets.all(5),
            child: IconButton.filledTonal(
              tooltip: l.home,
              style: IconButton.styleFrom(backgroundColor: ClubColors.surface),
              onPressed: () =>
                  context.canPop() ? context.pop() : context.go('/home'),
              icon: const Icon(CupertinoIcons.chevron_back, size: 23),
            ),
          ),
          children: [
            const Center(
              child: Icon(
                CupertinoIcons.desktopcomputer,
                size: 64,
                color: ClubColors.muted,
              ),
            ),
            const SizedBox(height: 16),
            Text(
              pc.label,
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.headlineMedium,
            ),
            const SizedBox(height: 6),
            Text(
              pc.zone,
              textAlign: TextAlign.center,
              style: const TextStyle(color: ClubColors.muted),
            ),
            const SizedBox(height: 28),
            SettingsGroup(
              children: [
                SettingsRow(
                  icon: CupertinoIcons.desktopcomputer,
                  color: pc.status == 'available'
                      ? ClubColors.green
                      : ClubColors.orange,
                  title: status,
                ),
                if (pc.previewOnly)
                  SettingsRow(
                    icon: CupertinoIcons.info,
                    color: const Color(0xff8e8e93),
                    title: l.liveCatalogPreview,
                    onTap: () => showCupertinoDialog<void>(
                      context: context,
                      builder: (dialogContext) => CupertinoAlertDialog(
                        title: Text(l.liveCatalogPreview),
                        content: Text(l.liveCatalogPreviewHelp),
                        actions: [
                          CupertinoDialogAction(
                            onPressed: () => Navigator.pop(dialogContext),
                            child: Text(l.gotIt),
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(height: 32),
            SectionCaption(pc.previewOnly ? l.gameTime : l.packages),
            SettingsGroup(
              inset: 16,
              children: [
                for (final t in pc.tariffs)
                  SettingsRow(
                    title: t.name,
                    trailing: Text(l.price(t.price)),
                    selected:
                        !pc.previewOnly &&
                        selectedTariff == t.id &&
                        _amount.text.isEmpty,
                    onTap: _busy || pc.previewOnly
                        ? null
                        : () => setState(() {
                            _tariff = t.id;
                            _amount.clear();
                          }),
                  ),
              ],
            ),
            if (pc.hourlyPrice > 0 && !pc.previewOnly) ...[
              gap,
              TextField(
                controller: _amount,
                keyboardType: TextInputType.number,
                inputFormatters: [
                  FilteringTextInputFormatter.digitsOnly,
                  LengthLimitingTextInputFormatter(9),
                ],
                decoration: InputDecoration(labelText: l.customAmount),
                onChanged: (_) => setState(() {}),
              ),
              gap,
              Text(l.customHelp),
            ],
            if (!pc.previewOnly) ...[
              const SizedBox(height: 28),
              SectionCaption(l.providers),
              gap,
              if (available.isEmpty)
                Text(l.noProviders)
              else
                SettingsGroup(
                  inset: 16,
                  children: [
                    for (final p in available)
                      SettingsRow(
                        title: providerName(p.id),
                        selected: selectedProvider == p.id,
                        onTap: _busy
                            ? null
                            : () => setState(() => _provider = p.id),
                      ),
                  ],
                ),
              ActionButton(
                label: selectedProvider == 'mock' ? l.testPayAndStart : l.pay,
                icon: selectedProvider == 'mock'
                    ? Icons.play_arrow
                    : Icons.open_in_new,
                busy: _busy,
                onPressed:
                    pc.canStart &&
                        selectedProvider != null &&
                        (selectedTariff != null || _amount.text.isNotEmpty)
                    ? () => _submit(pc, false)
                    : null,
              ),
              const SizedBox(height: 28),
              InfoCard(
                accent: true,
                children: [
                  Text(
                    l.balanceInZone(pc.zone),
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  gap,
                  balances.when(
                    data: (_) => Text(
                      seconds <= 0
                          ? l.zeroMinutes
                          : timeLabel(context, seconds),
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                    loading: () => Text(l.balancePending),
                    error: (e, _) => Text(errorLabel(context, e)),
                  ),
                  if (club?.stale == true || club?.online == false) ...[
                    const SizedBox(height: 8),
                    Text(
                      club?.stale == true ? l.balanceStale : l.clubOffline,
                      style: const TextStyle(
                        color: ClubColors.muted,
                        fontSize: 13,
                      ),
                    ),
                  ],
                  const SizedBox(height: 8),
                  Text(
                    l.zoneConversionHelp,
                    style: const TextStyle(
                      color: ClubColors.muted,
                      fontSize: 13,
                    ),
                  ),
                  if (seconds > 0)
                    ActionButton(
                      label: l.useBalance,
                      icon: Icons.play_arrow,
                      onPressed: pc.canStart && balanceUsable && !_busy
                          ? () => _submit(pc, true)
                          : null,
                      secondary: true,
                    ),
                ],
              ),
            ],
          ],
        );
      },
    );
  }
}
