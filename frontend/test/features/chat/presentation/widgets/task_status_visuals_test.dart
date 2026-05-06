import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/chat/presentation/widgets/task_status_visuals.dart';
import 'package:frontend/l10n/app_localizations.dart';

void main() {
  const delegates = <LocalizationsDelegate<dynamic>>[
    AppLocalizations.delegate,
    GlobalMaterialLocalizations.delegate,
    GlobalWidgetsLocalizations.delegate,
    GlobalCupertinoLocalizations.delegate,
  ];

  test('SSOT: каждый kNormativeTaskStatuses имеет ветку в taskStatusVisualCategory', () {
    for (final s in kNormativeTaskStatuses) {
      expect(
        taskStatusVisualCategory(s),
        isNot(TaskStatusVisualCategory.unknown),
        reason: 'нет ветки в taskStatusVisualCategory для $s',
      );
    }
  });

  testWidgets('SSOT: каждый kNormativeTaskStatuses имеет ветку в taskStatusLabel (ru)', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        locale: Locale('ru'),
        localizationsDelegates: delegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: SizedBox.shrink()),
      ),
    );
    await tester.pumpAndSettle();
    final l10n = AppLocalizations.of(
      tester.element(find.byType(SizedBox)),
    )!;
    for (final s in kNormativeTaskStatuses) {
      expect(
        taskStatusLabel(l10n, s),
        isNot(equals(l10n.taskStatusUnknownStatus)),
        reason: 'нет ветки в taskStatusLabel для $s',
      );
    }
  });
}
