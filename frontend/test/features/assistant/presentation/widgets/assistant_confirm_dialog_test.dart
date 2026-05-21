// @dart=2.19
@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/features/assistant/presentation/widgets/assistant_confirm_dialog.dart';

import '../../helpers/test_wrappers.dart';

WsAssistantConfirmRequestEvent _ev({
  Map<String, dynamic> args = const {'id': 'p1'},
  String? summary,
}) =>
    WsAssistantConfirmRequestEvent(
      ts: DateTime.utc(2026, 1, 3),
      v: 1,
      userId: 'u',
      sessionId: 's',
      toolCallId: 'tc-1',
      toolName: 'project_delete',
      arguments: args,
      summary: summary,
    );

void main() {
  group('AssistantConfirmDialog', () {
    testWidgets('renders title, Approve and Deny localized buttons',
        (tester) async {
      var approveCount = 0;
      var denyCount = 0;
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantConfirmDialog(
          event: _ev(summary: 'Delete project p1?'),
          onApprove: () => approveCount++,
          onDeny: () => denyCount++,
        ),
      ));

      expect(find.text('Confirm action'), findsOneWidget);
      expect(find.text('Delete project p1?'), findsOneWidget);

      await tester.tap(find.text('Approve'));
      await tester.pump();
      expect(approveCount, 1);

      await tester.tap(find.text('Deny'));
      await tester.pump();
      expect(denyCount, 1);
    });

    testWidgets('falls back to summary template when event.summary is null',
        (tester) async {
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantConfirmDialog(
          event: _ev(),
          onApprove: () {},
          onDeny: () {},
        ),
      ));
      // English template: "The assistant wants to run project_delete..."
      expect(find.textContaining('project_delete'), findsOneWidget);
    });

    testWidgets('busy=true disables both buttons (no callbacks fire)',
        (tester) async {
      var approveCount = 0;
      var denyCount = 0;
      await tester.pumpWidget(wrapAssistantWidget(
        AssistantConfirmDialog(
          event: _ev(),
          busy: true,
          onApprove: () => approveCount++,
          onDeny: () => denyCount++,
        ),
      ));

      // FilledButton/ TextButton с onPressed=null не реагируют на tap.
      await tester.tap(find.text('Approve'), warnIfMissed: false);
      await tester.tap(find.text('Deny'), warnIfMissed: false);
      await tester.pump();

      expect(approveCount, 0);
      expect(denyCount, 0);
    });
  });
}
