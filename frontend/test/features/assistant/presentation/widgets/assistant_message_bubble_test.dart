// Sprint 21 — widget-тест для AssistantMessageBubble.
//
// Виджет stateless и не зависит от Riverpod-провайдеров, поэтому достаточно
// MaterialApp+l10n из общего helper'a [wrapAssistantWidget]. Покрываем матрицу
// ролей (user/assistant/system) и базовый рендер контента.

// @dart=2.19
@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_message_bubble.dart';

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

      final body =
          tester.widget<SelectableText>(find.byType(SelectableText));
      expect(body.style?.fontStyle, FontStyle.italic);
    });

    testWidgets('null content rendered as empty string (no crash)',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantMessageBubble(
          message: _msg(role: assistantMessageRoleAssistant),
        ),
      ));

      final body =
          tester.widget<SelectableText>(find.byType(SelectableText));
      expect(body.data, '');
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
