// @Tags(['widget'])
//
// Базовые widget-тесты для [IntegrationProviderCard]: каждое из 4 состояний
// (`connected | disconnected | error | pending`) корректно рендерит chip и actions.
// Покрывает AC задачи 1.1 из docs/tasks/ui_refactoring/tasks-breakdown.md.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/shared/widgets/integration_action.dart';
import 'package:frontend/shared/widgets/integration_provider_card.dart';
import 'package:frontend/shared/widgets/integration_status.dart';

Future<void> _pump(
  WidgetTester tester,
  Widget child, {
  bool hasInfiniteAnim = false,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      home: Scaffold(body: Padding(padding: const EdgeInsets.all(16), child: child)),
    ),
  );
  if (hasInfiniteAnim) {
    // `pumpAndSettle` зацикливается на CircularProgressIndicator — даём один кадр.
    await tester.pump();
  } else {
    await tester.pumpAndSettle();
  }
}

void main() {
  group('IntegrationProviderCard', () {
    testWidgets('connected: chip и actions отрисованы', (tester) async {
      var tapped = false;
      await _pump(
        tester,
        IntegrationProviderCard(
          logo: const Icon(Icons.cloud, key: ValueKey('logo')),
          title: 'Anthropic',
          subtitle: 'API key configured',
          status: IntegrationStatus.connected,
          statusDetail: 'last check 5m ago',
          actions: [
            IntegrationAction(
              label: 'Disconnect',
              onPressed: () => tapped = true,
              style: IntegrationActionStyle.destructive,
            ),
          ],
        ),
      );

      expect(find.text('Anthropic'), findsOneWidget);
      expect(find.text('API key configured'), findsOneWidget);
      expect(find.text('last check 5m ago'), findsOneWidget);
      expect(find.text('Connected'), findsOneWidget);
      expect(find.byKey(const ValueKey('logo')), findsOneWidget);

      await tester.tap(find.text('Disconnect'));
      expect(tapped, isTrue);
    });

    testWidgets('disconnected: показывает стандартный chip', (tester) async {
      await _pump(
        tester,
        const IntegrationProviderCard(
          logo: Icon(Icons.cloud),
          title: 'OpenAI',
          status: IntegrationStatus.disconnected,
        ),
      );
      expect(find.text('Not connected'), findsOneWidget);
    });

    testWidgets('error: chip "Error" и иконка ошибки', (tester) async {
      await _pump(
        tester,
        const IntegrationProviderCard(
          logo: Icon(Icons.cloud),
          title: 'GitHub',
          status: IntegrationStatus.error,
          statusDetail: 'token expired',
        ),
      );
      expect(find.text('Error'), findsOneWidget);
      expect(find.text('token expired'), findsOneWidget);
    });

    testWidgets('pending: показывает спиннер вместо иконки', (tester) async {
      await _pump(
        tester,
        const IntegrationProviderCard(
          logo: Icon(Icons.cloud),
          title: 'Claude Code',
          status: IntegrationStatus.pending,
        ),
        hasInfiniteAnim: true,
      );
      expect(find.text('Connecting…'), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });

    testWidgets('disabled конструктор: actions не рендерятся', (tester) async {
      await _pump(
        tester,
        IntegrationProviderCard.disabled(
          logo: const Icon(Icons.cloud),
          title: 'Coming Soon',
          statusLabel: 'Coming soon',
        ),
      );
      expect(find.text('Coming Soon'), findsOneWidget);
      expect(find.text('Coming soon'), findsOneWidget);
      expect(find.byType(OutlinedButton), findsNothing);
      expect(find.byType(FilledButton), findsNothing);
    });
  });
}
