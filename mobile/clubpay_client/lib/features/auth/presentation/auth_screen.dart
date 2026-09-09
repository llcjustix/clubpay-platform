import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/cupertino.dart';
import '../../../core/club_theme.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/providers.dart';
import '../../../core/ui.dart';
import '../../../core/dev_mode.dart';
import '../domain/auth_models.dart';

class AuthScreen extends ConsumerStatefulWidget {
  const AuthScreen({super.key});
  @override
  ConsumerState<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends ConsumerState<AuthScreen> {
  final _phone = TextEditingController(text: '+998');
  final _otp = TextEditingController();
  AuthChallenge? _challenge;
  bool _busy = false;
  String? _error;
  Timer? _timer;
  @override
  void dispose() {
    _phone.dispose();
    _otp.dispose();
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _start() async {
    final phone = normalizeUzPhone(_phone.text);
    if (phone == null) {
      setState(() => _error = context.l.phoneError);
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final result = await ref.read(authRepositoryProvider).challenge(phone);
      if (!mounted) return;
      setState(() {
        _challenge = result;
        _otp.clear();
      });
      _timer?.cancel();
      _timer = Timer(
        result.expiresAt.difference(DateTime.now()).isNegative
            ? Duration.zero
            : result.expiresAt.difference(DateTime.now()),
        () {
          if (mounted) setState(() {});
        },
      );
    } catch (e) {
      if (mounted) setState(() => _error = errorLabel(context, e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _verify() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(authProvider.notifier).verify(_challenge!, _otp.text);
    } catch (e) {
      if (mounted) setState(() => _error = errorLabel(context, e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l = context.l;
    final expired = _challenge?.expiresAt.isBefore(DateTime.now()) ?? false;
    final developmentOtp = _challenge?.developmentOtp;
    return AppPage(
      title: l.appName,
      children: [
        const Align(alignment: Alignment.centerRight, child: LanguagePicker()),
        const SizedBox(height: 40),
        Align(
          alignment: Alignment.centerLeft,
          child: Container(
            width: 76,
            height: 76,
            decoration: BoxDecoration(
              color: ClubColors.surface,
              borderRadius: BorderRadius.circular(38),
            ),
            child: const Icon(
              CupertinoIcons.person_crop_circle_fill,
              size: 42,
              color: ClubColors.muted,
            ),
          ),
        ),
        const SizedBox(height: 28),
        Text(
          _challenge == null ? l.welcome : l.telegramTitle,
          style: Theme.of(context).textTheme.displaySmall?.copyWith(
            fontWeight: FontWeight.w700,
            height: 1.2,
          ),
        ),
        gap,
        Text(
          _challenge == null
              ? l.intro
              : (developmentOtp == null ? l.telegramHelp : l.localOtpHelp),
          style: Theme.of(context).textTheme.bodyLarge,
        ),
        const SizedBox(height: 32),
        if (_challenge == null) ...[
          TextField(
            controller: _phone,
            keyboardType: TextInputType.phone,
            autofillHints: const [AutofillHints.telephoneNumber],
            textInputAction: TextInputAction.done,
            decoration: InputDecoration(
              labelText: l.phone,
              hintText: l.phoneHint,
            ),
            onSubmitted: (_) => _busy ? null : _start(),
          ),
          ActionButton(label: l.continueLabel, onPressed: _start, busy: _busy),
          gap,
          Text(
            localOtpTestMode ? l.localSignInHelp : l.signInHelp,
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ] else ...[
          InfoCard(
            children: [
              Text(
                normalizeUzPhone(_phone.text)!,
                style: Theme.of(context).textTheme.titleLarge,
              ),
              if (developmentOtp != null) ...[
                gap,
                Text(l.localOtpLabel),
                SelectableText(
                  developmentOtp,
                  style: Theme.of(context).textTheme.headlineMedium,
                ),
              ] else
                ActionButton(
                  label: l.openTelegram,
                  icon: Icons.open_in_new,
                  onPressed: expired
                      ? null
                      : () async {
                          try {
                            await openExternal(_challenge!.telegramLink);
                          } catch (e) {
                            if (context.mounted) showFailure(context, e);
                          }
                        },
                  secondary: true,
                ),
            ],
          ),
          TextField(
            controller: _otp,
            keyboardType: TextInputType.number,
            autofillHints: const [AutofillHints.oneTimeCode],
            inputFormatters: [
              FilteringTextInputFormatter.digitsOnly,
              LengthLimitingTextInputFormatter(6),
            ],
            onChanged: (_) => setState(() {}),
            decoration: InputDecoration(
              labelText: developmentOtp == null ? l.otp : l.localOtpInput,
            ),
            onSubmitted: (_) =>
                !_busy && !expired && _otp.text.length == 6 ? _verify() : null,
          ),
          if (expired) ...[gap, Text(l.challengeExpired)],
          ActionButton(
            label: l.verify,
            onPressed: expired || _otp.text.length != 6 ? null : _verify,
            busy: _busy,
          ),
          TextButton(
            onPressed: _busy
                ? null
                : () {
                    _timer?.cancel();
                    setState(() {
                      _challenge = null;
                      _error = null;
                    });
                  },
            child: Text(l.newChallenge),
          ),
        ],
        if (_error != null) ...[
          gap,
          Semantics(
            liveRegion: true,
            child: Text(
              _error!,
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ),
        ],
      ],
    );
  }
}
