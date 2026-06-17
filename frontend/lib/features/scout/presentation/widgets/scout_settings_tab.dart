import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/scout/domain/scout_exceptions.dart';
import 'package:frontend/features/scout/presentation/controllers/scout_config_controller.dart';
import 'package:frontend/features/scout/presentation/controllers/scout_runs_controller.dart';
import 'package:frontend/features/scout/presentation/widgets/scout_config_form.dart';
import 'package:frontend/features/scout/presentation/widgets/scout_run_card.dart';
import 'package:frontend/features/settings/presentation/widgets/assistant_prompt_editor.dart';

/// Вкладка «Разведчик» в настройках проекта: конфиг + промпт + прогоны.
class ScoutSettingsTab extends ConsumerStatefulWidget {
  const ScoutSettingsTab({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<ScoutSettingsTab> createState() => _ScoutSettingsTabState();
}

class _ScoutSettingsTabState extends ConsumerState<ScoutSettingsTab> {
  Timer? _pollTimer;
  bool _runBusy = false;

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  /// Пока есть прогон в running — опрашиваем список раз в 5 секунд.
  void _syncPolling(bool hasRunning) {
    if (hasRunning && _pollTimer == null) {
      _pollTimer = Timer.periodic(const Duration(seconds: 5), (_) {
        ref
            .read(scoutRunsControllerProvider(widget.projectId).notifier)
            .refresh();
      });
    } else if (!hasRunning && _pollTimer != null) {
      _pollTimer?.cancel();
      _pollTimer = null;
    }
  }

  Future<void> _onDispatch() async {
    final l10n = requireAppLocalizations(context, where: 'ScoutSettingsTab');
    final problem = await _askProblem(l10n);
    if (problem == null || problem.trim().isEmpty) {
      return;
    }
    if (!mounted) {
      return;
    }
    final messenger = ScaffoldMessenger.of(context);
    setState(() => _runBusy = true);
    try {
      await ref
          .read(scoutRunsControllerProvider(widget.projectId).notifier)
          .dispatch(problem.trim());
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.scoutRunStartedSnack)),
      );
    } on ScoutException catch (e) {
      messenger.showSnackBar(SnackBar(content: Text(e.message)));
    } finally {
      if (mounted) {
        setState(() => _runBusy = false);
      }
    }
  }

  Future<String?> _askProblem(dynamic l10n) {
    final ctrl = TextEditingController();
    return showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.scoutRunDialogTitle),
        content: TextField(
          controller: ctrl,
          autofocus: true,
          minLines: 3,
          maxLines: 6,
          decoration: InputDecoration(
            hintText: l10n.scoutRunDialogHint,
            border: const OutlineInputBorder(),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text(l10n.scoutRunDialogCancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(ctrl.text),
            child: Text(l10n.scoutRunDialogStart),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'ScoutSettingsTab');
    final theme = Theme.of(context);
    final asyncConfig =
        ref.watch(scoutConfigControllerProvider(widget.projectId));
    final asyncRuns =
        ref.watch(scoutRunsControllerProvider(widget.projectId));

    final hasRunning =
        asyncRuns.value?.any((r) => r.status == 'running') ?? false;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        _syncPolling(hasRunning);
      }
    });

    return RefreshIndicator(
      onRefresh: () async {
        ref.invalidate(scoutConfigControllerProvider(widget.projectId));
        await ref
            .read(scoutRunsControllerProvider(widget.projectId).notifier)
            .refresh();
      },
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 24),
        children: [
          Text(l10n.scoutHeading, style: theme.textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            l10n.scoutDescription,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 16),
          asyncConfig.when(
            data: (config) => Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                ScoutConfigForm(
                  key: ValueKey(
                    'scout-config-${config.isEnabled}-'
                    '${config.codeBackend}-${config.timeoutSeconds}',
                  ),
                  config: config,
                  onSave: ({
                    bool? isEnabled,
                    String? codeBackend,
                    String? providerKind,
                    double? temperature,
                    Map<String, dynamic>? codeBackendSettings,
                    Map<String, dynamic>? sandboxPermissions,
                    int? timeoutSeconds,
                  }) async {
                    final messenger = ScaffoldMessenger.of(context);
                    await ref
                        .read(scoutConfigControllerProvider(widget.projectId)
                            .notifier)
                        .save(
                          isEnabled: isEnabled,
                          codeBackend: codeBackend,
                          providerKind: providerKind,
                          temperature: temperature,
                          codeBackendSettings: codeBackendSettings,
                          sandboxPermissions: sandboxPermissions,
                          timeoutSeconds: timeoutSeconds,
                        );
                    messenger.showSnackBar(
                      SnackBar(content: Text(l10n.scoutSavedSnack)),
                    );
                  },
                ),
                const SizedBox(height: 16),
                AssistantPromptEditor(
                  key: ValueKey('scout-prompt-${config.prompt.length}'),
                  heading: l10n.scoutPromptHeading,
                  hint: l10n.scoutPromptHint,
                  initialValue: config.prompt,
                  inheritedNotice:
                      config.prompt.isEmpty ? l10n.scoutPromptDefaultNotice : null,
                  onSave: (value) async {
                    final messenger = ScaffoldMessenger.of(context);
                    await ref
                        .read(scoutConfigControllerProvider(widget.projectId)
                            .notifier)
                        .save(prompt: value);
                    messenger.showSnackBar(
                      SnackBar(content: Text(l10n.scoutSavedSnack)),
                    );
                  },
                  onReset: config.prompt.isEmpty
                      ? null
                      : () async {
                          await ref
                              .read(
                                  scoutConfigControllerProvider(widget.projectId)
                                      .notifier)
                              .save(prompt: '');
                        },
                ),
              ],
            ),
            loading: () => const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: CircularProgressIndicator(),
              ),
            ),
            error: (e, _) => _ScoutErrorBox(message: l10n.scoutLoadError),
          ),
          const SizedBox(height: 24),
          Row(
            children: [
              Expanded(
                child: Text(
                  l10n.scoutRunsTitle,
                  style: theme.textTheme.titleMedium,
                ),
              ),
              FilledButton.icon(
                onPressed: _runBusy ? null : _onDispatch,
                icon: const Icon(Icons.travel_explore),
                label: Text(l10n.scoutRunButton),
              ),
            ],
          ),
          const SizedBox(height: 12),
          asyncRuns.when(
            data: (runs) => runs.isEmpty
                ? Padding(
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    child: Text(
                      l10n.scoutRunsEmpty,
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
                          child: ScoutRunCard(run: run),
                        ),
                    ],
                  ),
            loading: () => const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: CircularProgressIndicator(),
              ),
            ),
            error: (e, _) => _ScoutErrorBox(message: l10n.scoutLoadError),
          ),
        ],
      ),
    );
  }
}

class _ScoutErrorBox extends StatelessWidget {
  const _ScoutErrorBox({required this.message});

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
