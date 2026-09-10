import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../features/auth/presentation/auth_screen.dart';
import '../features/catalog/presentation/club_browser_screen.dart';
import '../features/profile/presentation/profile_screen.dart';
import '../features/qr/presentation/computer_screen.dart';
import '../features/payment/presentation/session_screen.dart';
import '../l10n/generated/app_localizations.dart';
import 'providers.dart';
import 'club_theme.dart';
import 'ui.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final refresh = ValueNotifier(0);
  ref.listen(authProvider, (_, next) => refresh.value++);
  final router = GoRouter(
    refreshListenable: refresh,
    redirect: (context, state) {
      final auth = ref.read(authProvider);
      final path = state.uri.path;
      final gate = path == '/splash' || path == '/auth';
      // Carry incoming payment/QR links through session restoration and login.
      final destination = gate
          ? state.uri.queryParameters['next']
          : state.uri.toString();
      if (auth.isLoading || auth.hasError) {
        return path == '/splash'
            ? null
            : Uri(
                path: '/splash',
                queryParameters: {'next': ?destination},
              ).toString();
      }
      if (auth.asData?.value == null) {
        return path == '/auth'
            ? null
            : Uri(
                path: '/auth',
                queryParameters: {'next': ?destination},
              ).toString();
      }
      if (gate) {
        return destination != null &&
                destination.startsWith('/') &&
                !destination.startsWith('//')
            ? destination
            : '/home';
      }
      if (path == '/') return '/home';
      return null;
    },
    routes: [
      GoRoute(path: '/', builder: (_, _) => const SplashScreen()),
      GoRoute(path: '/splash', builder: (_, _) => const SplashScreen()),
      GoRoute(path: '/auth', builder: (_, _) => const AuthScreen()),
      GoRoute(path: '/home', builder: (_, _) => const ClubBrowserScreen()),
      GoRoute(
        path: '/profile',
        builder: (_, _) => const ProfileScreen(profile: true),
      ),
      GoRoute(path: '/scan', redirect: (_, _) => '/home'),
      GoRoute(
        path: '/clubs/:clubId',
        builder: (_, s) =>
            ClubDetailScreen(clubId: s.pathParameters['clubId']!),
      ),
      GoRoute(
        path: '/computer/:token',
        builder: (_, s) => ComputerScreen(token: s.pathParameters['token']!),
      ),
      GoRoute(
        path: '/qr/:token',
        builder: (_, s) => ComputerScreen(token: s.pathParameters['token']!),
      ),
      GoRoute(path: '/session', builder: (_, _) => const SessionScreen()),
      GoRoute(
        path: '/payment/return',
        builder: (_, s) =>
            SessionScreen(invoice: s.uri.queryParameters['invoice_id']),
      ),
    ],
    errorBuilder: (context, state) => AppPage(
      title: context.l.appName,
      children: [
        Text(context.l.invalidQr),
        ActionButton(
          label: context.l.home,
          onPressed: () => context.go('/home'),
        ),
      ],
    ),
  );
  ref.onDispose(() {
    router.dispose();
    refresh.dispose();
  });
  return router;
});

class ClubPayApp extends ConsumerWidget {
  const ClubPayApp({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = clubTheme();
    return MaterialApp.router(
      debugShowCheckedModeBanner: false,
      onGenerateTitle: (context) => context.l.appName,
      theme: theme,
      locale: ref.watch(localeProvider),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      routerConfig: ref.watch(routerProvider),
    );
  }
}

class SplashScreen extends ConsumerWidget {
  const SplashScreen({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    return AppPage(
      title: context.l.appName,
      children: [
        const SizedBox(height: 100),
        const Center(
          child: Icon(
            Icons.sports_esports_outlined,
            size: 64,
            color: ClubColors.blue,
          ),
        ),
        gap,
        if (auth.hasError) ...[
          Text(errorLabel(context, auth.error!)),
          ActionButton(
            label: context.l.refresh,
            onPressed: () => ref.invalidate(authProvider),
          ),
        ] else ...[
          Center(child: Text(context.l.loading)),
          gap,
          const Center(child: CircularProgressIndicator()),
        ],
      ],
    );
  }
}
