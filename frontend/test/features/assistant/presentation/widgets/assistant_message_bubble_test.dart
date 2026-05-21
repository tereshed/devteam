import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_message_bubble.dart';
import 'package:frontend/features/chat/presentation/widgets/chat_message.dart';

import '../../helpers/test_wrappers.dart';

AssistantMessageModel _msg({
  required String role,
  String? content,
}) =>
    AssistantMessageModel(
      id: 'm-1',
      sessionId: 'sess-1',
      role: role,
      content: content,
      createdAt: DateTime.utc(2026, 1, 1),
    );

void main() {
  group('AssistantMessageBubble', () {
    testWidgets('user role: shows "You" label and content, right-aligned',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantMessageBubble(
          message: _msg(role: assistantMessageRoleUser, content: 'hello'),
        ),
      ));

      expect(find.text('You'), findsOneWidget);
      expect(find.text('hello'), findsOneWidget);

      final align = tester.widget<Align>(find.byType(Align));
      expect(align.alignment, Alignment.centerRight);
    });

    testWidgets('assistant role: shows "Assistant" label, left-aligned',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantMessageBubble(
          message: _msg(
            role: assistantMessageRoleAssistant,
            content: 'Привет!',
          ),
        ),
      ));

      expect(find.text('Assistant'), findsOneWidget);
      expect(find.text('Привет!'), findsOneWidget);

      final align = tester.widget<Align>(find.byType(Align));
      expect(align.alignment, Alignment.centerLeft);
    });

    testWidgets('system role: shows "System" label and italic style',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantMessageBubble(
          message: _msg(
            role: assistantMessageRoleSystem,
            content: 'session reset',
          ),
        ),
      ));

      expect(find.text('System'), findsOneWidget);

      final themeFinder = find.descendant(
        of: find.byType(AssistantMessageBubble),
        matching: find.byType(Theme),
      );
      expect(themeFinder, findsWidgets);
      final theme = tester.widget<Theme>(themeFinder.last);
      expect(theme.data.textTheme.bodyMedium?.fontStyle, FontStyle.italic);
    });

    testWidgets('null content rendered as empty string (no crash)',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantMessageBubble(
          message: _msg(role: assistantMessageRoleAssistant),
        ),
      ));

      expect(find.byType(ChatMessage), findsOneWidget);
      final chatMsg = tester.widget<ChatMessage>(find.byType(ChatMessage));
      expect(chatMsg.content, '');
    });

    testWidgets('unknown role: falls back to raw role string label',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantMessageBubble(
          message: _msg(role: 'tool', content: 'tool-result'),
        ),
      ));

      // 'tool' не входит в [user|assistant|system], поэтому label — само 'tool'.
      expect(find.text('tool'), findsOneWidget);
      expect(find.text('tool-result'), findsOneWidget);
    });
  });
}
