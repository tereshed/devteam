// Phase 3 §Task 3.2 — Team settings flow.
//
// Что покрываем (ограничено возможностями REST API):
//   1. Team page бутстрапится: `GET /projects/:id/team` отдаёт автоматически
//      созданную команду без агентов → виден `teamEmptyAgents` плюс заголовок
//      team.name.
//   2. PUT /projects/:id/team — переименование команды через REST → invalid
//      кеша projectProvider'а → UI рисует новое имя. (UI rename'а нет, но мы
//      проверяем, что фронтенд реактивно подхватывает обновление через
//      RefreshIndicator pull-to-refresh.)
//   3. Cross-tenant: alice создаёт проект → bob под своими кредами заходит
//      на этот URL → должен попасть на `dataLoadError` (не 200, не падение).
//
// Привязка v2-агента к команде требует прямого SQL UPDATE (см.
// `attachAgentToTeam` в backend/test/featuresmoke/team_smoke_test.go),
// поэтому full happy-path PATCH /team/agents/:id из Flutter не покрывается —
// он целиком на стороне backend smoke. Если когда-нибудь появится REST
// `POST /projects/:id/team/agents/:agentId` — добавим сюда UI-тест.
//
// **Никаких LLM-вызовов.**

import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'test_support/backend_available.dart';
import 'test_support/eventually.dart';
import 'test_support/rest_client.dart';
import 'test_support/seed_creds.dart';
import 'test_support/test_app.dart';

/// PUT /projects/:id/team { name: new } — для теста реактивного rename.
Future<void> _renameTeam({
  required String token,
  required String projectId,
  required String newName,
}) => TestRestClient.put(
  '/projects/$projectId/team',
  token: token,
  body: {'name': newName},
);

/// GET /projects/:id/team — нужно тесту, чтобы получить актуальное team.name
/// (для последующего assert'а в UI).
Future<String> _teamName({
  required String token,
  required String projectId,
}) async {
  final json = await TestRestClient.get(
    '/projects/$projectId/team',
    token: token,
  );
  return json['name'] as String;
}

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'team page: auto-created team renders empty agents state',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      final creds = await pumpFreshAuthedApp(tester, prefix: 'team-empty');
      final projectId = await createLocalProject(
        creds.token,
        namePrefix: 'team-empty',
      );

      GoRouter.of(anyScaffoldContext(tester)).go('/projects/$projectId/team');
      await pumpForSeconds(tester, 6);

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;

      // Команда автоматически создаётся при создании проекта (см.
      // backend TestTeam_GetAutoCreatedOnProject) → имя должно появиться.
      final expectedTeamName = await _teamName(
        token: creds.token,
        projectId: projectId,
      );
      await expectEventually(
        tester,
        find.text(expectedTeamName),
        timeout: const Duration(seconds: 15),
        reason: 'team header (team.name) visible',
      );

      // Без агентов — фронт показывает `teamEmptyAgents`.
      await expectEventually(
        tester,
        find.text(l10n.teamEmptyAgents),
        timeout: const Duration(seconds: 10),
        reason: 'empty agents hint visible',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );

  testWidgets(
    'team page reactively reflects REST rename after pull-to-refresh',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      final creds = await pumpFreshAuthedApp(tester, prefix: 'team-rename');
      final projectId = await createLocalProject(
        creds.token,
        namePrefix: 'team-rename',
      );

      GoRouter.of(anyScaffoldContext(tester)).go('/projects/$projectId/team');
      await pumpForSeconds(tester, 6);

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;
      final initialName = await _teamName(
        token: creds.token,
        projectId: projectId,
      );
      await expectEventually(
        tester,
        find.text(initialName),
        timeout: const Duration(seconds: 10),
        reason: 'initial team name in UI',
      );

      // Меняем имя через REST.
      final newName = 'renamed-team-${DateTime.now().millisecondsSinceEpoch}';
      await _renameTeam(
        token: creds.token,
        projectId: projectId,
        newName: newName,
      );

      // Симулируем pull-to-refresh: чтобы не зависеть от gesture-API
      // (drag-from-top — flaky на разных размерах окна), используем
      // прямой re-entry на ту же страницу через go_router. ConsumerWidget
      // invalidate'нет teamProvider при следующем build'е (см. team_screen.dart).
      GoRouter.of(
        anyScaffoldContext(tester),
      ).go('/projects/$projectId/team?_=1');
      await pumpForSeconds(tester, 4);
      GoRouter.of(anyScaffoldContext(tester)).go('/projects/$projectId/team');
      await pumpForSeconds(tester, 6);

      await expectEventually(
        tester,
        find.text(newName),
        timeout: const Duration(seconds: 15),
        reason: 'team name in UI reflects REST rename',
      );
      // sanity: empty-state виден (агентов не привязывали).
      expect(find.text(l10n.teamEmptyAgents), findsWidgets);
    },
    timeout: const Timeout(Duration(minutes: 3)),
  );

  testWidgets(
    'team page: cross-tenant access shows data-load-error',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      // alice создаёт проект (отдельным запросом, чтобы получить её токен).
      final alice = await registerSeedUser(prefix: 'team-xtenant-alice');
      final projectId = await createLocalProject(
        alice.token,
        namePrefix: 'team-xtenant',
      );

      // bob поднимает UI под своим токеном и идёт на alice'ин URL.
      await pumpFreshAuthedApp(tester, prefix: 'team-xtenant-bob');
      GoRouter.of(anyScaffoldContext(tester)).go('/projects/$projectId/team');
      await pumpForSeconds(tester, 8);

      // Бек отвечает 403/404 → teamProvider.hasError → отображается
      // DataLoadErrorMessage (см. team_screen.dart).
      await expectEventually(
        tester,
        find.byType(DataLoadErrorMessage),
        timeout: const Duration(seconds: 15),
        reason: 'cross-tenant access surfaces DataLoadErrorMessage',
      );
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );
}
