import 'dart:async';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/club_navigation.dart';
import '../../../core/club_theme.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../../profile/presentation/profile_screen.dart' show showClubGuide;
import '../data/club_catalog_repository.dart';
import '../domain/club_catalog.dart';

final clubCatalogRepositoryProvider = Provider(
  (ref) => ClubCatalogRepository(ref.watch(apiProvider)),
);

class ClubBrowserScreen extends ConsumerStatefulWidget {
  const ClubBrowserScreen({super.key});
  @override
  ConsumerState<ClubBrowserScreen> createState() => _ClubBrowserScreenState();
}

class _ClubBrowserScreenState extends ConsumerState<ClubBrowserScreen> {
  final _search = TextEditingController();
  Timer? _debounce;
  Timer? _refreshTimer;
  List<ClubSearchResult>? _clubs;
  List<MobileReservation> _reservations = const [];
  Object? _catalogError;
  bool _loadingInitialCatalog = true;
  bool _refreshingCatalog = false;
  bool _refreshQueued = false;
  bool _searchOpen = false;
  bool _hasFavorites = false;
  int _catalogRevision = -1;

  @override
  void initState() {
    super.initState();
    _load();
    _refreshTimer = Timer.periodic(const Duration(seconds: 15), (_) => _load());
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _refreshTimer?.cancel();
    _search.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    if (_refreshingCatalog) {
      _refreshQueued = true;
      return;
    }
    _refreshingCatalog = true;
    final query = _search.text;
    try {
      final repository = ref.read(clubCatalogRepositoryProvider);
      final result = await Future.wait([
        repository.search(query),
        _loadReservations(),
      ]);
      if (!mounted) return;
      // A user can change the query while an earlier request is in flight.
      // Keep the visible list intact and fetch that newer query next.
      if (query != _search.text) {
        _refreshQueued = true;
        return;
      }
      final clubs = result[0] as List<ClubSearchResult>;
      final reservations = result[1] as List<MobileReservation>;
      final clubsChanged = !_sameClubResults(_clubs, clubs);
      final reservationsChanged = !_sameReservations(
        _reservations,
        reservations,
      );
      final favoriteStateChanged =
          query.trim().isEmpty &&
          _hasFavorites != clubs.any((club) => club.favorite);
      if (clubsChanged ||
          reservationsChanged ||
          favoriteStateChanged ||
          _loadingInitialCatalog ||
          _catalogError != null) {
        setState(() {
          if (clubsChanged || _clubs == null) _clubs = clubs;
          if (reservationsChanged) _reservations = reservations;
          if (query.trim().isEmpty) {
            _hasFavorites = clubs.any((club) => club.favorite);
            ref.read(favoriteTabProvider.notifier).set(_hasFavorites);
          }
          _catalogError = null;
          _loadingInitialCatalog = false;
        });
      }
    } catch (error) {
      if (mounted && _clubs == null) {
        setState(() {
          _catalogError = error;
          _loadingInitialCatalog = false;
        });
      }
    } finally {
      _refreshingCatalog = false;
      if (_refreshQueued) {
        _refreshQueued = false;
        unawaited(_load());
      }
    }
  }

  void _track(String event) {
    ref.read(analyticsProvider).track(event, screen: 'club_catalog');
  }

  // The catalog remains usable when the optional reservation endpoint is
  // temporarily unavailable; the next revision/poll retries it automatically.
  Future<List<MobileReservation>> _loadReservations() async {
    try {
      return await ref.read(clubCatalogRepositoryProvider).reservations();
    } catch (_) {
      return const <MobileReservation>[];
    }
  }

