import 'package:flutter/material.dart';
import 'package:frontend/features/projects/presentation/widgets/project_settings_tech_field_row.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Tech stack: строки key/value, добавление, очистка (13.4).
class ProjectSettingsTechStackSection extends StatelessWidget {
  const ProjectSettingsTechStackSection({
    super.key,
    required this.rows,
    required this.onAddRow,
    required this.onRemoveRow,
    required this.onClearTechStack,
    required this.onRowChanged,
  });

  final List<ProjectSettingsTechFieldRow> rows;
  final VoidCallback onAddRow;
  final ValueChanged<int> onRemoveRow;
  final VoidCallback onClearTechStack;
  final VoidCallback onRowChanged;

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          l10n.projectSettingsSectionTechStack,
          style: theme.textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        for (var i = 0; i < rows.length; i++)
          Padding(
            padding: const EdgeInsets.only(bottom: 8),
            child: Row(
              children: [
                Expanded(
                  child: TextFormField(
                    controller: rows[i].keyCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.projectSettingsTechStackKeyLabel,
                    ),
                    onChanged: (_) => onRowChanged(),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: TextFormField(
                    controller: rows[i].valueCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.projectSettingsTechStackValueLabel,
                    ),
                    onChanged: (_) => onRowChanged(),
                  ),
                ),
                IconButton(
                  onPressed:
                      rows.length > 1 ? () => onRemoveRow(i) : null,
                  icon: const Icon(Icons.delete_outline),
                ),
              ],
            ),
          ),
        Align(
          alignment: Alignment.centerLeft,
          child: TextButton.icon(
            onPressed: onAddRow,
            icon: const Icon(Icons.add),
            label: Text(l10n.projectSettingsTechStackAddRow),
          ),
        ),
        OutlinedButton(
          onPressed: onClearTechStack,
          child: Text(l10n.projectSettingsTechStackClear),
        ),
      ],
    );
  }
}
