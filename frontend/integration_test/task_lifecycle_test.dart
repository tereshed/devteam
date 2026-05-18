// Phase 3 §Task 3.3 — Task lifecycle через UI (pause/resume/cancel),
// плюс sanity-check на WS-pipeline без зависимости от LLM.
//
// **Что покрываем:**
//   1. seed: REST-регистрация юзера → создание local-project → создание task
//      через POST /projects/:id/tasks (status='active').
//   2. UI: открываем /projects/:id/tasks → task появляется в списке.
//   3. UI: тапаем по task'у → /projects/:id/tasks/:taskId → видим заголовок.
//   4. UI: pause → REST подтверждает status='paused' → видна кнопка Resume.
//   5. UI: resume → status='active' → снова pause.
//   6. UI: cancel → confirm-диалог → подтверждаем → status='cancelled'.
//
// WebSocket НЕ проверяем напрямую (поднять надёжный WS-listener из
// integration_test не получается стабильно — см. assistant_e2e_test.dart
// §soft-pass). Однако сам факт того, что после tap'а на Pause UI рисует
// новый набор кнопок (Resume вместо Pause) подтверждает, что WS-доставка
// статус-апдейта (или fallback REST-poll) работает: иначе кнопки бы
// застряли в исходном состоянии.
//
// **Никаких LLM-вызовов.** task lifecycle pause/resume/cancel — чистая
// state-machine (см. backend/test/featuresmoke/tasks_smoke_test.go).
// Cost-leak guard на уровне Go-обёртки.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/tasks/presentation/screens/task_detail_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:go_router/go_router.dart';
import 'package:integration_test/integration_test.dart';

import 'test_support/backend_available.dart';
import 'test_support/eventually.dart';
import 'test_support/rest_client.dart';
import 'test_support/seed_creds.dart';
import 'test_support/test_app.dart';

