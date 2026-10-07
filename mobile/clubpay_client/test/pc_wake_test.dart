import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/core/ui.dart';
import 'package:clubpay_client/features/catalog/data/club_catalog_repository.dart';
import 'package:clubpay_client/features/catalog/domain/club_catalog.dart';
import 'package:clubpay_client/features/catalog/presentation/club_browser_screen.dart';
import 'package:clubpay_client/l10n/generated/app_localizations.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'helpers.dart';

DioException responseError(String path, int status, String code) {
  final request = RequestOptions(path: path);
  return DioException(
    requestOptions: request,
    type: DioExceptionType.badResponse,
    response: Response(
      requestOptions: request,
      statusCode: status,
      data: {'error': code},
    ),
  );
}

Widget localized(Widget child, String locale) => MaterialApp(
  locale: Locale(locale),
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  home: child,
);

class WakeCatalog extends ClubCatalogRepository {
  WakeCatalog(this.failure) : super(ApiClient(SessionVault(MemoryStore())));
  final Object? failure;
  int reads = 0, wakes = 0;

  @override
  Future<ClubCatalog> club(String id) async {
    reads++;
    return ClubCatalog.fromJson({
      'club': {
        'club_id': id,
        'club_name': 'Pilot',
        'club_online': true,
        'zones': [
          {
            'zone_id': 'zone',
            'zone_name': 'Standard',
            'pcs': [
              {
                'id': 'laptop',
                'label': 'Pilot laptop [TEST]',
                'number': 2,
                'status': reads == 1 ? 'offline' : 'available',
                'qr_token': 'fixture',
              },
            ],
          },
        ],
      },
    });
  }

  @override
  Future<void> wake(String pcID) async {
    expect(pcID, 'laptop');
    wakes++;
    if (failure != null) throw failure!;
  }
}

void main() {
  for (final locale in ['ru', 'uz']) {
    testWidgets(
      'Wake errors use PC messages, keep auth/rate/payment meaning ($locale)',
      (tester) async {
        late BuildContext context;
        await tester.pumpWidget(
          localized(
            Builder(
              builder: (value) {
                context = value;
                return const SizedBox();
              },
            ),
            locale,
          ),
        );
        await tester.pumpAndSettle();
        final l = context.l;
        const path = '/api/mobile/pcs/laptop/wake';
        for (final status in [400, 409, 502, 503]) {
          expect(
            errorLabel(
              context,
              responseError(path, status, 'wake failed: unavailable'),
            ),
            l.pcWakeUnavailable,
          );
        }
        expect(
          errorLabel(context, responseError(path, 409, 'pc is already online')),
          l.pcAlreadyOnline,
        );
        expect(
          errorLabel(context, responseError(path, 404, 'pc not found')),
          l.pcNotFound,
        );
        expect(
          errorLabel(context, responseError(path, 401, 'unauthorized')),
          l.sessionExpired,
        );
        expect(
          errorLabel(context, responseError(path, 429, 'rate_limited')),
          l.rateLimited,
        );
        expect(
          errorLabel(context, responseError('/api/checkouts', 409, 'pending')),
          l.operationHelp,
        );
        expect(
          errorLabel(
            context,
            responseError('/api/qr/fixture', 404, 'not found'),
          ),
          l.qrNotFound,
        );
      },
    );
  }

  for (final failure in [
    null,
    responseError('/api/mobile/pcs/laptop/wake', 409, 'pc is already online'),
    responseError(
      '/api/mobile/pcs/laptop/wake',
      409,
      'wake-on-LAN is not configured for this club',
    ),
  ]) {
    testWidgets(
      'Wake refreshes stale catalog after ${failure == null ? 'ACK' : failure.response?.data}',
      (tester) async {
        final repository = WakeCatalog(failure);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              clubCatalogRepositoryProvider.overrideWithValue(repository),
            ],
            child: localized(const ClubDetailScreen(clubId: 'pilot'), 'ru'),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Включить'), findsOneWidget);
        await tester.tap(find.text('Pilot laptop [TEST]'));
        await tester.pumpAndSettle();
        expect(repository.wakes, 1);
        expect(repository.reads, 2);
        expect(find.text('Доступен'), findsOneWidget);
        expect(find.text('Включить'), findsNothing);
        expect(
          find.text(
            'Не создавайте повторную оплату. Проверьте статус ещё раз.',
          ),
          findsNothing,
        );
        if (failure == null) {
          expect(
            find.text(
              'Команда на включение отправлена. Обновим список, когда ПК появится в сети.',
            ),
            findsOneWidget,
          );
        } else {
          expect(
            find.text(
              failure.response?.data['error'] == 'pc is already online'
                  ? 'ПК уже на связи. Обновляем его статус.'
                  : 'Не удалось отправить команду включения ПК. Проверьте его статус или обратитесь к администратору клуба.',
            ),
            findsOneWidget,
          );
        }
        await tester.pumpWidget(const SizedBox());
      },
    );
  }

  for (final automatic in [false, true]) {
    testWidgets(
      '${automatic ? 'Timer' : 'Refresh button'} replaces stale offline status',
      (tester) async {
        final repository = WakeCatalog(null);
        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              clubCatalogRepositoryProvider.overrideWithValue(repository),
            ],
            child: localized(const ClubDetailScreen(clubId: 'pilot'), 'ru'),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.text('Включить'), findsOneWidget);
        if (automatic) {
          await tester.pump(const Duration(seconds: 15));
        } else {
          await tester.tap(find.byTooltip('Обновить'));
        }
        await tester.pumpAndSettle();
        expect(tester.takeException(), isNull);
        expect(repository.reads, 2);
        expect(repository.wakes, 0);
        expect(find.text('Доступен'), findsOneWidget);
        expect(find.text('Включить'), findsNothing);
        await tester.pumpWidget(const SizedBox());
      },
    );
  }
}
