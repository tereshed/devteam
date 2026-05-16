import 'package:flutter/material.dart';

/// Состояние подключения внешнего сервиса (LLM / Git / OAuth).
///
/// Используется в [IntegrationProviderCard] и stat-карточках dashboard.
/// См. dashboard-redesign-plan.md §4a.3.
enum IntegrationStatus {
  /// Подключено и работает.
  connected,

  /// Не подключено (можно подключить).
  disconnected,

  /// Подключение есть, но что-то сломано (истёк токен, healthcheck упал).
  error,

  /// В процессе подключения (OAuth pending, healthcheck идёт).
  pending,
}

/// Маппинг [IntegrationStatus] в цветовую схему текущей темы.
///
/// Никаких прямых [Colors] — всё через [ColorScheme], чтобы тёмная/светлая
/// темы работали корректно.
class IntegrationStatusColors {
  final Color background;
  final Color foreground;
  final Color icon;

  const IntegrationStatusColors({
    required this.background,
    required this.foreground,
    required this.icon,
  });

  factory IntegrationStatusColors.of(
    BuildContext context,
    IntegrationStatus status,
  ) {
    final scheme = Theme.of(context).colorScheme;
    switch (status) {
      case IntegrationStatus.connected:
        return IntegrationStatusColors(
          background: scheme.secondaryContainer,
          foreground: scheme.onSecondaryContainer,
          icon: scheme.primary,
        );
      case IntegrationStatus.disconnected:
        return IntegrationStatusColors(
          background: scheme.surfaceContainerHighest,
          foreground: scheme.onSurfaceVariant,
          icon: scheme.onSurfaceVariant,
        );
      case IntegrationStatus.error:
        return IntegrationStatusColors(
          background: scheme.errorContainer,
          foreground: scheme.onErrorContainer,
          icon: scheme.error,
        );
      case IntegrationStatus.pending:
        return IntegrationStatusColors(
          background: scheme.tertiaryContainer,
          foreground: scheme.onTertiaryContainer,
          icon: scheme.tertiary,
        );
    }
  }
}

/// Иконка по умолчанию для каждого состояния.
IconData defaultIconForIntegrationStatus(IntegrationStatus status) {
  switch (status) {
    case IntegrationStatus.connected:
      return Icons.check_circle_outline;
    case IntegrationStatus.disconnected:
      return Icons.power_settings_new;
    case IntegrationStatus.error:
      return Icons.error_outline;
    case IntegrationStatus.pending:
      return Icons.hourglass_top_outlined;
  }
}
