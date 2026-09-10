import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/auth/domain/auth_models.dart';
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