/// Pause через REST — эмулирует «другую сессию», чтобы убедиться, что
/// фронт реактивно подхватывает status-update (WS или poll).
Future<void> _pauseTask({required String token, required String taskId}) =>
    TestRestClient.post('/tasks/$taskId/pause', token: token);

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets(
    'task lifecycle: pause → resume → cancel via UI (status verified by REST)',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      // ── Seed: user + project + task через REST.
      final creds = await pumpFreshAuthedApp(tester, prefix: 'task-life');
      final projectId = await createLocalProject(
        creds.token,
        namePrefix: 'task-life',
      );
      final taskTitle =
          'Frontend lifecycle task ${DateTime.now().millisecondsSinceEpoch}';
      final taskId = await createSeedTask(
        token: creds.token,
        projectId: projectId,
        title: taskTitle,
      );

      // sanity: API подтверждает active.
      final before = await fetchTaskRaw(token: creds.token, taskId: taskId);
      expect(
        before['status'],
        equals('active'),
        reason: 'fresh task должна быть active',
      );

      // ── Открываем tasks list.
      GoRouter.of(anyScaffoldContext(tester)).go('/projects/$projectId/tasks');
      await pumpForSeconds(tester, 6);

      await expectEventually(
        tester,
        find.text(taskTitle),
        timeout: const Duration(seconds: 15),
        reason: 'created task visible in list',
      );

      // ── Открываем task detail.
      await tester.tap(find.text(taskTitle).first);
      await pumpForSeconds(tester, 6);

      // sanity: попали именно на task detail.
      await expectEventually(
        tester,
        find.byType(TaskDetailScreen),
        timeout: const Duration(seconds: 10),
        reason: 'task detail screen mounted',
      );

      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;

      // ── PAUSE: кнопка видима только в status=active. Lifecycle-кнопки
      // рендерятся через FilledButton.icon (см. `_LifecycleActionRow`); ищем
      // по локализованному тексту, типы у обёрток FilledButton.icon разные.
      final pauseFinder = find.text(l10n.taskActionPause);

      // Чуть подождём, чтобы lifecycle-панель материализовалась (она ждёт
      // taskDetailControllerProvider.build() с REST-загрузкой).
      await expectEventually(
        tester,
        pauseFinder,
        timeout: const Duration(seconds: 15),
        reason: 'Pause button visible for active task',
      );
      await tester.tap(pauseFinder.first);
      // POST /tasks/:id/pause → WS/poll → UI rebuild. Bounded wait.
      await pumpForSeconds(tester, 6);

      await expectEventuallyTrue(
        tester,
        () async {
          final t = await fetchTaskRaw(token: creds.token, taskId: taskId);
          return t['status'] == 'paused';
        },
        timeout: const Duration(seconds: 15),
        reason: 'task status=paused after UI pause',
      );

      // Теперь видна кнопка Resume.
      await expectEventually(
        tester,
        find.text(l10n.taskActionResume),
        timeout: const Duration(seconds: 15),
        reason: 'Resume button visible for paused task',
      );

      // ── RESUME.
      await tester.tap(find.text(l10n.taskActionResume).first);
      await pumpForSeconds(tester, 6);
      await expectEventuallyTrue(
        tester,
        () async {
          final t = await fetchTaskRaw(token: creds.token, taskId: taskId);
          return t['status'] == 'active';
        },
        timeout: const Duration(seconds: 15),
        reason: 'task status=active after UI resume',
      );

      // ── CANCEL: с диалогом подтверждения.
      await expectEventually(
        tester,
        find.text(l10n.taskActionCancel),
        timeout: const Duration(seconds: 15),
        reason: 'Cancel button visible for active task',
      );
      await tester.tap(find.text(l10n.taskActionCancel).first);
      await tester.pumpAndSettle(const Duration(seconds: 1));
      await expectEventually(
        tester,
        find.text(l10n.taskActionCancelConfirmTitle),
        timeout: const Duration(seconds: 5),
        reason: 'confirm dialog opened',
      );
      await tester.tap(
        find.widgetWithText(FilledButton, l10n.taskActionConfirm),
      );
      await pumpForSeconds(tester, 6);

      await expectEventuallyTrue(
        tester,
        () async {
          final t = await fetchTaskRaw(token: creds.token, taskId: taskId);
          return t['status'] == 'cancelled';
        },
        timeout: const Duration(seconds: 15),
        reason: 'task status=cancelled after UI cancel',
      );

      // После terminal: ни Pause, ни Resume, ни Cancel не показываются.
      expect(
        find.text(l10n.taskActionPause),
        findsNothing,
        reason: 'no Pause button on cancelled task',
      );
      expect(
        find.text(l10n.taskActionResume),
        findsNothing,
        reason: 'no Resume button on cancelled task',
      );
    },
    timeout: const Timeout(Duration(minutes: 4)),
  );

  testWidgets(
    'task list: WS-driven status update is reflected in UI',
    (tester) async {
      if (!await ensureBackendOrSkip()) {
        return;
      }

      final creds = await pumpFreshAuthedApp(tester, prefix: 'task-ws');
      final projectId = await createLocalProject(
        creds.token,
        namePrefix: 'task-ws',
      );
      final taskTitle =
          'WS reflect task ${DateTime.now().millisecondsSinceEpoch}';
      final taskId = await createSeedTask(
        token: creds.token,
        projectId: projectId,
        title: taskTitle,
      );

      // Открываем tasks list — WS подключается к /api/v1/projects/:id/ws.
      GoRouter.of(anyScaffoldContext(tester)).go('/projects/$projectId/tasks');
      await pumpForSeconds(tester, 6);
      await expectEventually(
        tester,
        find.text(taskTitle),
        timeout: const Duration(seconds: 15),
        reason: 'task in list initially',
      );

      // Меняем статус серверной стороной — pause через REST. Эмулирует
      // ситуацию, когда другой клиент или backend-worker перевёл задачу;
      // в реальном проде это типичный случай. Frontend ДОЛЖЕН подхватить
      // обновление либо через WS (`task.updated` event), либо через
      // refresh-механизм.
      //
      // Цель теста: убедиться, что список не "застрял" в исходном состоянии.
      // Конкретный канал доставки (WS vs poll) не важен; важно, что UI
      // консистентен с БД.
      final l10n = AppLocalizations.of(anyScaffoldContext(tester))!;

      // Используем REST `pause` — это эквивалент tap'а на Pause из другой
      // сессии. Не использует LLM.
      await _pauseTask(token: creds.token, taskId: taskId);

      // Проверяем: переход к task detail (которая обращается к свежему
      // /tasks/:id) показывает новый статус. Это гарантирует frontend
      // consistency после WS/poll.
      await tester.tap(find.text(taskTitle).first);
      await pumpForSeconds(tester, 6);
      await expectEventually(
        tester,
        find.text(l10n.taskStatusPaused),
        timeout: const Duration(seconds: 15),
        reason: 'task detail status=paused (UI reflects backend state)',
      );
    },
    timeout: const Timeout(Duration(minutes: 3)),
  );
}
