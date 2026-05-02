import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/presentation/utils/git_provider_display.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Exhaustive по [gitProviders] — тот же паттерн, что
/// `project_status_display_test.dart` для [projectStatuses].
void main() {
  testWidgets('все провайдеры из gitProviders не попадают в fallback Unknown',
      (tester) async {
    late BuildContext capturedContext;

    await tester.pumpWidget(
      MaterialApp(
        localizationsDelegates: const [
          AppLocalizations.delegate,
          GlobalMaterialLocalizations.delegate,
          GlobalCupertinoLocalizations.delegate,
          GlobalWidgetsLocalizations.delegate,
        ],
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('en'),
        home: Builder(
          builder: (ctx) {
            capturedContext = ctx;
            return const SizedBox();
          },
        ),
      ),
    );
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(capturedContext)!;
    for (final provider in gitProviders) {
      final label = gitProviderDisplayLabel(capturedContext, provider);
      expect(
        label,
        isNot(equals(l10n.gitProviderUnknown)),
        reason:
            'Провайдер "$provider" попал в fallback — добавьте кейс в gitProviderDisplayLabel()',
      );
    }
  });
}