  Future<void> _openReservation(MobileReservation reservation) async {
    var starting = false;
    await showModalBottomSheet<void>(
      context: context,
      builder: (sheet) => StatefulBuilder(
        builder: (sheet, setSheetState) {
          final canStart =
              !DateTime.now().isBefore(reservation.heldFrom) &&
              DateTime.now().isBefore(reservation.checkinDeadline);
          return SafeArea(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(20, 20, 20, 28),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'Ваша бронь',
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '${reservation.clubName} · ${reservation.pcLabel}',
                    style: const TextStyle(color: ClubColors.muted),
                  ),
                  const SizedBox(height: 20),
                  if (canStart) ...[
                    const Text(
                      'Вы на месте? Начните игру в ClubPay — затем выберите оплату или используйте уже оплаченное время.',
                    ),
                    const SizedBox(height: 14),
                    ActionButton(
                      label: 'Начать игру',
                      icon: CupertinoIcons.play_fill,
                      busy: starting,
                      onPressed: starting
                          ? null
                          : () async {
                              setSheetState(() => starting = true);
                              try {
                                final token = await ref
                                    .read(clubCatalogRepositoryProvider)
                                    .startReservation(id: reservation.id);
                                ref
                                    .read(analyticsProvider)
                                    .track(
                                      'reservation_started',
                                      screen: 'club_catalog',
                                    );
                                ref
                                    .read(catalogRevisionProvider.notifier)
                                    .bump();
                                if (!mounted || !sheet.mounted) return;
                                setState(() => _reservations = const []);
                                Navigator.pop(sheet);
                                context.push(
                                  '/computer/$token',
                                  extra: reservation.id,
                                );
                              } catch (error) {
                                if (mounted) {
                                  showFailure(context, error);
                                }
                              } finally {
                                if (sheet.mounted) {
                                  setSheetState(() => starting = false);
                                }
                              }
                            },
                    ),
                  ] else
                    Text(
                      DateTime.now().isBefore(reservation.heldFrom)
                          ? 'Кнопка «Начать игру» станет доступна в ${_time(reservation.heldFrom)} — за 15 минут до начала брони.'
                          : 'Время для начала игры закончилось. Бронь больше недоступна.',
                      style: const TextStyle(color: ClubColors.muted),
                    ),
                  const SizedBox(height: 14),
                  if (DateTime.now().isBefore(reservation.heldFrom)) ...[
                    ActionButton(
                      label: 'Изменить бронь',
                      icon: CupertinoIcons.calendar,
                      secondary: true,
                      onPressed: () {
                        Navigator.pop(sheet);
                        context.push(
                          '/reservation/${reservation.pcID}?club=${Uri.encodeComponent(reservation.clubName)}&zone=${Uri.encodeComponent(reservation.zoneName)}&pc=${Uri.encodeComponent(reservation.pcLabel)}',
                          extra: reservation,
                        );
                      },
                    ),
                    const SizedBox(height: 8),
                    OutlinedButton.icon(
                      onPressed: () async {
                        Navigator.pop(sheet);
                        try {
                          await ref
                              .read(clubCatalogRepositoryProvider)
                              .cancelReservation(reservation.id);
                          ref
                              .read(analyticsProvider)
                              .track(
                                'reservation_cancelled',
                                screen: 'club_catalog',
                              );
                          ref.read(catalogRevisionProvider.notifier).bump();
                          if (mounted) {
                            setState(() => _reservations = const []);
                            ScaffoldMessenger.of(context).showSnackBar(
                              const SnackBar(content: Text('Бронь отменена.')),
                            );
                          }
                        } catch (error) {
                          if (mounted) showFailure(context, error);
                        }
                      },
                      icon: const Icon(CupertinoIcons.xmark_circle),
                      label: const Text('Отменить бронь'),
                      style: OutlinedButton.styleFrom(
                        foregroundColor: ClubColors.red,
                        side: const BorderSide(color: ClubColors.red),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final revision = ref.watch(catalogRevisionProvider);
    if (revision != _catalogRevision) {
      _catalogRevision = revision;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          _load();
        }
      });
    }
    return AppPage(
      title: context.l.appName,
      largeTitle: true,
      actions: [
        IconButton(
          tooltip: 'Поддержка',
          onPressed: () {
            _track('support_opened');
            context.push('/support');
          },
          icon: const Icon(CupertinoIcons.chat_bubble_text),
        ),
        IconButton(
          tooltip: 'Поиск клубов',
          onPressed: () {
            _track('club_search_opened');
            setState(() => _searchOpen = !_searchOpen);
          },
          icon: const Icon(CupertinoIcons.search),
        ),
        IconButton(
          tooltip: context.l.howItWorks,
          onPressed: () {
            _track('how_it_works_opened');
            showClubGuide(context);
          },
          icon: const Icon(CupertinoIcons.question_circle),
        ),
      ],
      bottom: _loadingInitialCatalog
          ? const SizedBox.shrink()
          : ClubNavigation(profile: false, showFavorites: _hasFavorites),
      children: [
        SectionCaption(context.l.clubSearchTitle),
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton.icon(
            onPressed: () {
              _track('club_map_opened');
              context.push('/clubs-map');
            },
            icon: const Icon(CupertinoIcons.map),
            label: const Text('Открыть карту клубов'),
          ),
        ),
        _ReservationCard(reservations: _reservations, onTap: _openReservation),
        if (_searchOpen) ...[
          TextField(
            controller: _search,
            autofocus: true,
            textInputAction: TextInputAction.search,
            onChanged: (_) {
              _debounce?.cancel();
              _debounce = Timer(const Duration(milliseconds: 250), _load);
            },
            onSubmitted: (_) => _load(),
            decoration: InputDecoration(
              hintText: context.l.clubSearchHint,
              prefixIcon: const Icon(CupertinoIcons.search),
              suffixIcon: IconButton(
                onPressed: () {
                  _search.clear();
                  _load();
                  setState(() => _searchOpen = false);
                },
                icon: const Icon(CupertinoIcons.clear_circled_solid),
              ),
            ),
          ),
          const SizedBox(height: 18),
        ],
        if (_loadingInitialCatalog)
          const PageSkeleton(rows: 4)
        else if (_catalogError != null)
          SettingsGroup(
            children: [
              SettingsRow(
                icon: CupertinoIcons.exclamationmark_triangle,
                color: ClubColors.orange,
                title: errorLabel(context, _catalogError!),
                onTap: _load,
              ),
            ],
          )
        else if ((_clubs ?? const <ClubSearchResult>[]).isEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 36, horizontal: 16),
            child: Text(
              context.l.clubSearchEmpty,
              textAlign: TextAlign.center,
              style: const TextStyle(color: ClubColors.muted),
            ),
          )
        else
          SettingsGroup(
            children: [
              for (final club in _clubs!)
                ClubCatalogRow(
                  club: club,
                  onTap: () {
                    _track('club_opened');
                    context.push('/clubs/${club.id}');
                  },
                ),
            ],
          ),
      ],
    );
  }
}

class _ReservationCard extends StatelessWidget {
  const _ReservationCard({required this.reservations, required this.onTap});

