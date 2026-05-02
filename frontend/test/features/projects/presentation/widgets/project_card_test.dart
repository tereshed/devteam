import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/presentation/widgets/project_card.dart';
import 'package:frontend/features/projects/presentation/widgets/project_status_chip.dart';
import 'package:frontend/l10n/app_localizations.dart';

import 'package:go_router/go_router.dart';

import '../../helpers/project_fixtures.dart';
import '../../helpers/test_wrappers.dart';

void main() {
  group('ProjectCard', () {
    testWidgets('отображает название и описание', (tester) async {
      await tester.pumpWidget(
        wrapRouter(
          builder: (context, state) =>
              Scaffold(body: ProjectCard(project: makeProject())),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('Test Project'), findsOneWidget);
      expect(find.text('A test project'), findsOneWidget);
    });

    testWidgets('не отображает описание если оно пустое', (tester) async {
      await tester.pumpWidget(
        wrapRouter(
          builder: (context, state) => Scaffold(
            body: ProjectCard(project: makeProject(description: '')),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('A test project'), findsNothing);
    });

    testWidgets('содержит ProjectStatusChip', (tester) async {
      await tester.pumpWidget(
        wrapRouter(
          builder: (context, state) =>
              Scaffold(body: ProjectCard(project: makeProject())),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(ProjectStatusChip), findsOneWidget);
    });

    testWidgets('отображает локализованное имя git-провайдера', (tester) async {
      await tester.pumpWidget(
        wrapRouter(
          builder: (context, state) =>
              Scaffold(body: ProjectCard(project: makeProject())),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('github'), findsNothing);
      expect(find.text('GitHub'), findsOneWidget);
    });

    testWidgets('переходит на /projects/:id при нажатии', (tester) async {
      final navigatedIds = <String>[];
      await tester.pumpWidget(
        MaterialApp.router(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('en'),
          routerConfig: GoRouter(
            routes: [
              GoRoute(
                path: '/',
                builder: (context, state) => Scaffold(
                  body: ProjectCard(project: makeProject(id: 'proj-42')),
                ),
              ),
              GoRoute(
                path: '/projects/:id',
                builder: (context, state) {
                  navigatedIds.add(state.pathParameters['id']!);
                  return const SizedBox();
                },
              ),
            ],
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('project-card-proj-42')));
      await tester.pumpAndSettle();
      expect(navigatedIds, contains('proj-42'));
    });
  });
}
