import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/scout/domain/models/scout_run_model.dart';
import 'package:intl/intl.dart';

/// Карточка прогона разведчика: статус, проблема и (раскрытие) досье.
class ScoutRunCard extends StatelessWidget {
  const ScoutRunCard({super.key, required this.run});

  final ScoutRunModel run;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ScoutRunCard');
    final theme = Theme.of(context);
    final dateFmt = DateFormat.yMd(
      Localizations.localeOf(context).toLanguageTag(),
    ).add_Hm();

    final (statusLabel, statusColor, statusOnColor) = switch (run.status) {
      'running' => (
          l10n.scoutRunStatusRunning,
          theme.colorScheme.secondaryContainer,
          theme.colorScheme.onSecondaryContainer,
        ),
      'failed' => (
          l10n.scoutRunStatusFailed,
          theme.colorScheme.errorContainer,
          theme.colorScheme.onErrorContainer,
        ),
      _ => (
          l10n.scoutRunStatusDone,
          theme.colorScheme.primaryContainer,
          theme.colorScheme.onPrimaryContainer,
        ),
    };

    return Card(
      margin: EdgeInsets.zero,
      child: ExpansionTile(
        shape: const Border(),
        title: Row(
          children: [
            Chip(
              label: Text(statusLabel),
              backgroundColor: statusColor,
              labelStyle: theme.textTheme.labelMedium?.copyWith(
                color: statusOnColor,
              ),
              visualDensity: VisualDensity.compact,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                dateFmt.format(run.startedAt.toLocal()),
                style: theme.textTheme.bodyMedium,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            if (run.status == 'running')
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              ),
          ],
        ),
        subtitle: run.problem.isEmpty
            ? null
            : Padding(
                padding: const EdgeInsets.only(top: 4),
                child: Text(
                  run.problem,
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (run.status == 'failed' && run.error.isNotEmpty) ...[
                  Text(
                    run.error,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.error,
                    ),
                  ),
                  const SizedBox(height: 12),
                ],
                Text(l10n.scoutDossierTitle, style: theme.textTheme.titleSmall),
                const SizedBox(height: 4),
                SelectableText(
                  run.dossier.isEmpty ? l10n.scoutDossierEmpty : run.dossier,
                  style: theme.textTheme.bodyMedium,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}
