import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/screens/projects_list_screen.dart';

import '../../helpers/project_fixtures.dart';
import '../../helpers/test_wrappers.dart';

void main() {
  group('ProjectsListScreen', () {
    testWidgets('показывает loading state при первой загрузке', (tester) async {
      final override = projectListProvider.overrideWith(
        (ref, args) => Completer<ProjectListResponse>().future,
      );
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });

    testWidgets(
      'показывает empty state без фильтра (folder_open + FilledButton)',
      (tester) async {
        final override = projectListProvider.overrideWith(
          (ref, args) async => makeResponse([]),
        );
        await tester.pumpWidget(
          wrapRouter(
            overrides: [override],
            builder: (context, state) => const ProjectsListScreen(),
          ),
        );
        await tester.pump();
        await tester.pump();
        await tester.pumpAndSettle();
        expect(find.byIcon(Icons.folder_open), findsOneWidget);
        expect(find.bySubtype<FilledButton>(), findsOneWidget);
      },
    );

    testWidgets('показывает карточки проектов', (tester) async {
      final projects = [
        makeProject(id: 'p1', name: 'Alpha'),
        makeProject(id: 'p2', name: 'Beta'),
      ];
      final override = projectListProvider.overrideWith(
        (ref, args) async => makeResponse(projects),
      );
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();
      expect(find.text('Alpha'), findsOneWidget);
      expect(find.text('Beta'), findsOneWidget);
    });

    testWidgets(
      'показывает error state при первой ошибке (error_outline + FilledButton retry)',
      (tester) async {
        final override = projectListProvider.overrideWith(
          (ref, args) async => throw ProjectApiException(
            'server error',
            statusCode: 500,
          ),
        );
        await tester.pumpWidget(
          wrapRouter(
            overrides: [override],
            builder: (context, state) => const ProjectsListScreen(),
          ),
        );
        await tester.pump();
        await tester.pump();
        await tester.pumpAndSettle();
        expect(find.byIcon(Icons.error_outline), findsOneWidget);
        expect(find.bySubtype<FilledButton>(), findsOneWidget);
      },
    );

    testWidgets('статус-фильтр: тап активирует, повторный тап снимает',
        (tester) async {
      final override = projectListProvider.overrideWith(
        (ref, args) async => makeResponse([]),
      );
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();

      await tester.tap(find.text('Active'));
      await tester.pumpAndSettle();
      final activeChip = tester.widget<FilterChip>(
        find.ancestor(
          of: find.text('Active'),
          matching: find.byType(FilterChip),
        ),
      );
      expect(activeChip.selected, isTrue);
      expect(find.byIcon(Icons.search_off), findsOneWidget);

      await tester.tap(find.text('Active'));
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.folder_open), findsOneWidget);
    });

    testWidgets('empty state с фильтром: search_off + TextButton очистки',
        (tester) async {
      final override = projectListProvider.overrideWith(
        (ref, args) async => makeResponse([]),
      );
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();

      await tester.tap(find.text('Active'));
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.search_off), findsOneWidget);
      expect(find.byType(TextButton), findsOneWidget);

      await tester.tap(find.byType(TextButton));
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.folder_open), findsOneWidget);
    });

    testWidgets('кнопка очистки поиска: появляется при вводе, исчезает по тапу',
        (tester) async {
      final override = projectListProvider.overrideWith(
        (ref, args) async => makeResponse([]),
      );
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();

      expect(find.byIcon(Icons.clear), findsNothing);

      await tester.enterText(find.byType(TextField), 'test');
      await tester.pump();
      expect(find.byIcon(Icons.clear), findsOneWidget);

      await tester.tap(find.byIcon(Icons.clear));
      await tester.pump();
      expect(find.byIcon(Icons.clear), findsNothing);
    });

    testWidgets('дебаунс: запрос не отправляется до 400 мс, отправляется после',
        (tester) async {
      final override = projectListProvider.overrideWith(
        (ref, args) async => makeResponse([]),
      );
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.folder_open), findsOneWidget);

      await tester.enterText(find.byType(TextField), 'abc');
      await tester.pump();
      expect(find.byIcon(Icons.clear), findsOneWidget);

      await tester.pump(const Duration(milliseconds: 200));
      expect(find.byIcon(Icons.folder_open), findsOneWidget);

      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.search_off), findsOneWidget);
    });

    testWidgets(
      'pull-to-refresh: после settle карточка на месте, список перезагружен',
      (tester) async {
        useViewSize(tester, const Size(400, 800));

        var loadCount = 0;
        final projects = [makeProject(id: 'p1', name: 'Alpha')];
        final override = projectListProvider.overrideWith((ref, args) async {
          loadCount++;
          if (loadCount > 1) {
            await Future<void>.delayed(const Duration(milliseconds: 120));
          }
          return makeResponse(projects);
        });
        await tester.pumpWidget(
          wrapRouter(
            overrides: [override],
            builder: (context, state) => const ProjectsListScreen(),
          ),
        );
        await tester.pump();
        await tester.pump();
        await tester.pumpAndSettle();
        expect(find.byKey(const Key('project-card-p1')), findsOneWidget);

        final refreshFuture = tester
            .state<RefreshIndicatorState>(find.byType(RefreshIndicator))
            .show();
        // show() цепляет onRefresh после snap; Riverpod — после vsync: нужны pump'и.
        await tester.pump();
        await tester.pumpAndSettle(const Duration(seconds: 5));
        await refreshFuture;
        await tester.pumpAndSettle();

        expect(loadCount, greaterThanOrEqualTo(2));
        expect(find.byKey(const Key('project-card-p1')), findsOneWidget);
      },
    );

    testWidgets('ошибка при refresh: SnackBar и список остаётся на экране',
        (tester) async {
      useViewSize(tester, const Size(400, 800));

      var n = 0;
      final projects = [makeProject(id: 'p1', name: 'Alpha')];
      final override = projectListProvider.overrideWith((ref, args) async {
        n++;
        if (n == 1) {
          return makeResponse(projects);
        }
        throw ProjectApiException('e', statusCode: 500);
      });
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();
      expect(find.text('Alpha'), findsOneWidget);

      final refreshFuture = tester
          .state<RefreshIndicatorState>(find.byType(RefreshIndicator))
          .show();
      await tester.pumpAndSettle(const Duration(seconds: 5));
      await refreshFuture;

      expect(find.byType(SnackBar), findsOneWidget);
      expect(find.text('Alpha'), findsOneWidget);
    });

    testWidgets('error state: Retry загружает данные', (tester) async {
      var n = 0;
      final projects = [makeProject(id: 'p1', name: 'Alpha')];
      final override = projectListProvider.overrideWith((ref, args) async {
        n++;
        if (n == 1) {
          throw ProjectApiException('e', statusCode: 500);
        }
        return makeResponse(projects);
      });
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.error_outline), findsOneWidget);

      await tester.tap(find.bySubtype<FilledButton>());
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();

      expect(find.text('Alpha'), findsOneWidget);
      expect(find.byIcon(Icons.error_outline), findsNothing);
    });

    testWidgets('дебаунс: после dispose таймер не бросает исключение',
        (tester) async {
      final override = projectListProvider.overrideWith(
        (ref, args) async => makeResponse([]),
      );
      await tester.pumpWidget(
        wrapRouter(
          overrides: [override],
          builder: (context, state) => const ProjectsListScreen(),
        ),
      );
      await tester.pump();
      await tester.pump();
      await tester.pumpAndSettle();

      await tester.enterText(find.byType(TextField), 'x');
      await tester.pump(const Duration(milliseconds: 50));
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump(const Duration(milliseconds: 400));
      expect(tester.takeException(), isNull);
    });

    group('адаптивная верстка', () {
      testWidgets('mobile (400×800): рендерит ListView, не GridView',
          (tester) async {
        useViewSize(tester, const Size(400, 800));

        final projects = [makeProject(id: 'p1', name: 'Alpha')];
        final override = projectListProvider.overrideWith(
          (ref, args) async => makeResponse(projects),
        );
        await tester.pumpWidget(
          wrapRouter(
            overrides: [override],
            builder: (context, state) => const ProjectsListScreen(),
          ),
        );
        await tester.pump();
        await tester.pump();
        await tester.pumpAndSettle();

        expect(find.byType(GridView), findsNothing);
        expect(find.byType(ListView), findsWidgets);
      });

      testWidgets('desktop (1400×900): рендерит GridView, не plain ListView',
          (tester) async {
        useViewSize(tester, const Size(1400, 900));

        final projects = [makeProject(id: 'p1', name: 'Alpha')];
        final override = projectListProvider.overrideWith(
          (ref, args) async => makeResponse(projects),
        );
        await tester.pumpWidget(
          wrapRouter(
            overrides: [override],
            builder: (context, state) => const ProjectsListScreen(),
          ),
        );
        await tester.pump();
        await tester.pump();
        await tester.pumpAndSettle();

        expect(find.byType(GridView), findsOneWidget);
      });

      testWidgets('GridView карточка не overflow при textScaleFactor 1.5',
          (tester) async {
        useViewSize(tester, const Size(1400, 900));

        final override = projectListProvider.overrideWith(
          (ref, args) async => makeResponse([makeProject()]),
        );
        await tester.pumpWidget(
          MediaQuery(
            data: const MediaQueryData(textScaler: TextScaler.linear(1.5)),
            child: wrapRouter(
              overrides: [override],
              builder: (context, state) => const ProjectsListScreen(),
            ),
          ),
        );
        await tester.pump();
        await tester.pump();
        await tester.pumpAndSettle();
        expect(
          tester.takeException(),
          isNull,
          reason:
              'RenderFlex overflow при textScaleFactor 1.5 — увеличьте mainAxisExtent',
        );
      });
    });
  });
}
