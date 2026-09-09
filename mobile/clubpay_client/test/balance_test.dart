import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/club_theme.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/profile/data/profile_repository.dart';
import 'package:clubpay_client/features/profile/domain/club_balance.dart';
import 'package:clubpay_client/features/profile/presentation/profile_screen.dart';
import 'package:clubpay_client/l10n/generated/app_localizations.dart';
import 'helpers.dart';

Map<String, dynamic> balance() => {
  'club_id': 'pilot',
  'club_name': 'Pilot Network',
  'seconds_balance': 3600,
  'updated_at': '2026-09-09T10:00:00Z',
  'club_online': true,
  'zones': [
    {
      'id': 'standard',
      'name': 'Standard',
      'hourly_price_tiyin': 1500000,
      'seconds_available': 3600,
    },
    {
      'id': 'vip',
      'name': 'VIP',
      'hourly_price_tiyin': 3000000,
      'seconds_available': 1800,
    },
  ],
};

void main() {
  test(
    'Offline cache preserves zone equivalents and is isolated by account',
    () async {
      var online = true;
      final api = ApiClient(
        SessionVault(MemoryStore()),
        dio: stubDio(
          (_) async => online
              ? (
                  200,
                  {
                    'balances': [balance()],
                  },
                )
              : (503, {'error': 'unavailable'}),
        ),
      );
      final repo = ProfileRepository(api, playerId: 'one');
      final fresh = (await repo.balances()).single;
      expect(fresh.secondsForZone('VIP'), 1800);
      expect(fresh.stale, isFalse);
      online = false;
      final cached = (await repo.balances()).single;
      expect(cached.secondsForZone('Standard'), 3600);
      expect(cached.stale, isTrue);
      expect(cached.updatedAt, DateTime.utc(2026, 9, 9, 10));
      await expectLater(
        ProfileRepository(api, playerId: 'two').balances(),
        throwsA(isA<DioException>()),
      );
    },
  );
  test('An unavailable API never becomes an empty club balance', () async {
    var online = true;
    final api = ApiClient(
      SessionVault(MemoryStore()),
      dio: stubDio(
        (_) async => online
            ? (200, <String, dynamic>{'balances': []})
            : (503, <String, dynamic>{}),
      ),
    );
    final repo = ProfileRepository(api, playerId: 'one');
    expect(await repo.balances(), isEmpty);
    online = false;
    await expectLater(repo.balances(), throwsA(isA<DioException>()));
  });
  for (final language in ['ru', 'uz']) {
    testWidgets('$language club and zone list fits large text at 320px', (
      tester,
    ) async {
      await tester.binding.setSurfaceSize(const Size(320, 1000));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      tester.platformDispatcher.textScaleFactorTestValue = 1.5;
      addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
      await tester.pumpWidget(
        MaterialApp(
          theme: clubTheme(),
          locale: Locale(language),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(
            body: SingleChildScrollView(
              child: BalanceList(
                balances: [ClubBalance.fromJson(balance(), stale: true)],
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Pilot Network'), findsOneWidget);
      expect(find.text('Standard'), findsOneWidget);
      expect(find.text('VIP'), findsOneWidget);
      expect(
        find.text(language == 'ru' ? '0 ч 30 мин 0 с' : '0 soat 30 daq 0 son'),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
    });
  }
}
