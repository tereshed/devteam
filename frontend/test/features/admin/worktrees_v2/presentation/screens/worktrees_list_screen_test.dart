// @Tags(['widget'])

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/admin/worktrees_v2/data/worktrees_providers.dart';
import 'package:frontend/features/admin/worktrees_v2/data/worktrees_repository.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktree_model.dart';
import 'package:frontend/features/admin/worktrees_v2/domain/worktrees_exceptions.dart';
import 'package:frontend/features/admin/worktrees_v2/presentation/screens/worktrees_list_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mocktail/mocktail.dart';

// worktrees_list_screen_test.dart — Sprint 17 / 6.3.
//
// Покрывает frontend-side ручного manual unstick: confirm-dialog, маппинг
// WorktreesConflictException в info-snackbar, прочие ошибки в error-snackbar,
// видимость кнопки только для не-released worktree'ев. Backend-логика отдельно
// покрыта handler/MCP/integration-тестами на Go.

class _MockRepo extends Mock implements WorktreesRepository {}

WorktreeV2 _wt({
  String? id,
  String state = 'in_use',
}) =>
    WorktreeV2(
      id: id ?? '11111111-1111-1111-1111-111111111111',
      taskId: '22222222-2222-2222-2222-222222222222',
      baseBranch: 'main',
      branchName: 'task-22222222-wt-11111111',
      state: state,
      allocatedAt: DateTime.utc(2026, 5, 15, 12),
    );

