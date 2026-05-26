import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/api/api_exceptions.dart';
import 'package:frontend/core/api/websocket_events.dart';
import 'package:frontend/core/api/websocket_providers.dart';
import 'package:frontend/core/api/websocket_service.dart';
import 'package:frontend/features/chat/data/chat_providers.dart';
import 'package:frontend/features/chat/data/conversation_repository.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/presentation/screens/project_dashboard_screen.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_list_controller.dart';
import 'package:frontend/features/tasks/presentation/screens/tasks_list_screen.dart';
import 'package:frontend/features/tasks/presentation/state/task_states.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:mocktail/mocktail.dart';

import '../../../chat/helpers/chat_fixtures.dart';
import '../../../tasks/helpers/task_fixtures.dart';
import '../../helpers/project_dashboard_test_router.dart';
import '../../helpers/project_fixtures.dart';
import '../../helpers/test_wrappers.dart';

/// Список задач без HTTP — ветка shell «Tasks» монтирует [TasksListScreen] (12.4+).
class _StubTaskListForDashboardShellTest extends TaskListController {
  _StubTaskListForDashboardShellTest(this._seed);
  final TaskListState _seed;

  @override
  FutureOr<TaskListState> build({required String projectId}) => _seed;
}

class MockProjectRepository extends Mock implements ProjectRepository {}
class MockConversationRepository extends Mock implements ConversationRepository {}

class FakeCancelToken extends Fake implements CancelToken {}

ProviderScope _buildProviderScope({
  required List<Override> overrides,
  required Widget child,
}) {
  final taskSeed = makeTaskListStateFixture(
    isLoadingInitial: false,
    items: const [],
    total: 0,
  );
  return ProviderScope(
    retry: (_, _) => null,
    overrides: [
      taskListControllerProvider.overrideWith(
        () => _StubTaskListForDashboardShellTest(taskSeed),
      ),
      ...overrides,
    ],
    child: child,
  );
}

