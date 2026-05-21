import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/utils/responsive.dart';
import 'package:frontend/core/widgets/adaptive_layout.dart';
import 'package:frontend/features/admin/agents_v2/data/agents_v2_providers.dart';
import 'package:frontend/features/admin/agents_v2/data/mcp_registry_providers.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/team/data/agent_settings_providers.dart';
import 'package:frontend/features/team/domain/models/agent_config_model.dart';
import 'package:frontend/features/team/domain/models/agent_settings_model.dart';

class AgentConfigScreen extends ConsumerStatefulWidget {
  const AgentConfigScreen({super.key, required this.agentId});
  final String agentId;

  @override
  ConsumerState<AgentConfigScreen> createState() => _AgentConfigScreenState();
}

class _AgentConfigScreenState extends ConsumerState<AgentConfigScreen> {
  late String _role;
  late String _executionKind;
  String? _providerKind;
  String? _model;
  double? _temperature;
  bool _internalMcpEnabled = false;
  bool _isActive = true;
  bool _isDirty = false;
  bool _isSaving = false;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AgentConfigScreen');
    final agentAsync = ref.watch(agentV2DetailProvider(widget.agentId));
    final settingsAsync = ref.watch(agentSettingsProvider(widget.agentId));

    return Scaffold(
      appBar: AppBar(
        title: Text(l10n.agentConfigScreenTitle),
        actions: [
          if (_isDirty)
            TextButton.icon(
              onPressed: _isSaving ? null : _save,
              icon: _isSaving
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.save),
              label: Text(l10n.agentConfigSaveButton),
            ),
        ],
      ),
      body: agentAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(child: Text('${l10n.agentConfigLoadError}: $err')),
        data: (agent) {
          return settingsAsync.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (err, _) => Center(child: Text('${l10n.agentConfigLoadError}: $err')),
            data: (settings) {
              return _buildBody(context, agent, settings);
            },
          );
        },
      ),
    );
  }

  bool _initialized = false;

  void _initFromAgent(AgentV2 agent) {
    if (_initialized) return;
    _initialized = true;
    _role = agent.role.isEmpty ? 'developer' : agent.role;
    _executionKind = agent.executionKind;
    _providerKind = agent.providerKind;
    _model = agent.model;
    _temperature = agent.temperature;
    _internalMcpEnabled = agent.internalMcpEnabled;
    _isActive = agent.isActive;
  }

  Widget _buildBody(BuildContext context, AgentV2 agent, AgentSettingsModel settings) {
    _initFromAgent(agent);
    final l10n = requireAppLocalizations(context, where: 'AgentConfigScreen');
    final mcpRegistryAsync = ref.watch(mcpRegistryListProvider);
    final isAutoCreated = kAutoCreatedRoles.contains(agent.role);

    return SafeArea(
      child: AdaptiveContainer(
        child: SingleChildScrollView(
          padding: Spacing.cardPadding(context),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              // Agent name header
              _AgentNameHeader(name: agent.name, role: _role),
              SizedBox(height: Spacing.medium(context)),

              // Active toggle
              SwitchListTile(
                title: Text(l10n.agentConfigActiveLabel),
                subtitle: Text(
                  _isActive ? l10n.agentConfigActiveOn : l10n.agentConfigActiveOff,
                ),
                value: _isActive,
                onChanged: (v) => setState(() {
                  _isActive = v;
                  _isDirty = true;
                }),
              ),
              const Divider(),

              // Section: Role
              _SectionHeader(title: l10n.agentConfigRoleSectionTitle),
              SizedBox(height: Spacing.small(context)),
              _RoleSection(
                role: _role,
                isAutoCreated: isAutoCreated,
                onChanged: isAutoCreated
                    ? null
                    : (v) => setState(() {
                          _role = v;
                          _isDirty = true;
                        }),
              ),
              SizedBox(height: Spacing.medium(context)),
              const Divider(),

              // Section: Type (read-only — changing execution kind requires re-creating the agent)
              _SectionHeader(title: l10n.agentConfigTypeSectionTitle),
              SizedBox(height: Spacing.small(context)),
              _TypeSection(executionKind: _executionKind),
              SizedBox(height: Spacing.medium(context)),
              const Divider(),

              // Section: LLM Settings (only for llm type)
              if (_executionKind == 'llm') ...[
                _SectionHeader(title: l10n.agentConfigLLMSectionTitle),
                SizedBox(height: Spacing.small(context)),
                _LLMSettingsSection(
                  providerKind: _providerKind,
                  model: _model,
                  temperature: _temperature,
                  onProviderChanged: (v) => setState(() {
                    _providerKind = v;
                    _isDirty = true;
                  }),
                  onModelChanged: (v) => setState(() {
                    _model = v;
                    _isDirty = true;
                  }),
                  onTemperatureChanged: (v) => setState(() {
                    _temperature = v;
                    _isDirty = true;
                  }),
                ),
                SizedBox(height: Spacing.medium(context)),
                const Divider(),
              ],

              // Section: MCP Tools
              _SectionHeader(title: l10n.agentConfigMCPSectionTitle),
              SizedBox(height: Spacing.small(context)),
              _MCPToolsSection(
                internalMcpEnabled: _internalMcpEnabled,
                onInternalMcpChanged: (v) => setState(() {
                  _internalMcpEnabled = v;
                  _isDirty = true;
                }),
                settings: settings,
                mcpRegistryAsync: mcpRegistryAsync,
              ),
              SizedBox(height: Spacing.medium(context)),
              const Divider(),

              // Section: Skills
              _SectionHeader(title: l10n.agentConfigSkillsSectionTitle),
              SizedBox(height: Spacing.small(context)),
              _SkillsSection(settings: settings),
              SizedBox(height: Spacing.large(context)),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _save() async {
    setState(() => _isSaving = true);
    try {
      final agentsRepo = ref.read(agentsV2RepositoryProvider);
      await agentsRepo.update(
        id: widget.agentId,
        role: _role,
        model: _model,
        providerKind: _providerKind,
        temperature: _temperature,
        isActive: _isActive,
        internalMcpEnabled: _internalMcpEnabled,
      );
      // Invalidate providers
      ref.invalidate(agentV2DetailProvider(widget.agentId));
      ref.invalidate(agentSettingsProvider(widget.agentId));
      setState(() {
        _isDirty = false;
        _isSaving = false;
      });
      if (mounted) {
        final l10n = requireAppLocalizations(context, where: 'AgentConfigScreen');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(l10n.agentConfigSaveSuccess)),
        );
      }
    } catch (e) {
      setState(() => _isSaving = false);
      if (mounted) {
        final l10n = requireAppLocalizations(context, where: 'AgentConfigScreen');
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${l10n.agentConfigSaveError}: $e')),
        );
      }
    }
  }
}

