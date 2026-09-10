import 'dart:ui';
import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'club_theme.dart';
import 'ui.dart';

class ClubNavigation extends StatelessWidget {
  const ClubNavigation({super.key, required this.profile});
  final bool profile;

  @override
  Widget build(BuildContext context) => ClipRect(
    child: BackdropFilter(
      filter: ImageFilter.blur(sigmaX: 20, sigmaY: 20),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: ClubColors.background.withValues(alpha: .94),
          border: const Border(
            top: BorderSide(color: ClubColors.elevated, width: .5),
          ),
        ),
        child: SafeArea(
          top: false,
          child: Center(
            heightFactor: 1,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 480),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 10, 16, 8),
                child: Row(
                  children: [
                    Expanded(
                      child: _Tab(
                        icon: CupertinoIcons.house,
                        label: context.l.home,
                        selected: !profile,
                        onTap: () => context.go('/home'),
                      ),
                    ),
                    Expanded(
                      child: _Tab(
                        icon: CupertinoIcons.person_crop_circle,
                        label: context.l.profile,
                        selected: profile,
                        onTap: () => context.go('/profile'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

class _Tab extends StatelessWidget {
  const _Tab({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });
  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;
  @override
  Widget build(BuildContext context) => Semantics(
    selected: selected,
    button: true,
    child: InkWell(
      borderRadius: BorderRadius.circular(16),
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 6),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              icon,
              size: 23,
              color: selected ? ClubColors.blue : ClubColors.muted,
            ),
            const SizedBox(height: 5),
            Text(
              label,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w500,
                color: selected ? ClubColors.blue : ClubColors.muted,
              ),
            ),
          ],
        ),
      ),
    ),
  );
}
