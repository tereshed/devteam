import 'package:flutter/material.dart';

/// Карточка-статистика на dashboard hub.
///
/// Семантика отличается от [IntegrationProviderCard] (там — конкретный
/// провайдер + actions), поэтому это отдельный лёгкий виджет.
/// См. dashboard-redesign-plan.md §4.2 / §4a.3.
class StatCard extends StatelessWidget {
  final IconData icon;
  final String title;

  /// Большое число / заголовок-значение (например, "3 active").
  final String primaryValue;

  /// Опциональная вторая строка под значением (например, "1 archived").
  final String? secondaryValue;

  /// Лейбл CTA (например, "Управлять →").
  final String ctaLabel;

  final VoidCallback onTap;

  const StatCard({
    super.key,
    required this.icon,
    required this.title,
    required this.primaryValue,
    required this.ctaLabel,
    required this.onTap,
    this.secondaryValue,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.all(20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                children: [
                  Container(
                    width: 40,
                    height: 40,
                    decoration: BoxDecoration(
                      color: scheme.secondaryContainer,
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Icon(icon, size: 22, color: scheme.primary),
                  ),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text(
                      title,
                      style: theme.textTheme.titleMedium,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 16),
              Text(
                primaryValue,
                style: theme.textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
              if (secondaryValue != null) ...[
                const SizedBox(height: 2),
                Text(
                  secondaryValue!,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
              const SizedBox(height: 16),
              Align(
                alignment: Alignment.centerLeft,
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      ctaLabel,
                      style: theme.textTheme.labelLarge?.copyWith(
                        color: scheme.primary,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(width: 4),
                    Icon(Icons.arrow_forward, size: 16, color: scheme.primary),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
