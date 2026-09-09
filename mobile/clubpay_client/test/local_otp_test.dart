import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'helpers.dart';

Map<String, dynamic> challenge() => {
  'challenge': 'm_local_fixture',
  'telegram_link': '',
  'expires_at': DateTime.now()
      .add(const Duration(minutes: 3))
      .toIso8601String(),
  'development_otp': '018910',
};

void main() {
  testWidgets(
    'The app always directs the player to Telegram and never displays an API OTP',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(600, 1800));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      final store = MemoryStore();
      var verified = false;
      final api = ApiClient(
        SessionVault(store),
        dio: widgetDio((request) async {
          switch (request.path) {
            case '/api/mobile/auth/challenge':
              return (201, challenge());
            case '/api/mobile/auth/verify':
              expect(request.data['otp'], '018910');
              verified = true;
              return (
                200,
                {'access_token': 'access', 'refresh_token': 'refresh'},
              );
            case '/api/mobile/me':
              expect(verified, isTrue);
              return (200, {'id': 'local', 'phone': '+998900000001'});
            case '/api/mobile/balances':
              return (200, {'balances': []});
          }
          fail('Unexpected API call ${request.path}');
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
      await tester.enterText(find.byType(TextField), '+998900000001');
      await tester.tap(find.text('Продолжить'));
      await tester.pumpAndSettle();
      expect(find.text('018910'), findsNothing);
      expect(find.text('Открыть Telegram'), findsOneWidget);
      expect(find.text('Код из Telegram'), findsOneWidget);
      expect(verified, isFalse);
      await tester.enterText(find.byType(TextField), '018910');
      await tester.pumpAndSettle();
      await tester.tap(find.text('Войти'));
      await tester.pumpAndSettle();
      expect(find.text('У вас пока нет игрового времени'), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );
}
