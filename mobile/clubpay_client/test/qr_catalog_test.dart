import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/club_theme.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/auth/domain/auth_models.dart';
import 'package:clubpay_client/features/profile/domain/club_balance.dart';
import 'package:clubpay_client/features/qr/data/qr_repository.dart';
import 'package:clubpay_client/features/qr/domain/qr_models.dart';
import 'package:clubpay_client/features/qr/presentation/computer_screen.dart';
import 'package:clubpay_client/features/qr/presentation/qr_screen.dart';
import 'package:clubpay_client/features/qr/presentation/scanner_view.dart';
import 'package:clubpay_client/l10n/generated/app_localizations.dart';
import 'helpers.dart';

Map<String, dynamic> livePc() => {
  'club': {'id': 'live', 'name': 'Live Club'},
  'pc': {'label': 'PC 7', 'status': 'available'},
  'zone': {'name': 'Standard', 'hourly_price_uzs': 10000},
  'qr_type': 'static_pc',
  'development_catalog_preview': true,
  'tariffs': [
    {'id': 't', 'name': '1 час', 'price_uzs': 10000, 'duration_minutes': 60},
  ],
  'payment_providers': [
    {'provider': 'click', 'configured': true},
  ],
};

Widget localized(Widget home) => MaterialApp(
  theme: clubTheme(),
  locale: const Locale('ru'),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  home: home,
);

void main() {
  test(
    'Actual ClubPay URL parses; public lookup never sends local credentials',
    () async {
      const token =
          'pc_9fa6a7d6ec327a8242fed53307b021b8d67593f9cf3664563e5d09d539801b94';
      final vault = SessionVault(MemoryStore());
      await vault.save(const TokenPair('mob_a_local', 'mob_r_local'));
      final api = ApiClient(
        vault,
        dio: stubDio((r) async {
          expect(r.path, '/api/qr/$token');
          expect(r.headers['Authorization'], isNull);
          return (200, livePc());
        }),
      );
      final pc = await QrRepository(
        api,
      ).resolve('https://clubpay.justix.uz/qr/$token');
      expect(pc.previewOnly, isTrue);
      expect(pc.canStart, isFalse);
      expect(
        QrComputer.fromJson(token, {
          ...livePc(),
          'qr_type': 'session_extend',
          'pc': {'label': 'PC', 'status': 'occupied'},
        }).canStart,
        isFalse,
      );
    },
  );

  testWidgets('Live preview shows catalog but no payment or local balance', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(600, 1400));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final api = ApiClient(
      SessionVault(MemoryStore()),
      dio: widgetDio((r) async => (200, livePc())),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiProvider.overrideWithValue(api),
          balancesProvider.overrideWith(
            (_) async => [const ClubBalance('live', 'Live Club', 7200, 30000)],
          ),
        ],
        child: localized(const ComputerScreen(token: 'pc_live')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Рабочий ПК · просмотр'), findsOneWidget);
    expect(find.text('PC 7'), findsOneWidget);
    expect(find.text('1 час'), findsOneWidget);
    expect(find.text('Оплатить'), findsNothing);
    expect(find.text('Ваше время в этом клубе'), findsNothing);
    expect(find.text('2 ч 0 мин 0 с'), findsNothing);
  });

  testWidgets(
    'Catalog outage is distinct from expired QR; retry preserves scan',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(390, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      var requests = 0;
      final dio = Dio(BaseOptions(baseUrl: 'http://localhost:8088'));
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (r, h) {
            requests++;
            expect(r.path, '/api/qr/pc_live');
            h.reject(
              DioException(
                requestOptions: r,
                type: DioExceptionType.badResponse,
                response: Response(
                  requestOptions: r,
                  statusCode: 503,
                  data: {'error': 'qr_catalog_unavailable'},
                ),
              ),
            );
          },
        ),
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            apiProvider.overrideWithValue(
              ApiClient(SessionVault(MemoryStore()), dio: dio),
            ),
            scannerBuilderProvider.overrideWithValue(
              (onCode) => Center(
                child: TextButton(
                  onPressed: () => onCode('pc_live'),
                  child: const Text('Simulate scan'),
                ),
              ),
            ),
          ],
          child: localized(const QrScreen()),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('Simulate scan'));
      await tester.pumpAndSettle();
      expect(
        find.textContaining('сервер клуба сейчас недоступен'),
        findsOneWidget,
      );
      expect(find.textContaining('Вход через Telegram'), findsNothing);
      await tester.tap(find.byKey(const ValueKey('retry-qr')));
      await tester.pumpAndSettle();
      expect(requests, 2);
      expect(tester.takeException(), isNull);
    },
  );
}
