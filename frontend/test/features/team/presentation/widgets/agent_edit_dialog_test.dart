@TestOn('vm')
@Tags(['widget'])
library;

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/features/admin/prompts/data/prompts_providers.dart';
import 'package:frontend/features/admin/prompts/data/prompts_repository.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/team/data/team_repository.dart';
import 'package:frontend/features/team/data/tools_providers.dart';
import 'package:frontend/features/team/data/tools_repository.dart';
import 'package:frontend/features/team/domain/models/tool_binding_response_model.dart';
import 'package:frontend/features/team/presentation/widgets/agent_card.dart';
import 'package:frontend/features/team/presentation/widgets/agent_edit_dialog.dart';
import 'package:frontend/features/tasks/data/task_providers.dart';
import 'package:frontend/features/tasks/data/task_repository.dart';
import 'package:frontend/l10n/app_localizations_ru.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import '../../../projects/helpers/test_wrappers.dart';
import 'agent_edit_dialog_test.mocks.dart';

class FakeLlmIntegrationsController extends ChangeNotifier implements LlmIntegrationsController {
  @override
  Future<void> refresh() async {}

  @override
  LlmIntegrationsState get state => const LlmIntegrationsState(connections: {});

  @override
  void applyLocal(LlmProviderConnection connection) {}

  @override
  bool get debugNeedsResyncOnNextServerEvent => false;

