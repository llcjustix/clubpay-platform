import 'package:flutter/cupertino.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/providers.dart';
import '../../../core/ui.dart';

class SupportScreen extends ConsumerStatefulWidget {
  const SupportScreen({super.key});
  @override
  ConsumerState<SupportScreen> createState() => _SupportScreenState();
}

class _SupportScreenState extends ConsumerState<SupportScreen> {
  final _message = TextEditingController();
  bool _sending = false;
  @override
  void initState() { super.initState(); ref.read(analyticsProvider).track('support_opened', screen: 'support'); }
  @override
  void dispose() { _message.dispose(); super.dispose(); }
  Future<void> _send() async {
    final body = _message.text.trim();
    if (body.isEmpty) return;
    setState(() => _sending = true);
    try {
      await ref.read(apiProvider).post('/api/mobile/support/messages', {'body': body});
      _message.clear();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Сообщение отправлено в поддержку.')));
    } catch (error) { if (mounted) showFailure(context, error); }
    finally { if (mounted) setState(() => _sending = false); }
  }
  @override
  Widget build(BuildContext context) => AppPage(
    title: 'Поддержка',
    children: [
      const InfoCard(children: [Text('Опишите проблему: клуб, компьютер и что произошло. Мы увидим сообщение и ответим в этом чате.')]),
      const SizedBox(height: 20),
      TextField(controller: _message, minLines: 4, maxLines: 7, maxLength: 2000, textInputAction: TextInputAction.newline, decoration: const InputDecoration(hintText: 'Напишите сообщение')),
      ActionButton(label: 'Отправить', icon: CupertinoIcons.paperplane_fill, busy: _sending, onPressed: _send),
    ],
  );
}
