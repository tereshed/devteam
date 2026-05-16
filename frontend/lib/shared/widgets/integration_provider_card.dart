import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/shared/widgets/integration_action.dart';
import 'package:frontend/shared/widgets/integration_status.dart';

/// Универсальная карточка подключения внешнего сервиса.
///
/// Используется на экранах `/integrations/llm`, `/integrations/git`,
/// а также как контейнер для stat-карточек dashboard.
///
/// См. dashboard-redesign-plan.md §4a.3.
class IntegrationProviderCard extends StatelessWidget {
  /// Логотип / иконка слева (svg, изображение или `Icon`).
  final Widget logo;

  /// Заголовок карточки (имя провайдера).
  final String title;

  /// Опциональный subtitle (например, аккаунт пользователя).
  final String? subtitle;

  /// Состояние подключения.
  final IntegrationStatus status;

  /// Опциональный текст в chip'е (override человеко-читаемого статуса).
  ///
  /// Если не задан — используется дефолтный лейбл из l10n.
  final String? statusLabel;

  /// Опциональная вторая строка под chip'ом — например, "истекает через 12 дней".
  final String? statusDetail;

  /// Кнопки действий (Тест / Обновить / Отключить).
  final List<IntegrationAction> actions;

  /// Тап по всей карточке (например, для перехода в детали).
  final VoidCallback? onTap;

  /// Если `true` — карточка отрендерится как «нет действий» (для disabled-стейта в stub'ах).
  final bool disabled;

  const IntegrationProviderCard({
    super.key,
    required this.logo,
    required this.title,
    required this.status,
    this.subtitle,
    this.statusLabel,
    this.statusDetail,
    this.actions = const [],
    this.onTap,
    this.disabled = false,
  });

  /// Конструктор stub-карточки «Скоро» (нет действий, серый статус).
  ///
  /// Используется в `llm_integrations_screen` / `git_integrations_screen`
  /// stub-экранах этапа 1.
  factory IntegrationProviderCard.disabled({
    Key? key,
    required Widget logo,
    required String title,
    String? subtitle,
    String? statusLabel,
  }) {
    return IntegrationProviderCard(
      key: key,
      logo: logo,
      title: title,
      subtitle: subtitle,
      status: IntegrationStatus.disconnected,
      statusLabel: statusLabel,
      disabled: true,
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'IntegrationProviderCard',
    );
    final theme = Theme.of(context);
    final colors = IntegrationStatusColors.of(context, status);

    final defaultLabel = () {
      switch (status) {
        case IntegrationStatus.connected:
          return l10n.integrationStatusConnected;
        case IntegrationStatus.disconnected:
          return l10n.integrationStatusDisconnected;
        case IntegrationStatus.error:
          return l10n.integrationStatusError;
        case IntegrationStatus.pending:
          return l10n.integrationStatusPending;
      }
    }();

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 40,
                    height: 40,
                    child: Center(child: logo),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          title,
                          style: theme.textTheme.titleMedium,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        if (subtitle != null) ...[
                          const SizedBox(height: 2),
                          Text(
                            subtitle!,
                            style: theme.textTheme.bodySmall?.copyWith(
                              color: theme.colorScheme.onSurfaceVariant,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              _StatusChip(
                colors: colors,
                icon: defaultIconForIntegrationStatus(status),
                label: statusLabel ?? defaultLabel,
                isPending: status == IntegrationStatus.pending,
              ),
              if (statusDetail != null) ...[
                const SizedBox(height: 6),
                Text(
                  statusDetail!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                ),
              ],
              if (!disabled && actions.isNotEmpty) ...[
                const SizedBox(height: 12),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: actions
                      .map((a) => _ActionButton(action: a))
                      .toList(growable: false),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  final IntegrationStatusColors colors;
  final IconData icon;
  final String label;
  final bool isPending;

  const _StatusChip({
    required this.colors,
    required this.icon,
    required this.label,
    required this.isPending,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: colors.background,
        borderRadius: BorderRadius.circular(999),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (isPending)
            SizedBox(
              width: 14,
              height: 14,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: colors.icon,
              ),
            )
          else
            Icon(icon, size: 16, color: colors.icon),
          const SizedBox(width: 6),
          Text(
            label,
            style: Theme.of(context).textTheme.labelMedium?.copyWith(
                  color: colors.foreground,
                  fontWeight: FontWeight.w600,
                ),
          ),
        ],
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final IntegrationAction action;

  const _ActionButton({required this.action});

  @override
  Widget build(BuildContext context) {
    final onPressed = action.isBusy ? null : action.onPressed;
    final iconWidget = action.isBusy
        ? const SizedBox(
            width: 16,
            height: 16,
            child: CircularProgressIndicator(strokeWidth: 2),
          )
        : (action.icon != null ? Icon(action.icon, size: 18) : null);
    final label = Text(action.label);

    switch (action.style) {
      case IntegrationActionStyle.primary:
        return iconWidget != null
            ? FilledButton.icon(
                onPressed: onPressed,
                icon: iconWidget,
                label: label,
              )
            : FilledButton(onPressed: onPressed, child: label);
      case IntegrationActionStyle.secondary:
        return iconWidget != null
            ? OutlinedButton.icon(
                onPressed: onPressed,
                icon: iconWidget,
                label: label,
              )
            : OutlinedButton(onPressed: onPressed, child: label);
      case IntegrationActionStyle.destructive:
        final scheme = Theme.of(context).colorScheme;
        return iconWidget != null
            ? TextButton.icon(
                onPressed: onPressed,
                icon: iconWidget,
                label: label,
                style: TextButton.styleFrom(foregroundColor: scheme.error),
              )
            : TextButton(
                onPressed: onPressed,
                style: TextButton.styleFrom(foregroundColor: scheme.error),
                child: label,
              );
    }
  }
}
