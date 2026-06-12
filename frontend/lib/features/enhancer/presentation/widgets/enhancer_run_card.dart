import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_change_model.dart';
import 'package:frontend/features/enhancer/domain/models/enhancer_run_model.dart';
import 'package:frontend/features/enhancer/presentation/controllers/enhancer_runs_controller.dart';
import 'package:intl/intl.dart';

/// Карточка прогона энхансера: статус, отчёт и (лениво) предложения изменений.
class EnhancerRunCard extends ConsumerWidget {
  const EnhancerRunCard({
    super.key,
    required this.projectId,
    required this.run,
  });

  final String projectId;
  final EnhancerRunModel run;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'EnhancerRunCard');
    final theme = Theme.of(context);
    final dateFmt = DateFormat.yMd(
      Localizations.localeOf(context).toLanguageTag(),
    ).add_Hm();

    final (statusLabel, statusColor, statusOnColor) = switch (run.status) {
      'running' => (
          l10n.enhancerRunStatusRunning,
          theme.colorScheme.secondaryContainer,
          theme.colorScheme.onSecondaryContainer,
        ),
      'failed' => (
          l10n.enhancerRunStatusFailed,
          theme.colorScheme.errorContainer,
          theme.colorScheme.onErrorContainer,
        ),
      _ => (
          l10n.enhancerRunStatusDone,
          theme.colorScheme.primaryContainer,
          theme.colorScheme.onPrimaryContainer,
        ),
    };
    final triggerLabel = run.triggerKind == 'cron'
        ? l10n.enhancerTriggerCron
        : l10n.enhancerTriggerManual;

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
                '${dateFmt.format(run.startedAt.toLocal())} · $triggerLabel',
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
                Text(l10n.enhancerReportTitle,
                    style: theme.textTheme.titleSmall),
                const SizedBox(height: 4),
                SelectableText(
                  run.report.isEmpty ? l10n.enhancerReportEmpty : run.report,
                  style: theme.textTheme.bodyMedium,
                ),
                const SizedBox(height: 12),
                Text(l10n.enhancerChangesTitle,
                    style: theme.textTheme.titleSmall),
                const SizedBox(height: 4),
                _EnhancerRunChanges(projectId: projectId, runId: run.id),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Лениво загружаемый список предложений прогона.
class _EnhancerRunChanges extends ConsumerWidget {
  const _EnhancerRunChanges({required this.projectId, required this.runId});

  final String projectId;
  final String runId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l10n = requireAppLocalizations(context, where: 'EnhancerRunChanges');
    final theme = Theme.of(context);
    final asyncChanges =
        ref.watch(enhancerRunChangesProvider(projectId, runId));

    return asyncChanges.when(
      data: (changes) => changes.isEmpty
          ? Text(
              l10n.enhancerChangesEmpty,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            )
          : Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                for (final change in changes)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: _EnhancerChangeTile(change: change),
                  ),
              ],
            ),
      loading: () => const Padding(
        padding: EdgeInsets.all(8),
        child: Center(
          child: SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
      ),
      error: (e, _) => Text(
        l10n.enhancerLoadError,
        style: theme.textTheme.bodyMedium?.copyWith(
          color: theme.colorScheme.error,
        ),
      ),
    );
  }
}

/// Одно предложение: цель, обоснование, ожидаемый эффект и дифф (JSON).
class _EnhancerChangeTile extends StatelessWidget {
  const _EnhancerChangeTile({required this.change});

  final EnhancerChangeModel change;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'EnhancerChangeTile');
    final theme = Theme.of(context);

    final targetLabel = switch (change.targetKind) {
      'agent_override' => l10n.enhancerTargetAgentOverride,
      'project_description' => l10n.enhancerTargetProjectDescription,
      _ => l10n.enhancerTargetProjectSettings,
    };
    final statusLabel = switch (change.status) {
      'approved' => l10n.enhancerChangeStatusApproved,
      'applied' => l10n.enhancerChangeStatusApplied,
      'rejected' => l10n.enhancerChangeStatusRejected,
      'rolled_back' => l10n.enhancerChangeStatusRolledBack,
      _ => l10n.enhancerChangeStatusProposed,
    };
    final payloadPretty =
        const JsonEncoder.withIndent('  ').convert(change.payload);

    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(targetLabel, style: theme.textTheme.titleSmall),
              ),
              Chip(
                label: Text(statusLabel),
                visualDensity: VisualDensity.compact,
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            l10n.enhancerChangeReasonLabel,
            style: theme.textTheme.labelMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          Text(change.reason, style: theme.textTheme.bodyMedium),
          const SizedBox(height: 8),
          Text(
            l10n.enhancerChangeEffectLabel,
            style: theme.textTheme.labelMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          Text(change.expectedEffect, style: theme.textTheme.bodyMedium),
          const SizedBox(height: 8),
          Text(
            l10n.enhancerChangePayloadLabel,
            style: theme.textTheme.labelMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 4),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: theme.colorScheme.surface,
              borderRadius: BorderRadius.circular(8),
            ),
            child: SelectableText(
              payloadPretty,
              style: theme.textTheme.bodySmall?.copyWith(
                fontFamily: 'monospace',
              ),
            ),
          ),
        ],
      ),
    );
  }
}
