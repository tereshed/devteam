// @dart=2.19
@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/assistant/domain/assistant_message_model.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_tool_call_card.dart';

import '../../helpers/test_wrappers.dart';

AssistantMessageModel _assistantToolCall({
  Map<String, dynamic>? args,
}) =>
    AssistantMessageModel(
      id: 'asst-1',
      sessionId: 'sess-1',
      role: assistantMessageRoleAssistant,
      content: null,
      toolCallId: 'tc-1',
      toolName: 'project_list',
      toolArguments: args,
      createdAt: DateTime.utc(2026, 1, 3),
    );

AssistantMessageModel _toolResult({
  String status = 'ok',
  Map<String, dynamic>? result,
}) =>
    AssistantMessageModel(
      id: 'tool-1',
      sessionId: 'sess-1',
      role: assistantMessageRoleTool,
      toolCallId: 'tc-1',
      toolName: 'project_list',
      toolResult: {'status': status, ...?result},
      createdAt: DateTime.utc(2026, 1, 3, 0, 0, 2),
    );

void main() {
  group('AssistantToolCallCard', () {
    testWidgets('collapsed by default: shows only tool title', (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantToolCallCard(
          assistantMessage: _assistantToolCall(args: const {'limit': 10}),
          toolResult: _toolResult(),
        ),
      ));

      expect(find.textContaining('project_list'), findsOneWidget);
      // Arguments label виден только когда раскрыто.
      expect(find.text('Arguments'), findsNothing);
      expect(find.text('Result'), findsNothing);
    });

    testWidgets('tap expands and reveals Arguments + Result blocks',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantToolCallCard(
          assistantMessage: _assistantToolCall(args: const {'limit': 10}),
          toolResult: _toolResult(result: {'items': []}),
        ),
      ));

      await tester.tap(find.byType(InkWell).first);
      await tester.pumpAndSettle();

      expect(find.text('Arguments'), findsOneWidget);
      expect(find.text('Result'), findsOneWidget);
      // JSON-блок содержит подстроку 'limit'.
      expect(find.textContaining('"limit"'), findsOneWidget);
    });

    testWidgets('status badge: OK для status=ok', (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantToolCallCard(
          assistantMessage: _assistantToolCall(),
          toolResult: _toolResult(status: 'ok'),
        ),
      ));
      expect(find.text('OK'), findsOneWidget);
    });

    testWidgets('status badge: Forbidden для status=forbidden',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantToolCallCard(
          assistantMessage: _assistantToolCall(),
          toolResult: _toolResult(status: 'forbidden'),
        ),
      ));
      expect(find.text('Forbidden'), findsOneWidget);
    });

    testWidgets('status badge: Pending когда toolResult.result отсутствует',
        (tester) async {
      // null toolResult у tool-row → status="pending" по умолчанию.
      final pending = AssistantMessageModel(
        id: 'tool-1',
        sessionId: 'sess-1',
        role: assistantMessageRoleTool,
        toolCallId: 'tc-1',
        toolName: 'project_list',
        toolResult: null,
        createdAt: DateTime.utc(2026, 1, 3),
      );
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantToolCallCard(
          assistantMessage: _assistantToolCall(),
          toolResult: pending,
        ),
      ));
      expect(find.text('Pending'), findsOneWidget);
    });

    testWidgets('no badge when toolResult is null (yet to arrive)',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantToolCallCard(
          assistantMessage: _assistantToolCall(),
        ),
      ));
      // Никакого OK/Forbidden/Pending — ничего не показываем, чтобы UI
      // не врал о результате.
      expect(find.text('OK'), findsNothing);
      expect(find.text('Pending'), findsNothing);
      expect(find.text('Forbidden'), findsNothing);
    });
  });
}
