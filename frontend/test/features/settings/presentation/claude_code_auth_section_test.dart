import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/settings/data/claude_code_auth_providers.dart';
import 'package:frontend/features/settings/domain/models/claude_code_auth_status.dart';
import 'package:frontend/features/settings/presentation/widgets/claude_code_auth_section.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Sprint 15.34 — widget-тесты ClaudeCodeAuthSection.
void main() {
  Future<void> pump(
    WidgetTester tester, {
    required ClaudeCodeAuthStatus status,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        retry: (_, _) => null,
        overrides: [
          claudeCodeAuthStatusProvider.overrideWith((ref) async => status),
        ],
        child: const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Scaffold(body: ClaudeCodeAuthSection()),
        ),
      ),
    );
    await tester.pumpAndSettle();
  }

  testWidgets('disconnected: show login button and hint',
      (tester) async {
    await pump(tester, status: const ClaudeCodeAuthStatus(connected: false));
    final l10n = AppLocalizations.of(
      tester.element(find.byType(ClaudeCodeAuthSection)),
    )!;
    expect(find.text(l10n.claudeCodeAuthDisconnectedTitle), findsOneWidget);
    expect(find.text(l10n.claudeCodeAuthDisconnectedHint), findsOneWidget);
    // Sprint 15.minor: FilledButton.icon — фабрика, возвращает приватный
    // _FilledButtonWithIcon (extends FilledButton). find.byType matches только по
    // runtimeType, поэтому используем предикат с is-проверкой через ancestor finder.
    expect(
      find.ancestor(
        of: find.text(l10n.claudeCodeAuthLogin),
        matching: find.byWidgetPredicate((w) => w is FilledButton),
      ),
      findsOneWidget,
    );
    expect(find.text(l10n.claudeCodeAuthRevoke), findsNothing);
  });

  testWidgets('connected: show status table and Revoke button',
      (tester) async {
    await pump(
      tester,
      status: const ClaudeCodeAuthStatus(
        connected: true,
        tokenType: 'Bearer',
        scopes: 'user:inference',
      ),
    );
    final l10n = AppLocalizations.of(
      tester.element(find.byType(ClaudeCodeAuthSection)),
    )!;
    expect(find.text(l10n.claudeCodeAuthConnectedTitle), findsOneWidget);
    expect(find.text('Bearer'), findsOneWidget);
    expect(find.text('user:inference'), findsOneWidget);
    expect(
      find.ancestor(
        of: find.text(l10n.claudeCodeAuthLogin),
        matching: find.byWidgetPredicate((w) => w is FilledButton),
      ),
      findsNothing,
    );
    expect(find.text(l10n.claudeCodeAuthRevoke), findsOneWidget);
  });
}
