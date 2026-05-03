import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/presentation/screens/project_dashboard_screen.dart';
import 'package:frontend/features/projects/presentation/widgets/project_destination_placeholder.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:mocktail/mocktail.dart';

import '../../helpers/project_dashboard_test_router.dart';
import '../../helpers/project_fixtures.dart';
import '../../helpers/test_wrappers.dart';

class MockProjectRepository extends Mock implements ProjectRepository {}

class FakeCancelToken extends Fake implements CancelToken {}

void main() {
  setUpAll(() {
    registerFallbackValue(FakeCancelToken());
  });

  group('ProjectDashboardScreen', () {
    testWidgets('успешная загрузка: заголовок AppBar — имя проекта', (
      tester,
    ) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
      expect(find.text(kTestDashboardProjectNameFixtureAlpha), findsOneWidget);
    });

    testWidgets('загрузка: до ответа виден CircularProgressIndicator', (
      tester,
    ) async {
      final completer = Completer<ProjectModel>();
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      completer.complete(
        makeProject(
          id: kTestProjectUuid,
          name: kTestDashboardProjectNameAfterLoading,
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text(kTestDashboardProjectNameAfterLoading), findsOneWidget);
    });

    testWidgets(
      'ошибка API (не 404): AppBar «Project», dataLoadError + Retry; после retry — имя',
      (tester) async {
        var attempt = 0;
        final router = buildProjectDashboardTestRouter(
          initialLocation: '/projects/$kTestProjectUuid/chat',
        );
        await tester.pumpWidget(
          ProviderScope(
            retry: (_, _) => null,
            overrides: [
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
        expect(find.text(l10n.dataLoadError), findsOneWidget);
        await tester.tap(find.text(l10n.retry));
        await tester.pumpAndSettle();
        expect(find.text(kTestDashboardProjectNameAfterRetry), findsOneWidget);
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
          ProviderScope(
            retry: (_, _) => null,
            overrides: [
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
          ProviderScope(
            retry: (_, _) => null,
            overrides: [
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
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
      expect(find.text(kTestDashboardProjectNamePopped), findsOneWidget);
      await tester.tap(find.byIcon(Icons.arrow_back));
      await tester.pumpAndSettle();
      expect(find.text('__OPEN_PROJECT__'), findsOneWidget);
    });

    testWidgets('Назад: deep link без стека — go на /projects', (tester) async {
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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

    testWidgets('смена раздела shell: Tasks показывает плейсхолдер задач', (
      tester,
    ) async {
      useViewSize(tester, const Size(400, 800));
      final router = buildProjectDashboardTestRouter(
        initialLocation: '/projects/$kTestProjectUuid/chat',
      );
      await tester.pumpWidget(
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
      expect(
        find.descendant(
          of: find.byType(ProjectDestinationPlaceholder),
          matching: find.text(l10n.projectDashboardTasks),
        ),
        findsOneWidget,
      );
    });

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
        ProviderScope(
          retry: (_, _) => null,
          overrides: [projectRepositoryProvider.overrideWithValue(repo)],
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
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
        ProviderScope(
          retry: (_, _) => null,
          overrides: [
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
      expect(find.text(kTestDashboardNavProjectNameA), findsOneWidget);

      router.go('/projects/$kTestProjectUuidNavB/chat');
      await tester.pumpAndSettle();
      expect(find.text(kTestDashboardNavProjectNameB), findsOneWidget);
    });
  });
}
