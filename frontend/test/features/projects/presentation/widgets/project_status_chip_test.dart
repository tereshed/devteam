import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/projects/presentation/widgets/project_status_chip.dart';
import 'package:frontend/l10n/app_localizations.dart';

import '../../helpers/test_wrappers.dart';

void main() {
  group('ProjectStatusChip', () {
    for (final status in projectStatuses) {
      testWidgets('рендерится для статуса $status', (tester) async {
        await tester.pumpWidget(wrapSimple(ProjectStatusChip(status: status)));
        await tester.pumpAndSettle();
        expect(find.byType(Chip), findsOneWidget);
      });
    }

    testWidgets('неизвестный статус не показывает сырой ключ', (tester) async {
      await tester.pumpWidget(
        wrapSimple(const ProjectStatusChip(status: 'unknown_xyz')),
      );
      await tester.pumpAndSettle();
      expect(find.byType(Chip), findsOneWidget);
      expect(find.text('unknown_xyz'), findsNothing);
      expect(find.text('Unknown'), findsOneWidget);
    });

    testWidgets('русская локаль: известные статусы не сводятся к statusUnknown',
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
          locale: const Locale('ru'),
          home: Builder(
            builder: (ctx) {
              capturedContext = ctx;
              return const Scaffold(
                body: ProjectStatusChip(status: 'active'),
              );
            },
          ),
        ),
      );
      await tester.pumpAndSettle();

      final l10n = AppLocalizations.of(capturedContext)!;
      expect(find.text(l10n.statusActive), findsOneWidget);
      expect(find.text(l10n.statusUnknown), findsNothing);
    });
  });
}
