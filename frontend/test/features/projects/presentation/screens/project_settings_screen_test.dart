import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/projects/data/project_providers.dart';
import 'package:frontend/features/projects/data/project_repository.dart';
import 'package:frontend/features/projects/domain/project_exceptions.dart';
import 'package:frontend/features/projects/domain/requests.dart';
import 'package:frontend/features/projects/presentation/screens/project_settings_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mocktail/mocktail.dart';

import '../../helpers/project_dashboard_test_router.dart';
import '../../helpers/project_fixtures.dart';
import '../../helpers/test_wrappers.dart';

class MockProjectRepository extends Mock implements ProjectRepository {}

class FakeCancelToken extends Fake implements CancelToken {}

Widget _harness({
  required Widget child,
  List<Override> overrides = const [],
}) {
  return ProviderScope(
    retry: (_, _) => null,
    overrides: overrides,
    child: MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('en'),
      home: Scaffold(body: child),
    ),
  );
}

void main() {
  setUpAll(() {
    registerFallbackValue(FakeCancelToken());
    registerFallbackValue(const UpdateProjectRequest());
  });

  group('ProjectSettingsScreen', () {
    testWidgets('успешная загрузка: видны данные проекта (URL и коллекция)', (
      tester,
    ) async {
      const url = 'https://github.com/org/repo.git';
      const vector = 'MyCollection';
      await tester.pumpWidget(
        _harness(
          child: const ProjectSettingsScreen(projectId: kTestProjectUuid),
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => makeProject(
                id: kTestProjectUuid,
                gitUrl: url,
                vectorCollection: vector,
              ),
            ),
          ],
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(ProjectSettingsScreen), findsOneWidget);
      final urlField = tester.widget<TextFormField>(
        find.byKey(const ValueKey('project-settings-git-url')),
      );
      expect(urlField.controller?.text, url);
      final vectorField = tester.widget<TextFormField>(
        find.byKey(const ValueKey('project-settings-vector-collection')),
      );
      expect(vectorField.controller?.text, vector);
    });

    testWidgets('ошибка загрузки (не 404): DataLoadErrorMessage и Retry', (
      tester,
    ) async {
      await tester.pumpWidget(
        _harness(
          child: const ProjectSettingsScreen(projectId: kTestProjectUuid),
          overrides: [
            projectProvider(kTestProjectUuid).overrideWith(
              (ref) async => throw ProjectApiException(
                'server',
                statusCode: 503,
              ),
            ),
          ],
        ),
      );
      await tester.pumpAndSettle();
      final l10n = AppLocalizations.of(
        tester.element(find.byType(ProjectSettingsScreen)),
      )!;
      expect(find.byType(DataLoadErrorMessage), findsOneWidget);
      expect(find.text(l10n.dataLoadError), findsOneWidget);
      expect(find.text(l10n.retry), findsOneWidget);
      await tester.tap(find.text(l10n.retry));
      await tester.pump();
    });

    testWidgets('Save: updateProject и invalidate провайдера', (tester) async {
      useViewSize(tester, const Size(800, 1600));
      final mockRepo = MockProjectRepository();
      var loadCount = 0;

      when(
        () => mockRepo.getProject(any(), cancelToken: any(named: 'cancelToken')),
      ).thenAnswer((_) async {
        loadCount++;
        return makeProject(
          id: kTestProjectUuid,
          gitDefaultBranch: loadCount >= 2 ? 'develop' : 'main',
        );
      });

      when(
        () => mockRepo.updateProject(
          any(),
          any(),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).thenAnswer(
        (_) async => makeProject(
          id: kTestProjectUuid,
          gitDefaultBranch: 'develop',
        ),
      );

      await tester.pumpWidget(
        _harness(
          child: const ProjectSettingsScreen(projectId: kTestProjectUuid),
          overrides: [
            projectRepositoryProvider.overrideWithValue(mockRepo),
            projectProvider(kTestProjectUuid).overrideWith((ref) async {
              final repo = ref.read(projectRepositoryProvider);
              return repo.getProject(kTestProjectUuid);
            }),
          ],
        ),
      );
      await tester.pumpAndSettle();

      final branchField = find.byKey(const ValueKey('project-settings-git-branch'));
      await tester.enterText(branchField, 'develop');
      await tester.pump();

      final l10n = AppLocalizations.of(
        tester.element(find.byType(ProjectSettingsScreen)),
      )!;
      final saveBtn = find.text(l10n.projectSettingsSave);
      await tester.ensureVisible(saveBtn);
      await tester.tap(saveBtn);
      await tester.pumpAndSettle();

      verify(
        () => mockRepo.updateProject(
          kTestProjectUuid,
          any(
            that: predicate<UpdateProjectRequest>((r) {
              final j = r.toJson();
              return j['git_default_branch'] == 'develop' &&
                  !j.containsKey('name') &&
                  !j.containsKey('description') &&
                  !j.containsKey('status') &&
                  !j.containsKey('settings') &&
                  !j.containsKey('clear_settings') &&
                  !j.containsKey('git_credential_id') &&
                  !j.containsKey('tech_stack') &&
                  !j.containsKey('clear_tech_stack') &&
                  !j.containsKey('remove_git_credential');
            }),
          ),
          cancelToken: any(named: 'cancelToken'),
        ),
      ).called(1);
      expect(loadCount, greaterThanOrEqualTo(2));
    });

    testWidgets('Reindex 409: SnackBar с projectSettingsReindexConflict', (
      tester,
    ) async {
      useViewSize(tester, const Size(800, 1600));
      final mockRepo = MockProjectRepository();

      when(
        () => mockRepo.getProject(any(), cancelToken: any(named: 'cancelToken')),
      ).thenAnswer(
        (_) async => makeProject(id: kTestProjectUuid),
      );

      when(
        () => mockRepo.reindex(any(), cancelToken: any(named: 'cancelToken')),
      ).thenThrow(
        ProjectConflictException('indexing', apiErrorCode: 'indexing_conflict'),
      );

      await tester.pumpWidget(
        _harness(
          child: const ProjectSettingsScreen(projectId: kTestProjectUuid),
          overrides: [
            projectRepositoryProvider.overrideWithValue(mockRepo),
            projectProvider(kTestProjectUuid).overrideWith((ref) async {
              final repo = ref.read(projectRepositoryProvider);
              return repo.getProject(kTestProjectUuid);
            }),
          ],
        ),
      );
      await tester.pumpAndSettle();

      final l10n = AppLocalizations.of(
        tester.element(find.byType(ProjectSettingsScreen)),
      )!;
      final reindexBtn = find.text(l10n.projectSettingsReindex);
      await tester.ensureVisible(reindexBtn);
      await tester.tap(reindexBtn);
      await tester.pumpAndSettle();

      expect(find.text(l10n.projectSettingsReindexConflict), findsOneWidget);
    });
  });
}
