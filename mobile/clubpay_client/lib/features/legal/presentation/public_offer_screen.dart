import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../core/ui.dart';

class PublicOfferScreen extends StatelessWidget {
  const PublicOfferScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final l = context.l;
    final offerAsset = Localizations.localeOf(context).languageCode == 'uz'
        ? 'assets/legal/public_offer_uz.md'
        : 'assets/legal/public_offer_ru.md';
    return AppPage(
      title: l.publicOffer,
      children: [
        Text(
          l.publicOfferTitle,
          style: Theme.of(context).textTheme.headlineSmall,
        ),
        const SizedBox(height: 12),
        Text(l.publicOfferDescription),
        const SizedBox(height: 20),
        FutureBuilder<String>(
          future: rootBundle.loadString(offerAsset),
          builder: (context, snapshot) {
            if (!snapshot.hasData) {
              return const PageSkeleton(rows: 5);
            }
            return SelectableText(
              snapshot.data!,
              style: Theme.of(
                context,
              ).textTheme.bodyMedium?.copyWith(height: 1.45),
            );
          },
        ),
      ],
    );
  }
}
