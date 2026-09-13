import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'helpers.dart';

void main() {
  testWidgets('Phone → OTP → club → PC → checkout → pending grant → accepted', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(600, 1800));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final store = MemoryStore();
    final vault = SessionVault(store);
    var checkoutCount = 0;
    var orderChecks = 0;
    final api = ApiClient(
      vault,
      dio: widgetDio((r) async {
        switch (r.path) {
          case '/api/mobile/auth/challenge':
            return (
              201,
              {
                'challenge': 'm_fixture',
                'telegram_link': 'https://t.me/clubpay_test',
                'expires_at': DateTime.now()
                    .add(const Duration(minutes: 5))
                    .toIso8601String(),
              },
            );
          case '/api/mobile/auth/verify':
            return (
              200,
              {
                'access_token': 'mob_a_fixture',
                'refresh_token': 'mob_r_fixture',
              },
            );
          case '/api/mobile/me':
            return (
              200,
              {'id': 'fixture', 'phone': '+998901234567', 'first_name': 'Ali'},
            );
          case '/api/mobile/clubs':
            return (
              200,
              {
                'clubs': [
                  {
                    'club_id': 'club',
                    'club_name': 'Test Club',
                    'address': 'Tashkent',
                    'club_online': true,
                    'available_pcs': 1,
                  },
                ],
              },
            );
          case '/api/mobile/clubs/club':
            return (
              200,
              {
                'club': {
                  'club_id': 'club',
                  'club_name': 'Test Club',
                  'address': 'Tashkent',
                  'club_online': true,
                  'zones': [
                    {
                      'zone_id': 'standard',
                      'zone_name': 'Standard',
                      'hourly_price_uzs': 10000,
                      'pcs': [
                        {
                          'id': 'pc',
                          'label': 'PC 07',
                          'number': 7,
                          'status': 'available',
                          'qr_token': 'pc_fixture',
                        },
                      ],
                    },
                  ],
                },
              },
            );
          case '/api/mobile/balances':
            return (200, {'balances': []});
          case '/api/qr/pc_fixture':
            return (
              200,
              {
                'club': {'id': 'club', 'name': 'Test Club'},
                'pc': {'label': 'PC 07', 'status': 'available'},
                'zone': {'name': 'Standard', 'hourly_price_uzs': 10000},
                'qr_type': 'static_pc',
                'tariffs': [
                  {
                    'id': 'tariff',
                    'name': '60 минут',
                    'duration_minutes': 60,
                    'price_uzs': 10000,
                  },
                ],
                'payment_providers': [
                  {'provider': 'click', 'configured': true},
                ],
              },
            );
          case '/api/checkouts':
            checkoutCount++;
            return (
              201,
              {
                'order': {'invoice_id': 'cp_fixture'},
                'checkout_url': 'https://my.click.uz/test',
              },
            );
          case '/api/mobile/orders/cp_fixture':
            orderChecks++;
            return (
              200,
              {
                'status': 'paid',
                'grant_status': orderChecks < 2 ? 'pending' : 'accepted',
                'pc_label': 'PC 07',
                'session_seconds': 3600,
              },
            );
        }
        fail('Unexpected request ${r.path}');
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
    await tester.enterText(find.byType(TextField), '+998901234567');
    await tester.tap(find.text('Продолжить'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField).last, '123456');
    await tester.pump();
    await tester.tap(find.text('Войти'));
    await tester.pumpAndSettle();
    expect(find.text('Test Club'), findsOneWidget);
    await tester.tap(find.text('Test Club'));
    await tester.pumpAndSettle();
    expect(find.text('Standard · 10000 сум/ч'), findsOneWidget);
    await tester.tap(find.text('PC 07'));
    await tester.pumpAndSettle();
    expect(find.text('PC 07'), findsOneWidget);
    await tester.tap(find.text('Оплатить'));
    await tester.pumpAndSettle();
    expect(find.text('Запускаем сессию'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
    expect(find.text('Сессия запущена'), findsOneWidget);
    expect(checkoutCount, 1);
    expect(await vault.store.read('mobile.pending'), isNull);
  });
}
