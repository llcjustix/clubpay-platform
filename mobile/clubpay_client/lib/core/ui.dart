import 'package:dio/dio.dart';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import 'package:url_launcher/url_launcher.dart';
import '../l10n/generated/app_localizations.dart';
import 'providers.dart';
import 'club_theme.dart';

extension LocalizedContext on BuildContext {
  AppLocalizations get l => AppLocalizations.of(this);
}

String timeLabel(BuildContext context, int seconds) =>
    context.l.duration(seconds ~/ 3600, (seconds % 3600) ~/ 60, seconds % 60);

String moneyLabel(BuildContext context, int amount) {
  final locale = Localizations.localeOf(context).toLanguageTag();
  return '${NumberFormat.decimalPattern(locale).format(amount)} ${context.l.currencySuffix}';
}

String errorLabel(BuildContext context, Object error) {
  final l = context.l;
  if (error is DioException) {
    final data = error.response?.data;
    final code = data is Map ? data['error'] : null;
    if (error.response?.statusCode == 429) return l.rateLimited;
    if (code == 'otp_invalid_or_expired') return l.invalidOtp;
    if (code == 'qr_catalog_unavailable') return l.qrCatalogUnavailable;
    if (error.response?.statusCode == 401) return l.sessionExpired;
    if (error.response?.statusCode == 404) return l.qrNotFound;
    if (error.response?.statusCode == 503) return l.serviceUnavailable;
    if (error.response?.statusCode == 409) return l.operationHelp;
    if (error.response?.statusCode == 400) return l.paymentUnavailable;
  }
  if (error is PlatformException) return l.storageError;
  if (error is FormatException) {
    if (error.message == 'image_too_large') return l.imageTooLarge;
    if (error.message == 'operation_failed') return l.paymentUnavailable;
    return l.invalidQr;
  }
  return l.networkError;
}

void showFailure(BuildContext context, Object error) => ScaffoldMessenger.of(
  context,
).showSnackBar(SnackBar(content: Text(errorLabel(context, error))));
Future<void> openExternal(String value) async {
  final uri = Uri.parse(value);
  final local = uri.host == 'localhost' || uri.host == '127.0.0.1';
  if (uri.scheme != 'https' && !(uri.scheme == 'http' && local)) {
    throw const FormatException('invalid_external_url');
  }
  if (!await launchUrl(
    uri,
    mode: LaunchMode.externalApplication,
    webOnlyWindowName: '_blank',
  )) {
    throw StateError('launch_failed');
  }
}

class AppPage extends StatelessWidget {
  const AppPage({
    super.key,
    required this.title,
    required this.children,
    this.bottom,
    this.actions,
    this.leading,
    this.largeTitle = false,
  });
  final String title;
  final List<Widget> children;
  final Widget? bottom, leading;
  final List<Widget>? actions;
  final bool largeTitle;
  @override
  Widget build(BuildContext context) => Scaffold(
    appBar: largeTitle
        ? null
        : AppBar(
            title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
            actions: actions,
            automaticallyImplyLeading: false,
            leading:
                leading ??
                (Navigator.canPop(context)
                    ? Padding(
                        padding: const EdgeInsets.all(5),
                        child: IconButton.filledTonal(
                          tooltip: MaterialLocalizations.of(
                            context,
                          ).backButtonTooltip,
                          style: IconButton.styleFrom(
                            backgroundColor: ClubColors.surface,
                          ),
                          onPressed: () => Navigator.maybePop(context),
                          icon: const Icon(
                            CupertinoIcons.chevron_back,
                            size: 23,
                          ),
                        ),
                      )
                    : null),
          ),
    bottomNavigationBar: bottom,
    body: SafeArea(
      child: Align(
        alignment: Alignment.topCenter,
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 560),
          child: ListView(
            padding: EdgeInsets.fromLTRB(16, largeTitle ? 48 : 24, 16, 32),
            children: [
              if (largeTitle)
                Padding(
                  padding: const EdgeInsets.only(bottom: 16, left: 4),
                  child: Text(
                    title,
                    style: Theme.of(context).textTheme.headlineLarge,
                  ),
                ),
              ...children,
            ],
          ),
        ),
      ),
    ),
  );
}

/// iOS inset grouped section. Separators align with the row's text column.
class SettingsGroup extends StatelessWidget {
  const SettingsGroup({super.key, required this.children, this.inset = 60});
  final List<Widget> children;
  final double inset;
  @override
  Widget build(BuildContext context) => ClipRRect(
    borderRadius: BorderRadius.circular(26),
    child: ColoredBox(
      color: ClubColors.surface,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (var i = 0; i < children.length; i++) ...[
            if (i > 0)
              Padding(
                padding: EdgeInsets.only(left: inset, right: 16),
                child: const Divider(),
              ),
            children[i],
          ],
        ],
      ),
    ),
  );
}

