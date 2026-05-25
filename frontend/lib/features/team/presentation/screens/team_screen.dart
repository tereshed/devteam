import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/core/l10n/require.dart';
import 'package:frontend/core/widgets/data_load_error_message.dart';
import 'package:frontend/features/onboarding/presentation/widgets/project_onboarding_banner.dart';
import 'package:frontend/features/projects/domain/models.dart';
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/team/domain/models/team_type_model.dart';
import 'package:frontend/features/team/presentation/widgets/agent_card.dart';
import 'package:frontend/features/team/presentation/widgets/agent_edit_dialog.dart';

/// Вкладка «Команда»: состав без второго [Scaffold] (13.1).
class TeamScreen extends ConsumerStatefulWidget {
  const TeamScreen({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<TeamScreen> createState() => _TeamScreenState();
}

class _TeamScreenState extends ConsumerState<TeamScreen> {
  String? _selectedTeamId;

  String _translateTeamType(BuildContext context, String type, List<TeamTypeModel> types) {
    final isRu = Localizations.localeOf(context).languageCode == 'ru';
    switch (type) {
      case 'development':
        return isRu ? 'Разработка' : 'Development';
      case 'research':
        return isRu ? 'Исследования' : 'Research';
      case 'analytics':
        return isRu ? 'Аналитика' : 'Analytics';
      case 'marketing':
        return isRu ? 'Маркетинг' : 'Marketing';
      case 'smm':
        return isRu ? 'SMM' : 'SMM';
      case 'rd':
        return isRu ? 'R&D' : 'R&D';
      case 'hr':
        return isRu ? 'HR' : 'HR';
      case 'legal':
        return isRu ? 'Юристы' : 'Legal';
      case 'other':
        return isRu ? 'Другое' : 'Other';
      default:
        for (final t in types) {
          if (t.code == type) {
            return t.name;
          }
        }
        return type;
    }
  }

  Future<void> _showAddTeamDialog(BuildContext context, String projectId) async {
    final nameController = TextEditingController();
    String selectedType = 'research';
    final isRu = Localizations.localeOf(context).languageCode == 'ru';

    final result = await showDialog<TeamModel?>(
      context: context,
      builder: (ctx) {
        return Consumer(
          builder: (context, ref, child) {
            final asyncTeamTypes = ref.watch(teamTypesProvider);
            return asyncTeamTypes.when(
              loading: () => AlertDialog(
                content: const SizedBox(
                  height: 100,
                  child: Center(child: CircularProgressIndicator()),
                ),
              ),
              error: (err, stack) => AlertDialog(
                title: Text(isRu ? 'Ошибка' : 'Error'),
                content: Text(
                  isRu
                      ? 'Не удалось загрузить типы команд: $err'
                      : 'Failed to load team types: $err',
                ),
                actions: [
                  TextButton(
                    onPressed: () => Navigator.of(ctx).pop(),
                    child: Text(isRu ? 'Закрыть' : 'Close'),
                  ),
                ],
              ),
              data: (typesList) {
                final filteredTypes = typesList.where((t) => t.code != 'development').toList();

                return StatefulBuilder(
                  builder: (context, setStateDialog) {
                    if (filteredTypes.isNotEmpty && !filteredTypes.any((t) => t.code == selectedType)) {
                      selectedType = filteredTypes.first.code;
                    }

                    return AlertDialog(
                      title: Text(isRu ? 'Создать команду' : 'Create Team'),
                      content: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          TextField(
                            controller: nameController,
                            decoration: InputDecoration(
                              labelText: isRu ? 'Название команды' : 'Team Name',
                              hintText: isRu ? 'Например, Команда исследований' : 'e.g. Research Team',
                              border: const OutlineInputBorder(),
                            ),
                          ),
                          const SizedBox(height: 16),
                          if (filteredTypes.isEmpty)
                            Text(
                              isRu
                                  ? 'Нет доступных дополнительных типов команд.'
                                  : 'No additional team types available.',
                              style: const TextStyle(color: Colors.red),
                            )
                          else
                            DropdownButtonFormField<String>(
                              value: selectedType,
                              decoration: InputDecoration(
                                labelText: isRu ? 'Тип команды' : 'Team Type',
                                border: const OutlineInputBorder(),
                              ),
                              items: filteredTypes.map((t) {
                                final displayName = _translateTeamType(context, t.code, filteredTypes);
                                return DropdownMenuItem(
                                  value: t.code,
                                  child: Text(displayName),
                                );
                              }).toList(),
                              onChanged: (val) {
                                if (val != null) {
                                  setStateDialog(() {
                                    selectedType = val;
                                  });
                                }
                              },
                            ),
                        ],
                      ),
                      actions: [
                        TextButton(
                          onPressed: () => Navigator.of(ctx).pop(),
                          child: Text(isRu ? 'Отмена' : 'Cancel'),
                        ),
                        FilledButton(
                          onPressed: filteredTypes.isEmpty
                              ? null
                              : () async {
                                  final name = nameController.text.trim();
                                  if (name.isEmpty) return;
                                  try {
                                    final newTeam = await ref.read(teamRepositoryProvider).createTeam(
                                          projectId,
                                          name: name,
                                          type: selectedType,
                                        );
                                    ref.invalidate(teamsProvider(projectId));
                                    if (ctx.mounted) {
                                      Navigator.of(ctx).pop(newTeam);
                                    }
                                  } catch (e) {
                                    if (ctx.mounted) {
                                      ScaffoldMessenger.of(ctx).showSnackBar(
                                        SnackBar(
                                          content: Text(
                                            isRu
                                                ? 'Не удалось создать команду: $e'
                                                : 'Failed to create team: $e',
                                          ),
                                        ),
                                      );
                                    }
                                  }
                                },
                          child: Text(isRu ? 'Создать' : 'Create'),
                        ),
                      ],
                    );
                  },
                );
              },
            );
          },
        );
      },
    );

    if (result != null && mounted) {
      setState(() {
        _selectedTeamId = result.id;
      });
    }
  }