class _AgentNameHeader extends StatelessWidget {
  const _AgentNameHeader({required this.name, required this.role});
  final String name;
  final String role;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        CircleAvatar(
          backgroundColor: theme.colorScheme.primaryContainer,
          child: Icon(Icons.smart_toy, color: theme.colorScheme.onPrimaryContainer),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(name, style: theme.textTheme.titleLarge),
              Text(role, style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurface.withValues(alpha: 0.6),
              )),
            ],
          ),
        ),
      ],
    );
  }
}

class _SectionHeader extends StatelessWidget {
  const _SectionHeader({required this.title});
  final String title;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Text(title, style: theme.textTheme.titleMedium);
  }
}

class _RoleSection extends StatelessWidget {
  const _RoleSection({
    required this.role,
    required this.isAutoCreated,
    required this.onChanged,
  });
  final String role;
  final bool isAutoCreated;
  final ValueChanged<String>? onChanged;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AgentRoleSection');
    if (isAutoCreated) {
      return InputDecorator(
        decoration: InputDecoration(
          labelText: l10n.agentConfigRoleLabel,
          helperText: l10n.agentConfigRoleReadOnly,
          border: const OutlineInputBorder(),
        ),
        child: Text(role),
      );
    }
    return DropdownButtonFormField<String>(
      value: kAgentRoles.contains(role) ? role : null,
      decoration: InputDecoration(
        labelText: l10n.agentConfigRoleLabel,
        border: const OutlineInputBorder(),
      ),
      items: kAgentRoles
          .where((r) => !kAutoCreatedRoles.contains(r))
          .map((r) => DropdownMenuItem(value: r, child: Text(r)))
          .toList(),
      onChanged: (v) {
        if (v != null) onChanged?.call(v);
      },
    );
  }
}

class _TypeSection extends StatelessWidget {
  const _TypeSection({required this.executionKind});
  final String executionKind;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AgentTypeSection');
    return SegmentedButton<String>(
      segments: [
        ButtonSegment(
          value: 'llm',
          label: Text(l10n.agentConfigTypeAPI),
          icon: const Icon(Icons.cloud_outlined),
        ),
        ButtonSegment(
          value: 'sandbox',
          label: Text(l10n.agentConfigTypeSandbox),
          icon: const Icon(Icons.terminal),
        ),
      ],
      selected: {executionKind},
      onSelectionChanged: (_) {},
    );
  }
}

