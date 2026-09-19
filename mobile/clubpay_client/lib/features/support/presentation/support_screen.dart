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
  late Future<List<_SupportMessage>> _messages;
  @override
  void initState() {
    super.initState();
    _messages = _loadMessages();
    ref.read(analyticsProvider).track('support_opened', screen: 'support');
  }

  @override
  void dispose() {
    _message.dispose();
    super.dispose();
  }

  Future<List<_SupportMessage>> _loadMessages() async {
    final data = await ref
        .read(apiProvider)
        .get('/api/mobile/support/messages');
    return ((data['messages'] as List?) ?? [])
        .map(
          (item) =>
              _SupportMessage.fromJson(Map<String, dynamic>.from(item as Map)),
        )
        .toList();
  }

  Future<void> _send() async {
    final body = _message.text.trim();
    if (body.isEmpty) return;
    setState(() => _sending = true);
    try {
      await ref.read(apiProvider).post('/api/mobile/support/messages', {
        'body': body,
      });
      ref
          .read(analyticsProvider)
          .track('support_message_sent', screen: 'support');
      _message.clear();
      if (mounted) setState(() => _messages = _loadMessages());
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(context.l.supportMessageSent)));
      }
    } catch (error) {
      if (mounted) showFailure(context, error);
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) => AppPage(
    title: context.l.support,
    children: [
      InfoCard(children: [Text(context.l.supportHelp)]),
      const SizedBox(height: 20),
      FutureBuilder<List<_SupportMessage>>(
        future: _messages,
        builder: (context, snapshot) {
          final messages = snapshot.data ?? const <_SupportMessage>[];
          if (messages.isEmpty) return const SizedBox.shrink();
          return Column(
            children: [
              for (final message in messages)
                Align(
                  alignment: message.sender == 'player'
                      ? Alignment.centerRight
                      : Alignment.centerLeft,
                  child: Container(
                    margin: const EdgeInsets.only(bottom: 8),
                    padding: const EdgeInsets.all(12),
                    decoration: BoxDecoration(
                      color: message.sender == 'player'
                          ? Theme.of(context).colorScheme.primary
                          : Colors.white12,
                      borderRadius: BorderRadius.circular(14),
                    ),
                    child: Text(message.body),
                  ),
                ),
              const SizedBox(height: 12),
            ],
          );
        },
      ),
      TextField(
        controller: _message,
        minLines: 4,
        maxLines: 7,
        maxLength: 2000,
        textInputAction: TextInputAction.newline,
        decoration: InputDecoration(hintText: context.l.supportMessageHint),
      ),
      ActionButton(
        label: context.l.send,
        icon: CupertinoIcons.paperplane_fill,
        busy: _sending,
        onPressed: _send,
      ),
    ],
  );
}

class _SupportMessage {
  const _SupportMessage({required this.sender, required this.body});
  final String sender, body;
  factory _SupportMessage.fromJson(Map<String, dynamic> value) =>
      _SupportMessage(
        sender: value['sender'] as String? ?? 'support',
        body: value['body'] as String? ?? '',
      );
}
