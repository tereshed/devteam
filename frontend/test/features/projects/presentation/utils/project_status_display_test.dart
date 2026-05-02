import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/presentation/utils/project_status_display.dart';
import 'package:frontend/l10n/app_localizations.dart';

void main() {
  testWidgets('все статусы из projectStatuses не попадают в fallback',
      (tester) async {
    late BuildContext capturedContext;

    await tester.pumpWidget(MaterialApp(
      localizationsDelegates: const [
        AppLocalizations.delegate,
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
      ],
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('en'),
      home: Builder(builder: (ctx) {
        capturedContext = ctx;
        return const SizedBox();
      }),
    ));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(capturedContext)!;
    for (final status in projectStatuses) {
      final display = projectStatusDisplay(capturedContext, status);
      expect(
        display.label,
        isNot(equals(l10n.statusUnknown)),
        reason:
            'Статус "$status" попал в fallback — добавьте кейс в projectStatusDisplay()',
      );
    }
  });
}
