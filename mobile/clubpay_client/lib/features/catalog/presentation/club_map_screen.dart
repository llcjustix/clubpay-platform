import 'dart:convert';

import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:geolocator/geolocator.dart';
import 'package:go_router/go_router.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../../core/club_theme.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../domain/club_catalog.dart';
import 'club_browser_screen.dart';

const _yandexMapsKey = String.fromEnvironment('YANDEX_MAPS_API_KEY');

class ClubMapScreen extends ConsumerStatefulWidget {
  const ClubMapScreen({super.key});
  @override
  ConsumerState<ClubMapScreen> createState() => _ClubMapScreenState();
}

class _ClubMapScreenState extends ConsumerState<ClubMapScreen> {
  late Future<List<ClubSearchResult>> _clubs;
  ClubSearchResult? _selected;
  MapPoint? _playerLocation;
  bool _locating = false;

  @override
  void initState() {
    super.initState();
    _clubs = ref.read(clubCatalogRepositoryProvider).search('');
    ref.read(analyticsProvider).track('club_map_opened', screen: 'club_map');
  }

  Future<void> _locatePlayer() async {
    setState(() => _locating = true);
    try {
      var permission = await Geolocator.checkPermission();
      if (permission == LocationPermission.denied) {
        permission = await Geolocator.requestPermission();
      }
      if (permission == LocationPermission.denied ||
          permission == LocationPermission.deniedForever) {
        return;
      }
      final point = await Geolocator.getCurrentPosition();
      if (mounted) {
        setState(
          () => _playerLocation = MapPoint(point.latitude, point.longitude),
        );
      }
      ref
          .read(analyticsProvider)
          .track('club_map_location_enabled', screen: 'club_map');
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text(
              'Не удалось определить геопозицию. Проверьте разрешение.',
            ),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _locating = false);
    }
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
            ? const SafeArea(child: PageSkeleton(rows: 3))
            : snapshot.hasError
            ? Center(child: Text(errorLabel(context, snapshot.error!)))
            : _yandexMapsKey.isEmpty
            ? const Center(
                child: Padding(
                  padding: EdgeInsets.all(24),
                  child: Text(
                    'Карта Яндекс временно не настроена.',
                    textAlign: TextAlign.center,
                  ),
                ),
              )
            : Stack(
                children: [
                  YandexClubMap(
                    key: ValueKey(
                      '${clubs.map((club) => club.id).join(',')}:$_playerLocation',
                    ),
                    clubs: clubs,
                    playerLocation: _playerLocation,
                    onClubTap: (id) {
                      final club = clubs
                          .where((item) => item.id == id)
                          .firstOrNull;
                      if (club == null) return;
                      ref
                          .read(analyticsProvider)
                          .track('club_map_pin_opened', screen: 'club_map');
                      setState(() => _selected = club);
                    },
                  ),
                  Positioned(
                    top: 16,
                    right: 16,
                    child: FloatingActionButton.small(
                      heroTag: 'player-location',
                      onPressed: _locating ? null : _locatePlayer,
                      child: _locating
                          ? const SkeletonBox(
                              width: 20,
                              height: 20,
                              color: ClubColors.muted,
                            )
                          : const Icon(CupertinoIcons.location_solid),
                    ),
                  ),
                  if (_selected != null)
                    Align(
                      alignment: Alignment.bottomCenter,
                      child: SafeArea(
                        child: Padding(
                          padding: const EdgeInsets.all(16),
                          child: _ClubMapCard(
                            club: _selected!,
                            onClose: () => setState(() => _selected = null),
                            onOpen: () {
                              ref
                                  .read(analyticsProvider)
                                  .track(
                                    'club_opened_from_map',
                                    screen: 'club_map',
                                  );
                              context.push('/clubs/${_selected!.id}');
                            },
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

class _ClubMapCard extends StatelessWidget {
  const _ClubMapCard({
    required this.club,
    required this.onOpen,
    required this.onClose,
  });
  final ClubSearchResult club;
  final VoidCallback onOpen, onClose;

  @override
  Widget build(BuildContext context) => Material(
    color: ClubColors.surface,
    borderRadius: BorderRadius.circular(24),
    child: Padding(
      padding: const EdgeInsets.all(18),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const SettingsIcon(
                CupertinoIcons.game_controller_solid,
                color: ClubColors.purple,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  club.name,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              IconButton(
                tooltip: 'Закрыть',
                onPressed: onClose,
                icon: const Icon(CupertinoIcons.xmark_circle_fill),
              ),
            ],
          ),
          if (club.address.isNotEmpty) ...[
            const SizedBox(height: 10),
            Text(club.address, style: const TextStyle(color: ClubColors.muted)),
          ],
          const SizedBox(height: 8),
          Text(
            club.online
                ? '${club.availablePCs} из ${club.totalPCs} свободных ПК'
                : 'Клуб сейчас не на связи',
            style: TextStyle(
              color: club.online && club.availablePCs > 0
                  ? ClubColors.green
                  : ClubColors.muted,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 14),
          SizedBox(
            width: double.infinity,
            child: ActionButton(
              label: 'Открыть клуб',
              onPressed: onOpen,
              icon: CupertinoIcons.arrow_right,
            ),
          ),
        ],
      ),
    ),
  );
}

class MapPoint {
  const MapPoint(this.latitude, this.longitude);
  final double latitude, longitude;
  @override
  String toString() => '$latitude,$longitude';
}

class YandexClubMap extends StatefulWidget {
  const YandexClubMap({
    super.key,
    required this.clubs,
    required this.playerLocation,
    required this.onClubTap,
  });
  final List<ClubSearchResult> clubs;
  final MapPoint? playerLocation;
  final ValueChanged<String> onClubTap;
  @override
  State<YandexClubMap> createState() => _YandexClubMapState();
}

class _YandexClubMapState extends State<YandexClubMap> {
  late final WebViewController _controller;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setBackgroundColor(ClubColors.background)
      ..addJavaScriptChannel(
        'ClubPayMap',
        onMessageReceived: (message) => widget.onClubTap(message.message),
      )
      ..loadHtmlString(_mapHTML(widget.clubs, widget.playerLocation));
  }

  @override
  Widget build(BuildContext context) => WebViewWidget(controller: _controller);
}

String _mapHTML(List<ClubSearchResult> clubs, MapPoint? player) {
  final markers = clubs
      .map(
        (club) => {
          'id': club.id,
          'name': club.name,
          'lat': club.latitude,
          'lon': club.longitude,
          'available': club.availablePCs,
          'total': club.totalPCs,
          'online': club.online,
        },
      )
      .toList();
  final center = player == null
      ? {
          'lat': clubs.firstOrNull?.latitude ?? 41.3111,
          'lon': clubs.firstOrNull?.longitude ?? 69.2797,
        }
      : {'lat': player.latitude, 'lon': player.longitude};
  return '''<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1,maximum-scale=1,user-scalable=no"><style>html,body,#map{margin:0;width:100%;height:100%;background:#000}.ymaps-2-1-79-map{font-family:-apple-system,BlinkMacSystemFont,sans-serif!important}</style><script src="https://api-maps.yandex.ru/2.1/?apikey=$_yandexMapsKey&lang=ru_RU"></script></head><body><div id="map"></div><script>const clubs=${jsonEncode(markers)}, center=${jsonEncode(center)}; ymaps.ready(()=>{const map=new ymaps.Map('map',{center:[center.lat,center.lon],zoom:13,controls:['zoomControl']}); clubs.forEach(c=>{const marker=new ymaps.Placemark([c.lat,c.lon],{balloonContentHeader:c.name,balloonContentBody:(c.online?c.available+' из '+c.total+' свободных ПК':'Клуб не на связи'),balloonContentFooter:'<button onclick="window.ClubPayMap.postMessage(\\''+c.id+'\\')">Открыть клуб</button>'},{preset:c.online?'islands#violetIcon':'islands#grayIcon'}); marker.events.add('click',()=>window.ClubPayMap.postMessage(c.id));map.geoObjects.add(marker);});${player == null ? '' : "map.geoObjects.add(new ymaps.Placemark([${player.latitude},${player.longitude}],{balloonContent:'Вы здесь'},{preset:'islands#blueCircleIcon'}));"}});</script></body></html>''';
}
