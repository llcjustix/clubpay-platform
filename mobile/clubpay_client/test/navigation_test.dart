import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/auth/domain/auth_models.dart';
import 'package:clubpay_client/features/catalog/presentation/reservation_screen.dart';
import 'helpers.dart';

Future<Widget> app({String locale = 'ru'}) async {
  final store = MemoryStore();
  store.values['mobile.locale'] = locale;
  final vault = SessionVault(store);
  await vault.save(const TokenPair('access', 'refresh'));
  final api = ApiClient(
    vault,
    dio: widgetDio(
      (r) async => switch (r.path) {
        '/api/mobile/me' => (200, {'id': 'player', 'phone': '+998900000001'}),
        '/api/mobile/clubs' => (
          200,
          {
            'clubs': [
              {
                'club_id': 'club',
                'club_name': 'Pilot',
                'address': 'Tashkent',
                'club_online': true,
                'available_pcs': 3,
              },
            ],
          },
        ),
        _ => throw StateError('Unexpected API request ${r.path}'),
      },
    ),
  );
  return ProviderScope(
    overrides: [
      secureStoreProvider.overrideWithValue(store),
      apiProvider.overrideWithValue(api),
    ],
    child: const ClubPayApp(),
  );
}

void main() {
  testWidgets('Editing a reservation never opens the QR error screen', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(600, 1800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final store = MemoryStore();
    final vault = SessionVault(store);
    await vault.save(const TokenPair('access', 'refresh'));
    final start = DateTime.now().add(const Duration(hours: 2));
    final api = ApiClient(
      vault,
      dio: widgetDio((request) async {
        switch (request.path) {
          case '/api/mobile/me':
            return (200, {'id': 'player', 'phone': '+998900000001'});
          case '/api/mobile/clubs':
            return (200, {'clubs': const []});
          case '/api/mobile/active-session':
            return (200, {'session': null});
          case '/api/mobile/reservations':
            return (
              200,
              {
                'reservations': [
                  {
                    'id': 'reservation-1',
                    'pc_id': 'pc/with-url-sensitive-value',
                    'status': 'confirmed',
                    'club_name': 'Pilot',
                    'zone_name': 'Standard',
                    'pc_label': 'Pilot PC #01',
                    'starts_at': start.toUtc().toIso8601String(),
                    'ends_at': start
                        .add(const Duration(hours: 2))
                        .toUtc()
                        .toIso8601String(),
                    'held_from': start
                        .subtract(const Duration(minutes: 15))
                        .toUtc()
                        .toIso8601String(),
                    'checkin_deadline': start
                        .add(const Duration(minutes: 15))
                        .toUtc()
                        .toIso8601String(),
                    'duration_hours': 2,
                  },
                ],
              },
            );
          default:
            throw StateError('Unexpected API request ${request.path}');
        }
      }),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          secureStoreProvider.overrideWithValue(store),
          apiProvider.overrideWithValue(api),
        ],
        child: const ClubPayApp(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Ваша бронь · Pilot'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Изменить бронь'));
    await tester.pumpAndSettle();
    expect(find.byType(ReservationScreen), findsOneWidget);
    expect(find.text('Перенести бронь'), findsWidgets);
    expect(
      find.text('QR не распознан. Используйте QR ClubPay с экрана компьютера.'),
      findsNothing,
    );
  });

  testWidgets('Home shows club search and has no QR scanner action', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    await tester.pumpWidget(await app());
    await tester.pumpAndSettle();

    expect(find.text('Выберите компьютерный клуб'), findsOneWidget);
    expect(find.text('Pilot'), findsOneWidget);
    expect(find.byKey(const ValueKey('bottom-qr')), findsNothing);
    expect(find.byIcon(Icons.qr_code_scanner), findsNothing);
    expect(find.text('+998900000001'), findsNothing);
  });

  for (final locale in ['ru', 'uz']) {
    testWidgets('$locale home and profile fit 320px with large text', (
      tester,
    ) async {
      await tester.binding.setSurfaceSize(const Size(320, 740));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      tester.platformDispatcher.textScaleFactorTestValue = 1.5;
      addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
      await tester.pumpWidget(await app(locale: locale));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      await tester.tap(find.text(locale == 'ru' ? 'Профиль' : 'Profil').last);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });
  }
}
