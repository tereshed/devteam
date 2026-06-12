import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/enhancer/domain/enhancer_exceptions.dart';
import 'package:frontend/features/enhancer/presentation/controllers/enhancer_config_controller.dart';
import 'package:frontend/features/enhancer/presentation/controllers/enhancer_runs_controller.dart';
import 'package:frontend/features/enhancer/presentation/widgets/enhancer_config_form.dart';
import 'package:frontend/features/enhancer/presentation/widgets/enhancer_run_card.dart';

/// Вкладка «Улучшение» в настройках проекта: конфиг энхансера + прогоны.
class EnhancerSettingsTab extends ConsumerStatefulWidget {
  const EnhancerSettingsTab({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<EnhancerSettingsTab> createState() =>
      _EnhancerSettingsTabState();
}

class _EnhancerSettingsTabState extends ConsumerState<EnhancerSettingsTab> {
  Timer? _pollTimer;
  bool _runBusy = false;

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  /// Пока есть прогон в running — опрашиваем список раз в 5 секунд,
  /// чтобы отчёт появился без ручного рефреша.
  void _syncPolling(bool hasRunning) {
    if (hasRunning && _pollTimer == null) {
      _pollTimer = Timer.periodic(const Duration(seconds: 5), (_) {
        ref
            .read(enhancerRunsControllerProvider(widget.projectId).notifier)
            .refresh();
      });
    } else if (!hasRunning && _pollTimer != null) {
      _pollTimer?.cancel();
      _pollTimer = null;
    }
  }

  Future<void> _onRunNow() async {
    final l10n = requireAppLocalizations(context, where: 'EnhancerSettingsTab');
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _runBusy = true);
    try {
      await ref
          .read(enhancerRunsControllerProvider(widget.projectId).notifier)
          .runNow();
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.enhancerRunStartedSnack)),
      );
    } on EnhancerConflictException {
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.enhancerRunInProgressSnack)),
      );
    } on EnhancerException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    } finally {
      if (mounted) {
        setState(() => _runBusy = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'EnhancerSettingsTab');
    final theme = Theme.of(context);
    final asyncConfig =
        ref.watch(enhancerConfigControllerProvider(widget.projectId));
    final asyncRuns =
        ref.watch(enhancerRunsControllerProvider(widget.projectId));

    final hasRunning =
        asyncRuns.value?.any((r) => r.status == 'running') ?? false;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _syncPolling(hasRunning);
      }
    });

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(enhancerConfigControllerProvider(widget.projectId));
        await ref
            .read(enhancerRunsControllerProvider(widget.projectId).notifier)
            .refresh();
      },
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
        children: [
          Text(l10n.enhancerHeading, style: theme.textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            l10n.enhancerDescription,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          asyncConfig.when(
            data: (config) => EnhancerConfigForm(
              // Пересоздаём форму при внешнем изменении конфига.
              key: ValueKey(
                'enhancer-config-${config.isActive}-'
                '${config.cronExpression ?? ''}-'
                '${config.analysisWindowDays}-${config.maxChangesPerRun}',
              ),
              config: config,
              onSave: ({
                bool? isActive,
                String? cronExpression,
                int? analysisWindowDays,
                int? maxChangesPerRun,
              }) async {
                final messenger = ScaffoldMessenger.of(context);
                await ref
                    .read(enhancerConfigControllerProvider(widget.projectId)
                        .notifier)
                    .save(
                      isActive: isActive,
                      cronExpression: cronExpression,
                      analysisWindowDays: analysisWindowDays,
                      maxChangesPerRun: maxChangesPerRun,
                    );
                messenger.showSnackBar(
                  SnackBar(content: Text(l10n.enhancerSavedSnack)),
                );
              },
            ),
            loading: () => const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: CircularProgressIndicator(),
              ),
            ),
            error: (e, _) => _EnhancerErrorBox(message: l10n.enhancerLoadError),
          ),
          const SizedBox(height: 24),
          Row(
            children: [
              Expanded(
                child: Text(
                  l10n.enhancerRunsTitle,
                  style: theme.textTheme.titleMedium,
                ),
              ),
              FilledButton.icon(
                onPressed: (_runBusy || hasRunning) ? null : _onRunNow,
                icon: const Icon(Icons.auto_fix_high),
                label: Text(l10n.enhancerRunNowButton),
              ),
            ],
          ),
          const SizedBox(height: 12),
          asyncRuns.when(
            data: (runs) => runs.isEmpty
                ? Padding(
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    child: Text(
                      l10n.enhancerRunsEmpty,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                  )
                : Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      for (final run in runs)
                        Padding(
                          padding: const EdgeInsets.only(bottom: 8),
                          child: EnhancerRunCard(
                            projectId: widget.projectId,
                            run: run,
                          ),
                        ),
                    ],
                  ),
            loading: () => const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: CircularProgressIndicator(),
              ),
            ),
            error: (e, _) => _EnhancerErrorBox(message: l10n.enhancerLoadError),
          ),
        ],
      ),
    );
  }
}

class _EnhancerErrorBox extends StatelessWidget {
  const _EnhancerErrorBox({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: theme.colorScheme.errorContainer,
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        message,
        style: theme.textTheme.bodyMedium?.copyWith(
          color: theme.colorScheme.onErrorContainer,
        ),
      ),
    );
  }
}
