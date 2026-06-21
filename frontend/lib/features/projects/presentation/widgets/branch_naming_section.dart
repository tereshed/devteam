import 'package:flutter/material.dart';
import 'package:frontend/features/projects/presentation/utils/branch_template_preview.dart';
import 'package:frontend/l10n/app_localizations.dart';

/// Под-секция «Именование веток» в настройках проекта (Git-секция).
///
/// Шаблон имён веток + опциональный явный regex формата + замок ручного override,
/// с живым превью результата. Stateful — чтобы превью обновлялось по мере ввода
/// шаблона (слушает контроллер), не завязываясь на dirty-логику экрана.
class BranchNamingSection extends StatefulWidget {
  const BranchNamingSection({
    super.key,
    required this.templateController,
    required this.patternController,
    required this.mrTitleController,
    required this.locked,
    required this.onLockedChanged,
    required this.onChanged,
  });

  final TextEditingController templateController;
  final TextEditingController patternController;
  final TextEditingController mrTitleController;
  final bool locked;
  final ValueChanged<bool> onLockedChanged;
  final VoidCallback onChanged;

  @override
  State<BranchNamingSection> createState() => _BranchNamingSectionState();
}

class _BranchNamingSectionState extends State<BranchNamingSection> {
  @override
  void initState() {
    super.initState();
    widget.templateController.addListener(_onTemplateChanged);
    widget.mrTitleController.addListener(_onTemplateChanged);
  }

  @override
  void dispose() {
    widget.templateController.removeListener(_onTemplateChanged);
    widget.mrTitleController.removeListener(_onTemplateChanged);
    super.dispose();
  }

  void _onTemplateChanged() {
    setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final theme = Theme.of(context);
    final preview = branchTemplatePreview(widget.templateController.text);
    final mrPreview = mrTitlePreview(widget.mrTitleController.text);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 16),
        Text(
          l10n.projectSettingsBranchNamingTitle,
          style: theme.textTheme.titleSmall,
        ),
        const SizedBox(height: 8),
        TextFormField(
          key: const ValueKey('project-settings-branch-template'),
          controller: widget.templateController,
          decoration: InputDecoration(
            labelText: l10n.projectSettingsBranchTemplateLabel,
            hintText: 'issue/{ticket}_{slug}',
            helperText: l10n.projectSettingsBranchTemplateHint,
            helperMaxLines: 3,
          ),
          onChanged: (_) => widget.onChanged(),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: theme.colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            children: [
              Icon(
                Icons.account_tree_outlined,
                size: 18,
                color: theme.colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      l10n.projectSettingsBranchPreviewLabel,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 2),
                    SelectableText(
                      preview,
                      style: theme.textTheme.bodyMedium?.copyWith(
                        fontFamily: 'monospace',
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        TextFormField(
          key: const ValueKey('project-settings-branch-pattern'),
          controller: widget.patternController,
          decoration: InputDecoration(
            labelText: l10n.projectSettingsBranchPatternLabel,
            hintText: r'^issue/[A-Z]+-\d+_.+$',
            helperText: l10n.projectSettingsBranchPatternHint,
            helperMaxLines: 3,
          ),
          onChanged: (_) => widget.onChanged(),
        ),
        const SizedBox(height: 4),
        SwitchListTile(
          contentPadding: EdgeInsets.zero,
          value: widget.locked,
          onChanged: widget.onLockedChanged,
          title: Text(l10n.projectSettingsBranchLockLabel),
          subtitle: Text(l10n.projectSettingsBranchLockSubtitle),
        ),
        const SizedBox(height: 12),
        const Divider(height: 1),
        const SizedBox(height: 16),
        Text(
          l10n.projectSettingsMrTitleTitle,
          style: theme.textTheme.titleSmall,
        ),
        const SizedBox(height: 8),
        TextFormField(
          key: const ValueKey('project-settings-mr-title-template'),
          controller: widget.mrTitleController,
          decoration: InputDecoration(
            labelText: l10n.projectSettingsMrTitleLabel,
            hintText: '[{ticket}] {title}',
            helperText: l10n.projectSettingsMrTitleHint,
            helperMaxLines: 3,
          ),
          onChanged: (_) => widget.onChanged(),
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
          decoration: BoxDecoration(
            color: theme.colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(8),
          ),
          child: Row(
            children: [
              Icon(
                Icons.merge_type,
                size: 18,
                color: theme.colorScheme.onSurfaceVariant,
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      l10n.projectSettingsBranchPreviewLabel,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.onSurfaceVariant,
                      ),
                    ),
                    const SizedBox(height: 2),
                    SelectableText(
                      mrPreview,
                      style: theme.textTheme.bodyMedium,
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
