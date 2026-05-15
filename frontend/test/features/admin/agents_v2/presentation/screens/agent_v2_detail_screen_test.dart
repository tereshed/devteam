@TestOn('vm')
@Tags(['widget'])
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_repository.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_exceptions.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/admin/agents_v2/presentation/screens/agent_v2_detail_screen.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:mocktail/mocktail.dart';

import '../../../../../_fixtures/orchestration_v2_fixtures.dart';
import '../../../../../support/widget_test_harness.dart';

// agent_v2_detail_screen_test.dart — Sprint 17 / 6.7.
//
// Покрывает submit valid form (repo.update + success-snackbar), error handling
// (exception → inline error-text) и hidden secret field (SecretDialog не
// показывает value по умолчанию, добавляет visibility toggle).

class _MockRepo extends Mock implements AgentsV2Repository {}

const _kId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';

const _kSaveBtn = Key('agent_v2_detail_save_button');
const _kAddSecretBtn = Key('agent_v2_detail_add_secret_button');
const _kSecretDialogSaveBtn = Key('agent_v2_secret_dialog_save_button');

Future<void> _pump(
  WidgetTester tester, {
  required _MockRepo repo,
  required AgentV2 agent,
}) =>
    pumpAppWidget(
      tester,
      child: const AgentV2DetailScreen(agentId: _kId),
      overrides: [
        agentsV2RepositoryProvider.overrideWithValue(repo),
        agentV2DetailProvider(_kId).overrideWith((_) async => agent),
      ],
      // Высокий viewport — экран длинный (header + 6 полей + 2 кнопки + hint),
      // на дефолтных 800x600 кнопки уходят за пределы и tap-pointer не
      // попадает.
      screenSize: const Size(1200, 2200),
    );

