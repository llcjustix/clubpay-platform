import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../../core/ui.dart';

class PublicOfferScreen extends StatelessWidget {
  const PublicOfferScreen({super.key});

  @override
  Widget build(BuildContext context) => AppPage(
        title: 'Публичная оферта',
        children: [
          Text('Публичная оферта ClubPay',
              style: Theme.of(context).textTheme.headlineSmall),
          const SizedBox(height: 12),
          const Text('Условия использования сервиса и игрового времени.'),
          const SizedBox(height: 20),
          FutureBuilder<String>(
            future: rootBundle.loadString('assets/legal/public_offer_ru.md'),
            builder: (context, snapshot) {
              if (!snapshot.hasData) {
                return const Center(child: CircularProgressIndicator());
              }
              return SelectableText(
                snapshot.data!,
                style: Theme.of(context).textTheme.bodyMedium?.copyWith(height: 1.45),
              );
            },
          ),
        ],
      );
}
