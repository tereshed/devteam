import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/assistant_mcp/domain/models/assistant_mcp_server_model.dart';

/// Результат формы MCP-сервера.
class AssistantMcpFormResult {
  const AssistantMcpFormResult({
    required this.name,
    required this.transport,
    required this.url,
    required this.headers,
    required this.requireConfirmation,
    required this.isEnabled,
  });

  final String name;
  final String transport;
  final String url;
  final Map<String, String> headers;
  final bool requireConfirmation;
  final bool isEnabled;
}

/// Сериализует заголовки в построчный вид `Name: value`.
String _headersToText(Map<String, String> headers) =>
    headers.entries.map((e) => '${e.key}: ${e.value}').join('\n');

/// Парсит построчный `Name: value` в map (пустые строки и строки без ':' пропускаются).
Map<String, String> _headersFromText(String text) {
  final out = <String, String>{};
  for (final raw in text.split('\n')) {
    final line = raw.trim();
    if (line.isEmpty) {
      continue;
    }
    final idx = line.indexOf(':');
    if (idx <= 0) {
      continue;
    }
    final key = line.substring(0, idx).trim();
    final value = line.substring(idx + 1).trim();
    if (key.isEmpty) {
      continue;
    }
    out[key] = value;
  }
  return out;
}

/// Диалог создания/редактирования MCP-сервера ассистента.
class AssistantMcpFormDialog extends StatefulWidget {
  const AssistantMcpFormDialog({super.key, this.existing});

  final AssistantMcpServerModel? existing;

  @override
  State<AssistantMcpFormDialog> createState() => _AssistantMcpFormDialogState();
}

class _AssistantMcpFormDialogState extends State<AssistantMcpFormDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _name;
  late final TextEditingController _url;
  late final TextEditingController _headers;
  late String _transport;
  late bool _requireConfirmation;
  late bool _isEnabled;

  static const _transports = ['http', 'sse'];

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _name = TextEditingController(text: e?.name ?? '');
    _url = TextEditingController(text: e?.url ?? '');
    _headers = TextEditingController(
      text: e == null ? '' : _headersToText(e.headers),
    );
    _transport = e?.transport ?? 'http';
    _requireConfirmation = e?.requireConfirmation ?? true;
    _isEnabled = e?.isEnabled ?? true;
  }

  @override
  void dispose() {
    _name.dispose();
    _url.dispose();
    _headers.dispose();
    super.dispose();
  }

  void _submit() {
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    Navigator.of(context).pop(
      AssistantMcpFormResult(
        name: _name.text.trim(),
        transport: _transport,
        url: _url.text.trim(),
        headers: _headersFromText(_headers.text),
        requireConfirmation: _requireConfirmation,
        isEnabled: _isEnabled,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AssistantMcpForm');
    final theme = Theme.of(context);
    return AlertDialog(
      title: Text(widget.existing == null
          ? l10n.assistantMcpFormTitleNew
          : l10n.assistantMcpFormTitleEdit),
      content: SizedBox(
        width: 460,
        child: SingleChildScrollView(
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(l10n.assistantMcpEnabledLabel),
                  value: _isEnabled,
                  onChanged: (v) => setState(() => _isEnabled = v),
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(l10n.assistantMcpRequireConfirmationLabel),
                  value: _requireConfirmation,
                  onChanged: (v) => setState(() => _requireConfirmation = v),
                ),
                _field(_name, l10n.assistantMcpNameLabel, required: true),
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: DropdownButtonFormField<String>(
                    initialValue: _transport,
                    decoration: InputDecoration(
                      labelText: l10n.assistantMcpTransportLabel,
                      border: const OutlineInputBorder(),
                    ),
                    items: [
                      for (final t in _transports)
                        DropdownMenuItem(value: t, child: Text(t)),
                    ],
                    onChanged: (v) => setState(() => _transport = v ?? 'http'),
                  ),
                ),
                _field(_url, l10n.assistantMcpUrlLabel,
                    required: true, keyboardType: TextInputType.url),
                _field(_headers, l10n.assistantMcpHeadersLabel,
                    minLines: 2, maxLines: 6),
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(
                    l10n.assistantMcpHeadersHint,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ),
                // Литеральный пример синтаксиса ссылки на секрет. Держим в коде (raw-строка):
                // в .arb фигурные скобки ${...} ломают ICU-парсер gen-l10n.
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(10),
                    decoration: BoxDecoration(
                      color: theme.colorScheme.surfaceContainerHighest,
                      borderRadius: BorderRadius.circular(6),
                    ),
                    child: SelectableText(
                      // raw-строка: $ и {} остаются литералами, без интерполяции Dart.
                      r'Authorization: Bearer ${secret:SECRET_NAME}',
                      style: const TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 12.5,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.assistantMcpCancel),
        ),
        FilledButton(
          onPressed: _submit,
          child: Text(l10n.assistantMcpSave),
        ),
      ],
    );
  }

  Widget _field(
    TextEditingController c,
    String label, {
    bool required = false,
    TextInputType? keyboardType,
    int? minLines,
    int? maxLines,
  }) {
    return Padding(
      padding: const EdgeInsets.only(top: 8),
      child: TextFormField(
        controller: c,
        keyboardType: keyboardType,
        minLines: minLines,
        maxLines: maxLines ?? 1,
        decoration: InputDecoration(
          labelText: label,
          border: const OutlineInputBorder(),
        ),
        validator: required
            ? (v) => (v == null || v.trim().isEmpty) ? '' : null
            : null,
      ),
    );
  }
}
