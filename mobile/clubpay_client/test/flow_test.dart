import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'helpers.dart';

void main() {
  for (final redeem in [false, true]) {
    testWidgets(
      'Phone → OTP → club → PC → ${redeem ? '35-hour balance with old payment' : 'checkout'} → accepted',
      (tester) async {
        await tester.binding.setSurfaceSize(const Size(600, 1800));
        addTearDown(() => tester.binding.setSurfaceSize(null));
        final store = MemoryStore();
        final vault = SessionVault(store);
        if (redeem) {
          await store.write(
            'mobile.pending',
            '{"key":"legacy-payment","invoice":"cp_old_next_pc"}',
          );
        }
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
                  {
                    'id': 'fixture',
                    'phone': '+998901234567',
                    'first_name': 'Ali',
                  },
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
                return (
                  200,
                  {
                    'balances': redeem
                        ? [
                            {
                              'club_id': 'club',
                              'club_name': 'Test Club',
                              'seconds_balance': 127129,
                              'balance_uzs': 529704,
                              'updated_at': DateTime.now().toIso8601String(),
                              'club_online': true,
                              'zones': [
                                {
                                  'id': 'standard',
                                  'name': 'Standard',
                                  'hourly_price_tiyin': 1000000,
                                  'seconds_available': 127129,
                                },
                              ],
                            },
                          ]
                        : [],
                  },
                );
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
                expect(redeem, isFalse);
                checkoutCount++;
                return (
                  201,
                  {
                    'order': {'invoice_id': 'cp_fixture'},
                    'checkout_url': 'https://my.click.uz/test',
                  },
                );
              case '/api/player-balance/redeem':
                expect(redeem, isTrue);
                expect(r.data['qr_token'], 'pc_fixture');
                checkoutCount++;
                return (200, {'grant_id': 'grant_35_hours'});
              case '/api/mobile/orders/cp_fixture':
              case '/api/mobile/sessions/grant_35_hours':
                orderChecks++;
                return (
                  200,
                  {
                    'status': 'paid',
                    'grant_status': orderChecks < 2 ? 'pending' : 'accepted',
                    'pc_label': 'PC 07',
                    'session_seconds': redeem ? 127129 : 3600,
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
        if (redeem) {
          expect(find.text('35 ч 18 мин 49 с'), findsOneWidget);
        }
        await tester.tap(find.text(redeem ? 'Использовать время' : 'Оплатить'));
        await tester.pumpAndSettle();
        expect(find.text('Запускаем сессию'), findsOneWidget);
        if (redeem) {
          expect(find.text('35 ч 18 мин 49 с'), findsOneWidget);
          expect(find.text('Next PC #01'), findsNothing);
          expect(find.text('PC 07'), findsOneWidget);
        }
        await tester.pump(const Duration(seconds: 3));
        await tester.pumpAndSettle();
        expect(find.text('Сессия запущена'), findsOneWidget);
        expect(checkoutCount, 1);
        if (redeem) {
          expect(
            await vault.store.read('mobile.pending'),
            contains('cp_old_next_pc'),
          );
        } else {
          expect(await vault.store.read('mobile.pending'), isNull);
        }
      },
    );
  }
}
