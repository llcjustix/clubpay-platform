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

  @override
  void initState() {
    super.initState();
    _clubs = ref.read(clubCatalogRepositoryProvider).search('');
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _search.dispose();
    super.dispose();
  }

  void _load() => setState(
    () => _clubs = ref.read(clubCatalogRepositoryProvider).search(_search.text),
  );

  @override
  Widget build(BuildContext context) => AppPage(
    title: context.l.appName,
    largeTitle: true,
    bottom: const ClubNavigation(profile: false),
    children: [
      SettingsGroup(
        children: [
          SettingsRow(
            icon: CupertinoIcons.question,
            color: ClubColors.blue,
            title: context.l.howItWorks,
            onTap: () => showClubGuide(context),
          ),
        ],
      ),
      const SizedBox(height: 28),
      SectionCaption(context.l.clubSearchTitle),
      TextField(
        controller: _search,
        textInputAction: TextInputAction.search,
        onChanged: (_) {
          _debounce?.cancel();
          _debounce = Timer(const Duration(milliseconds: 250), _load);
        },
        onSubmitted: (_) => _load(),
        decoration: InputDecoration(
          hintText: context.l.clubSearchHint,
          prefixIcon: const Icon(CupertinoIcons.search),
          suffixIcon: _search.text.isEmpty
              ? null
              : IconButton(
                  onPressed: () {
                    _search.clear();
                    _load();
                  },
                  icon: const Icon(CupertinoIcons.clear_circled_solid),
                ),
        ),
      ),
      const SizedBox(height: 18),
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
                SettingsRow(
                  icon: CupertinoIcons.game_controller_solid,
                  color: club.online ? ClubColors.purple : ClubColors.muted,
                  title: club.name,
                  subtitle: club.address.isNotEmpty
                      ? club.address
                      : club.online
                      ? context.l.clubSelectZonePc
                      : context.l.clubOffline,
                  trailing: Text(
                    club.online
                        ? context.l.freePcs(club.availablePCs)
                        : context.l.noConnection,
                    style: TextStyle(
                      color: club.online && club.availablePCs > 0
                          ? ClubColors.green
                          : ClubColors.muted,
                    ),
                  ),
                  onTap: () => context.push('/clubs/${club.id}'),
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

  @override
  Widget build(BuildContext context) => FutureBuilder<ClubCatalog>(
    future: _club,
    builder: (context, snapshot) {
      final title = snapshot.data?.name ?? context.l.chooseClub;
      return AppPage(
        title: title,
        actions: [
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
                          : _statusLabel(context, pc.status),
                      trailing: Text(
                        pc.selectable
                            ? context.l.available
                            : _statusLabel(context, pc.status),
                        style: TextStyle(
                          color: pc.selectable
                              ? ClubColors.green
                              : ClubColors.muted,
                        ),
                      ),
                      onTap: !snapshot.data!.online || !pc.selectable
                          ? null
                          : () => context.push('/computer/${pc.token}'),
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
}

String _statusLabel(BuildContext context, String status) => switch (status) {
  'occupied' || 'frozen' => context.l.occupied,
  'sleeping' => context.l.sleeping,
  _ => context.l.maintenance,
};
