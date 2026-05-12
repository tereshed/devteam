@TestOn('vm')
@Tags(['widget'])
library;

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/dio_providers.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/admin/prompts/data/prompts_providers.dart';
import 'package:frontend/features/admin/prompts/data/prompts_repository.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/team/data/team_repository.dart';
import 'package:frontend/features/team/data/tools_providers.dart';
import 'package:frontend/features/team/data/tools_repository.dart';
import 'package:frontend/features/team/domain/team_exceptions.dart';
import 'package:frontend/features/team/presentation/screens/team_screen.dart';
import 'package:frontend/features/team/presentation/widgets/agent_card.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/l10n/app_localizations_ru.dart';
import 'package:mockito/mockito.dart';

import '../widgets/agent_edit_dialog_test.mocks.dart';

const _projectId = '550e8400-e29b-41d4-a716-446655440000';
const _agentId = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';

TeamModel _team({
  String name = 'Dev Team',
  String type = 'development',
  List<AgentModel> agents = const <AgentModel>[],
}) {
  return TeamModel(
    id: 'team-1',
    name: name,
    projectId: _projectId,
    type: type,
    agents: agents,
    createdAt: DateTime.utc(2026, 4, 27, 9),
    updatedAt: DateTime.utc(2026, 4, 27, 9, 15),
  );
}

AgentModel _agent({
  String id = _agentId,
  String name = 'Planner',
  String role = 'planner',
  bool isActive = true,
}) {
  return AgentModel(
    id: id,
    name: name,
    role: role,
    model: 'claude-opus-4-7',
    promptName: null,
    promptId: null,
    codeBackend: 'claude-code',
    isActive: isActive,
  );
}

Widget _harness({
  required List<Override> overrides,
  Locale locale = const Locale('ru'),
  Size? viewSize,
}) {
  final scoped = ProviderScope(
    retry: (_, _) => null,
    overrides: overrides,
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: locale,
      home: const Scaffold(body: TeamScreen(projectId: _projectId)),
    ),
  );
  if (viewSize == null) {
    return scoped;
  }
  return MediaQuery(
    data: MediaQueryData(size: viewSize),
    child: scoped,
  );
}

