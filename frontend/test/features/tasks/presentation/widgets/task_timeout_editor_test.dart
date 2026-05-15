@TestOn('vm')
@Tags(['widget'])
library;

import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_errors.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/features/tasks/presentation/widgets/task_timeout_editor.dart';
import 'package:frontend/l10n/app_localizations.dart';

const _kProjectId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';
const _kTaskId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb';

class _StubDetailController extends TaskDetailController {
  _StubDetailController({this.throwOnUpdate});

  /// Если задано — updateTask бросает это исключение. Иначе записывает
  /// последний переданный customTimeout и возвращает [TaskMutationOutcome.completed].
  final Object? throwOnUpdate;

  String? lastCustomTimeout;
  int updateCalls = 0;

  @override
  FutureOr<TaskDetailState> build({
    required String projectId,
    required String taskId,
  }) =>
      TaskDetailState.initial();

  @override
  Future<TaskMutationOutcome> updateTask(UpdateTaskRequest request) async {
    updateCalls++;
    lastCustomTimeout = request.customTimeout;
    if (throwOnUpdate != null) {
      throw throwOnUpdate!;
    }
    return TaskMutationOutcome.completed;
  }
}

Future<void> _openDialog(
  WidgetTester tester, {
  required String? currentValue,
  required _StubDetailController stub,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      retry: (_, _) => null,
      overrides: [
        taskDetailControllerProvider(
          projectId: _kProjectId,
          taskId: _kTaskId,
        ).overrideWith(() => stub),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(
          body: Consumer(
            builder: (context, ref, _) => Center(
              child: ElevatedButton(
                onPressed: () => showTaskTimeoutDialog(
                  context: context,
                  ref: ref,
                  projectId: _kProjectId,
                  taskId: _kTaskId,
                  currentValue: currentValue,
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
}

void main() {
  group('TaskTimeoutDialog', () {
    testWidgets('valid input saves and pops with true', (tester) async {
      final stub = _StubDetailController();
      await _openDialog(tester, currentValue: null, stub: stub);

      await tester.enterText(find.byType(TextField), '90m');
      await tester.tap(find.text('Save'));
      await tester.pumpAndSettle();

      expect(stub.updateCalls, 1);
      expect(stub.lastCustomTimeout, '90m');
      expect(find.byType(TaskTimeoutDialog), findsNothing);
    });

    testWidgets('malformed input shows client error and does not call backend',
        (tester) async {
      final stub = _StubDetailController();
      await _openDialog(tester, currentValue: null, stub: stub);

      await tester.enterText(find.byType(TextField), '1h2m3s');
      await tester.tap(find.text('Save'));
      await tester.pump();

      expect(stub.updateCalls, 0);
      expect(find.byType(TaskTimeoutDialog), findsOneWidget);
      expect(
        find.text('Invalid duration. Use Nh / Nm / Ns.'),
        findsOneWidget,
      );
    });

    testWidgets('backend 400 invalid_timeout shows server message inline',
        (tester) async {
      final stub = _StubDetailController(
        throwOnUpdate: TaskApiException(
          'custom_timeout must be in range 1m..72h',
          statusCode: 400,
          apiErrorCode: 'invalid_timeout',
        ),
      );
      await _openDialog(tester, currentValue: null, stub: stub);

      await tester.enterText(find.byType(TextField), '5m');
      await tester.tap(find.text('Save'));
      await tester.pump();
      await tester.pump();

      expect(stub.updateCalls, 1);
      expect(
        find.text('custom_timeout must be in range 1m..72h'),
        findsOneWidget,
      );
      expect(find.byType(TaskTimeoutDialog), findsOneWidget);
    });

    testWidgets('clear button (with confirm) submits empty string',
        (tester) async {
      final stub = _StubDetailController();
      await _openDialog(tester, currentValue: '2h', stub: stub);

      await tester.tap(find.text('Reset to default'));
      await tester.pumpAndSettle();
      // Confirm dialog
      await tester.tap(find.widgetWithText(FilledButton, 'Reset to default'));
      await tester.pumpAndSettle();

      expect(stub.updateCalls, 1);
      expect(stub.lastCustomTimeout, '');
      expect(find.byType(TaskTimeoutDialog), findsNothing);
    });

    testWidgets('cancel pops without calling backend', (tester) async {
      final stub = _StubDetailController();
      await _openDialog(tester, currentValue: '4h', stub: stub);

      await tester.tap(find.widgetWithText(TextButton, 'Cancel'));
      await tester.pumpAndSettle();

      expect(stub.updateCalls, 0);
      expect(find.byType(TaskTimeoutDialog), findsNothing);
    });
  });

  group('kTaskCustomTimeoutPattern', () {
    test('accepts valid duration formats', () {
      for (final raw in ['4h', '90m', '3600s', '1h30m', '1h30s', '5m30s']) {
        expect(
          kTaskCustomTimeoutPattern.hasMatch(raw),
          isTrue,
          reason: 'should accept "$raw"',
        );
      }
    });

    test('rejects malformed input', () {
      for (final raw in ['', '4', '4 hours', '-1h', 'abc', '1h2m3s', '4hours']) {
        expect(
          kTaskCustomTimeoutPattern.hasMatch(raw),
          isFalse,
          reason: 'should reject "$raw"',
        );
      }
    });
  });
}
