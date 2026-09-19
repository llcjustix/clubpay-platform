import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/ui.dart';
import 'package:clubpay_client/features/catalog/domain/club_catalog.dart';
import 'package:clubpay_client/features/catalog/presentation/club_browser_screen.dart';
import 'package:clubpay_client/features/profile/domain/club_balance.dart';
import 'package:clubpay_client/features/profile/presentation/profile_screen.dart';
import 'package:clubpay_client/l10n/generated/app_localizations.dart';
import 'helpers.dart';

Widget localized(Widget child, String locale) => MaterialApp(
  locale: Locale(locale),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  home: Scaffold(body: SingleChildScrollView(child: child)),
);
void main() {
  testWidgets('Uzbek catalog counts and tariff durations are localized', (
    tester,
  ) async {
    await tester.pumpWidget(
      localized(
        Column(
          children: [
            ClubCatalogRow(
              club: const ClubSearchResult(
                id: 'pilot',
                name: 'Pilot',
                address: '',
                online: true,
                availablePCs: 5,
                totalPCs: 10,
              ),
              onTap: () {},
            ),
            Builder(
              builder: (context) => Text(
                '${tariffLabel(context, 30 * 60)} / '
                '${tariffLabel(context, 2 * 60 * 60)}',
              ),
            ),
          ],
        ),
        'uz',
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('10 tadan 5 ta bo‘sh kompyuter'), findsOneWidget);
    expect(find.text('30 daqiqa / 2 soat'), findsOneWidget);
  });

  testWidgets('Unauthenticated startup renders login, not fake profile', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [secureStoreProvider.overrideWithValue(MemoryStore())],
        child: const ClubPayApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Твоё время.\nТвоя игра.'), findsOneWidget);
    expect(find.text('Игровое время'), findsNothing);
    expect(
      find.text('Вводя номер, вы соглашаетесь с публичной офертой'),
      findsOneWidget,
    );
  });
  for (final locale in ['ru', 'uz']) {
    testWidgets('$locale shows club game balance in sums', (tester) async {
      await tester.pumpWidget(
        localized(const BalanceList(balances: []), locale),
      );
      await tester.pumpAndSettle();
      expect(
        find.text(
          locale == 'ru'
              ? 'Игрового баланса пока нет'
              : 'Hozircha o‘yin balansi yo‘q',
        ),
        findsOneWidget,
      );
      await tester.pumpWidget(
        localized(
          const BalanceList(
            balances: [
              ClubBalance('one', 'Club One', 3661, 15000),
              ClubBalance('two', 'Club Two', 0, 0),
            ],
          ),
          locale,
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Club One'), findsOneWidget);
      expect(find.text('Club Two'), findsOneWidget);
      expect(
        find.textContaining(locale == 'ru' ? 'сум' : 'so‘m'),
        findsNWidgets(2),
      );
      expect(
        find.text(
          locale == 'ru' ? 'Игровой баланс: 0 сум' : 'O‘yin balansi: 0 so‘m',
        ),
        findsOneWidget,
      );
    });
  }
  testWidgets('Language switch persists and changes auth strings', (
    tester,
  ) async {
    final store = MemoryStore();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [secureStoreProvider.overrideWithValue(store)],
        child: const ClubPayApp(),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('O‘zbekcha'));
    await tester.pumpAndSettle();
    expect(find.text('Vaqtingiz.\nO‘yiningiz.'), findsOneWidget);
    expect(store.values['mobile.locale'], 'uz');
  });
  testWidgets('Phone layout has no overflow at 390px and large text', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    tester.platformDispatcher.textScaleFactorTestValue = 1.5;
    addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [secureStoreProvider.overrideWithValue(MemoryStore())],
        child: const ClubPayApp(),
      ),
    );
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });
}