  Future<void> _showDeleteConfirmDialog(BuildContext context, TeamModel team) async {
    final isRu = Localizations.localeOf(context).languageCode == 'ru';
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text(isRu ? 'Удалить команду?' : 'Delete Team?'),
        content: Text(
          isRu
              ? 'Это действие удалит команду «${team.name}», всех её агентов и связанные секреты. Восстановить эти данные будет невозможно.'
              : 'This action will delete the team "${team.name}", all of its agents, and associated secrets. This cannot be undone.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: Text(isRu ? 'Отмена' : 'Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            style: FilledButton.styleFrom(
              backgroundColor: Theme.of(context).colorScheme.error,
              foregroundColor: Theme.of(context).colorScheme.onError,
            ),
            child: Text(isRu ? 'Удалить' : 'Delete'),
          ),
        ],
      ),
    );

    if (confirmed == true && mounted) {
      try {
        await ref.read(teamRepositoryProvider).deleteTeam(widget.projectId, team.id);
        ref.invalidate(teamsProvider(widget.projectId));
        setState(() {
          _selectedTeamId = null;
        });
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(
                isRu ? 'Команда успешно удалена' : 'Team deleted successfully',
              ),
            ),
          );
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(
                isRu ? 'Не удалось удалить команду: $e' : 'Failed to delete team: $e',
              ),
            ),
          );
        }
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    assert(widget.projectId.isNotEmpty);
    final l10n = requireAppLocalizations(context, where: 'teamScreen');
    final asyncTeams = ref.watch(teamsProvider(widget.projectId));
    final asyncTeamTypes = ref.watch(teamTypesProvider);
    final teamTypesList = asyncTeamTypes.hasValue ? asyncTeamTypes.requireValue : const <TeamTypeModel>[];
    final isRu = Localizations.localeOf(context).languageCode == 'ru';

    if (asyncTeams.hasError) {
      return DataLoadErrorMessage(
        title: l10n.dataLoadError,
        actionLabel: l10n.retry,
        onAction: () => ref.invalidate(teamsProvider(widget.projectId)),
      );
    }

    if (asyncTeams.isLoading || !asyncTeams.hasValue) {
      return const Center(child: CircularProgressIndicator());
    }

    final teams = List<TeamModel>.from(asyncTeams.requireValue);
    // Сортируем: development всегда первая
    teams.sort((a, b) {
      if (a.type == 'development') return -1;
      if (b.type == 'development') return 1;
      return a.createdAt.compareTo(b.createdAt);
    });

    if (teams.isEmpty) {
      return Center(
        child: Text(
          isRu ? 'Команды не найдены' : 'No teams found',
          style: Theme.of(context).textTheme.bodyLarge,
        ),
      );
    }

    // Если выбранная команда не найдена в списке, сбрасываем на первую (development)
    if (_selectedTeamId == null || !teams.any((t) => t.id == _selectedTeamId)) {
      _selectedTeamId = teams.first.id;
    }

    final activeTeam = teams.firstWhere((t) => t.id == _selectedTeamId, orElse: () => teams.first);

    Future<void> onRefresh() async {
      ref.invalidate(teamsProvider(widget.projectId));
      ref.invalidate(teamTypesProvider);
      try {
        await Future.wait([
          ref.read(teamsProvider(widget.projectId).future),
          ref.read(teamTypesProvider.future),
        ]);
      } on Exception {
        // Ошибка уже в asyncTeams или asyncTeamTypes
      }
    }

    final agents = activeTeam.agents;
    final itemCount = agents.isEmpty ? 1 : agents.length;
    final theme = Theme.of(context);
    final scheme = theme.colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        // Премиальная панель выбора команды с размытием / отступами
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
          child: Row(
            children: [
              Expanded(
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: Row(
                    children: teams.map((t) {
                      final isSelected = t.id == activeTeam.id;
                      IconData icon;
                      if (t.type == 'development') {
                        icon = Icons.developer_mode;
                      } else if (t.type == 'research') {
                        icon = Icons.science;
                      } else if (t.type == 'analytics') {
                        icon = Icons.analytics;
                      } else if (t.type == 'marketing') {
                        icon = Icons.campaign;
                      } else if (t.type == 'smm') {
                        icon = Icons.share;
                      } else if (t.type == 'rd') {
                        icon = Icons.biotech;
                      } else if (t.type == 'hr') {
                        icon = Icons.people;
                      } else if (t.type == 'legal') {
                        icon = Icons.gavel;
                      } else if (t.type == 'other') {
                        icon = Icons.category;
                      } else {
                        icon = Icons.group;
                      }

                      return Padding(
                        padding: const EdgeInsets.only(right: 8),
                        child: ChoiceChip(
                          avatar: Icon(
                            icon,
                            size: 16,
                            color: isSelected ? scheme.onPrimaryContainer : scheme.onSurfaceVariant,
                          ),
                          label: Text(_translateTeamType(context, t.type, teamTypesList)),
                          selected: isSelected,
                          onSelected: (selected) {
                            if (selected) {
                              setState(() {
                                _selectedTeamId = t.id;
                              });
                            }
                          },
                        ),
                      );
                    }).toList(),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              IconButton.filledTonal(
                onPressed: () => _showAddTeamDialog(context, widget.projectId),
                icon: const Icon(Icons.add),
                tooltip: isRu ? 'Создать команду' : 'Create Team',
              ),
            ],
          ),
        ),

        // Карточка деталей выбранной команды
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Card(
            elevation: 0,
            shape: RoundedRectangleBorder(
              side: BorderSide(color: theme.dividerColor.withOpacity(0.08)),
              borderRadius: BorderRadius.circular(16),
            ),
            color: scheme.surfaceContainerLow,
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          activeTeam.name,
                          style: theme.textTheme.titleLarge?.copyWith(
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        const SizedBox(height: 4),
                        Text(
                          _translateTeamType(context, activeTeam.type, teamTypesList),
                          style: theme.textTheme.bodyMedium?.copyWith(
                            color: scheme.onSurfaceVariant,
                          ),
                        ),
                      ],
                    ),
                  ),
                  if (activeTeam.type != 'development')
                    IconButton(
                      icon: Icon(Icons.delete_outline, color: scheme.error),
                      tooltip: isRu ? 'Удалить команду' : 'Delete Team',
                      onPressed: () => _showDeleteConfirmDialog(context, activeTeam),
                    ),
                ],
              ),
            ),
          ),
        ),

        ProjectOnboardingBanner(projectId: widget.projectId),
        
        Expanded(
          child: RefreshIndicator(
            onRefresh: onRefresh,
            child: ListView.builder(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
              itemCount: itemCount,
              itemBuilder: (context, index) {
                if (agents.isEmpty) {
                  return Padding(
                    padding: const EdgeInsets.symmetric(vertical: 24),
                    child: Text(
                      l10n.teamEmptyAgents,
                      style: theme.textTheme.bodyLarge,
                      textAlign: TextAlign.center,
                    ),
                  );
                }
                final agent = agents[index];
                return Padding(
                  padding: const EdgeInsets.only(bottom: 12),
                  child: AgentCard(
                    key: ValueKey(agent.id),
                    agent: agent,
                    onTap: () => showAgentEditDialog(
                      context,
                      projectId: widget.projectId,
                      agent: agent,
                    ),
                  ),
                );
              },
            ),
          ),
        ),
      ],
    );
  }
}
