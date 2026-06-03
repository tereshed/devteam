import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/features/webhooks/domain/models/webhook_model.dart';
import 'package:frontend/l10n/app_localizations.dart';
import 'package:frontend/features/team/data/team_providers.dart';
enum RouteTo { project, team }

class WebhookEditDialog extends ConsumerStatefulWidget {
  final String? projectId;
  final WebhookModel? webhook;

  const WebhookEditDialog({
    super.key,
    this.projectId,
    this.webhook,
  });

  @override
  ConsumerState<WebhookEditDialog> createState() => _WebhookEditDialogState();
}

class _WebhookEditDialogState extends ConsumerState<WebhookEditDialog> {
  final _formKey = GlobalKey<FormState>();
  late TextEditingController _nameCtrl;
  late TextEditingController _instructionsCtrl;
  late TextEditingController _descriptionCtrl;
  late TextEditingController _allowedIpsCtrl;
  late TextEditingController _taskTitleTplCtrl;
  late TextEditingController _taskDescTplCtrl;
  late TextEditingController _taskPrioTplCtrl;
  late bool _requireSecret;
  late bool _isActive;
  bool _regenerateSecret = false;
  
  RouteTo _routeTo = RouteTo.project;
  String? _selectedTeamId;

  @override
  void initState() {
    super.initState();
    _nameCtrl = TextEditingController(text: widget.webhook?.name ?? '');
    _instructionsCtrl = TextEditingController(text: widget.webhook?.instructions ?? '');
    _descriptionCtrl = TextEditingController(text: widget.webhook?.description ?? '');
    _allowedIpsCtrl = TextEditingController(text: widget.webhook?.allowedIps ?? '');
    _taskTitleTplCtrl = TextEditingController(text: widget.webhook?.taskTitleTemplate ?? '');
    _taskDescTplCtrl = TextEditingController(text: widget.webhook?.taskDescriptionTemplate ?? '');
    _taskPrioTplCtrl = TextEditingController(text: widget.webhook?.taskPriorityTemplate ?? '');
    _requireSecret = widget.webhook?.requireSecret ?? false;
    _isActive = widget.webhook?.isActive ?? true;

    if (widget.webhook?.teamId != null && widget.webhook!.teamId!.isNotEmpty) {
      _routeTo = RouteTo.team;
      _selectedTeamId = widget.webhook!.teamId;
    }
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _instructionsCtrl.dispose();
    _descriptionCtrl.dispose();
    _allowedIpsCtrl.dispose();
    _taskTitleTplCtrl.dispose();
    _taskDescTplCtrl.dispose();
    _taskPrioTplCtrl.dispose();
    super.dispose();
  }

  void _submit() {
    if (!_formKey.currentState!.validate()) {
      return;
    }
    
    if (widget.webhook == null) {
      final req = CreateWebhookRequest(
        name: _nameCtrl.text.trim(),
        projectId: widget.projectId,
        teamId: _routeTo == RouteTo.team ? _selectedTeamId : null,
        instructions: _instructionsCtrl.text.trim(),
        description: _descriptionCtrl.text.trim(),
        allowedIps: _allowedIpsCtrl.text.trim(),
        requireSecret: _requireSecret,
        taskTitleTemplate: _taskTitleTplCtrl.text.trim(),
        taskDescriptionTemplate: _taskDescTplCtrl.text.trim(),
        taskPriorityTemplate: _taskPrioTplCtrl.text.trim(),
      );
      Navigator.of(context).pop(req);
    } else {
      final req = UpdateWebhookRequest(
        projectId: widget.projectId ?? '',
        teamId: _routeTo == RouteTo.team ? _selectedTeamId : null,
        instructions: _instructionsCtrl.text.trim(),
        description: _descriptionCtrl.text.trim(),
        allowedIps: _allowedIpsCtrl.text.trim(),
        requireSecret: _requireSecret,
        isActive: _isActive,
        regenerateSecret: _regenerateSecret,
        taskTitleTemplate: _taskTitleTplCtrl.text.trim(),
        taskDescriptionTemplate: _taskDescTplCtrl.text.trim(),
        taskPriorityTemplate: _taskPrioTplCtrl.text.trim(),
      );
      Navigator.of(context).pop(req);
    }
  }

