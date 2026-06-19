import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/tasks/data/task_exceptions.dart';
import 'package:frontend/features/tasks/domain/requests.dart';
import 'package:frontend/features/tasks/presentation/controllers/task_detail_controller.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Клиентская валидация формата external_key (зеркало `externalKeyRe` бэкенда:
/// `^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`). Bounds/строгость — обязанность сервера.
final RegExp kTaskExternalKeyPattern =
    RegExp(r'^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$');

/// Открыть диалог правки external_key задачи (PATCH `/tasks/:id`).
///
/// `currentValue` — текущий ключ (например `"DEV-123"`) или `null`. Возвращает
/// `true`, если значение успешно сохранено (включая сброс), иначе `null`/`false`.
Future<bool?> showTaskExternalKeyDialog({
  required BuildContext context,
  required WidgetRef ref,
  required String projectId,
  required String taskId,
  required String? currentValue,
}) {
  return showDialog<bool>(
    context: context,
    builder: (ctx) => TaskExternalKeyDialog(
      projectId: projectId,
      taskId: taskId,
      currentValue: currentValue,
    ),
  );
}

/// Содержимое диалога правки external_key. Вынесен отдельно для widget-тестов.
class TaskExternalKeyDialog extends ConsumerStatefulWidget {
  const TaskExternalKeyDialog({
    super.key,
    required this.projectId,
    required this.taskId,
    required this.currentValue,
  });

  final String projectId;
  final String taskId;
  final String? currentValue;

  @override
  ConsumerState<TaskExternalKeyDialog> createState() =>
      _TaskExternalKeyDialogState();
}

class _TaskExternalKeyDialogState extends ConsumerState<TaskExternalKeyDialog> {
  late final TextEditingController _controller =
      TextEditingController(text: widget.currentValue ?? '');
  bool _busy = false;
  String? _serverError;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _onSave() async {
    final l10n = AppLocalizations.of(context)!;
    final raw = _controller.text.trim();
    if (raw.isEmpty) {
      // Пустое поле в режиме сохранения = сброс ключа.
      await _submit('', l10n.tasksExternalKeyClearedSnack);
      return;
    }
    if (!kTaskExternalKeyPattern.hasMatch(raw)) {
      setState(() => _serverError = l10n.tasksExternalKeyInvalid);
      return;
    }
    if (raw == (widget.currentValue ?? '')) {
      Navigator.of(context).pop(false);
      return;
    }
    await _submit(raw, l10n.tasksExternalKeySavedSnack);
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
      await notifier.updateTask(UpdateTaskRequest(externalKey: value));
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

    return AlertDialog(
      title: Text(l10n.tasksExternalKeyTitle),
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
                FilteringTextInputFormatter.allow(RegExp(r'[A-Za-z0-9_-]')),
                LengthLimitingTextInputFormatter(64),
              ],
              decoration: InputDecoration(
                labelText: l10n.tasksExternalKeyLabel,
                helperText: l10n.tasksExternalKeyHelper,
                helperMaxLines: 2,
                hintText: 'DEV-123',
                errorText: _serverError,
                errorMaxLines: 3,
                border: const OutlineInputBorder(),
              ),
              onSubmitted: (_) {
                FocusManager.instance.primaryFocus?.unfocus();
                unawaited(_onSave());
              },
            ),
          ],
        ),
      ),
      actions: [
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
          label: Text(l10n.tasksExternalKeySave),
        ),
      ],
    );
  }
}