  final List<MobileReservation> reservations;
  final ValueChanged<MobileReservation> onTap;

  @override
  Widget build(BuildContext context) {
    final now = DateTime.now();
    MobileReservation? reservation;
    for (final item in reservations) {
      if ((item.status == 'confirmed' || item.status == 'checked_in') &&
          item.checkinDeadline.isAfter(now)) {
        reservation = item;
        break;
      }
    }
    if (reservation == null) return const SizedBox.shrink();
    final booking = reservation;
    final held = !booking.heldFrom.isAfter(now);
    return Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          borderRadius: BorderRadius.circular(24),
          onTap: () => onTap(booking),
          child: InfoCard(
            children: [
              Text(
                'Ваша бронь · ${booking.clubName}',
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 6),
              Text(
                '${booking.pcLabel} · ${_reservationDate(booking.startsAt)}',
                style: const TextStyle(color: ClubColors.muted),
              ),
              const SizedBox(height: 8),
              Text(
                held
                    ? 'ПК зарезервирован для вас. Откройте бронь и нажмите «Начать игру», чтобы выбрать оплату или использовать уже оплаченное время.'
                    : 'ПК будет отмечен как забронированный за 15 минут до начала. В это время в ClubPay станет доступна кнопка «Начать игру».',
                style: const TextStyle(color: ClubColors.muted),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

bool _sameClubResults(
  List<ClubSearchResult>? before,
  List<ClubSearchResult> after,
) {
  if (before == null || before.length != after.length) return false;
  for (var index = 0; index < before.length; index++) {
    final left = before[index];
    final right = after[index];
    if (left.id != right.id ||
        left.name != right.name ||
        left.address != right.address ||
        left.online != right.online ||
        left.availablePCs != right.availablePCs ||
        left.totalPCs != right.totalPCs ||
        left.latitude != right.latitude ||
        left.longitude != right.longitude ||
        left.favorite != right.favorite) {
      return false;
    }
  }
  return true;
}

bool _sameReservations(
  List<MobileReservation> before,
  List<MobileReservation> after,
) {
  if (before.length != after.length) return false;
  for (var index = 0; index < before.length; index++) {
    final left = before[index];
    final right = after[index];
    if (left.id != right.id ||
        left.pcID != right.pcID ||
        left.status != right.status ||
        left.startsAt != right.startsAt ||
        left.endsAt != right.endsAt ||
        left.heldFrom != right.heldFrom ||
        left.checkinDeadline != right.checkinDeadline ||
        left.durationHours != right.durationHours) {
      return false;
    }
  }
  return true;
}

class ClubDetailScreen extends ConsumerStatefulWidget {
  const ClubDetailScreen({super.key, required this.clubId});
  final String clubId;
  @override
  ConsumerState<ClubDetailScreen> createState() => _ClubDetailScreenState();
}

class _ClubDetailScreenState extends ConsumerState<ClubDetailScreen> {
  late Future<ClubCatalog> _club;
  Timer? _refreshTimer;
  @override
  void initState() {
    super.initState();
    _club = ref.read(clubCatalogRepositoryProvider).club(widget.clubId);
    _refreshTimer = Timer.periodic(
      const Duration(seconds: 15),
      (_) => _reload(),
    );
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    super.dispose();
  }

  void _reload() => setState(
    () => _club = ref.read(clubCatalogRepositoryProvider).club(widget.clubId),
  );

  Future<void> _toggleFavorite(ClubCatalog club) async {
    try {
      final favorite = await ref
          .read(clubCatalogRepositoryProvider)
          .toggleFavorite(club.id, favorite: club.favorite);
      ref.read(favoriteTabProvider.notifier).set(favorite);
      ref.read(catalogRevisionProvider.notifier).bump();
      if (mounted) _reload();
    } catch (error) {
      if (mounted) showFailure(context, error);
    }
  }

  Future<void> _wake(ClubComputer pc) async {
    try {
      await ref.read(clubCatalogRepositoryProvider).wake(pc.id);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'Команда на включение отправлена. Обновим список, когда ПК появится в сети.',
            ),
          ),
        );
      }
    } catch (error) {
      if (mounted) showFailure(context, error);
    }
  }

  @override
  Widget build(BuildContext context) => FutureBuilder<ClubCatalog>(
    future: _club,
    builder: (context, snapshot) {
      final title = snapshot.data?.name ?? context.l.chooseClub;
      return AppPage(
        title: title,
        actions: [
          if (snapshot.data != null)
            IconButton(
              tooltip: snapshot.data!.favorite
                  ? 'Убрать из избранного'
                  : 'Добавить в избранное',
              onPressed: () => _toggleFavorite(snapshot.data!),
              icon: Icon(
                snapshot.data!.favorite
                    ? CupertinoIcons.heart_fill
                    : CupertinoIcons.heart,
                color: snapshot.data!.favorite
                    ? ClubColors.favorite
                    : ClubColors.text,
              ),
            ),
          IconButton(
            tooltip: context.l.refresh,
            onPressed: _reload,
            icon: const Icon(CupertinoIcons.arrow_clockwise),
          ),
        ],
        children: [
          if (!snapshot.hasData) ...[
            if (snapshot.hasError)
              SettingsGroup(
                children: [
                  SettingsRow(
                    icon: CupertinoIcons.exclamationmark_triangle,
                    color: ClubColors.orange,
                    title: errorLabel(context, snapshot.error!),
                    onTap: _reload,
                  ),
                ],
              )
            else
              const PageSkeleton(rows: 3),
          ] else ...[
            if (snapshot.data!.address.isNotEmpty)
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 20),
                child: Text(
                  snapshot.data!.address,
                  style: const TextStyle(color: ClubColors.muted),
                ),
              ),
            if (!snapshot.data!.online)
              const InfoCard(
                children: [
                  Text(
                    'Клуб сейчас не на связи. Список ПК может быть неактуальным.',
                  ),
                ],
              ),
            if (!snapshot.data!.online) const SizedBox(height: 20),
            for (final zone in snapshot.data!.zones) ...[
              SectionCaption(
                '${zone.name} · ${context.l.zoneHourlyPrice(zone.hourlyPrice)}',
              ),
              SettingsGroup(
                children: [
                  for (final pc in zone.computers)
                    SettingsRow(
                      icon: CupertinoIcons.desktopcomputer,
                      color: pc.selectable
                          ? ClubColors.green
                          : ClubColors.muted,
                      title: pc.label,
                      subtitle: pc.selectable
                          ? context.l.selectThisPc
                          : _pcSubtitle(context, pc),
                      trailing: Text(
                        pc.selectable
                            ? context.l.available
                            : pc.wakeable
                            ? 'Включить'
                            : _statusLabel(context, pc.status),
                        style: TextStyle(
                          color: pc.selectable
                              ? ClubColors.green
                              : ClubColors.muted,
                        ),
                      ),
                      onTap: !snapshot.data!.online
                          ? null
                          : pc.selectable
                          ? () => _selectPC(snapshot.data!, zone, pc)
                          : pc.wakeable
                          ? () => _wake(pc)
                          : null,
                    ),
                ],
              ),
              const SizedBox(height: 20),
            ],
            if (snapshot.data!.zones.isEmpty)
              const InfoCard(
                children: [Text('В этом клубе пока нет доступных зон.')],
              ),
          ],
        ],
      );
    },
  );

