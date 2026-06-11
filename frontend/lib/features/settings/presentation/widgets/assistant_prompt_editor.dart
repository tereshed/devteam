import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/utils/responsive.dart';

/// Переиспользуемый редактор системного промпта ассистента (user / project).
/// Без знания об источнике данных: получает текущее значение, отдаёт сохранение
/// и (опц.) сброс наверх. Кнопка «Сохранить» активна только при изменении.
class AssistantPromptEditor extends StatefulWidget {
  const AssistantPromptEditor({
    super.key,
    required this.heading,
    required this.hint,
    required this.initialValue,
    required this.onSave,
    this.onReset,
    this.inheritedNotice,
  });

  /// Заголовок секции.
  final String heading;

  /// Поясняющий текст под заголовком (модель наследования).
  final String hint;

  /// Текущее значение промпта (пусто — наследуется/не задан).
  final String initialValue;

  /// Сохранение нового текста. Бросает при ошибке — виджет покажет снек.
  final Future<void> Function(String value) onSave;

  /// Сброс к вышестоящему уровню (project → user). null — кнопка скрыта.
  final Future<void> Function()? onReset;

  /// Плашка-уведомление о наследовании (показывается, когда своего промпта нет).
  final String? inheritedNotice;

  @override
  State<AssistantPromptEditor> createState() => _AssistantPromptEditorState();
}

class _AssistantPromptEditorState extends State<AssistantPromptEditor> {
  late final TextEditingController _controller;
  late String _saved;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    _saved = widget.initialValue;
    _controller = TextEditingController(text: widget.initialValue);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  bool get _dirty => _controller.text != _saved;

  Future<void> _run(Future<void> Function() action, String okMsg) async {
    setState(() => _busy = true);
    final l10n = requireAppLocalizations(
      context,
      where: 'assistantPromptEditor',
    );
    final messenger = ScaffoldMessenger.of(context);
    try {
      await action();
      if (!mounted) {
        return;
      }
      messenger.showSnackBar(SnackBar(content: Text(okMsg)));
    } catch (_) {
      if (!mounted) {
        return;
      }
      messenger.showSnackBar(
        SnackBar(content: Text(l10n.assistantPromptSaveError)),
      );
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(
      context,
      where: 'assistantPromptEditor',
    );
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(widget.heading, style: theme.textTheme.titleMedium),
        SizedBox(height: Spacing.small(context)),
        Text(
          widget.hint,
          style: theme.textTheme.bodySmall?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        if (widget.inheritedNotice != null) ...[
          SizedBox(height: Spacing.small(context)),
          Container(
            padding: Spacing.cardPadding(context),
            decoration: BoxDecoration(
              color: theme.colorScheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.info_outline,
                  size: 18,
                  color: theme.colorScheme.onSurfaceVariant,
                ),
                SizedBox(width: Spacing.small(context)),
                Expanded(
                  child: Text(
                    widget.inheritedNotice!,
                    style: theme.textTheme.bodySmall,
                  ),
                ),
              ],
            ),
          ),
        ],
        SizedBox(height: Spacing.medium(context)),
        TextField(
          controller: _controller,
          minLines: 8,
          maxLines: 24,
          enabled: !_busy,
          onChanged: (_) => setState(() {}),
          decoration: const InputDecoration(
            border: OutlineInputBorder(),
            alignLabelWithHint: true,
          ),
          style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
        ),
        SizedBox(height: Spacing.medium(context)),
        Row(
          children: [
            if (widget.onReset != null)
              TextButton.icon(
                onPressed: _busy
                    ? null
                    : () => _run(() async {
                        await widget.onReset!();
                        if (mounted) {
                          setState(() {
                            _saved = '';
                            _controller.text = '';
                          });
                        }
                      }, l10n.assistantPromptSaved),
                icon: const Icon(Icons.restart_alt),
                label: Text(l10n.assistantPromptReset),
              ),
            const Spacer(),
            FilledButton.icon(
              onPressed: (!_dirty || _busy)
                  ? null
                  : () => _run(() async {
                      final v = _controller.text;
                      await widget.onSave(v);
                      if (mounted) {
                        setState(() => _saved = v);
                      }
                    }, l10n.assistantPromptSaved),
              icon: _busy
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.save),
              label: Text(l10n.assistantPromptSave),
            ),
          ],
        ),
      ],
    );
  }
}