void main() {
  final l10nRu = AppLocalizationsRu();

  testWidgets('loading: CircularProgressIndicator до резолва future', (tester) async {
    final completer = Completer<TeamModel>();
    await tester.pumpWidget(
      _harness(
        overrides: [
          teamProvider(_projectId).overrideWith((ref) => completer.future),
        ],
      ),
    );
    await tester.pump();
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.byType(DataLoadErrorMessage), findsNothing);

    completer.complete(_team());
    await tester.pumpAndSettle();
    expect(find.byType(CircularProgressIndicator), findsNothing);
  });

  testWidgets('error: DataLoadErrorMessage с dataLoadError + retry; tap → перезагрузка', (tester) async {
    var attempt = 0;
    await tester.pumpWidget(
      _harness(
        overrides: [
          teamProvider(_projectId).overrideWith((ref) async {
            attempt++;
            if (attempt == 1) {
              throw TeamApiException('boom', statusCode: 500);
            }
            return _team(agents: [_agent()]);
          }),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(DataLoadErrorMessage), findsOneWidget);
    expect(find.text(l10nRu.dataLoadError), findsOneWidget);
    expect(find.text(l10nRu.retry), findsOneWidget);
    expect(find.byType(AgentCard), findsNothing);

    await tester.tap(find.text(l10nRu.retry));
    await tester.pumpAndSettle();

    expect(attempt, 2);
    expect(find.byType(DataLoadErrorMessage), findsNothing);
    expect(find.byType(AgentCard), findsOneWidget);
  });

  testWidgets('успех: заголовок (team.name + team.type) + AgentCard по агенту', (tester) async {
    final team = _team(
      name: 'Команда «Альфа»',
      type: 'development',
      agents: [
        _agent(id: _agentId, name: 'Planner', role: 'planner'),
        _agent(
          id: 'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',
          name: 'Dev',
          role: 'developer',
        ),
      ],
    );
    await tester.pumpWidget(
      _harness(
        overrides: [
          teamProvider(_projectId).overrideWith((ref) async => team),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Команда «Альфа»'), findsOneWidget);
    expect(find.text('development'), findsOneWidget);
    expect(find.byType(AgentCard), findsNWidgets(2));
    expect(find.text('Planner'), findsOneWidget);
    expect(find.text('Dev'), findsOneWidget);
    expect(find.text(l10nRu.teamEmptyAgents), findsNothing);
  });

  testWidgets('пустой список агентов: teamEmptyAgents, нет AgentCard', (tester) async {
    await tester.pumpWidget(
      _harness(
        overrides: [
          teamProvider(_projectId).overrideWith((ref) async => _team()),
        ],
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text(l10nRu.teamEmptyAgents), findsOneWidget);
    expect(find.byType(AgentCard), findsNothing);
  });

  testWidgets('RefreshIndicator: pull-to-refresh инвалидирует teamProvider', (tester) async {
    var calls = 0;
    final teams = [
      _team(agents: [_agent(name: 'First')]),
      _team(agents: [_agent(name: 'Second')]),
    ];
    await tester.pumpWidget(
      _harness(
        overrides: [
          teamProvider(_projectId).overrideWith((ref) async {
            final i = calls;
            calls++;
            return teams[i.clamp(0, teams.length - 1)];
          }),
        ],
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('First'), findsOneWidget);
    expect(calls, 1);

    await tester.fling(find.byType(ListView), const Offset(0, 400), 1000);
    await tester.pumpAndSettle();

    expect(calls, 2);
    expect(find.text('Second'), findsOneWidget);
  });

  testWidgets('tap по AgentCard открывает диалог редактирования (широкий экран)', (tester) async {
    // 13.3 диалог монтируется как showDialog: для широкого экрана нужны валидные Dio-стабы.
    final dio = MockDio();
    when(
      dio.get('/prompts', cancelToken: anyNamed('cancelToken')),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: <dynamic>[],
        statusCode: 200,
        requestOptions: RequestOptions(path: '/prompts'),
      ),
    );
    when(
      dio.get('/tool-definitions', cancelToken: anyNamed('cancelToken')),
    ).thenAnswer(
      (_) async => Response<dynamic>(
        data: <dynamic>[],
        statusCode: 200,
        requestOptions: RequestOptions(path: '/tool-definitions'),
      ),
    );

    final team = _team(agents: [_agent(name: 'Planner')]);
    await tester.pumpWidget(
      _harness(
        viewSize: const Size(900, 800),
        overrides: [
          dioClientProvider.overrideWithValue(dio),
          teamRepositoryProvider.overrideWithValue(TeamRepository(dio: dio)),
          promptsRepositoryProvider
              .overrideWithValue(PromptsRepository(dio: dio)),
          toolsRepositoryProvider.overrideWithValue(ToolsRepository(dio: dio)),
          teamProvider(_projectId).overrideWith((ref) async => team),
        ],
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byType(AgentCard));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('agentEditDialogBody')), findsOneWidget);
  });

  testWidgets('нет хардкода: текст ошибки и retry приходят из AppLocalizations', (tester) async {
    await tester.pumpWidget(
      _harness(
        locale: const Locale('en'),
        overrides: [
          teamProvider(_projectId).overrideWith(
            (ref) async => throw TeamApiException('x', statusCode: 500),
          ),
        ],
      ),
    );
    await tester.pumpAndSettle();

    final ctx = tester.element(find.byType(DataLoadErrorMessage));
    final l10nEn = AppLocalizations.of(ctx)!;
    expect(find.text(l10nEn.dataLoadError), findsOneWidget);
    expect(find.text(l10nEn.retry), findsOneWidget);
    expect(l10nEn.dataLoadError, isNot(equals(l10nRu.dataLoadError)));
  });
}