  @override
  VoidCallback? get onConnectionChanged => null;
}

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  const projectId = '550e8400-e29b-41d4-a716-446655440000';
  const agentId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';

  // Подключённые LLM-провайдеры для каскада формы (см. agent_provider_rules).
  final connectedLlm = LlmIntegrationsState(
    connections: {
      for (final p in const [
        LlmIntegrationProvider.anthropic,
        LlmIntegrationProvider.deepseek,
        LlmIntegrationProvider.openrouter,
        LlmIntegrationProvider.zhipu,
        LlmIntegrationProvider.claudeCodeOAuth,
      ])
        p: LlmProviderConnection(
          provider: p,
          status: LlmProviderConnectionStatus.connected,
        ),
    },
  );
  final llmOverride = llmIntegrationsStateProvider
      .overrideWith((ref) => Stream.value(connectedLlm));
  final llmControllerOverride = llmIntegrationsControllerProvider
      .overrideWithValue(FakeLlmIntegrationsController());

  Map<String, dynamic> teamJson() => <String, dynamic>{
        'id': 'team-1',
        'name': 'Dev Team',
        'project_id': projectId,
        'type': 'development',
        'agents': <Map<String, dynamic>>[],
        'created_at': '2026-04-27T09:00:00Z',
        'updated_at': '2026-04-27T09:15:00Z',
      };

  Map<String, dynamic> taskJson(String taskId) => <String, dynamic>{
        'id': taskId,
        'project_id': projectId,
        'title': 'Test Task',
        'description': '',
        'status': 'pending',
        'priority': 'medium',
        'assigned_agent_id': agentId,
        'created_by_type': 'user',
        'created_by_id': 'user-1',
        'created_at': '2026-04-27T09:00:00Z',
        'updated_at': '2026-04-27T09:15:00Z',
      };

  Widget wrapWithRouter({
    required Widget body,
    required MockDio dio,
    required String targetPath,
    required List<String> navigatedPaths,
    Size? viewSize,
  }) {
    final scoped = ProviderScope(
      retry: (_, _) => null,
      overrides: [
        dioClientProvider.overrideWithValue(dio),
        teamRepositoryProvider.overrideWithValue(TeamRepository(dio: dio)),
        promptsRepositoryProvider
            .overrideWithValue(PromptsRepository(dio: dio)),
        toolsRepositoryProvider.overrideWithValue(ToolsRepository(dio: dio)),
        taskRepositoryProvider.overrideWithValue(TaskRepository(dio: dio)),
        llmOverride,
        llmControllerOverride,
      ],
      child: MaterialApp.router(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('ru'),
        theme: ThemeData(splashFactory: NoSplash.splashFactory),
        routerConfig: GoRouter(
          routes: [
            GoRoute(
              path: '/',
              builder: (context, state) => Scaffold(
                body: Builder(
                  builder: (ctx) => Center(
                    child: TextButton(
                      onPressed: () {
                        Navigator.of(ctx).push(
                          MaterialPageRoute<void>(
                            builder: (_) => Scaffold(body: body),
                          ),
                        );
                      },
                      child: const Text('__open_dialog__'),
                    ),
                  ),
                ),
              ),
            ),
            GoRoute(
              path: targetPath,
              builder: (context, state) {
                navigatedPaths.add(state.uri.toString());
                return const SizedBox();
              },
            ),
          ],
        ),
      ),
    );
    if (viewSize != null) {
      return MediaQuery(
        data: MediaQueryData(size: viewSize),
        child: scoped,
      );
    }
    return scoped;
  }

  AgentModel sampleAgent({
    String? promptId,
    String? promptName,
    bool isActive = true,
    String? model = 'm1',
    List<ToolBindingResponseModel>? toolBindings,
    String? providerKind,
    String role = 'planner', // llm-роль: модель+провайдер активны, бекенд скрыт
    String? codeBackend,
  }) {
    return AgentModel(
      id: agentId,
      name: 'Agent',
      role: role,
      model: model,
      promptName: promptName,
      promptId: promptId,
      codeBackend: codeBackend,
      providerKind: providerKind,
      isActive: isActive,
      toolBindings: toolBindings ?? const [],
    );
  }

  MockDio createDio() => MockDio();

  void stubPrompts(MockDio dio) {
    when(
      dio.get(
        '/prompts',
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: <dynamic>[],
        statusCode: 200,
        requestOptions: RequestOptions(path: '/prompts'),
      ),
    );
  }

  void stubToolDefinitions(MockDio dio) {
    when(
      dio.get(
        '/tool-definitions',
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: <dynamic>[
          <String, dynamic>{
            'id': '11111111-1111-4111-8111-111111111111',
            'name': 'Tool One',
            'description': 'd1',
            'category': 'cat',
            'is_builtin': true,
          },
        ],
        statusCode: 200,
        requestOptions: RequestOptions(path: '/tool-definitions'),
      ),
    );
  }

  void stubDialogDio(MockDio dio) {
    stubPrompts(dio);
    stubToolDefinitions(dio);
  }

  void stubTeamGet(MockDio dio) {
    when(
      dio.get(
        '/projects/$projectId/team',
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: teamJson(),
        statusCode: 200,
        requestOptions: RequestOptions(path: '/projects/$projectId/team'),
      ),
    );
  }

  Widget wrap(
    Widget child, {
    required MockDio dio,
    Size? viewSize,
    Locale locale = const Locale('ru'),
    bool scrollableBody = true,
  }) {
    final scoped = ProviderScope(
      retry: (_, _) => null,
      overrides: [
        dioClientProvider.overrideWithValue(dio),
        teamRepositoryProvider.overrideWithValue(TeamRepository(dio: dio)),
        promptsRepositoryProvider
            .overrideWithValue(PromptsRepository(dio: dio)),
        toolsRepositoryProvider.overrideWithValue(ToolsRepository(dio: dio)),
        llmOverride,
        llmControllerOverride,
      ],
      child: wrapSimple(child, locale: locale, scrollableBody: scrollableBody),
    );
    if (viewSize != null) {
      return MediaQuery(
        data: MediaQueryData(size: viewSize),
        child: scoped,
      );
    }
    return scoped;
  }

  Widget dialogPushedHost({
    required MockDio dio,
    required Size viewSize,
    required Widget body,
  }) {
    return wrap(
      Builder(
        builder: (ctx) {
          return Scaffold(
            body: Center(
              child: TextButton(
                onPressed: () {
                  Navigator.of(ctx).push(
                    MaterialPageRoute<void>(
                      builder: (_) => Scaffold(body: body),
                    ),
                  );
                },
                child: const Text('__open_dialog__'),
              ),
            ),
          );
        },
      ),
      dio: dio,
      viewSize: viewSize,
      scrollableBody: false,
    );
  }

  bool modelFieldHasFocus(WidgetTester tester) {
    final editable = find.descendant(
      of: find.byKey(const Key('agentEditDialog_modelField')),
      matching: find.byType(EditableText),
    );
    return tester.widget<EditableText>(editable).focusNode.hasFocus;
  }

  testWidgets('форма: поля видны (широкий)', (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: true,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('agentEditDialogBody')), findsOneWidget);
    final l10n = AppLocalizationsRu();
    expect(find.text(l10n.teamAgentEditFieldModel), findsWidgets);
    expect(find.text(l10n.teamAgentEditFieldPrompt), findsWidgets);
    expect(find.text(l10n.teamAgentEditFieldCodeBackend), findsWidgets);
    expect(find.text(l10n.teamAgentEditFieldTools), findsOneWidget);
  });

  testWidgets('форма: поля видны (узкий)', (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(400, 800),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('agentEditDialogBody')), findsOneWidget);
  });

  testWidgets('Cancel без изменений — PATCH не вызывается (широкий)', (tester) async {
    useViewSize(tester, const Size(800, 1000));
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 1000),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    await tester.tap(find.text(l10n.teamAgentEditCancel));
    await tester.pumpAndSettle();
    verifyNever(
      dio.patch(
        any,
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    );
  });

  testWidgets('Cancel при dirty — discard, без PATCH', (tester) async {
    useViewSize(tester, const Size(800, 900));
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 900),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(isActive: false),
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.teamAgentEditCancel));
    await tester.pumpAndSettle();
    expect(find.text(l10n.teamAgentEditDiscardTitle), findsOneWidget);
    await tester.tap(find.text(l10n.teamAgentEditDiscardConfirm));
    await tester.pumpAndSettle();
    verifyNever(
      dio.patch(
        any,
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    );
    expect(find.byKey(const Key('agentEditDialogBody')), findsNothing);
  });

  testWidgets('успешное сохранение: PATCH затем GET team (verifyInOrder)', (tester) async {
    useViewSize(tester, const Size(800, 900));
    final dio = createDio();
    stubDialogDio(dio);
    stubTeamGet(dio);
    when(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: teamJson(),
        statusCode: 200,
        requestOptions: RequestOptions(
          path: '/projects/$projectId/team/agents/$agentId',
        ),
      ),
    );

    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 900),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(isActive: false),
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();
    clearInteractions(dio);

    final l10n = AppLocalizationsRu();
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.teamAgentEditSave));
    await tester.pumpAndSettle();

    // Полная цепочка на экране команды: getTeam (экран) → patch → invalidate → getTeam.
    // dialogPushedHost не монтирует TeamScreen — первый getTeam здесь не фиксируется.
    verifyInOrder([
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: argThat(
          isA<Map<String, dynamic>>()
              .having((m) => m['is_active'], 'is_active', true)
              .having(
                (m) => m.containsKey('tool_bindings'),
                'no tool_bindings',
                false,
              ),
          named: 'data',
        ),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
      dio.get(
        '/projects/$projectId/team',
        cancelToken: anyNamed('cancelToken'),
      ),
    ]);
    expect(find.byKey(const Key('agentEditDialogBody')), findsNothing);
  });

  testWidgets('ошибка PATCH 500 — SnackBar, форма остаётся', (tester) async {
    useViewSize(tester, const Size(800, 900));
    final dio = createDio();
    stubDialogDio(dio);
    when(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(
          path: '/projects/$projectId/team/agents/$agentId',
        ),
        response: Response<dynamic>(
          statusCode: 500,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/team/agents/$agentId',
          ),
        ),
        type: DioExceptionType.badResponse,
      ),
    );

    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 600),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(isActive: false),
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.teamAgentEditSave));
    await tester.pumpAndSettle();
    expect(find.text(l10n.teamAgentEditSaveError), findsOneWidget);
    expect(find.byKey(const Key('agentEditDialogBody')), findsOneWidget);
    verifyNever(
      dio.get(
        '/projects/$projectId/team',
        cancelToken: anyNamed('cancelToken'),
      ),
    );
  });

  testWidgets('PATCH 409 — конфликт, без GET team после', (tester) async {
    useViewSize(tester, const Size(800, 900));
    final dio = createDio();
    stubDialogDio(dio);
    when(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(
          path: '/projects/$projectId/team/agents/$agentId',
        ),
        response: Response<dynamic>(
          statusCode: 409,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/team/agents/$agentId',
          ),
        ),
        type: DioExceptionType.badResponse,
      ),
    );

    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 600),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(isActive: false),
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    await tester.tap(find.text(l10n.teamAgentEditSave));
    await tester.pumpAndSettle();
    expect(find.text(l10n.teamAgentEditConflictError), findsOneWidget);
    expect(find.byKey(const Key('agentEditDialogBody')), findsOneWidget);
    verifyNever(
      dio.get(
        '/projects/$projectId/team',
        cancelToken: anyNamed('cancelToken'),
      ),
    );
  });

  testWidgets('ошибка GET /prompts — отказоустойчивость, загрузка заглушки', (tester) async {
    final dio = createDio();
    when(
      dio.get(
        '/prompts',
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenThrow(
      DioException(
        requestOptions: RequestOptions(path: '/prompts'),
        response: Response<dynamic>(
          statusCode: 500,
          requestOptions: RequestOptions(path: '/prompts'),
        ),
        type: DioExceptionType.badResponse,
      ),
    );
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(
            promptId: 'stub_id',
            promptName: 'Stub Prompt Name',
          ),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    expect(find.text(l10n.teamAgentEditPromptsLoadError), findsNothing);
    expect(find.byType(DropdownButtonFormField<String?>), findsWidgets);
    expect(find.text('Stub Prompt Name'), findsOneWidget);
  });

  testWidgets('dispose во время загрузки промптов — без exception', (tester) async {
    final dio = createDio();
    // MockDio не отменяет completer при CancelToken.cancel() (в отличие от реального Dio);
    // проверяем только отсутствие setState после dispose при позднем завершении GET /prompts.
    final completer = Completer<Response<dynamic>>();
    when(
      dio.get(
        '/prompts',
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) => completer.future);
    stubToolDefinitions(dio);

    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    completer.complete(
      Response<dynamic>(
        data: <dynamic>[],
        statusCode: 200,
        requestOptions: RequestOptions(path: '/prompts'),
      ),
    );
    await tester.pump();
    expect(tester.takeException(), isNull);
  });

  testWidgets('dispose во время загрузки tool-definitions — без exception', (tester) async {
    final dio = createDio();
    stubPrompts(dio);
    final completer = Completer<Response<dynamic>>();
    when(
      dio.get(
        '/tool-definitions',
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer((_) => completer.future);

    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pump();
    await tester.pumpWidget(const SizedBox.shrink());
    completer.complete(
      Response<dynamic>(
        data: <dynamic>[],
        statusCode: 200,
        requestOptions: RequestOptions(path: '/tool-definitions'),
      ),
    );
    await tester.pump();
    expect(tester.takeException(), isNull);
  });

  testWidgets('13.3.1: пустой каталог — teamAgentEditToolsEmpty', (tester) async {
    final dio = createDio();
    stubPrompts(dio);
    when(
      dio.get(
        '/tool-definitions',
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: <dynamic>[],
        statusCode: 200,
        requestOptions: RequestOptions(path: '/tool-definitions'),
      ),
    );

    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    expect(find.text(l10n.teamAgentEditToolsEmpty), findsOneWidget);
  });

  testWidgets(
    '13.3.1 B.6: первый GET каталога ок, после тоггла повторный GET падает — PATCH сохраняет локальный выбор',
    (tester) async {
      useViewSize(tester, const Size(800, 1200));
      final dio = createDio();
      stubPrompts(dio);
      final toolsErr = DioException(
        requestOptions: RequestOptions(path: '/tool-definitions'),
        type: DioExceptionType.badResponse,
        response: Response<dynamic>(
          statusCode: 500,
          requestOptions: RequestOptions(path: '/tool-definitions'),
        ),
      );
      var toolsGetCount = 0;
      when(
        dio.get(
          '/tool-definitions',
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async {
        toolsGetCount++;
        if (toolsGetCount == 1) {
          return Response<dynamic>(
            data: <dynamic>[
              <String, dynamic>{
                'id': '11111111-1111-4111-8111-111111111111',
                'name': 'Tool One',
                'description': 'd1',
                'category': 'cat',
                'is_builtin': true,
              },
            ],
            statusCode: 200,
            requestOptions: RequestOptions(path: '/tool-definitions'),
          );
        }
        throw toolsErr;
      });

      stubTeamGet(dio);
      when(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: teamJson(),
          statusCode: 200,
          requestOptions: RequestOptions(path: '/p'),
        ),
      );

      final agent = sampleAgent(
        toolBindings: const [
          ToolBindingResponseModel(
            toolDefinitionId: '11111111-1111-4111-8111-111111111111',
            name: 'Tool One',
            category: 'cat',
          ),
        ],
      );

      await tester.pumpWidget(
        dialogPushedHost(
          dio: dio,
          viewSize: const Size(800, 1200),
          body: agentEditDialogBodyForTesting(
            projectId: projectId,
            agent: agent,
            useAutofocus: false,
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('__open_dialog__'));
      await tester.pumpAndSettle();
      final l10n = AppLocalizationsRu();
      expect(find.byType(FilterChip), findsOneWidget);
      await tester.tap(find.byType(FilterChip));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('agentEditToolsRefreshCatalog')));
      await tester.pumpAndSettle();
      expect(find.text(l10n.teamAgentEditToolsLoadError), findsOneWidget);
      await tester.tap(find.text(l10n.teamAgentEditSave));
      await tester.pumpAndSettle();
      verify(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: argThat(
            isA<Map<String, dynamic>>().having(
              (m) => m['tool_bindings'],
              'tool_bindings',
              isA<List<dynamic>>().having((l) => l.length, 'len', 0),
            ),
            named: 'data',
          ),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    },
  );

  // П. 11 (канон): Padding(viewInsets) на узком shell — в showAgentEditDialog; тесты
  // помпают только тело формы — регрессию оболочки см. интеграцию / экран команды.

  testWidgets('автофокус поля модели при useAutofocus true', (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(providerKind: 'anthropic'),
          useAutofocus: true,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    expect(modelFieldHasFocus(tester), isTrue);
  });

  testWidgets('без автофокуса при useAutofocus false', (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(providerKind: 'anthropic'),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(400, 800),
      ),
    );
    await tester.pumpAndSettle();
    expect(modelFieldHasFocus(tester), isFalse);
  });

  testWidgets('двойной Save — один PATCH', (tester) async {
    useViewSize(tester, const Size(800, 900));
    final dio = createDio();
    stubDialogDio(dio);
    stubTeamGet(dio);
    when(
      dio.patch(
        any,
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: teamJson(),
        statusCode: 200,
        requestOptions: RequestOptions(path: '/patch'),
      ),
    );

    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 600),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(isActive: false),
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();
    clearInteractions(dio);
    final l10n = AppLocalizationsRu();
    await tester.tap(find.byType(SwitchListTile));
    await tester.pumpAndSettle();
    final save = find.text(l10n.teamAgentEditSave);
    await tester.tap(save);
    await tester.tap(save, warnIfMissed: false);
    await tester.pumpAndSettle();
    verify(
      dio.patch(
        any,
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);
  });

  testWidgets('13.3.1: каталог — FilterChip после загрузки', (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(FilterChip), findsOneWidget);
  });

  testWidgets('13.3.1: снять инструмент — PATCH с пустым tool_bindings', (tester) async {
    useViewSize(tester, const Size(800, 900));
    final dio = createDio();
    stubDialogDio(dio);
    stubTeamGet(dio);
    when(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: teamJson(),
        statusCode: 200,
        requestOptions: RequestOptions(path: '/p'),
      ),
    );

    final agent = sampleAgent(
      toolBindings: const [
        ToolBindingResponseModel(
          toolDefinitionId: '11111111-1111-4111-8111-111111111111',
          name: 'Tool One',
          category: 'cat',
        ),
      ],
    );

    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 600),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: agent,
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();

    final chip = find.byType(FilterChip);
    await tester.tap(chip);
    await tester.pumpAndSettle();

    final l10n = AppLocalizationsRu();
    await tester.tap(find.text(l10n.teamAgentEditSave));
    await tester.pumpAndSettle();

    verify(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: argThat(
          isA<Map<String, dynamic>>().having(
            (m) => m['tool_bindings'],
            'tool_bindings',
            isA<List<dynamic>>().having((l) => l.length, 'len', 0),
          ),
          named: 'data',
        ),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);
  });

  testWidgets('13.3.1: выбрать инструмент — PATCH с tool_bindings', (tester) async {
    useViewSize(tester, const Size(800, 900));
    final dio = createDio();
    stubDialogDio(dio);
    stubTeamGet(dio);
    when(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: teamJson(),
        statusCode: 200,
        requestOptions: RequestOptions(path: '/p'),
      ),
    );

    await tester.pumpWidget(
      dialogPushedHost(
        dio: dio,
        viewSize: const Size(800, 600),
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(FilterChip));
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    await tester.tap(find.text(l10n.teamAgentEditSave));
    await tester.pumpAndSettle();

    verify(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: argThat(
          isA<Map<String, dynamic>>()
              .having((m) => m.containsKey('tool_bindings'), 'has tb', true),
          named: 'data',
        ),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);
  });

  testWidgets('под AgentCard нет Switch', (tester) async {
    await tester.pumpWidget(
      wrapSimple(
        AgentCard(agent: sampleAgent(), onTap: () {}),
        locale: const Locale('ru'),
      ),
    );
    await tester.pumpAndSettle();
    expect(
      find.descendant(
        of: find.byType(AgentCard),
        matching: find.byType(Switch),
      ),
      findsNothing,
    );
  });

  // --- Sprint 15.e2e: provider_kind dropdown ----------------------------------
  // Регресс-страховка: dropdown отрисован, выбор kind улетает в PATCH,
  // сброс на «—» (Unset) приходит как provider_kind: null.

  const providerFieldKey = Key('agentEditDialog_providerKindField');

  testWidgets('provider_kind: dropdown отрисован с локализованным label',
      (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 700),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    expect(find.byKey(providerFieldKey), findsOneWidget);
    expect(find.text(l10n.teamAgentEditFieldProviderKind), findsOneWidget);
    expect(find.text(l10n.teamAgentEditFieldProviderKindHelp), findsOneWidget);
  });

  testWidgets(
    'provider_kind: выбор «deepseek» → PATCH c provider_kind: "deepseek"',
    (tester) async {
      final dio = createDio();
      stubDialogDio(dio);
      stubTeamGet(dio);
      when(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: teamJson(),
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/team/agents/$agentId',
          ),
        ),
      );

      useViewSize(tester, const Size(900, 1100));
      await tester.pumpWidget(
        dialogPushedHost(
          dio: dio,
          viewSize: const Size(900, 1100),
          body: agentEditDialogBodyForTesting(
            projectId: projectId,
            // Стартуем с пустым provider_kind, чтобы зафиксировать «omit -> value».
            agent: sampleAgent(),
            useAutofocus: false,
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('__open_dialog__'));
      await tester.pumpAndSettle();
      clearInteractions(dio);

      // Открываем dropdown по ключу и выбираем «deepseek».
      await tester.tap(find.byKey(providerFieldKey));
      await tester.pumpAndSettle();
      // В popup появляется несколько одинаковых пунктов (по числу dropdowns
      // на экране), но .last гарантированно из текущего меню.
      await tester.tap(find.text('deepseek').last);
      await tester.pumpAndSettle();

      final l10n = AppLocalizationsRu();
      await tester.ensureVisible(find.text(l10n.teamAgentEditSave));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10n.teamAgentEditSave));
      await tester.pumpAndSettle();

      verify(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: argThat(
            isA<Map<String, dynamic>>().having(
              (m) => m['provider_kind'],
              'provider_kind',
              'deepseek',
            ),
            named: 'data',
          ),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    },
  );

  testWidgets(
    'provider_kind: сброс на «—» → PATCH c provider_kind: null',
    (tester) async {
      final dio = createDio();
      stubDialogDio(dio);
      stubTeamGet(dio);
      when(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: teamJson(),
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/team/agents/$agentId',
          ),
        ),
      );

      useViewSize(tester, const Size(900, 1100));
      await tester.pumpWidget(
        dialogPushedHost(
          dio: dio,
          viewSize: const Size(900, 1100),
          body: agentEditDialogBodyForTesting(
            projectId: projectId,
            agent: sampleAgent(providerKind: 'deepseek'),
            useAutofocus: false,
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('__open_dialog__'));
      await tester.pumpAndSettle();
      clearInteractions(dio);

      // Открываем dropdown и тапаем «—» (l10n.teamAgentEditUnset).
      await tester.tap(find.byKey(providerFieldKey));
      await tester.pumpAndSettle();
      final l10n = AppLocalizationsRu();
      // teamAgentEditUnset присутствует в нескольких dropdown'ах (provider_kind,
      // code_backend, prompt). .last — пункт из активного меню.
      await tester.tap(find.text(l10n.teamAgentEditUnset).last);
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10n.teamAgentEditSave));
      await tester.pumpAndSettle();

      verify(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: argThat(
            isA<Map<String, dynamic>>()
                .having(
                  (m) => m.containsKey('provider_kind'),
                  'provider_kind present',
                  true,
                )
                .having(
                  (m) => m['provider_kind'],
                  'provider_kind value',
                  isNull,
                ),
            named: 'data',
          ),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    },
  );

  testWidgets(
    'provider_kind: без изменений — поле НЕ попадает в PATCH (omit)',
    (tester) async {
      final dio = createDio();
      stubDialogDio(dio);
      stubTeamGet(dio);
      when(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: anyNamed('data'),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => Response<dynamic>(
          data: teamJson(),
          statusCode: 200,
          requestOptions: RequestOptions(
            path: '/projects/$projectId/team/agents/$agentId',
          ),
        ),
      );

      useViewSize(tester, const Size(900, 1100));
      await tester.pumpWidget(
        dialogPushedHost(
          dio: dio,
          viewSize: const Size(900, 1100),
          body: agentEditDialogBodyForTesting(
            projectId: projectId,
            // У агента kind есть, но dirty-флаг провоцируем сменой is_active.
            agent: sampleAgent(providerKind: 'anthropic_oauth', isActive: false),
            useAutofocus: false,
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('__open_dialog__'));
      await tester.pumpAndSettle();
      clearInteractions(dio);

      final l10n = AppLocalizationsRu();
      await tester.ensureVisible(find.byType(SwitchListTile));
      await tester.pumpAndSettle();
      await tester.tap(find.byType(SwitchListTile));
      await tester.pumpAndSettle();
      await tester.ensureVisible(find.text(l10n.teamAgentEditSave));
      await tester.pumpAndSettle();
      await tester.tap(find.text(l10n.teamAgentEditSave));
      await tester.pumpAndSettle();

      verify(
        dio.patch(
          '/projects/$projectId/team/agents/$agentId',
          data: argThat(
            isA<Map<String, dynamic>>().having(
              (m) => m.containsKey('provider_kind'),
              'provider_kind absent',
              false,
            ),
            named: 'data',
          ),
          options: anyNamed('options'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).called(1);
    },
  );

  testWidgets('модель: поле ввода заблокировано, если providerKind не выбран', (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(providerKind: null),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    final textFormField = tester.widget<TextFormField>(find.byKey(const Key('agentEditDialog_modelField')));
    expect(textFormField.enabled, isFalse);
  });

  testWidgets('модель: поле ввода разблокировано при выбранном провайдере и предлагает варианты при тапе', (tester) async {
    final dio = createDio();
    stubDialogDio(dio);
    await tester.pumpWidget(
      wrap(
        agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(providerKind: 'anthropic', model: ''),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    
    final textFormField = tester.widget<TextFormField>(find.byKey(const Key('agentEditDialog_modelField')));
    expect(textFormField.enabled, isTrue);

    await tester.tap(find.byKey(const Key('agentEditDialog_modelField')));
    await tester.pumpAndSettle();

    expect(find.text('claude-3-5-sonnet-latest'), findsOneWidget);
  });

  testWidgets('Test Run без изменений — создаёт задачу и переходит на её детальный экран', (tester) async {
    useViewSize(tester, const Size(800, 1000));
    final dio = createDio();
    stubDialogDio(dio);
    final testTaskId = '11111111-2222-3333-4444-555555555555';
    
    when(
      dio.post<Map<String, dynamic>>(
        '/projects/$projectId/tasks',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<Map<String, dynamic>>(
        data: taskJson(testTaskId),
        statusCode: 201,
        requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
      ),
    );

    final navigatedPaths = <String>[];
    await tester.pumpWidget(
      wrapWithRouter(
        dio: dio,
        navigatedPaths: navigatedPaths,
        targetPath: '/projects/:id/tasks/:taskId',
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        viewSize: const Size(800, 1000),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();

    final testBtn = find.byKey(const Key('agentEditDialog_testRunButton'));
    expect(testBtn, findsOneWidget);
    await tester.ensureVisible(testBtn);
    await tester.tap(testBtn);
    await tester.pumpAndSettle();

    // Verify task creation request was sent
    verify(
      dio.post<Map<String, dynamic>>(
        '/projects/$projectId/tasks',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);

    // Verify PATCH was not called (not dirty)
    verifyNever(
      dio.patch(
        any,
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    );

    // Verify navigated to the task detail page
    expect(navigatedPaths, contains('/projects/$projectId/tasks/$testTaskId'));
  });

  testWidgets('Test Run при наличии изменений — сначала PATCH сохраняет, затем создаёт задачу и переходит', (tester) async {
    useViewSize(tester, const Size(800, 1000));
    final dio = createDio();
    stubDialogDio(dio);
    final testTaskId = '11111111-2222-3333-4444-555555555555';

    // Stub PATCH
    when(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: teamJson(),
        statusCode: 200,
        requestOptions: RequestOptions(path: '/projects/$projectId/team/agents/$agentId'),
      ),
    );

    // Stub GET team (triggered by ref.invalidate / refetch in save)
    stubTeamGet(dio);

    // Stub POST task
    when(
      dio.post<Map<String, dynamic>>(
        '/projects/$projectId/tasks',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).thenAnswer(
      (_) async => Response<Map<String, dynamic>>(
        data: taskJson(testTaskId),
        statusCode: 201,
        requestOptions: RequestOptions(path: '/projects/$projectId/tasks'),
      ),
    );

    final navigatedPaths = <String>[];
    await tester.pumpWidget(
      wrapWithRouter(
        dio: dio,
        navigatedPaths: navigatedPaths,
        targetPath: '/projects/:id/tasks/:taskId',
        body: agentEditDialogBodyForTesting(
          projectId: projectId,
          agent: sampleAgent(isActive: false),
          useAutofocus: false,
        ),
        viewSize: const Size(800, 1000),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('__open_dialog__'));
    await tester.pumpAndSettle();

    // Make the form dirty (toggle isActive switch)
    final switchTile = find.byType(SwitchListTile);
    await tester.ensureVisible(switchTile);
    await tester.tap(switchTile);
    await tester.pumpAndSettle();

    final testBtn = find.byKey(const Key('agentEditDialog_testRunButton'));
    await tester.ensureVisible(testBtn);
    await tester.tap(testBtn);
    await tester.pumpAndSettle();

    // Verify PATCH was called first
    verify(
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);

    // Verify task creation was sent
    verify(
      dio.post<Map<String, dynamic>>(
        '/projects/$projectId/tasks',
        data: anyNamed('data'),
        options: anyNamed('options'),
        cancelToken: anyNamed('cancelToken'),
      ),
    ).called(1);

    // Verify navigated to the task detail page
    expect(navigatedPaths, contains('/projects/$projectId/tasks/$testTaskId'));
  });
}