  void _selectPC(ClubCatalog club, ClubZone zone, ClubComputer pc) async {
    ref.read(analyticsProvider).track('pc_selected', screen: 'club_detail');
    context.push('/computer/${pc.token}');
  }
}

String _statusLabel(BuildContext context, String status) => switch (status) {
  'occupied' || 'frozen' => context.l.occupied,
  'sleeping' => context.l.sleeping,
  _ => context.l.maintenance,
};

class ClubCatalogRow extends StatelessWidget {
  const ClubCatalogRow({super.key, required this.club, required this.onTap});
  final ClubSearchResult club;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) => Material(
    color: Colors.transparent,
    child: InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(16, 12, 12, 12),
        child: Row(
          children: [
            SettingsIcon(
              CupertinoIcons.game_controller_solid,
              color: club.online ? ClubColors.purple : ClubColors.muted,
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    club.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 17,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    club.online
                        ? '${club.availablePCs} из ${club.totalPCs} свободных ПК'
                        : context.l.noConnection,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      color: club.online && club.availablePCs > 0
                          ? ClubColors.green
                          : ClubColors.muted,
                      fontSize: 13,
                    ),
                  ),
                  if (club.address.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(
                        club.address,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          color: ClubColors.muted,
                          fontSize: 12,
                        ),
                      ),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            const Icon(
              CupertinoIcons.chevron_right,
              color: Color(0xff636366),
              size: 16,
            ),
          ],
        ),
      ),
    ),
  );
}

