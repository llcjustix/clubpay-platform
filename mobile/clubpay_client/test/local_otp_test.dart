import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/dev_mode.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/auth/domain/auth_models.dart';
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
  test('Sandbox code requires explicit debug mode and loopback API', () {
    expect(
      AuthChallenge.fromJson(challenge()).developmentOtp,
      localOtpTestMode ? '018910' : isNull,
    );
  });

  testWidgets(
    'Local OTP is labelled and entered manually before profile access',
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
      expect(find.text('Тестовый режим'), findsOneWidget);
      await tester.enterText(find.byType(TextField), '+998900000001');
      await tester.tap(find.text('Продолжить'));
      await tester.pumpAndSettle();
      expect(find.text('018910'), findsOneWidget);
      expect(find.text('Открыть Telegram'), findsNothing);
      expect(verified, isFalse);
      await tester.enterText(find.byType(TextField), '018910');
      await tester.pumpAndSettle();
      await tester.tap(find.text('Войти'));
      await tester.pumpAndSettle();
      expect(find.text('У вас пока нет игрового времени'), findsOneWidget);
      await tester.pumpWidget(const SizedBox.shrink());
    },
    skip: !localOtpTestMode,
  );
}