  @override
  Widget build(BuildContext context) {
    final l10n = AppLocalizations.of(context)!;
    final isEditing = widget.webhook != null;

    return Dialog(
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 600),
        child: Padding(
          padding: EdgeInsets.all(Spacing.large(context)),
          child: Form(
            key: _formKey,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    isEditing ? l10n.webhookEdit : l10n.webhookCreate,
                    style: Theme.of(context).textTheme.titleLarge,
                  ),
                  SizedBox(height: Spacing.large(context)),
                  TextFormField(
                    controller: _nameCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.webhookName,
                      hintText: l10n.webhookNameHint,
                    ),
                    enabled: !isEditing,
                    validator: (v) => v == null || v.trim().isEmpty ? l10n.webhookRequiredName : null,
                  ),
                  SizedBox(height: Spacing.medium(context)),
                  if (widget.projectId != null) ...[
                    Text(
                      l10n.webhookRouteTo,
                      style: Theme.of(context).textTheme.titleSmall,
                    ),
                    Row(
                      children: [
                        Expanded(
                          child: RadioListTile<RouteTo>(
                            title: Text(l10n.webhookRouteProject),
                            value: RouteTo.project,
                            groupValue: _routeTo,
                            onChanged: (v) => setState(() => _routeTo = v!),
                          ),
                        ),
                        Expanded(
                          child: RadioListTile<RouteTo>(
                            title: Text(l10n.webhookRouteTeam),
                            value: RouteTo.team,
                            groupValue: _routeTo,
                            onChanged: (v) => setState(() => _routeTo = v!),
                          ),
                        ),
                      ],
                    ),
                    if (_routeTo == RouteTo.team) ...[
                      SizedBox(height: Spacing.small(context)),
                      ref.watch(teamsProvider(widget.projectId!)).when(
                        data: (teamsList) {
                          if (teamsList.isEmpty) {
                            return const Text('No teams available');
                          }
                          return DropdownButtonFormField<String>(
                            decoration: InputDecoration(
                              labelText: l10n.webhookSelectTeam,
                            ),
                            value: _selectedTeamId != null && teamsList.any((t) => t.id == _selectedTeamId) ? _selectedTeamId : null,
                            items: teamsList.map((t) => DropdownMenuItem(
                              value: t.id,
                              child: Text(t.name),
                            )).toList(),
                            onChanged: (v) => setState(() => _selectedTeamId = v),
                            validator: (v) => v == null ? l10n.webhookSelectTeam : null,
                          );
                        },
                        loading: () => const Center(child: CircularProgressIndicator()),
                        error: (err, stack) => Text('Error loading teams: $err'),
                      ),
                      SizedBox(height: Spacing.medium(context)),
                      ExpansionTile(
                        title: Text(l10n.webhookTaskMappingTitle),
                        childrenPadding: const EdgeInsets.only(bottom: 16),
                        children: [
                          TextFormField(
                            controller: _taskTitleTplCtrl,
                            decoration: InputDecoration(
                              labelText: l10n.webhookTaskTitleTemplate,
                              hintText: l10n.webhookTaskTitleTemplateHint,
                            ),
                          ),
                          SizedBox(height: Spacing.small(context)),
                          TextFormField(
                            controller: _taskDescTplCtrl,
                            maxLines: 3,
                            decoration: InputDecoration(
                              labelText: l10n.webhookTaskDescTemplate,
                              hintText: l10n.webhookTaskDescTemplateHint,
                            ),
                          ),
                          SizedBox(height: Spacing.small(context)),
                          TextFormField(
                            controller: _taskPrioTplCtrl,
                            decoration: InputDecoration(
                              labelText: l10n.webhookTaskPriorityTemplate,
                              hintText: l10n.webhookTaskPriorityTemplateHint,
                            ),
                          ),
                        ],
                      ),
                    ],
                  ],
                  SizedBox(height: Spacing.medium(context)),
                  TextFormField(
                    controller: _instructionsCtrl,
                    maxLines: 4,
                    decoration: InputDecoration(
                      labelText: l10n.webhookInstructions,
                      hintText: l10n.webhookInstructionsHint,
                      alignLabelWithHint: true,
                    ),
                  ),
                  SizedBox(height: Spacing.medium(context)),
                  TextFormField(
                    controller: _descriptionCtrl,
                    decoration: InputDecoration(
                      labelText: l10n.webhookDescription,
                      hintText: l10n.webhookDescriptionHint,
                    ),
                  ),
                  SizedBox(height: Spacing.medium(context)),
                  ExpansionTile(
                    title: Text('Advanced'),
                    childrenPadding: const EdgeInsets.only(bottom: 16),
                    children: [
                      TextFormField(
                        controller: _allowedIpsCtrl,
                        decoration: InputDecoration(
                          labelText: l10n.webhookAllowedIps,
                        ),
                      ),
                      SwitchListTile(
                        title: Text(l10n.webhookRequireSecret),
                        value: _requireSecret,
                        onChanged: (v) => setState(() => _requireSecret = v),
                      ),
                      if (isEditing) ...[
                        SwitchListTile(
                          title: Text(l10n.webhookIsActive),
                          value: _isActive,
                          onChanged: (v) => setState(() => _isActive = v),
                        ),
                        SwitchListTile(
                          title: Text(l10n.webhookRegenerateSecret),
                          value: _regenerateSecret,
                          onChanged: (v) => setState(() => _regenerateSecret = v),
                        ),
                      ],
                    ],
                  ),
                  SizedBox(height: Spacing.large(context)),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.end,
                    children: [
                      TextButton(
                        onPressed: () => Navigator.of(context).pop(),
                        child: Text(MaterialLocalizations.of(context).cancelButtonLabel),
                      ),
                      const SizedBox(width: 8),
                      FilledButton(
                        onPressed: _submit,
                        child: Text(l10n.webhookSave),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
