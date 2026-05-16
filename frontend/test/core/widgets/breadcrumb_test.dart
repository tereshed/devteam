// @Tags(['widget'])
//
// Тесты для функции [buildBreadcrumbNodes] из core/widgets/breadcrumb.dart.
// Покрывает иерархию маршрутов для AC задачи 1.2.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/core/widgets/breadcrumb.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/l10n/app_localizations_en.dart';

void main() {
  late AppLocalizations l10n;

  setUpAll(() async {
    l10n = await AppLocalizations.delegate.load(const Locale('en'));
  });

  test('empty location → empty list', () {
    expect(buildBreadcrumbNodes('', l10n), isEmpty);
    expect(buildBreadcrumbNodes('/', l10n), isEmpty);
  });

  test('/dashboard → Home > Overview', () {
    final n = buildBreadcrumbNodes('/dashboard', l10n);
    expect(n.length, 2);
    expect(n[0].label, 'Home');
    expect(n[0].route, isNull);
    expect(n[1].label, 'Overview');
    expect(n[1].route, isNull);
  });

  test('/integrations/llm → Home > Integrations > LLM providers', () {
    final n = buildBreadcrumbNodes('/integrations/llm', l10n);
    expect(n.length, 3);
    expect(n[0].label, 'Home');
    expect(n[1].label, 'Integrations');
    expect(n[1].route, '/integrations');
    expect(n[2].label, 'LLM providers');
    expect(n[2].route, isNull);
  });

  test('/admin/agents-v2 → Home > Administration > Agents', () {
    final n = buildBreadcrumbNodes('/admin/agents-v2', l10n);
    expect(n.map((e) => e.label).toList(), ['Home', 'Administration', 'Agents']);
  });

  test('UUID-сегмент после /projects обрезается до 8 символов', () {
    final n = buildBreadcrumbNodes(
      '/projects/0123abcd-aaaa-bbbb-cccc-ddddeeeeffff',
      l10n,
    );
    expect(n.last.label, '0123abcd');
  });

  testWidgets('Breadcrumb виджет рендерится без падения', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(body: Breadcrumb(location: '/dashboard')),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Home'), findsOneWidget);
    expect(find.text('Overview'), findsOneWidget);
  });
}
