import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:clubpay_client/core/api_client.dart';
import 'package:clubpay_client/core/app.dart';
import 'package:clubpay_client/core/providers.dart';
import 'package:clubpay_client/core/secure_store.dart';
import 'package:clubpay_client/features/auth/domain/auth_models.dart';
import 'package:clubpay_client/features/qr/presentation/scanner_view.dart';
import 'helpers.dart';

class CameraProbe extends StatefulWidget {
  const CameraProbe({super.key, required this.onOpen, required this.onClose});
  final VoidCallback onOpen, onClose;
  @override
  State<CameraProbe> createState() => _CameraProbeState();
}

class _CameraProbeState extends State<CameraProbe> {
  @override
  void initState() {
    super.initState();
    widget.onOpen();
  }

  @override
  void dispose() {
    widget.onClose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => const ColoredBox(color: Colors.black);
}

Future<Widget> app({
  String locale = 'ru',
  VoidCallback? onOpen,
  VoidCallback? onClose,
}) async {
  final store = MemoryStore();
  store.values['mobile.locale'] = locale;
  final vault = SessionVault(store);
  await vault.save(const TokenPair('access', 'refresh'));
  final api = ApiClient(
    vault,
    dio: widgetDio(
      (r) async => switch (r.path) {
        '/api/mobile/me' => (200, {'id': 'player', 'phone': '+998900000001'}),
        '/api/mobile/balances' => (200, {'balances': []}),
        _ => throw StateError('Unexpected API request ${r.path}'),
      },
    ),
  );
  return ProviderScope(
    overrides: [
      secureStoreProvider.overrideWithValue(store),
      apiProvider.overrideWithValue(api),
      scannerBuilderProvider.overrideWithValue(
        (_) => CameraProbe(onOpen: onOpen ?? () {}, onClose: onClose ?? () {}),
      ),
    ],
    child: const ClubPayApp(),
  );
}

void main() {
  testWidgets('Round bottom QR immediately opens camera; close releases it', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    var opened = 0, closed = 0;
    final widget = await app(onOpen: () => opened++, onClose: () => closed++);
    await tester.pumpWidget(widget);
    await tester.pumpAndSettle();
    expect(
      tester.getSize(find.byKey(const ValueKey('bottom-qr'))),
      const Size.square(56),
    );
    expect(opened, 0);
    expect(find.text('+998900000001'), findsNothing);
    expect(find.text('Начать с баланса'), findsNothing);
    expect(find.text('Купить игровое время'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('bottom-qr')));
    await tester.pumpAndSettle();
    expect(opened, 1);
    expect(find.text('Открыть камеру'), findsNothing);
    expect(find.byKey(const ValueKey('manual-qr')), findsNothing);
    expect(find.text('Фото QR'), findsNothing);
    expect(find.byType(TextField), findsNothing);
    await tester.tap(find.byTooltip('Закрыть'));
    await tester.pumpAndSettle();
    expect(closed, 1);
    expect(find.byKey(const ValueKey('bottom-qr')), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  for (final locale in ['ru', 'uz']) {
    testWidgets('$locale home/profile/scan fit 320px and large text', (
      tester,
    ) async {
      await tester.binding.setSurfaceSize(const Size(320, 740));
      addTearDown(() => tester.binding.setSurfaceSize(null));
      tester.platformDispatcher.textScaleFactorTestValue = 1.5;
      addTearDown(tester.platformDispatcher.clearTextScaleFactorTestValue);
      await tester.pumpWidget(await app(locale: locale));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      final qr = find.byKey(const ValueKey('bottom-qr'));
      expect(tester.getBottomRight(qr).dy, lessThanOrEqualTo(740));
      await tester.tap(find.text(locale == 'ru' ? 'Профиль' : 'Profil').last);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      await tester.tap(qr);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(find.byKey(const ValueKey('manual-qr')), findsNothing);
    });
  }
}
