import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:latlong2/latlong.dart';

import '../../../core/club_theme.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../domain/club_catalog.dart';
import 'club_browser_screen.dart';

class ClubMapScreen extends ConsumerStatefulWidget {
  const ClubMapScreen({super.key});
  @override
  ConsumerState<ClubMapScreen> createState() => _ClubMapScreenState();
}

class _ClubMapScreenState extends ConsumerState<ClubMapScreen> {
  late Future<List<ClubSearchResult>> _clubs;
  ClubSearchResult? _selected;
  @override
  void initState() {
    super.initState();
    _clubs = ref.read(clubCatalogRepositoryProvider).search('');
    ref.read(analyticsProvider).track('club_map_opened', screen: 'club_map');
  }

  @override
  Widget build(BuildContext context) => FutureBuilder<List<ClubSearchResult>>(
    future: _clubs,
    builder: (context, snapshot) {
      final clubs = (snapshot.data ?? const <ClubSearchResult>[])
          .where((club) => club.latitude != null && club.longitude != null)
          .toList();
      return Scaffold(
        appBar: AppBar(
          title: const Text('Клубы на карте'),
          leading: IconButton(
            icon: const Icon(CupertinoIcons.chevron_back),
            onPressed: () => Navigator.maybePop(context),
          ),
        ),
        body: snapshot.connectionState != ConnectionState.done
            ? const Center(child: CupertinoActivityIndicator())
            : snapshot.hasError
            ? Center(child: Text(errorLabel(context, snapshot.error!)))
            : Stack(
                children: [
                  FlutterMap(
                    options: MapOptions(
                      initialCenter: clubs.isEmpty
                          ? const LatLng(41.3111, 69.2797)
                          : LatLng(
                              clubs.first.latitude!,
                              clubs.first.longitude!,
                            ),
                      initialZoom: clubs.isEmpty ? 11 : 13,
                      onTap: (_, point) => setState(() => _selected = null),
                    ),
                    children: [
                      TileLayer(
                        urlTemplate:
                            'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
                        userAgentPackageName: 'uz.clubpay.clubpayClient',
                      ),
                      MarkerLayer(
                        markers: [
                          for (final club in clubs)
                            Marker(
                              point: LatLng(club.latitude!, club.longitude!),
                              width: 46,
                              height: 46,
                              child: GestureDetector(
                                onTap: () {
                                  ref
                                      .read(analyticsProvider)
                                      .track(
                                        'club_map_pin_opened',
                                        screen: 'club_map',
                                      );
                                  setState(() => _selected = club);
                                },
                                child: DecoratedBox(
                                  decoration: BoxDecoration(
                                    color: club.online
                                        ? ClubColors.purple
                                        : ClubColors.muted,
                                    shape: BoxShape.circle,
                                    border: Border.all(
                                      color: Colors.white,
                                      width: 3,
                                    ),
                                  ),
                                  child: const Icon(
                                    CupertinoIcons.game_controller_solid,
                                    color: Colors.white,
                                    size: 24,
                                  ),
                                ),
                              ),
                            ),
                        ],
                      ),
                      const RichAttributionWidget(
                        attributions: [
                          TextSourceAttribution('© OpenStreetMap contributors'),
                        ],
                      ),
                    ],
                  ),
                  if (_selected != null)
                    Align(
                      alignment: Alignment.bottomCenter,
                      child: SafeArea(
                        child: Padding(
                          padding: const EdgeInsets.all(16),
                          child: Material(
                            color: ClubColors.surface,
                            borderRadius: BorderRadius.circular(24),
                            child: InkWell(
                              borderRadius: BorderRadius.circular(24),
                              onTap: () {
                                ref
                                    .read(analyticsProvider)
                                    .track(
                                      'club_opened_from_map',
                                      screen: 'club_map',
                                    );
                                context.push('/clubs/${_selected!.id}');
                              },
                              child: Padding(
                                padding: const EdgeInsets.all(16),
                                child: Row(
                                  children: [
                                    const SettingsIcon(
                                      CupertinoIcons.game_controller_solid,
                                      color: ClubColors.purple,
                                    ),
                                    const SizedBox(width: 12),
                                    Expanded(
                                      child: Column(
                                        crossAxisAlignment:
                                            CrossAxisAlignment.start,
                                        children: [
                                          Text(
                                            _selected!.name,
                                            style: Theme.of(
                                              context,
                                            ).textTheme.titleMedium,
                                          ),
                                          if (_selected!.address.isNotEmpty)
                                            Text(
                                              _selected!.address,
                                              maxLines: 1,
                                              overflow: TextOverflow.ellipsis,
                                              style: const TextStyle(
                                                color: ClubColors.muted,
                                              ),
                                            ),
                                          Text(
                                            _selected!.online
                                                ? '${_selected!.availablePCs} свободных ПК'
                                                : 'Клуб сейчас не на связи',
                                            style: TextStyle(
                                              color: _selected!.online
                                                  ? ClubColors.green
                                                  : ClubColors.muted,
                                            ),
                                          ),
                                        ],
                                      ),
                                    ),
                                    const Icon(
                                      CupertinoIcons.chevron_right,
                                      color: ClubColors.muted,
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                ],
              ),
      );
    },
  );
}
