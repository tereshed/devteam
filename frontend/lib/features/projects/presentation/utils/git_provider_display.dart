import 'package:flutter/widgets.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Локализованное имя git-провайдера для UI (как [projectStatusDisplay] для статусов).
String gitProviderDisplayLabel(BuildContext context, String provider) {
  final l10n = AppLocalizations.of(context)!;
  return switch (provider) {
    'github' => l10n.gitProviderGithub,
    'gitlab' => l10n.gitProviderGitlab,
    'bitbucket' => l10n.gitProviderBitbucket,
    'local' => l10n.gitProviderLocal,
    // Перечень [gitProviders] в domain; неизвестные — общий фоллбэк.
    _ => l10n.gitProviderUnknown,
  };
}
