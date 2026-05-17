import 'package:flutter/material.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Минимальный harness для widget-тестов фичи assistant:
/// [MaterialApp] с `AppLocalizations.localizationsDelegates`, без Riverpod —
/// для виджетов, не зависящих от провайдеров (например, [AssistantToolCallCard]
/// и [AssistantConfirmDialog] принимают модели через конструктор).
Widget wrapAssistantWidget(
  Widget child, {
  Locale locale = const Locale('en'),
}) {
  return MaterialApp(
    locale: locale,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: Scaffold(body: child),
  );
}
