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
  late Future<List<ClubSearchResult>> _clubs;
  late Future<List<MobileReservation>> _reservations;
  bool _searchOpen = false;
  bool _hasFavorites = false;

  @override
  void initState() {
    super.initState();
    _clubs = ref.read(clubCatalogRepositoryProvider).search('');
    _reservations = ref.read(clubCatalogRepositoryProvider).reservations();
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _search.dispose();
    super.dispose();
  }

  void _load() => setState(() {
    _clubs = ref.read(clubCatalogRepositoryProvider).search(_search.text);
    _reservations = ref.read(clubCatalogRepositoryProvider).reservations();
  });

  void _track(String event) {
    ref.read(analyticsProvider).track(event, screen: 'club_catalog');
  }

  @override
  Widget build(BuildContext context) => AppPage(
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
    bottom: ClubNavigation(profile: false, showFavorites: _hasFavorites),
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
      FutureBuilder<List<MobileReservation>>(
        future: _reservations,
        builder: (context, snapshot) {
          final now = DateTime.now();
          final reservation = (snapshot.data ?? const <MobileReservation>[])
              .cast<MobileReservation?>()
              .firstWhere(
                (item) =>
                    item != null &&
                    item.status == 'confirmed' &&
                    item.checkinDeadline.isAfter(now),
                orElse: () => null,
              );
          if (reservation == null) return const SizedBox.shrink();
          final held = !reservation.heldFrom.isAfter(now);
          return Padding(
            padding: const EdgeInsets.only(bottom: 16),
            child: InfoCard(
              children: [
                Text(
                  'Ваша бронь · ${reservation.clubName}',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                const SizedBox(height: 6),
                Text(
                  '${reservation.pcLabel} · ${_reservationDate(reservation.startsAt)}',
                  style: const TextStyle(color: ClubColors.muted),
                ),
                const SizedBox(height: 6),
                Text(
                  held
                      ? 'ПК уже зарезервирован для вас. У вас 15 минут после начала, чтобы прийти.'
                      : 'ПК будет отмечен как забронированный за 30 минут до начала.',
                  style: const TextStyle(color: ClubColors.muted),
                ),
              ],
            ),
          );
        },
      ),
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
      FutureBuilder<List<ClubSearchResult>>(
        future: _clubs,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const Padding(
              padding: EdgeInsets.all(32),
              child: Center(child: CupertinoActivityIndicator()),
            );
          }
          if (snapshot.hasError) {
            return SettingsGroup(
              children: [
                SettingsRow(
                  icon: CupertinoIcons.exclamationmark_triangle,
                  color: ClubColors.orange,
                  title: errorLabel(context, snapshot.error!),
                  onTap: _load,
                ),
              ],
            );
          }
          final clubs = snapshot.data ?? const <ClubSearchResult>[];
          final hasFavorites = clubs.any((club) => club.favorite);
          if (hasFavorites != _hasFavorites) {
            WidgetsBinding.instance.addPostFrameCallback((_) {
              if (mounted) setState(() => _hasFavorites = hasFavorites);
            });
          }
          if (clubs.isEmpty) {
            return Padding(
              padding: const EdgeInsets.symmetric(vertical: 36, horizontal: 16),
              child: Text(
                context.l.clubSearchEmpty,
                textAlign: TextAlign.center,
                style: const TextStyle(color: ClubColors.muted),
              ),
            );
          }
          return SettingsGroup(
            children: [
              for (final club in clubs)
                ClubCatalogRow(
                  club: club,
                  onTap: () {
                    _track('club_opened');
                    context.push('/clubs/${club.id}');
                  },
                ),
            ],
          );
        },
      ),
    ],
  );
}

class ClubDetailScreen extends ConsumerStatefulWidget {
  const ClubDetailScreen({super.key, required this.clubId});
  final String clubId;
  @override
  ConsumerState<ClubDetailScreen> createState() => _ClubDetailScreenState();
}