String _reservationDate(DateTime value) =>
    '${value.day.toString().padLeft(2, '0')}.${value.month.toString().padLeft(2, '0')} · ${value.hour.toString().padLeft(2, '0')}:${value.minute.toString().padLeft(2, '0')}';

String _time(DateTime value) =>
    '${value.hour.toString().padLeft(2, '0')}:${value.minute.toString().padLeft(2, '0')}';

String _pcSubtitle(BuildContext context, ClubComputer pc) {
  if (pc.status != 'reserved') return _statusLabel(context, pc.status);
  final when = pc.reservationStartsAt == null
      ? ''
      : ' · ${_reservationDate(pc.reservationStartsAt!)}';
  return pc.reservedByMe ? 'Ваша бронь$when' : 'Забронирован$when';
}

class FavoritesScreen extends ConsumerStatefulWidget {
  const FavoritesScreen({super.key});
  @override
  ConsumerState<FavoritesScreen> createState() => _FavoritesScreenState();
}

class _FavoritesScreenState extends ConsumerState<FavoritesScreen> {
  late Future<List<ClubSearchResult>> _clubs;
  int _catalogRevision = -1;
  @override
  void initState() {
    super.initState();
    _clubs = ref.read(clubCatalogRepositoryProvider).favorites();
  }

  void _reload() => setState(
    () => _clubs = ref.read(clubCatalogRepositoryProvider).favorites(),
  );
  @override
  Widget build(BuildContext context) {
    final revision = ref.watch(catalogRevisionProvider);
    if (revision != _catalogRevision) {
      _catalogRevision = revision;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _reload();
      });
    }
    return AppPage(
      title: 'Избранное',
      actions: [
        IconButton(
          tooltip: context.l.refresh,
          icon: const Icon(CupertinoIcons.arrow_clockwise),
          onPressed: _reload,
        ),
      ],
      bottom: const ClubNavigation(
        profile: false,
        showFavorites: true,
        favorites: true,
      ),
      children: [
        FutureBuilder<List<ClubSearchResult>>(
          future: _clubs,
          builder: (context, snapshot) {
            if (!snapshot.hasData &&
                snapshot.connectionState != ConnectionState.done) {
              return const PageSkeleton(rows: 3);
            }
            if (snapshot.hasError) {
              return InfoCard(
                children: [Text(errorLabel(context, snapshot.error!))],
              );
            }
            final clubs = snapshot.data ?? const <ClubSearchResult>[];
            if (clubs.isEmpty) {
              return const InfoCard(
                children: [
                  Text(
                    'Добавьте клуб в избранное — он появится здесь.',
                    style: TextStyle(color: ClubColors.muted),
                  ),
                ],
              );
            }
            return SettingsGroup(
              children: [
                for (final club in clubs)
                  ClubCatalogRow(
                    club: club,
                    onTap: () => context.push('/clubs/${club.id}'),
                  ),
              ],
            );
          },
        ),
      ],
    );
  }
}
