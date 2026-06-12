import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/enhancer/data/enhancer_providers.dart';
import 'package:frontend/features/enhancer/data/enhancer_repository.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_change_model.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_config_model.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_run_model.dart';
import 'package:frontend/features/enhancer/presentation/widgets/enhancer_settings_tab.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Фейковый репозиторий — реальные контроллеры выполняются поверх него.
class _FakeRepo extends EnhancerRepository {
  _FakeRepo({
    required this.config,
    required this.runs,
    this.changes = const [],
  }) : super(dio: Dio());

  final EnhancerConfigModel config;
  final List<EnhancerRunModel> runs;
  final List<EnhancerChangeModel> changes;

  @override
  Future<EnhancerConfigModel> getConfig(
    String projectId, {
    CancelToken? cancelToken,
  }) async =>
      config;

  @override
  Future<List<EnhancerRunModel>> listRuns(
    String projectId, {
    CancelToken? cancelToken,
  }) async =>
      runs;

  @override
  Future<List<EnhancerChangeModel>> listRunChanges(
    String projectId,
    String runId, {
    CancelToken? cancelToken,
  }) async =>
      changes;
}

const _config = EnhancerConfigModel(projectId: 'p1');

EnhancerRunModel _doneRun() => EnhancerRunModel(
      id: '22222222-2222-2222-2222-222222222222',
      projectId: 'p1',
      status: 'done',
      report: 'Проанализировано 5 задач, найдена петля ревью.',
      startedAt: DateTime(2026, 6, 10, 9),
      finishedAt: DateTime(2026, 6, 10, 9, 5),
    );

EnhancerChangeModel _change() => EnhancerChangeModel(
      id: '33333333-3333-3333-3333-333333333333',
      runId: '22222222-2222-2222-2222-222222222222',
      projectId: 'p1',
      targetKind: 'agent_override',
      targetAgentId: '44444444-4444-4444-4444-444444444444',
      payload: const {'prompt_addendum': 'always specify repo_slug'},
      reason: 'task abc looped on SupportAgent',
      expectedEffect: 'fewer router steps',
      createdAt: DateTime(2026, 6, 10, 9, 4),
    );

Widget _harness(_FakeRepo repo) {
  return ProviderScope(
    retry: (_, _) => null,
    overrides: [
      enhancerRepositoryProvider.overrideWithValue(repo),
    ],
    child: const MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: Locale('en'),
      home: Scaffold(body: EnhancerSettingsTab(projectId: 'p1')),
    ),
  );
}

void main() {
  // Вкладка — ленивый ListView: даём высокий вьюпорт, чтобы все секции
  // (форма + прогоны) построились без скролла.
  Future<void> pumpTall(WidgetTester tester, Widget widget) async {
    tester.view.physicalSize = const Size(1200, 2600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    await tester.pumpWidget(widget);
  }

  testWidgets('renders config form and empty runs state', (tester) async {
    await pumpTall(tester, _harness(_FakeRepo(config: _config, runs: [])));
    await tester.pumpAndSettle();

    expect(find.text('Enabled'), findsOneWidget);
    expect(find.text('Run analysis'), findsOneWidget);
    expect(
      find.text('No runs yet. Start an analysis manually or set up a schedule.'),
      findsOneWidget,
    );
    // Фаза 1: auto_apply виден, но недоступен.
    expect(find.text('Apply automatically'), findsOneWidget);
  });

  testWidgets('renders done run card and lazy-loads changes', (tester) async {
    await pumpTall(
      tester,
      _harness(
        _FakeRepo(config: _config, runs: [_doneRun()], changes: [_change()]),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Done'), findsOneWidget);

    // Раскрываем карточку — отчёт и предложения.
    await tester.tap(find.text('Done'));
    await tester.pumpAndSettle();

    expect(
      find.text('Проанализировано 5 задач, найдена петля ревью.'),
      findsOneWidget,
    );
    expect(find.text('Agent prompt/settings'), findsOneWidget);
    expect(find.text('task abc looped on SupportAgent'), findsOneWidget);
    expect(find.text('fewer router steps'), findsOneWidget);
    expect(find.text('Proposed'), findsOneWidget);
  });
}
