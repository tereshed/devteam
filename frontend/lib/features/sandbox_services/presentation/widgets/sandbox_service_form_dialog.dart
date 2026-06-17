import 'package:flutter/material.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/features/sandbox_services/domain/models/sandbox_service_model.dart';

/// Результат формы декларации сервис-сайдкара.
class SandboxServiceFormResult {
  const SandboxServiceFormResult({
    required this.alias,
    required this.isEnabled,
    required this.kind,
    required this.image,
    required this.dbName,
    required this.dbUser,
    required this.port,
    required this.seedKind,
    required this.seedValue,
    required this.readyTimeoutSeconds,
  });

  final String alias;
  final bool isEnabled;
  final String kind;
  final String image;
  final String dbName;
  final String dbUser;
  final int port;
  final String seedKind;
  final String seedValue;
  final int readyTimeoutSeconds;
}

/// Диалог создания/редактирования декларации сервис-сайдкара.
class SandboxServiceFormDialog extends StatefulWidget {
  const SandboxServiceFormDialog({super.key, this.existing});

  final SandboxServiceModel? existing;

  @override
  State<SandboxServiceFormDialog> createState() =>
      _SandboxServiceFormDialogState();
}

class _SandboxServiceFormDialogState extends State<SandboxServiceFormDialog> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _alias;
  late final TextEditingController _image;
  late final TextEditingController _dbName;
  late final TextEditingController _dbUser;
  late final TextEditingController _port;
  late final TextEditingController _seedValue;
  late final TextEditingController _readyTimeout;
  late bool _isEnabled;
  late String _seedKind;

  static const _seedKinds = ['none', 'repo_file', 'inline'];

  @override
  void initState() {
    super.initState();
    final e = widget.existing;
    _alias = TextEditingController(text: e?.alias ?? 'db');
    _image = TextEditingController(text: e?.image ?? 'postgres:16-alpine');
    _dbName = TextEditingController(text: e?.dbName ?? 'app');
    _dbUser = TextEditingController(text: e?.dbUser ?? 'postgres');
    _port = TextEditingController(text: (e?.port ?? 5432).toString());
    _seedValue = TextEditingController(text: e?.seedValue ?? '');
    _readyTimeout =
        TextEditingController(text: (e?.readyTimeoutSeconds ?? 60).toString());
    _isEnabled = e?.isEnabled ?? true;
    _seedKind = e?.seedKind ?? 'none';
  }

  @override
  void dispose() {
    _alias.dispose();
    _image.dispose();
    _dbName.dispose();
    _dbUser.dispose();
    _port.dispose();
    _seedValue.dispose();
    _readyTimeout.dispose();
    super.dispose();
  }

  void _submit() {
    if (!(_formKey.currentState?.validate() ?? false)) {
      return;
    }
    Navigator.of(context).pop(
      SandboxServiceFormResult(
        alias: _alias.text.trim(),
        isEnabled: _isEnabled,
        kind: 'postgres',
        image: _image.text.trim(),
        dbName: _dbName.text.trim(),
        dbUser: _dbUser.text.trim(),
        port: int.tryParse(_port.text.trim()) ?? 5432,
        seedKind: _seedKind,
        seedValue: _seedKind == 'none' ? '' : _seedValue.text,
        readyTimeoutSeconds: int.tryParse(_readyTimeout.text.trim()) ?? 60,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'SandboxServiceForm');
    return AlertDialog(
      title: Text(widget.existing == null
          ? l10n.sandboxServiceFormTitleNew
          : l10n.sandboxServiceFormTitleEdit),
      content: SizedBox(
        width: 420,
        child: SingleChildScrollView(
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: Text(l10n.sandboxServiceEnabledLabel),
                  value: _isEnabled,
                  onChanged: (v) => setState(() => _isEnabled = v),
                ),
                _field(_alias, l10n.sandboxServiceAliasLabel, required: true),
                _field(_image, l10n.sandboxServiceImageLabel, required: true),
                _field(_dbName, l10n.sandboxServiceDbNameLabel),
                _field(_dbUser, l10n.sandboxServiceDbUserLabel),
                _field(_port, l10n.sandboxServicePortLabel,
                    keyboardType: TextInputType.number),
                _field(_readyTimeout, l10n.sandboxServiceReadyTimeoutLabel,
                    keyboardType: TextInputType.number),
                Padding(
                  padding: const EdgeInsets.only(top: 8),
                  child: DropdownButtonFormField<String>(
                    initialValue: _seedKind,
                    decoration: InputDecoration(
                      labelText: l10n.sandboxServiceSeedKindLabel,
                      border: const OutlineInputBorder(),
                    ),
                    items: [
                      for (final k in _seedKinds)
                        DropdownMenuItem(value: k, child: Text(k)),
                    ],
                    onChanged: (v) => setState(() => _seedKind = v ?? 'none'),
                  ),
                ),
                if (_seedKind != 'none')
                  _field(_seedValue, l10n.sandboxServiceSeedValueLabel,
                      minLines: 2, maxLines: 5),
              ],
            ),
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(l10n.sandboxServiceCancel),
        ),
        FilledButton(
          onPressed: _submit,
          child: Text(l10n.sandboxServiceSave),
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
