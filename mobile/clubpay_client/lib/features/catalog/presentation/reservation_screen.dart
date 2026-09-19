import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:go_router/go_router.dart';

import '../../../core/club_theme.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../domain/club_catalog.dart';
import 'club_browser_screen.dart' show clubCatalogRepositoryProvider;

class ReservationScreen extends ConsumerStatefulWidget {
  const ReservationScreen({
    super.key,
    required this.pcID,
    required this.clubName,
    required this.zoneName,
    required this.pcLabel,
    this.reservation,
  });
  final String pcID, clubName, zoneName, pcLabel;
  final MobileReservation? reservation;
  @override
  ConsumerState<ReservationScreen> createState() => _ReservationScreenState();
}

class _ReservationScreenState extends ConsumerState<ReservationScreen> {
  late DateTime _startsAt;
  late final TextEditingController _hours;
  bool _saving = false;
  MobileReservation? _created;

  bool get _editing => widget.reservation != null;

  @override
  void initState() {
    super.initState();
    final reservation = widget.reservation;
    final now = DateTime.now().add(const Duration(minutes: 15));
    _startsAt =
        reservation?.startsAt ??
        DateTime(now.year, now.month, now.day, now.hour + 1);
    _hours = TextEditingController(text: '${reservation?.durationHours ?? 2}');
  }

  @override
  void dispose() {
    _hours.dispose();
    super.dispose();
  }

  String _dateTime(DateTime value) => DateFormat(
    'd MMMM, HH:mm',
    Localizations.localeOf(context).toLanguageTag(),
  ).format(value);

  Future<void> _chooseStart() async {
    final date = await showDatePicker(
      context: context,
      initialDate: _startsAt,
      firstDate: DateTime.now().add(const Duration(minutes: 15)),
      lastDate: DateTime.now().add(const Duration(days: 30)),
    );
    if (date == null || !mounted) return;
    final time = await showTimePicker(
      context: context,
      initialTime: TimeOfDay.fromDateTime(_startsAt),
    );
    if (time == null || !mounted) return;
    setState(
      () => _startsAt = DateTime(
        date.year,
        date.month,
        date.day,
        time.hour,
        time.minute,
      ),
    );
  }

  Future<void> _save() async {
    final hours = int.tryParse(_hours.text.trim());
    if (hours == null || hours < 1 || hours > 24) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(context.l.reservationInvalidHours)),
      );
      return;
    }
    if (_startsAt.isBefore(DateTime.now().add(const Duration(minutes: 15)))) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(context.l.reservationMinimumLead)));
      return;
    }
    setState(() => _saving = true);
    try {
      final repository = ref.read(clubCatalogRepositoryProvider);
      final reservation = _editing
          ? await repository.rescheduleReservation(
              id: widget.reservation!.id,
              startsAt: _startsAt,
              durationHours: hours,
            )
          : await repository.createReservation(
              pcID: widget.pcID,
              startsAt: _startsAt,
              durationHours: hours,
            );
      ref
          .read(analyticsProvider)
          .track(
            _editing ? 'reservation_rescheduled' : 'reservation_created',
            screen: 'reservation',
          );
      ref.read(catalogRevisionProvider.notifier).bump();
      if (mounted) setState(() => _created = reservation);
    } catch (error) {
      if (mounted) showFailure(context, error);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) => AppPage(
    title: _created != null
        ? (_editing
              ? context.l.reservationRescheduled
              : context.l.reservationCreated)
        : (_editing ? context.l.rescheduleReservation : context.l.reservePc),
    children: [
      if (_created != null)
        _confirmation(_created!)
      else ...[
        InfoCard(
          children: [
            Text(
              widget.clubName,
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 6),
            Text(
              '${widget.zoneName} · ${widget.pcLabel}',
              style: const TextStyle(color: ClubColors.muted),
            ),
          ],
        ),
        SectionCaption(context.l.reservationChooseStart),
        SettingsGroup(
          children: [
            SettingsRow(
              icon: CupertinoIcons.calendar,
              color: ClubColors.blue,
              title: _dateTime(_startsAt),
              subtitle: context.l.reservationHeldIn,
              onTap: _chooseStart,
            ),
          ],
        ),
        const SizedBox(height: 20),
        SectionCaption(context.l.reservationChooseHours),
        TextField(
          controller: _hours,
          keyboardType: TextInputType.number,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          decoration: InputDecoration(
            suffixText: context.l.hours,
            hintText: context.l.reservationHoursExample,
          ),
        ),
        const SizedBox(height: 18),
        InfoCard(
          children: [
            Text(context.l.reservationPaymentInfo),
            SizedBox(height: 10),
            Text(
              context.l.reservationStartInfo,
              style: TextStyle(color: ClubColors.muted),
            ),
          ],
        ),
        ActionButton(
          label: _editing ? context.l.rescheduleReservation : context.l.reserve,
          icon: CupertinoIcons.calendar_badge_plus,
          busy: _saving,
          onPressed: _save,
        ),
      ],
    ],
  );

  Widget _confirmation(MobileReservation item) => InfoCard(
    children: [
      Text(
        '${item.clubName}\n${item.zoneName} · ${item.pcLabel}',
        style: Theme.of(context).textTheme.titleLarge,
      ),
      const SizedBox(height: 18),
      Text(
        '${_dateTime(item.startsAt)} — ${DateFormat('HH:mm').format(item.endsAt)}',
        style: Theme.of(context).textTheme.headlineSmall,
      ),
      Text(
        context.l.hoursOfPlay(item.durationHours),
        style: const TextStyle(color: ClubColors.muted),
      ),
      const SizedBox(height: 18),
      Text(
        context.l.reservationConfirmationInfo(
          DateFormat('HH:mm').format(item.heldFrom),
        ),
      ),
      const SizedBox(height: 18),
      ActionButton(
        label: context.l.toReservation,
        onPressed: () => context.go('/home'),
      ),
    ],
  );
}