class _LLMSettingsSection extends StatelessWidget {
  const _LLMSettingsSection({
    required this.providerKind,
    required this.model,
    required this.temperature,
    required this.onProviderChanged,
    required this.onModelChanged,
    required this.onTemperatureChanged,
  });
  final String? providerKind;
  final String? model;
  final double? temperature;
  final ValueChanged<String?> onProviderChanged;
  final ValueChanged<String?> onModelChanged;
  final ValueChanged<double?> onTemperatureChanged;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AgentLLMSettingsSection');
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        DropdownButtonFormField<String>(
          value: providerKind,
          decoration: InputDecoration(
            labelText: l10n.agentConfigProviderLabel,
            border: const OutlineInputBorder(),
          ),
          items: kSupportedAgentProviderKinds
              .map((p) => DropdownMenuItem(value: p, child: Text(p)))
              .toList(),
          onChanged: onProviderChanged,
        ),
        const SizedBox(height: 16),
        TextFormField(
          initialValue: model ?? '',
          decoration: InputDecoration(
            labelText: l10n.agentConfigModelLabel,
            hintText: l10n.agentConfigModelHint,
            border: const OutlineInputBorder(),
          ),
          onChanged: (v) => onModelChanged(v.isEmpty ? null : v),
        ),
        const SizedBox(height: 16),
        Row(
          children: [
            Expanded(
              child: Text(
                '${l10n.agentConfigTemperatureLabel}: ${temperature?.toStringAsFixed(1) ?? l10n.agentConfigTemperatureDefault}',
              ),
            ),
            const SizedBox(width: 8),
            SizedBox(
              width: 48,
              child: Text(
                temperature?.toStringAsFixed(1) ?? '-',
                textAlign: TextAlign.end,
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ),
          ],
        ),
        Slider(
          value: temperature ?? 0.7,
          min: 0.0,
          max: 2.0,
          divisions: 20,
          label: (temperature ?? 0.7).toStringAsFixed(1),
          onChanged: (v) => onTemperatureChanged(v),
        ),
      ],
    );
  }
}

class _MCPToolsSection extends StatelessWidget {
  const _MCPToolsSection({
    required this.internalMcpEnabled,
    required this.onInternalMcpChanged,
    required this.settings,
    required this.mcpRegistryAsync,
  });
  final bool internalMcpEnabled;
  final ValueChanged<bool> onInternalMcpChanged;
  final AgentSettingsModel settings;
  final AsyncValue<List<MCPServerRegistryModel>> mcpRegistryAsync;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AgentMCPToolsSection');
    final mcpServers = settings.codeBackendSettings['mcp_servers'] as List<dynamic>? ?? [];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        SwitchListTile(
          title: Text(l10n.agentConfigDevTeamMCP),
          subtitle: Text(l10n.agentConfigDevTeamMCPDesc),
          value: internalMcpEnabled,
          onChanged: onInternalMcpChanged,
          secondary: Icon(
            Icons.hub_outlined,
            color: Theme.of(context).colorScheme.primary,
          ),
        ),
        const Divider(height: 1),
        Padding(
          padding: const EdgeInsets.symmetric(vertical: 8.0),
          child: Text(
            l10n.agentConfigExternalMCPTitle,
            style: Theme.of(context).textTheme.titleSmall,
          ),
        ),
        if (mcpServers.isEmpty)
          Padding(
            padding: const EdgeInsets.symmetric(vertical: 8.0),
            child: Text(
              l10n.agentConfigNoExternalMCP,
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Theme.of(context).colorScheme.onSurface.withValues(alpha: 0.6),
              ),
            ),
          )
        else
          ...mcpServers.map((server) {
            final name = (server as Map<String, dynamic>)['name'] ?? '';
            return ListTile(
              leading: const Icon(Icons.extension_outlined),
              title: Text(name.toString()),
              dense: true,
            );
          }),
        const SizedBox(height: 8),
        mcpRegistryAsync.when(
          loading: () => const SizedBox.shrink(),
          error: (_, __) => const SizedBox.shrink(),
          data: (registry) => OutlinedButton.icon(
            onPressed: () {
              // TODO: show MCP server picker dialog
            },
            icon: const Icon(Icons.add),
            label: Text(l10n.agentConfigAddMCPServer),
          ),
        ),
      ],
    );
  }
}

class _SkillsSection extends StatelessWidget {
  const _SkillsSection({required this.settings});
  final AgentSettingsModel settings;

  @override
  Widget build(BuildContext context) {
    final l10n = requireAppLocalizations(context, where: 'AgentSkillsSection');
    final skills = settings.codeBackendSettings['skills'] as List<dynamic>? ?? [];

    if (skills.isEmpty) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: 8.0),
        child: Text(
          l10n.agentConfigNoSkills,
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurface.withValues(alpha: 0.6),
          ),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        ...skills.map((skill) {
          final name = (skill as Map<String, dynamic>)['name'] ?? '';
          final source = skill['source'] ?? 'builtin';
          return ListTile(
            leading: Icon(
              source == 'builtin' ? Icons.extension : Icons.folder_outlined,
            ),
            title: Text(name.toString()),
            subtitle: Text(source.toString()),
            trailing: Switch(
              value: skill['is_active'] ?? true,
              onChanged: (_) {
                // TODO: toggle skill active state
              },
            ),
            dense: true,
          );
        }),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: () {
            // TODO: show skill picker dialog
          },
          icon: const Icon(Icons.add),
          label: Text(l10n.agentConfigAddSkill),
        ),
      ],
    );
  }
}