class SettingsIcon extends StatelessWidget {
  const SettingsIcon(this.icon, {super.key, this.color = ClubColors.blue});
  final IconData icon;
  final Color color;
  @override
  Widget build(BuildContext context) => Container(
    width: 29,
    height: 29,
    decoration: BoxDecoration(
      color: color,
      borderRadius: BorderRadius.circular(7),
    ),
    child: Icon(icon, color: Colors.white, size: 22),
  );
}

class SettingsRow extends StatelessWidget {
  const SettingsRow({
    super.key,
    required this.title,
    this.icon,
    this.color = ClubColors.blue,
    this.subtitle,
    this.trailing,
    this.onTap,
    this.selected = false,
  });
  final String title;
  final IconData? icon;
  final Color color;
  final String? subtitle;
  final Widget? trailing;
  final VoidCallback? onTap;
  final bool selected;
  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, rowConstraints) => Semantics(
      selected: selected,
      button: onTap != null,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: 52),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 11),
              child: Row(
                children: [
                  if (icon != null) ...[
                    SettingsIcon(icon!, color: color),
                    const SizedBox(width: 15),
                  ],
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          title,
                          style: const TextStyle(
                            fontSize: 17,
                            height: 1.3,
                            fontWeight: FontWeight.w400,
                            letterSpacing: -.4,
                          ),
                        ),
                        if (subtitle != null) ...[
                          const SizedBox(height: 3),
                          Text(
                            subtitle!,
                            style: const TextStyle(
                              fontSize: 13,
                              height: 1.35,
                              color: ClubColors.muted,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ),
                  if (trailing != null) ...[
                    const SizedBox(width: 10),
                    ConstrainedBox(
                      constraints: BoxConstraints(
                        maxWidth: rowConstraints.maxWidth * .35,
                      ),
                      child: DefaultTextStyle(
                        textAlign: TextAlign.right,
                        style: Theme.of(context).textTheme.bodyLarge!.copyWith(
                          color: ClubColors.muted,
                        ),
                        child: trailing!,
                      ),
                    ),
                  ],
                  if (selected) ...[
                    const SizedBox(width: 10),
                    const Icon(
                      CupertinoIcons.check_mark,
                      color: ClubColors.blue,
                      size: 20,
                    ),
                  ] else if (onTap != null) ...[
                    const SizedBox(width: 10),
                    const Icon(
                      CupertinoIcons.chevron_right,
                      color: Color(0xff636366),
                      size: 16,
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

class SectionCaption extends StatelessWidget {
  const SectionCaption(this.text, {super.key});
  final String text;
  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
    child: Text(
      text,
      style: const TextStyle(
        fontSize: 13,
        height: 1.3,
        color: ClubColors.muted,
      ),
    ),
  );
}

class ActionButton extends StatelessWidget {
  const ActionButton({
    super.key,
    required this.label,
    required this.onPressed,
    this.icon,
    this.secondary = false,
    this.busy = false,
  });
  final String label;
  final VoidCallback? onPressed;
  final IconData? icon;
  final bool secondary, busy;
  @override
  Widget build(BuildContext context) {
    final child = Text(label, textAlign: TextAlign.center);
    final symbol = busy
        ? const SizedBox(
            width: 20,
            height: 20,
            child: CupertinoActivityIndicator(color: Colors.white),
          )
        : (icon == null ? null : Icon(icon, size: 20));
    return Padding(
      padding: const EdgeInsets.only(top: 12),
      child: SizedBox(
        width: double.infinity,
        child: secondary
            ? OutlinedButton.icon(
                onPressed: busy ? null : onPressed,
                icon: symbol,
                label: child,
              )
            : FilledButton.icon(
                onPressed: busy ? null : onPressed,
                icon: symbol,
                label: child,
              ),
      ),
    );
  }
}

class InfoCard extends StatelessWidget {
  const InfoCard({super.key, required this.children, this.accent = false});
  final List<Widget> children;
  final bool accent;
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(16),
    margin: const EdgeInsets.only(bottom: 24),
    decoration: BoxDecoration(
      color: ClubColors.surface,
      borderRadius: BorderRadius.circular(26),
    ),
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: children,
    ),
  );
}

class LanguagePicker extends ConsumerWidget {
  const LanguagePicker({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final current = ref.watch(localeProvider).languageCode;
    return CupertinoSlidingSegmentedControl<String>(
      backgroundColor: ClubColors.elevated,
      thumbColor: const Color(0xff636366),
      groupValue: current,
      children: {
        'ru': Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 7),
          child: Text(
            context.l.russian,
            style: const TextStyle(fontSize: 13, color: Colors.white),
          ),
        ),
        'uz': Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 7),
          child: Text(
            context.l.uzbek,
            style: const TextStyle(fontSize: 13, color: Colors.white),
          ),
        ),
      },
      onValueChanged: (value) async {
        if (value == null) return;
        try {
          await ref.read(localeProvider.notifier).select(value);
        } catch (e) {
          if (context.mounted) showFailure(context, e);
        }
      },
    );
  }
}

const gap = SizedBox(height: 16);
