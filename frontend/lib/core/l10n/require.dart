import 'package:flutter/widgets.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Возвращает [AppLocalizations] для [context] или бросает [StateError].
///
/// [where] попадаёт в текст ошибки (например `'buildRouterErrorScreen'` или
/// `'project_dashboard_shell'`), чтобы быстрее находить место без делегатов.
AppLocalizations requireAppLocalizations(
  BuildContext context, {
  String? where,
}) {
  final l10n = AppLocalizations.of(context);
  if (l10n == null) {
    final hint = where != null ? ' ($where)' : '';
    throw StateError(
      'AppLocalizations not found$hint — add AppLocalizations.delegate to '
      'MaterialApp (see lib/main.dart MainApp or widget tests).',
    );
  }
  return l10n;
}