class _ClubDetailScreenState extends ConsumerState<ClubDetailScreen> {
  late Future<ClubCatalog> _club;
  @override
  void initState() {
    super.initState();
    _club = ref.read(clubCatalogRepositoryProvider).club(widget.clubId);
  }

  void _reload() => setState(
    () => _club = ref.read(clubCatalogRepositoryProvider).club(widget.clubId),
  );

  Future<void> _toggleFavorite(ClubCatalog club) async {
    try {
      await ref
          .read(clubCatalogRepositoryProvider)
          .toggleFavorite(club.id, favorite: club.favorite);
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
          if (snapshot.data?.latitude != null &&
              snapshot.data?.longitude != null)
            IconButton(
              tooltip: 'Открыть карту',
              onPressed: () => _openMaps(snapshot.data!),
              icon: const Icon(CupertinoIcons.map),
            ),
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
              const Padding(
                padding: EdgeInsets.all(36),
                child: Center(child: CupertinoActivityIndicator()),
              ),
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

  Future<void> _selectPC(
    ClubCatalog club,
    ClubZone zone,
    ClubComputer pc,
  ) async {
    ref.read(analyticsProvider).track('pc_selected', screen: 'club_detail');
    await showModalBottomSheet<void>(
      context: context,
      builder: (sheet) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(pc.label, style: Theme.of(context).textTheme.titleLarge),
              const SizedBox(height: 12),
              ListTile(
                leading: const Icon(CupertinoIcons.play_fill),
                title: const Text('Начать игру сейчас'),
                onTap: () {
                  Navigator.pop(sheet);
                  context.push('/computer/${pc.token}');
                },
              ),
              ListTile(
                leading: const Icon(CupertinoIcons.calendar_badge_plus),
                title: const Text('Забронировать на другое время'),
                subtitle: const Text('Выберите дату и сколько часов играть'),
                onTap: () {
                  Navigator.pop(sheet);
                  ref
                      .read(analyticsProvider)
                      .track('reservation_opened', screen: 'club_detail');
                  context.push(
                    '/reservation/${pc.id}?club=${Uri.encodeComponent(club.name)}&zone=${Uri.encodeComponent(zone.name)}&pc=${Uri.encodeComponent(pc.label)}',
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _openMaps(ClubCatalog club) async {
    final lat = club.latitude!;
    final lon = club.longitude!;
    await showModalBottomSheet<void>(
      context: context,
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                'Открыть ${club.name}',
                style: Theme.of(context).textTheme.titleLarge,
              ),
              const SizedBox(height: 12),
              ListTile(
                leading: const Icon(CupertinoIcons.map),
                title: const Text('Яндекс Карты'),
                onTap: () async {
                  Navigator.pop(sheetContext);
                  await openExternal(
                    'https://yandex.com/maps/?pt=$lon,$lat&z=17&l=map',
                  );
                },
              ),
              ListTile(
                leading: const Icon(CupertinoIcons.location_solid),
                title: const Text('2ГИС'),
                onTap: () async {
                  Navigator.pop(sheetContext);
                  await openExternal('https://2gis.uz/geo/$lon,$lat');
                },
              ),
            ],
          ),
        ),
      ),
    );
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
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          club.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 17,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      if (club.favorite)
                        const Padding(
                          padding: EdgeInsets.only(left: 8),
                          child: Icon(
                            CupertinoIcons.heart_fill,
                            color: ClubColors.purple,
                            size: 16,
                          ),
                        ),
                    ],
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
  @override
  void initState() {
    super.initState();
    _clubs = ref.read(clubCatalogRepositoryProvider).favorites();
  }

  void _reload() => setState(
    () => _clubs = ref.read(clubCatalogRepositoryProvider).favorites(),
  );
  @override
  Widget build(BuildContext context) => AppPage(
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
          if (snapshot.connectionState != ConnectionState.done) {
            return const Padding(
              padding: EdgeInsets.all(32),
              child: Center(child: CupertinoActivityIndicator()),
            );
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
