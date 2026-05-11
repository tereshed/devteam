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
import 'package:frontend/features/projects/domain/models/agent_model.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/team/data/team_repository.dart';
import 'package:frontend/features/team/presentation/widgets/agent_card.dart';
import 'package:frontend/features/team/presentation/widgets/agent_edit_dialog.dart';
import 'package:frontend/l10n/app_localizations_ru.dart';
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';

import '../../../projects/helpers/test_wrappers.dart';
import 'agent_edit_dialog_test.mocks.dart';

@GenerateNiceMocks([MockSpec<Dio>()])
void main() {
  const projectId = '550e8400-e29b-41d4-a716-446655440000';
  const agentId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';

  Map<String, dynamic> teamJson() => <String, dynamic>{
        'id': 'team-1',
        'name': 'Dev Team',
        'project_id': projectId,
        'type': 'development',
        'agents': <Map<String, dynamic>>[],
        'created_at': '2026-04-27T09:00:00Z',
        'updated_at': '2026-04-27T09:15:00Z',
      };

  AgentModel sampleAgent({
    String? promptId,
    bool isActive = true,
    String? model = 'm1',
  }) {
    return AgentModel(
      id: agentId,
      name: 'Agent',
      role: 'developer',
      model: model,
      promptName: null,
      promptId: promptId,
      codeBackend: 'claude-code',
      isActive: isActive,
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
      of: find.byType(TextFormField),
      matching: find.byType(EditableText),
    );
    return tester.widget<EditableText>(editable).focusNode.hasFocus;
  }

  testWidgets('форма: поля видны (широкий)', (tester) async {
    final dio = createDio();
    stubPrompts(dio);
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
  });

  testWidgets('форма: поля видны (узкий)', (tester) async {
    final dio = createDio();
    stubPrompts(dio);
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
    final dio = createDio();
    stubPrompts(dio);
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
    final dio = createDio();
    stubPrompts(dio);
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
    final dio = createDio();
    stubPrompts(dio);
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
    await tester.tap(find.text(l10n.teamAgentEditSave));
    await tester.pumpAndSettle();

    // Полная цепочка на экране команды: getTeam (экран) → patch → invalidate → getTeam.
    // dialogPushedHost не монтирует TeamScreen — первый getTeam здесь не фиксируется.
    verifyInOrder([
      dio.patch(
        '/projects/$projectId/team/agents/$agentId',
        data: argThat(
          isA<Map<String, dynamic>>()
              .having((m) => m['is_active'], 'is_active', true),
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
    final dio = createDio();
    stubPrompts(dio);
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
    final dio = createDio();
    stubPrompts(dio);
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

  testWidgets('ошибка GET /prompts — секция промпта в ошибке', (tester) async {
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
          agent: sampleAgent(),
          useAutofocus: false,
        ),
        dio: dio,
        viewSize: const Size(800, 600),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizationsRu();
    expect(find.text(l10n.teamAgentEditPromptsLoadError), findsWidgets);
    expect(find.byType(TextFormField), findsOneWidget);
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

  // П. 11 (канон): Padding(viewInsets) на узком shell — в showAgentEditDialog; тесты
  // помпают только тело формы — регрессию оболочки см. интеграцию / экран команды.

  testWidgets('автофокус поля модели при useAutofocus true', (tester) async {
    final dio = createDio();
    stubPrompts(dio);
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
    expect(modelFieldHasFocus(tester), isTrue);
  });

  testWidgets('без автофокуса при useAutofocus false', (tester) async {
    final dio = createDio();
    stubPrompts(dio);
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
    expect(modelFieldHasFocus(tester), isFalse);
  });

  testWidgets('двойной Save — один PATCH', (tester) async {
    final dio = createDio();
    stubPrompts(dio);
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
}
