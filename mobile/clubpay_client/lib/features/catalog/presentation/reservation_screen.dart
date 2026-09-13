import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

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
  });
  final String pcID, clubName, zoneName, pcLabel;
  @override
  ConsumerState<ReservationScreen> createState() => _ReservationScreenState();
}

class _ReservationScreenState extends ConsumerState<ReservationScreen> {
  late DateTime _startsAt;
  final _hours = TextEditingController(text: '2');
  bool _saving = false;
  MobileReservation? _created;

  @override
  void initState() {
    super.initState();
    final now = DateTime.now().add(const Duration(minutes: 30));
    _startsAt = DateTime(now.year, now.month, now.day, now.hour + 1);
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
      firstDate: DateTime.now().add(const Duration(minutes: 30)),
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

  Future<void> _create() async {
    final hours = int.tryParse(_hours.text.trim());
    if (hours == null || hours < 1 || hours > 24) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Введите целое число часов от 1 до 24.')),
      );
      return;
    }
    if (_startsAt.isBefore(DateTime.now().add(const Duration(minutes: 30)))) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Бронь можно оформить минимум за 30 минут.'),
        ),
      );
      return;
    }
    setState(() => _saving = true);
    try {
      final reservation = await ref
          .read(clubCatalogRepositoryProvider)
          .createReservation(
            pcID: widget.pcID,
            startsAt: _startsAt,
            durationHours: hours,
          );
      ref
          .read(analyticsProvider)
          .track('reservation_created', screen: 'reservation');
      if (mounted) setState(() => _created = reservation);
    } catch (error) {
      if (mounted) showFailure(context, error);
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) => AppPage(
    title: _created == null ? 'Забронировать ПК' : 'Бронь оформлена',
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
        SectionCaption('Когда хотите начать'),
        SettingsGroup(
          children: [
            SettingsRow(
              icon: CupertinoIcons.calendar,
              color: ClubColors.blue,
              title: _dateTime(_startsAt),
              subtitle: 'ПК будет отмечен как забронированный за 30 минут',
              onTap: _chooseStart,
            ),
          ],
        ),
        const SizedBox(height: 20),
        SectionCaption('Сколько часов играть'),
        TextField(
          controller: _hours,
          keyboardType: TextInputType.number,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          decoration: const InputDecoration(
            suffixText: 'часов',
            hintText: 'Например, 6',
          ),
        ),
        const SizedBox(height: 18),
        const InfoCard(
          children: [
            Text(
              'Деньги сейчас не списываем. Придите к выбранному времени и начните игру на этом ПК — оплатите клубу или используйте уже оплаченное время.',
            ),
            SizedBox(height: 10),
            Text(
              'После начала у вас будет 15 минут, чтобы войти. Если не прийти, бронь отменится и ПК снова станет свободным.',
              style: TextStyle(color: ClubColors.muted),
            ),
          ],
        ),
        ActionButton(
          label: 'Забронировать',
          icon: CupertinoIcons.calendar_badge_plus,
          busy: _saving,
          onPressed: _create,
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
        '${item.durationHours} ч игры',
        style: const TextStyle(color: ClubColors.muted),
      ),
      const SizedBox(height: 18),
      Text(
        'ПК будет отмечен как занятый с ${DateFormat('HH:mm').format(item.heldFrom)}. Введите код на ПК до ${DateFormat('HH:mm').format(item.checkinDeadline)}, чтобы начать игру.',
      ),
    ],
  );
}