Future<void> _pump(
  WidgetTester tester, {
  required _MockRepo repo,
  required List<WorktreeV2> items,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        worktreesRepositoryProvider.overrideWithValue(repo),
        // Override list-провайдер константой, чтобы избежать реального HTTP.
        worktreesListProvider.overrideWith((_) async => items),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const WorktreesListScreen(),
      ),
    ),
  );
  // List loads from FutureProvider — нужен один pump для перехода loading→data.
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() {
    // Mocktail требует регистрировать fallback для positional/optional аргументов
    // нестандартных типов. У repo.release параметров такого типа нет, но any() для
    // String работает из коробки — ничего не регистрируем.
  });

  group('WorktreesListScreen — release flow', () {
    testWidgets('IconButton НЕ рендерится для released worktree', (tester) async {
      final repo = _MockRepo();
      await _pump(tester, repo: repo, items: [_wt(state: 'released')]);

      // Нет ни одной cleaning_services иконки — кнопка скрыта целиком, не disabled.
      expect(find.byIcon(Icons.cleaning_services_outlined), findsNothing);
      verifyNever(() => repo.release(any()));
    });

    testWidgets('IconButton рендерится для in_use и открывает confirm-dialog', (tester) async {
      final repo = _MockRepo();
      await _pump(tester, repo: repo, items: [_wt(state: 'in_use')]);

      final btn = find.byIcon(Icons.cleaning_services_outlined);
      expect(btn, findsOneWidget);
      await tester.tap(btn);
      await tester.pumpAndSettle();

      // Dialog присутствует. Текст специфичен — проверка отдельной фразы из тела.
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.textContaining('git worktree remove --force'), findsOneWidget);
      // Repo НЕ должен быть дёрнут — пользователь ещё не подтвердил.
      verifyNever(() => repo.release(any()));
    });

    testWidgets('Cancel в диалоге НЕ вызывает repo.release', (tester) async {
      final repo = _MockRepo();
      await _pump(tester, repo: repo, items: [_wt()]);

      await tester.tap(find.byIcon(Icons.cleaning_services_outlined));
      await tester.pumpAndSettle();

      // В Material AlertDialog кнопка Cancel — TextButton; ищем по локализованной
      // строке через L10n root context.
      final BuildContext ctx = tester.element(find.byType(WorktreesListScreen));
      final l10n = AppLocalizations.of(ctx)!;
      await tester.tap(find.text(l10n.commonCancel));
      await tester.pumpAndSettle();

      verifyNever(() => repo.release(any()));
      // Snackbar тоже не должен быть показан.
      expect(find.byType(SnackBar), findsNothing);
    });

    testWidgets('Confirm + успешный release → success snackbar', (tester) async {
      final repo = _MockRepo();
      when(() => repo.release(any())).thenAnswer(
        (_) async => _wt(state: 'released'),
      );

      await _pump(tester, repo: repo, items: [_wt(id: 'abc')]);

      await tester.tap(find.byIcon(Icons.cleaning_services_outlined));
      await tester.pumpAndSettle();

      final BuildContext ctx = tester.element(find.byType(WorktreesListScreen));
      final l10n = AppLocalizations.of(ctx)!;

      // Подтверждаем — нажимаем кнопку с текстом worktreesReleaseButton (она же
      // в диалоге как destructive action, и отдельно как IconButton tooltip,
      // но IconButton не рендерит видимый текст — diалог сейчас на экране).
      await tester.tap(find.text(l10n.worktreesReleaseButton));
      await tester.pumpAndSettle();

      verify(() => repo.release('abc')).called(1);
      expect(find.text(l10n.worktreesReleasedSnackbar), findsOneWidget);
    });

    testWidgets('Confirm + 409 (already released) → info snackbar (НЕ error)', (tester) async {
      final repo = _MockRepo();
      when(() => repo.release(any())).thenThrow(
        WorktreesConflictException('already released'),
      );

      await _pump(tester, repo: repo, items: [_wt()]);

      await tester.tap(find.byIcon(Icons.cleaning_services_outlined));
      await tester.pumpAndSettle();

      final BuildContext ctx = tester.element(find.byType(WorktreesListScreen));
      final l10n = AppLocalizations.of(ctx)!;
      await tester.tap(find.text(l10n.worktreesReleaseButton));
      await tester.pumpAndSettle();

      // Видим сообщение про "already released", НЕ generic error-toast.
      expect(find.text(l10n.worktreesReleaseAlreadyReleased), findsOneWidget);
      expect(find.textContaining(l10n.worktreesReleaseFailed), findsNothing);
    });

    testWidgets('Confirm + 503 not_configured → специфичный snackbar', (tester) async {
      final repo = _MockRepo();
      when(() => repo.release(any())).thenThrow(
        WorktreesNotConfiguredException('feature off'),
      );

      await _pump(tester, repo: repo, items: [_wt()]);

      await tester.tap(find.byIcon(Icons.cleaning_services_outlined));
      await tester.pumpAndSettle();

      final BuildContext ctx = tester.element(find.byType(WorktreesListScreen));
      final l10n = AppLocalizations.of(ctx)!;
      await tester.tap(find.text(l10n.worktreesReleaseButton));
      await tester.pumpAndSettle();

      expect(find.text(l10n.worktreesReleaseNotConfigured), findsOneWidget);
      // НЕ generic error-toast — иначе оператор не поймёт что фича просто отключена.
      expect(find.textContaining(l10n.worktreesReleaseFailed), findsNothing);
    });

    testWidgets('Confirm + неизвестная ошибка → error snackbar', (tester) async {
      final repo = _MockRepo();
      when(() => repo.release(any())).thenThrow(
        WorktreesApiException('boom', statusCode: 500),
      );

      await _pump(tester, repo: repo, items: [_wt()]);

      await tester.tap(find.byIcon(Icons.cleaning_services_outlined));
      await tester.pumpAndSettle();

      final BuildContext ctx = tester.element(find.byType(WorktreesListScreen));
      final l10n = AppLocalizations.of(ctx)!;
      await tester.tap(find.text(l10n.worktreesReleaseButton));
      await tester.pumpAndSettle();

      // SnackBar содержит generic-failure префикс из l10n + детали исключения.
      expect(find.textContaining(l10n.worktreesReleaseFailed), findsOneWidget);
    });
  });
}
