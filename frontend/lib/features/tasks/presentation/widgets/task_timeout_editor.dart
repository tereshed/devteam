import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Регекс клиентской валидации формата (Sprint 17 §6.5).
/// Принимает: "4h", "90m", "3600s", "1h30m", "1h30s", "5m30s".
/// Bounds (1m..72h) — обязанность сервера, см. ErrTaskInvalidTimeout.
final RegExp kTaskCustomTimeoutPattern = RegExp(r'^\d+(h|m|s)(\d+(m|s))?$');

/// Открыть диалог редактирования per-task custom_timeout (PATCH `/tasks/:id`).
///
/// `currentValue` — текущая Go time.Duration строка (например `"4h"`) или `null`,
/// если override не задан. Возвращает `true`, если значение было успешно
/// сохранено (включая reset к дефолту), иначе `null`/`false`.
Future<bool?> showTaskTimeoutDialog({
  required BuildContext context,
  required WidgetRef ref,
  required String projectId,
  required String taskId,
  required String? currentValue,
}) {
  return showDialog<bool>(
    context: context,
    builder: (ctx) => TaskTimeoutDialog(
      projectId: projectId,
      taskId: taskId,
      currentValue: currentValue,
    ),
  );
}

/// Содержимое диалога правки custom_timeout. Вынесен отдельно для widget-тестов.
class TaskTimeoutDialog extends ConsumerStatefulWidget {
  const TaskTimeoutDialog({
    super.key,
    required this.projectId,
    required this.taskId,
    required this.currentValue,
  });

  final String projectId;
  final String taskId;
  final String? currentValue;

  @override
  ConsumerState<TaskTimeoutDialog> createState() => _TaskTimeoutDialogState();
}

class _TaskTimeoutDialogState extends ConsumerState<TaskTimeoutDialog> {
  late final TextEditingController _controller =
      TextEditingController(text: widget.currentValue ?? '');
  bool _busy = false;
  String? _serverError;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  String? _clientValidate(String raw, AppLocalizations l10n) {
    if (raw.isEmpty) {
      return l10n.tasksCustomTimeoutInvalid;
    }
    if (!kTaskCustomTimeoutPattern.hasMatch(raw)) {
      return l10n.tasksCustomTimeoutInvalid;
    }
    return null;
  }

  Future<void> _onSave() async {
    final l10n = AppLocalizations.of(context)!;
    final raw = _controller.text.trim();
    final clientErr = _clientValidate(raw, l10n);
    if (clientErr != null) {
      setState(() => _serverError = clientErr);
      return;
    }
    if (raw == (widget.currentValue ?? '')) {
      Navigator.of(context).pop(false);
      return;
    }
    await _submit(raw, l10n.tasksCustomTimeoutSavedSnack);
  }

  Future<void> _onClear() async {
    final l10n = AppLocalizations.of(context)!;
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(l10n.tasksCustomTimeoutClearDialogTitle),
        content: Text(l10n.tasksCustomTimeoutClearDialogBody),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(l10n.cancel),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: Text(l10n.tasksCustomTimeoutClear),
          ),
        ],
      ),
    );
    if (ok != true || !mounted) {
      return;
    }
    await _submit('', l10n.tasksCustomTimeoutClearedSnack);
  }

  Future<void> _submit(String value, String successMessage) async {
    final messenger = ScaffoldMessenger.of(context);
    final navigator = Navigator.of(context);
    setState(() {
      _busy = true;
      _serverError = null;
    });
    try {
      final notifier = ref.read(
        taskDetailControllerProvider(
          projectId: widget.projectId,
          taskId: widget.taskId,
        ).notifier,
      );
      await notifier.updateTask(UpdateTaskRequest(customTimeout: value));
      if (!mounted) {
        return;
      }
      messenger.showSnackBar(SnackBar(content: Text(successMessage)));
      navigator.pop(true);
    } on TaskApiException catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        // Backend → 400 invalid_timeout: message из ответа в form-error,
        // не дублируем клиентскую regex-ошибку (Sprint 17 §6.5 security note).
        _serverError = e.message;
      });
    } catch (e) {
      if (!mounted) {
        return;
      }
      setState(() {
        _busy = false;
        _serverError = e.toString();
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final hasOverride =
        widget.currentValue != null && widget.currentValue!.isNotEmpty;

    return AlertDialog(
      title: Text(l10n.tasksCustomTimeoutSectionTitle),
      content: SizedBox(
        width: 420,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            TextField(
              controller: _controller,
              enabled: !_busy,
              autofocus: true,
              inputFormatters: [
                FilteringTextInputFormatter.allow(RegExp(r'[0-9hms]')),
                LengthLimitingTextInputFormatter(16),
              ],
              decoration: InputDecoration(
                labelText: l10n.tasksCustomTimeoutLabel,
                helperText: l10n.tasksCustomTimeoutHelper,
                helperMaxLines: 2,
                errorText: _serverError,
                errorMaxLines: 3,
                border: const OutlineInputBorder(),
              ),
              onSubmitted: (_) => unawaited(_onSave()),
            ),
          ],
        ),
      ),
      actions: [
        if (hasOverride)
          TextButton.icon(
            onPressed: _busy ? null : () => unawaited(_onClear()),
            icon: const Icon(Icons.restart_alt, size: 18),
            label: Text(l10n.tasksCustomTimeoutClear),
          ),
        TextButton(
          onPressed: _busy ? null : () => Navigator.of(context).pop(false),
          child: Text(l10n.cancel),
        ),
        FilledButton.icon(
          onPressed: _busy ? null : () => unawaited(_onSave()),
          icon: _busy
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : const Icon(Icons.check, size: 18),
          label: Text(l10n.tasksCustomTimeoutSave),
        ),
      ],
    );
  }
}
