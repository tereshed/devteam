import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/routing/project_dashboard_routes.dart';
import 'package:frontend/features/chat/data/chat_providers.dart';
import 'package:frontend/features/chat/domain/models.dart';
import 'package:frontend/features/chat/domain/requests.dart';
import 'package:frontend/features/chat/presentation/screens/chat_conversation_placeholder_screen.dart';
import 'package:frontend/features/chat/presentation/screens/chat_screen.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mockito/mockito.dart';

import '../../features/chat/presentation/controllers/chat_controller_test.mocks.dart';
import '../../features/projects/helpers/project_dashboard_test_router.dart';
import '../../features/projects/helpers/project_fixtures.dart';

/// UUID беседы для smoke-маршрута `/projects/:id/chat/:conversationId`.
const kChatConversationUuidForRoutingTest =
    'f47ac10b-58cc-4372-a567-0e02b2c3d479';

void main() {
  test('projectDashboardShellBranchTasksSegment в projectDashboardShellBranchPaths', () {
    expect(
      projectDashboardShellBranchPaths.contains(projectDashboardShellBranchTasksSegment),
      isTrue,
    );
  });

  test('projectDashboardDefaultBranch совпадает с первой веткой (SSOT)', () {
    expect(
      projectDashboardDefaultBranch,
      projectDashboardShellBranchPaths.first,
    );
  });
  test(
    'projectDashboardShellBranchPaths — длина совпадает с buildProjectDashboardShellBranches',
    () {
      final branches = buildProjectDashboardShellBranches(
        chatNavigatorKey: kTestShellChatKey,
        tasksNavigatorKey: kTestShellTasksKey,
        teamNavigatorKey: kTestShellTeamKey,
        settingsNavigatorKey: kTestShellSettingsKey,
      );
      expect(branches.length, projectDashboardShellBranchPaths.length);
    },
  );

  testWidgets(
    '/projects/new матчится литералом new, не как :id (smoke)',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/new',
      );
      await tester.pumpWidget(
        MaterialApp.router(routerConfig: router),
      );
      await tester.pumpAndSettle();
      expect(router.state.uri.path, '/projects/new');
      expect(find.text('__TEST_PROJECTS_NEW__'), findsOneWidget);
    },
  );

  testWidgets(
    'редирект /projects/:id?from=x сохраняет query на целевом URL',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid?from=x',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(id: kTestProjectUuid, name: 'Q'),
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
        router.state.uri.path,
        '/projects/$kTestProjectUuid/$projectDashboardDefaultBranch',
      );
      expect(router.state.uri.queryParameters['from'], 'x');
    },
  );

  testWidgets(
    'редирект с корня дашборда сохраняет fragment (#)',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid?from=x#section',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(id: kTestProjectUuid, name: 'Frag'),
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
      expect(router.state.uri.fragment, 'section');
      expect(router.state.uri.queryParameters['from'], 'x');
    },
  );

  testWidgets(
    'Sprint 11: /projects/:id/chat/extra (не-UUID) → редирект на /projects/:id/chat',
    (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat/extra',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(id: kTestProjectUuid, name: 'Q'),
            ),
          ],
          child: MaterialApp.router(
            routerConfig: router,
            localizationsDelegates: const [
              AppLocalizations.delegate,
              GlobalMaterialLocalizations.delegate,
              GlobalWidgetsLocalizations.delegate,
              GlobalCupertinoLocalizations.delegate,
            ],
            supportedLocales: const [Locale('en')],
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        router.state.uri.path,
        '/projects/$kTestProjectUuid/chat',
      );
      final ctx = tester.element(find.byType(ChatConversationPlaceholderScreen));
      expect(
        find.text(AppLocalizations.of(ctx)!.chatScreenSelectConversationHint),
        findsOneWidget,
      );
    },
  );

  testWidgets(
    '/projects/:id/chat/:conversationId (UUID) показывает ChatScreen',
    (tester) async {
      final repo = MockConversationRepository();
      when(
        repo.getConversation(
          kChatConversationUuidForRoutingTest,
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer(
        (_) async => ConversationModel(
          id: kChatConversationUuidForRoutingTest,
          projectId: kTestProjectUuid,
          title: 'Conv',
          status: 'active',
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 2),
        ),
      );
      when(
        repo.getMessages(
          kChatConversationUuidForRoutingTest,
          limit: anyNamed('limit'),
          offset: anyNamed('offset'),
          cancelToken: anyNamed('cancelToken'),
        ),
      ).thenAnswer((_) async => const MessageListResponse());

      final router = buildProjectDashboardTestRouter(
        initialLocation:
            '/projects/$kTestProjectUuid/chat/$kChatConversationUuidForRoutingTest',
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(id: kTestProjectUuid, name: 'R'),
            ),
            conversationRepositoryProvider.overrideWithValue(repo),
          ],
          child: MaterialApp.router(
            routerConfig: router,
            localizationsDelegates: const [
              AppLocalizations.delegate,
              GlobalMaterialLocalizations.delegate,
              GlobalWidgetsLocalizations.delegate,
              GlobalCupertinoLocalizations.delegate,
            ],
            supportedLocales: const [Locale('en')],
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        router.state.uri.path,
        '/projects/$kTestProjectUuid/chat/$kChatConversationUuidForRoutingTest',
      );
      expect(find.byType(ChatScreen), findsOneWidget);
    },
  );
}