void main() {
  setUpAll(() {
    // Mocktail требует регистрировать fallback для non-nullable named-аргументов
    // в any(named:). Здесь все параметры update — nullable, так что fallback
    // не нужен. Регистрируем символический — guard от регрессий.
  });

  group('AgentV2DetailScreen', () {
    testWidgets(
        'submit valid form: вызывает repo.update и показывает snackbar',
        (tester) async {
      final repo = _MockRepo();
      final agent = fxAgent(
        id: _kId,
        name: 'planner-claude',
        role: 'planner',
        executionKind: 'llm',
        model: 'claude-sonnet-4-6',
        roleDescription: 'orig desc',
        systemPrompt: 'orig prompt',
      );
      when(() => repo.update(
            id: any(named: 'id'),
            roleDescription: any(named: 'roleDescription'),
            systemPrompt: any(named: 'systemPrompt'),
            model: any(named: 'model'),
            codeBackend: any(named: 'codeBackend'),
            temperature: any(named: 'temperature'),
            maxTokens: any(named: 'maxTokens'),
            isActive: any(named: 'isActive'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => agent);

      await _pump(tester, repo: repo, agent: agent);

      final BuildContext ctx =
          tester.element(find.byType(AgentV2DetailScreen));
      final l10n = AppLocalizations.of(ctx)!;
      await tester.tap(find.byKey(_kSaveBtn));
      await tester.pumpAndSettle();

      verify(() => repo.update(
            id: _kId,
            roleDescription: 'orig desc',
            systemPrompt: 'orig prompt',
            model: 'claude-sonnet-4-6',
            codeBackend: null,
            temperature: any(named: 'temperature'),
            maxTokens: any(named: 'maxTokens'),
            isActive: true,
          )).called(1);
      expect(find.text(l10n.agentsV2SavedSnackbar), findsOneWidget);
    });

    testWidgets('error handling: 409 conflict → inline error-text',
        (tester) async {
      final repo = _MockRepo();
      final agent = fxAgent(id: _kId);
      when(() => repo.update(
            id: any(named: 'id'),
            roleDescription: any(named: 'roleDescription'),
            systemPrompt: any(named: 'systemPrompt'),
            model: any(named: 'model'),
            codeBackend: any(named: 'codeBackend'),
            temperature: any(named: 'temperature'),
            maxTokens: any(named: 'maxTokens'),
            isActive: any(named: 'isActive'),
            cancelToken: any(named: 'cancelToken'),
          )).thenThrow(AgentV2ConflictException('name already taken'));

      await _pump(tester, repo: repo, agent: agent);

      final BuildContext ctx =
          tester.element(find.byType(AgentV2DetailScreen));
      final l10n = AppLocalizations.of(ctx)!;
      await tester.tap(find.byKey(_kSaveBtn));
      await tester.pumpAndSettle();

      // Inline error-text "$commonRequestFailed: $e". Snackbar НЕ должен
      // быть показан — для save-ошибок мы остаёмся на форме.
      expect(find.textContaining(l10n.commonRequestFailed), findsOneWidget);
      expect(find.textContaining('name already taken'), findsOneWidget);
      expect(find.text(l10n.agentsV2SavedSnackbar), findsNothing);
    });

    testWidgets(
        'hidden secret field: SecretDialog рендерит value как obscureText '
        'и переключает видимость по visibility-кнопке', (tester) async {
      final repo = _MockRepo();
      final agent = fxAgent(id: _kId);
      await _pump(tester, repo: repo, agent: agent);

      final BuildContext ctx =
          tester.element(find.byType(AgentV2DetailScreen));
      final l10n = AppLocalizations.of(ctx)!;

      // Открываем Secret dialog.
      await tester.tap(find.byKey(_kAddSecretBtn));
      await tester.pumpAndSettle();
      expect(find.text(l10n.agentsV2SecretDialogTitle), findsOneWidget);

      // По умолчанию value-поле скрыто (obscureText: true). Тогда:
      //   - иконка suffix = visibility (показать),
      //   - найти TextField с obscureText: true.
      expect(find.byIcon(Icons.visibility), findsOneWidget);
      final valueFieldFinder = find.widgetWithText(
        TextFormField,
        l10n.agentsV2SecretValue,
      );
      expect(valueFieldFinder, findsOneWidget);
      // Сам Widget level — у TextFormField нет obscureText getter, но он
      // прокидывает в нижележащий TextField. Достаём этот TextField.
      final textField = tester.widget<TextField>(
        find.descendant(
          of: valueFieldFinder,
          matching: find.byType(TextField),
        ),
      );
      expect(textField.obscureText, isTrue,
          reason: 'value по умолчанию должен быть скрыт');

      // Тапаем visibility-кнопку → toggle. Иконка меняется на visibility_off.
      await tester.tap(find.byIcon(Icons.visibility));
      await tester.pumpAndSettle();
      expect(find.byIcon(Icons.visibility_off), findsOneWidget);
      final after = tester.widget<TextField>(
        find.descendant(
          of: valueFieldFinder,
          matching: find.byType(TextField),
        ),
      );
      expect(after.obscureText, isFalse);
    });

    testWidgets('Add secret submit: repo.setSecret вызван + closed snackbar',
        (tester) async {
      final repo = _MockRepo();
      final agent = fxAgent(id: _kId);
      when(() => repo.setSecret(
            agentId: any(named: 'agentId'),
            keyName: any(named: 'keyName'),
            value: any(named: 'value'),
            cancelToken: any(named: 'cancelToken'),
          )).thenAnswer((_) async => AgentV2SecretRef(
            id: 'sec-1',
            keyName: 'GITHUB_TOKEN',
            createdAt: DateTime.utc(2026, 5, 15),
          ));

      await _pump(tester, repo: repo, agent: agent);

      final BuildContext ctx =
          tester.element(find.byType(AgentV2DetailScreen));
      final l10n = AppLocalizations.of(ctx)!;
      await tester.tap(find.byKey(_kAddSecretBtn));
      await tester.pumpAndSettle();

      await tester.enterText(
        find.widgetWithText(TextFormField, l10n.agentsV2SecretKeyName),
        'GITHUB_TOKEN',
      );
      await tester.enterText(
        find.widgetWithText(TextFormField, l10n.agentsV2SecretValue),
        's3cret',
      );
      await tester.tap(find.byKey(_kSecretDialogSaveBtn));
      await tester.pumpAndSettle();

      verify(() => repo.setSecret(
            agentId: _kId,
            keyName: 'GITHUB_TOKEN',
            value: 's3cret',
          )).called(1);
      expect(find.text(l10n.agentsV2SecretSaved), findsOneWidget);
    });
  });
}