void main() {
  setUpAll(() {
    registerFallbackValue(FakeCancelToken());
  });

  group('ProjectDashboardScreen', () {
    late MockConversationRepository mockConvRepo;
    late Map<String, String> convToProjectMap;

    setUp(() {
      mockConvRepo = MockConversationRepository();
      convToProjectMap = {};

      when(() => mockConvRepo.listConversations(
            any(),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((inv) async {
            final projectId = inv.positionalArguments[0] as String;
            final conv = makeConversation(projectId: projectId);
            convToProjectMap[conv.id] = projectId;
            return ConversationListResponse(
              conversations: [conv],
            );
          });
      when(() => mockConvRepo.getConversation(
            any(),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((inv) async {
            final convId = inv.positionalArguments[0] as String;
            final projectId = convToProjectMap[convId] ?? kTestChatProjectUuid;
            return makeConversation(id: convId, projectId: projectId);
          });
      when(() => mockConvRepo.getMessages(
            any(),
            limit: any(named: 'limit'),
            offset: any(named: 'offset'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => const MessageListResponse(
            messages: [],
            hasNext: false,
          ));
    });

    testWidgets('успешная загрузка: заголовок AppBar — имя проекта', (
      tester,
    ) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(
                id: kTestProjectUuid,
                name: kTestDashboardProjectNameFixtureAlpha,
              ),
            ),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        find.descendant(
          of: find.byType(AppBar),
          matching: find.text(kTestDashboardProjectNameFixtureAlpha),
        ),
        findsOneWidget,
      );
    });

    testWidgets('загрузка: до ответа виден CircularProgressIndicator', (
      tester,
    ) async {
      final completer = Completer<ProjectModel>();
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(
              kTestProjectUuid,
            ).overrideWith((ref) => completer.future),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pump();
      expect(find.byKey(const ValueKey('project-dashboard-loading')), findsOneWidget);
      completer.complete(
        makeProject(
          id: kTestProjectUuid,
          name: kTestDashboardProjectNameAfterLoading,
        ),
      );
      await tester.pumpAndSettle();
      expect(
        find.descendant(
          of: find.byType(AppBar),
          matching: find.text(kTestDashboardProjectNameAfterLoading),
        ),
        findsOneWidget,
      );
    });

    testWidgets(
      'ошибка API (не 404): AppBar «Project», dataLoadError + Retry; после retry — имя',
      (tester) async {
        var attempt = 0;
        final router = buildProjectDashboardTestRouter(
          initialLocation: '/projects/$kTestProjectUuid/chat',
        );
        await tester.pumpWidget(
          _buildProviderScope(
            overrides: [
              webSocketServiceProvider.overrideWithValue(_Ws()),
              conversationRepositoryProvider.overrideWithValue(mockConvRepo),
              projectProvider(kTestProjectUuid).overrideWith((ref) async {
                attempt++;
                if (attempt == 1) {
                  throw ProjectApiException('fail', statusCode: 500);
                }
                return makeProject(
                  id: kTestProjectUuid,
                  name: kTestDashboardProjectNameAfterRetry,
                );
              }),
            ],
            child: MaterialApp.router(
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              locale: const Locale('en'),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(ProjectDashboardScreen)),
        )!;
        expect(
          find.descendant(
            of: find.byType(AppBar),
            matching: find.text(l10n.projectDashboardFallbackTitle),
          ),
          findsOneWidget,
        );
        expect(find.byKey(const ValueKey('project-dashboard-error')), findsOneWidget);
        await tester.tap(find.text(l10n.retry));
        await tester.pumpAndSettle();
        expect(
          find.descendant(
            of: find.byType(AppBar),
            matching: find.text(kTestDashboardProjectNameAfterRetry),
          ),
          findsOneWidget,
        );
        expect(attempt, 2);
      },
    );

    testWidgets(
      '404: без shell; AppBar «Project»; текст «not found» только в body; кнопка → /projects',
      (tester) async {
        late GoRouter router;
        router = buildProjectDashboardTestRouter(
          initialLocation: '/projects/$kTestProjectUuid/chat',
        );
        await tester.pumpWidget(
          _buildProviderScope(
            overrides: [
              webSocketServiceProvider.overrideWithValue(_Ws()),
              conversationRepositoryProvider.overrideWithValue(mockConvRepo),
              projectProvider(kTestProjectUuid).overrideWith(
                (ref) async => throw ProjectNotFoundException('missing'),
              ),
            ],
            child: MaterialApp.router(
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              locale: const Locale('en'),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(ProjectDashboardScreen)),
        )!;
        expect(find.text(l10n.dataLoadError), findsNothing);
        expect(find.byType(NavigationBar), findsNothing);
        expect(find.byType(NavigationRail), findsNothing);
        expect(find.text(l10n.projectDashboardNotFoundTitle), findsOneWidget);
        expect(
          find.descendant(
            of: find.byType(AppBar),
            matching: find.text(l10n.projectDashboardFallbackTitle),
          ),
          findsOneWidget,
        );
        await tester.tap(find.text(l10n.projectDashboardNotFoundBackToList));
        await tester.pumpAndSettle();
        expect(router.state.uri.path, '/projects');
        expect(find.text('__TEST_PROJECTS_LIST__'), findsOneWidget);
      },
    );

    testWidgets(
      '404: стрелка в AppBar ведёт на /projects (как кнопка в body)',
      (tester) async {
        late GoRouter router;
        router = buildProjectDashboardTestRouter(
          initialLocation: '/projects/$kTestProjectUuid/chat',
        );
        await tester.pumpWidget(
          _buildProviderScope(
            overrides: [
              webSocketServiceProvider.overrideWithValue(_Ws()),
              conversationRepositoryProvider.overrideWithValue(mockConvRepo),
              projectProvider(kTestProjectUuid).overrideWith(
                (ref) async => throw ProjectNotFoundException('missing'),
              ),
            ],
            child: MaterialApp.router(
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              locale: const Locale('en'),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        await tester.tap(find.byIcon(Icons.arrow_back));
        await tester.pumpAndSettle();
        expect(router.state.uri.path, '/projects');
        expect(find.text('__TEST_PROJECTS_LIST__'), findsOneWidget);
      },
    );

    testWidgets('Назад: canPop — после push выполняется pop (не go)', (
      tester,
    ) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/',
        routesBeforeProjects: [
          GoRoute(
            path: '/',
            builder: (context, state) => Scaffold(
              body: TextButton(
                onPressed: () =>
                    context.push('/projects/$kTestProjectUuid/chat'),
                child: const Text('__OPEN_PROJECT__'),
              ),
            ),
          ),
        ],
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(
                id: kTestProjectUuid,
                name: kTestDashboardProjectNamePopped,
              ),
            ),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.text('__OPEN_PROJECT__'));
      await tester.pumpAndSettle();
      expect(
        find.descendant(
          of: find.byType(AppBar),
          matching: find.text(kTestDashboardProjectNamePopped),
        ),
        findsOneWidget,
      );
      await tester.tap(find.byIcon(Icons.arrow_back));
      await tester.pumpAndSettle();
      expect(find.text('__OPEN_PROJECT__'), findsOneWidget);
    });

    testWidgets('Назад: deep link без стека — go на /projects', (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(
              kTestProjectUuid,
            ).overrideWith((ref) async => makeProject(id: kTestProjectUuid)),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.arrow_back));
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/projects');
    });

    testWidgets('редирект: /projects/:id/ с trailing slash → chat', (
      tester,
    ) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/',
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(
              kTestProjectUuid,
            ).overrideWith((ref) async => makeProject(id: kTestProjectUuid)),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/projects/$kTestProjectUuid/chat');
    });

    testWidgets('редирект: невалидный UUID → /projects', (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/not-a-uuid',
      );
      await tester.pumpWidget(
        MaterialApp.router(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('en'),
          routerConfig: router,
        ),
      );
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/projects');
      expect(find.text('__TEST_PROJECTS_LIST__'), findsOneWidget);
    });

    testWidgets('редирект: неизвестный сегмент после id → chat', (
      tester,
    ) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/typo',
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(
              kTestProjectUuid,
            ).overrideWith((ref) async => makeProject(id: kTestProjectUuid)),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/projects/$kTestProjectUuid/chat');
    });

    testWidgets(
      'смена раздела shell: Tasks показывает TasksListScreen (stub)',
      (tester) async {
        useViewSize(tester, const Size(400, 800));
        final router = buildProjectDashboardTestRouter(
          initialLocation: '/projects/$kTestProjectUuid/chat',
        );
        await tester.pumpWidget(
          _buildProviderScope(
            overrides: [
              webSocketServiceProvider.overrideWithValue(_Ws()),
              conversationRepositoryProvider.overrideWithValue(mockConvRepo),
              projectProvider(
                kTestProjectUuid,
              ).overrideWith((ref) async => makeProject(id: kTestProjectUuid)),
            ],
            child: MaterialApp.router(
              localizationsDelegates: AppLocalizations.localizationsDelegates,
              supportedLocales: AppLocalizations.supportedLocales,
              locale: const Locale('en'),
              routerConfig: router,
            ),
          ),
        );
        await tester.pumpAndSettle();
        final l10n = AppLocalizations.of(
          tester.element(find.byType(ProjectDashboardScreen)),
        )!;
        await tester.tap(
          find.descendant(
            of: find.byType(NavigationBar),
            matching: find.text(l10n.projectDashboardTasks),
          ),
        );
        await tester.pumpAndSettle();
        expect(router.state.uri.path, '/projects/$kTestProjectUuid/tasks');
        expect(find.byType(TasksListScreen), findsOneWidget);
        expect(find.text(l10n.tasksEmpty), findsOneWidget);
      },
    );

    testWidgets('dispose: CancelToken отменяется при снятии дерева', (
      tester,
    ) async {
      final repo = MockProjectRepository();
      CancelToken? captured;
      when(
        () => repo.getProject(any(), cancelToken: any(named: 'cancelToken')),
      ).thenAnswer((invocation) async {
        captured = invocation.namedArguments[#cancelToken] as CancelToken?;
        return Completer<ProjectModel>().future;
      });

      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectRepositoryProvider.overrideWithValue(repo),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: buildProjectDashboardTestRouter(
              initialLocation: '/projects/$kTestProjectUuid/chat',
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pumpWidget(const SizedBox());
      await tester.pump();
      expect(captured, isNotNull);
      expect(captured!.isCancelled, isTrue);
    });

    testWidgets('ru: 404 — notFound title и кнопка списка через l10n', (
      tester,
    ) async {
      late GoRouter router;
      router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => throw ProjectNotFoundException('missing'),
            ),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('ru'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(ProjectDashboardScreen)),
      )!;
      expect(find.text(l10n.projectDashboardNotFoundTitle), findsOneWidget);
      expect(
        find.text(l10n.projectDashboardNotFoundBackToList),
        findsOneWidget,
      );
    });

    testWidgets('go на другой projectId перезагружает данные', (tester) async {
      late GoRouter router;
      router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        _buildProviderScope(
          overrides: [
            webSocketServiceProvider.overrideWithValue(_Ws()),
            conversationRepositoryProvider.overrideWithValue(mockConvRepo),
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(
                id: kTestProjectUuid,
                name: kTestDashboardNavProjectNameA,
              ),
            ),
            projectProvider(kTestProjectUuidNavB).overrideWith(
              (ref) async => makeProject(
                id: kTestProjectUuidNavB,
                name: kTestDashboardNavProjectNameB,
              ),
            ),
          ],
          child: MaterialApp.router(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('en'),
            routerConfig: router,
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        find.descendant(
          of: find.byType(AppBar),
          matching: find.text(kTestDashboardNavProjectNameA),
        ),
        findsOneWidget,
      );

      router.go('/projects/$kTestProjectUuidNavB/chat');
      await tester.pumpAndSettle();
      expect(
        find.descendant(
          of: find.byType(AppBar),
          matching: find.text(kTestDashboardNavProjectNameB),
        ),
        findsOneWidget,
      );
    });
  });
}

class _Ws extends WebSocketService {
  _Ws()
      : super(
          baseUrl: 'http://127.0.0.1:8080/api/v1',
          channelFactory: (_, {protocols}) =>
              throw UnimplementedError('not used'),
          authProvider: () async => const WsAuth.none(),
        );

  @override
  Stream<WsClientEvent> get events => const Stream.empty();

  @override
  Stream<WsClientEvent> connect(String projectId) => const Stream.empty();

  @override
  void disconnect() {}
}
