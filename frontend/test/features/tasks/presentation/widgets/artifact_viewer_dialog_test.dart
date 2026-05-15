@TestOn('vm')
@Tags(['widget'])
library;

import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/data/orchestration_v2_providers.dart';
import 'package:frontend/features/tasks/domain/models/artifact_model.dart';
import 'package:frontend/features/tasks/presentation/widgets/artifact_viewer_dialog.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/shared/widgets/diff_viewer.dart';

const _kTaskId = '11111111-1111-1111-1111-111111111111';
const _kArtifactId = '22222222-2222-2222-2222-222222222222';

Artifact _fakeArtifact({
  required String kind,
  Map<String, dynamic>? content,
}) {
  return Artifact(
    id: _kArtifactId,
    taskId: _kTaskId,
    producerAgent: 'developer',
    kind: kind,
    summary: 'summary',
    status: 'ready',
    iteration: 1,
    createdAt: DateTime.utc(2026, 5, 15),
    content: content,
  );
}

Widget _pumpHarness({
  required Artifact artifact,
  Size screenSize = const Size(1200, 900),
}) {
  return ProviderScope(
    retry: (_, _) => null,
    overrides: [
      artifactDetailProvider((_kTaskId, _kArtifactId)).overrideWith(
        (ref) => Future<Artifact>.value(artifact),
      ),
    ],
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: MediaQuery(
        data: MediaQueryData(size: screenSize),
        child: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: ElevatedButton(
                onPressed: () => showArtifactViewerDialog(
                  context,
                  taskId: _kTaskId,
                  artifactId: _kArtifactId,
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      ),
    ),
  );
}

void main() {
  group('ArtifactViewerDialog', () {
    testWidgets('code_diff: DiffViewer материализуется в диалоге',
        (tester) async {
      const diff = '''
--- a/x
+++ b/x
@@ -1,1 +1,1 @@
-a
+b
''';
      final artifact = _fakeArtifact(
        kind: 'code_diff',
        content: <String, dynamic>{'diff': diff},
      );
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.byType(DiffViewer), findsOneWidget);
      expect(find.textContaining('+b'), findsOneWidget);
    });

    testWidgets('review: показывает decision/issues/summary', (tester) async {
      final artifact = _fakeArtifact(
        kind: 'review',
        content: <String, dynamic>{
          'decision': 'approved',
          'issues': <String>['too few tests'],
          'summary': 'LGTM with caveats',
        },
      );
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('approved'), findsOneWidget);
      expect(find.text('too few tests'), findsOneWidget);
      expect(find.text('LGTM with caveats'), findsOneWidget);
    });

    testWidgets('test_result: показывает счётчики + expandable failures',
        (tester) async {
      final artifact = _fakeArtifact(
        kind: 'test_result',
        content: <String, dynamic>{
          'passed': 12,
          'failed': 1,
          'skipped': 2,
          'duration_ms': 5430,
          'build_passed': true,
          'lint_passed': true,
          'typecheck_passed': true,
          'failures': <Map<String, dynamic>>[
            <String, dynamic>{
              'test_name': 'TestFoo',
              'file': 'foo_test.go',
              'line': 42,
              'message': 'expected X got Y',
            },
          ],
        },
      );
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.text('12'), findsOneWidget);
      expect(find.text('TestFoo'), findsOneWidget);
      expect(find.text('foo_test.go:42'), findsOneWidget);

      // Раскрываем ExpansionTile и проверяем message.
      await tester.tap(find.text('TestFoo'));
      await tester.pumpAndSettle();
      expect(find.textContaining('expected X got Y'), findsOneWidget);
    });

    testWidgets(
        'test_result: failures > threshold пагинируется по '
        '$kArtifactFailuresPageSize, виден "Show next"', (tester) async {
      final failures = <Map<String, dynamic>>[
        for (var i = 0;
            i < kArtifactFailuresPaginationThreshold + 5;
            i++)
          <String, dynamic>{
            'test_name': 'Test_$i',
            'message': 'boom $i',
          },
      ];
      final artifact = _fakeArtifact(
        kind: 'test_result',
        content: <String, dynamic>{
          'passed': 0,
          'failed': failures.length,
          'skipped': 0,
          'duration_ms': 0,
          'build_passed': true,
          'lint_passed': true,
          'typecheck_passed': true,
          'failures': failures,
        },
      );
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      // Test_0 в viewport (top of list).
      expect(find.text('Test_0'), findsOneWidget);
      // Test за порогом пагинации НЕ построен (shown = первые
      // `kArtifactFailuresPageSize`, остальное закрыто за кнопкой).
      expect(
          find.text('Test_$kArtifactFailuresPaginationThreshold'), findsNothing);
      // Скроллим к концу — там должна быть кнопка "Show next".
      await tester.dragUntilVisible(
        find.textContaining('Show next'),
        find.byType(ListView).first,
        const Offset(0, -200),
      );
      expect(find.textContaining('Show next'), findsOneWidget);
    });

    testWidgets(
        'merged_code (огромный JSON > $kArtifactJsonTruncationChars): '
        'truncation-баннер + "Show full"-кнопка', (tester) async {
      final huge = List<String>.generate(
              5000, (i) => 'this is a long sha and metadata line $i')
          .join('\n');
      final artifact = _fakeArtifact(
        kind: 'merged_code',
        content: <String, dynamic>{'raw': huge},
      );
      // sanity: pretty JSON будет точно длиннее порога
      final pretty =
          const JsonEncoder.withIndent('  ').convert(artifact.content);
      expect(pretty.length, greaterThan(kArtifactJsonTruncationChars));

      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Show full'), findsOneWidget);
      expect(find.textContaining('Showing first'), findsOneWidget);
    });

    testWidgets('copy кнопка → Clipboard.setData + snackbar',
        (tester) async {
      final captured = <String>[];
      tester.binding.defaultBinaryMessenger.setMockMethodCallHandler(
        SystemChannels.platform,
        (call) async {
          if (call.method == 'Clipboard.setData') {
            captured.add((call.arguments as Map)['text'] as String);
          }
          return null;
        },
      );
      addTearDown(() {
        tester.binding.defaultBinaryMessenger
            .setMockMethodCallHandler(SystemChannels.platform, null);
      });
      final artifact = _fakeArtifact(
        kind: 'merged_code',
        content: <String, dynamic>{'head_commit_sha': 'deadbeef'},
      );
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      // На небольшом content'е "Show full" нет, но "Copy full" — есть всегда.
      final copyBtn = find.byIcon(Icons.copy);
      expect(copyBtn, findsOneWidget);
      await tester.tap(copyBtn);
      await tester.pumpAndSettle();
      expect(captured, hasLength(1));
      expect(captured.first, contains('deadbeef'));
      // Snackbar — текст "Copied N bytes"
      expect(find.textContaining('Copied'), findsOneWidget);
    });

    testWidgets('empty content: показывает "No content stored"',
        (tester) async {
      final artifact = _fakeArtifact(kind: 'plan', content: null);
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.textContaining('No content stored'), findsOneWidget);
    });

    testWidgets('error state: показывает текст ошибки', (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
            artifactDetailProvider((_kTaskId, _kArtifactId)).overrideWith(
              (ref) =>
                  Future<Artifact>.error(StateError('boom')),
            ),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Builder(
              builder: (context) => Scaffold(
                body: Center(
                  child: ElevatedButton(
                    onPressed: () => showArtifactViewerDialog(
                      context,
                      taskId: _kTaskId,
                      artifactId: _kArtifactId,
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
      // pumpAndSettle с timer-error — даём конечное число pump'ов.
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));
      expect(find.textContaining('Failed to load artifact'), findsOneWidget);
    });

    testWidgets(
        '"Show full" → полноэкранный диалог; для >50K текста compute() '
        'возвращает Future, спиннер сменяется ListView после runAsync',
        (tester) async {
      // 80K * "x" — pretty JSON будет ≥ 80K, выше _kFullScreenIsolateThreshold,
      // и значит парсинг идёт через compute() (изолят).
      final huge = 'x' * 80000;
      final artifact = _fakeArtifact(
        kind: 'merged_code',
        content: <String, dynamic>{'raw': huge},
      );
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      await tester.tap(find.textContaining('Show full'));
      await tester.pump();
      // Локализованный AppBar.title (en: "merged_code · full")
      expect(find.textContaining('· full'), findsOneWidget);
      // FutureBuilder ждёт результат compute() — пока CircularProgressIndicator.
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      // Реальный isolate-await: тестовый scheduler не пропускает isolate-
      // сообщения, поэтому используем runAsync().
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 200));
      });
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsNothing);
    });

    testWidgets('test_result без test_name: показывает локализованный fallback',
        (tester) async {
      final artifact = _fakeArtifact(
        kind: 'test_result',
        content: <String, dynamic>{
          'passed': 0,
          'failed': 1,
          'skipped': 0,
          'duration_ms': 0,
          'build_passed': true,
          'lint_passed': true,
          'typecheck_passed': true,
          'failures': <Map<String, dynamic>>[
            <String, dynamic>{'message': 'no name'},
          ],
        },
      );
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      // EN: "(unnamed)"
      expect(find.text('(unnamed)'), findsOneWidget);
    });

    testWidgets('close-кнопка закрывает диалог', (tester) async {
      final artifact = _fakeArtifact(kind: 'plan', content: <String, dynamic>{
        'items': <String>['a', 'b'],
      });
      await tester.pumpWidget(_pumpHarness(artifact: artifact));
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
      expect(find.byType(Dialog), findsOneWidget);
      await tester.tap(find.byIcon(Icons.close));
      await tester.pumpAndSettle();
      expect(find.byType(Dialog), findsNothing);
    });
  });
}

