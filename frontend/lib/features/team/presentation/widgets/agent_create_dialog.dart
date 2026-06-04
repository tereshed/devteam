import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/domain/agent_model_suggestions.dart';
import 'package:frontend/features/projects/domain/models/agent_model.dart'
    show codeBackends;
import 'package:frontend/features/team/data/team_providers.dart';
import 'package:frontend/features/team/domain/agent_provider_rules.dart';

class AgentCreateDialog extends ConsumerStatefulWidget {
  const AgentCreateDialog({
    required this.projectId,
    required this.teamId,
    super.key,
  });

  final String projectId;
  final String teamId;

  static Future<void> show(BuildContext context, String projectId, String teamId) {
    return showDialog(
      context: context,
      builder: (_) => AgentCreateDialog(projectId: projectId, teamId: teamId),
    );
  }

  @override
  ConsumerState<AgentCreateDialog> createState() => _AgentCreateDialogState();
}

class _AgentCreateDialogState extends ConsumerState<AgentCreateDialog> {
  final _nameController = TextEditingController();
  final _roleDescController = TextEditingController();
  // SearchController (а не TextEditingController) — нужен для SearchAnchor с
  // динамическим каталогом моделей, как в agent_edit_dialog.
  final SearchController _modelController = SearchController();
  final _customRoleController = TextEditingController();
  final _systemPromptController = TextEditingController();
  String _executionKind = 'llm';
  String _role = 'developer';

  // Спец-значение пункта «Своя роль…» в выпадашке роли.
  static const String _customRoleOption = '__custom__';
  // Кастомная роль обязана быть snake_case (зеркалит roleNameRE на бэке).
  static final RegExp _customRoleRe = RegExp(r'^[a-z][a-z0-9_]*$');

  bool get _isCustomRole => _role == _customRoleOption;
  String _effectiveRole() =>
      _isCustomRole ? _customRoleController.text.trim() : _role;
  String _codeBackend = 'claude-code'; // нужен только для execution_kind=sandbox
  String? _providerKind; // provider_kind агента; общий для llm и sandbox
  bool _isLoading = false;

  // Примеры описания роли по типу агента. Это описание попадает в промпт Router'а:
  // именно по нему оркестратор решает, какому агенту отдать задачу. Пустое описание —
  // агент не виден Router'у и задача зависает, поэтому показываем образец под выбранную роль.
  static const Map<String, String> _roleExamplesRu = {
    'developer':
        'Пишет и меняет код в песочнице: реализует фичи, чинит баги, создаёт ветку и коммиты.',
    'tester':
        'Пишет и запускает тесты, проверяет, что изменения работают и ничего не сломано.',
    'reviewer':
        'Проверяет код на ошибки и качество, оставляет замечания, одобряет или отклоняет изменения.',
    'planner':
        'Разбивает задачу на конкретные шаги и составляет план выполнения для остальных агентов.',
  };
  static const Map<String, String> _roleExamplesEn = {
    'developer':
        'Writes and edits code in the sandbox: implements features, fixes bugs, creates a branch and commits.',
    'tester':
        'Writes and runs tests, verifies the changes work and nothing is broken.',
    'reviewer':
        'Reviews code for bugs and quality, leaves comments, approves or rejects changes.',
    'planner':
        'Breaks the task into concrete steps and produces an execution plan for the other agents.',
  };

  String _roleExample(bool isRu) =>
      (isRu ? _roleExamplesRu : _roleExamplesEn)[_role] ?? '';

  @override
  void initState() {
    super.initState();
    // Подтягиваем подключённые LLM-провайдеры, чтобы выпадашка провайдера
    // показывала реально доступные варианты (см. configuredAgentProviderKinds).
    ref.read(llmIntegrationsControllerProvider).refresh().ignore();
  }

  @override
  void dispose() {
    _nameController.dispose();
    _roleDescController.dispose();
    _modelController.dispose();
    _customRoleController.dispose();
    _systemPromptController.dispose();
    super.dispose();
  }

  // При выборе провайдера подставляем дефолтную модель из подсказок (если поле
  // пустое или не из набора подсказок), чтобы пользователь не гадал формат.
  void _applyProviderDefaults(String? providerKind) {
    _providerKind = providerKind;
    if (providerKind != null) {
      final suggestions = agentModelSuggestions(providerKind);
      if (suggestions.isNotEmpty &&
          !suggestions.contains(_modelController.text.trim())) {
        _modelController.text = suggestions.first;
      }
    }
  }

  Future<void> _submit() async {
    final isRu = Localizations.localeOf(context).languageCode == 'ru';
    final name = _nameController.text.trim();
    if (name.isEmpty) return;

    final role = _effectiveRole();
    if (role.isEmpty) return;

    // Кастомная роль: формат snake_case (как на бэке).
    if (_isCustomRole && !_customRoleRe.hasMatch(role)) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(isRu
              ? 'Имя роли — snake_case: латиница, цифры, _ (например smm_writer)'
              : 'Role name must be snake_case: a-z, 0-9, _ (e.g. smm_writer)'),
        ),
      );
      return;
    }

    // role_description обязателен: без него агент не попадает в каталог Router'а
    // и задача не берётся в работу. Не даём создать агента с пустым описанием.
    final roleDescription = _roleDescController.text.trim();
    if (roleDescription.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(isRu
              ? 'Заполните «Описание роли» — по нему Router выбирает агента'
              : 'Fill in "Role description" — Router uses it to pick the agent'),
        ),
      );
      return;
    }

    // Кастомная роль не имеет дефолтного промпта — system_prompt обязателен.
    final systemPrompt = _systemPromptController.text.trim();
    if (_isCustomRole && systemPrompt.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(isRu
              ? 'Для своей роли заполните «Системный промпт»'
              : 'A custom role requires a system prompt'),
        ),
      );
      return;
    }

    // hermes-бекенд требует явного провайдера (нет дефолтного fallback).
    if (_executionKind == 'sandbox' &&
        backendRequiresProvider(_codeBackend) &&
        (_providerKind == null || _providerKind!.isEmpty)) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(isRu
              ? 'Бекенд «$_codeBackend» требует выбрать провайдера'
              : 'Backend "$_codeBackend" requires a provider'),
        ),
      );
      return;
    }

    final model = _modelController.text.trim();

    setState(() => _isLoading = true);
    try {
      await ref.read(teamRepositoryProvider).createAgent(
            widget.projectId,
            widget.teamId,
            name: name,
            role: role,
            executionKind: _executionKind,
            roleDescription: roleDescription,
            systemPrompt: systemPrompt.isEmpty ? null : systemPrompt,
            providerKind: _providerKind,
            model: model.isEmpty ? null : model,
            codeBackend: _executionKind == 'sandbox' ? _codeBackend : null,
          );
      ref.invalidate(teamsProvider(widget.projectId));
      if (mounted) {
        Navigator.of(context).pop();
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(isRu ? 'Ошибка: $e' : 'Error: $e')),
        );
      }
    } finally {
      if (mounted) {
        setState(() => _isLoading = false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final isRu = Localizations.localeOf(context).languageCode == 'ru';

    // Подключённые провайдеры; для sandbox дополнительно ограничиваем набор
    // выбранным бекендом (см. allowedProviderKindsForBackend).
    final integrationsState =
        ref.watch(llmIntegrationsStateProvider).asData?.value;
    final configuredKinds = configuredAgentProviderKinds(
      integrationsState?.connections.values ?? const [],
    );
    final providerOptions = _executionKind == 'sandbox'
        ? configuredKinds
            .where(allowedProviderKindsForBackend(_codeBackend).contains)
            .toList()
        : configuredKinds;

    // Динамический каталог моделей выбранного провайдера (как в edit-диалоге);
    // если пуст/не загрузился — фолбэк на статические agentModelSuggestions.
    final pk = _providerKind;
    final dynamicModelsAsync = pk != null
        ? ref.watch(availableModelsProvider(pk))
        : const AsyncValue<List<String>>.data([]);

    return AlertDialog(
      title: Text(isRu ? 'Добавить агента' : 'Add Agent'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            TextField(
              controller: _nameController,
              decoration: InputDecoration(
                labelText: isRu ? 'Имя агента' : 'Agent Name',
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 16),
            DropdownButtonFormField<String>(
              value: _role,
              decoration: InputDecoration(
                labelText: isRu ? 'Роль' : 'Role',
                border: const OutlineInputBorder(),
              ),
              items: [
                DropdownMenuItem(value: 'developer', child: Text(isRu ? 'Разработчик' : 'Developer')),
                DropdownMenuItem(value: 'tester', child: Text(isRu ? 'Тестировщик' : 'Tester')),
                DropdownMenuItem(value: 'reviewer', child: Text(isRu ? 'Ревьюер' : 'Reviewer')),
                DropdownMenuItem(value: 'planner', child: Text(isRu ? 'Планировщик' : 'Planner')),
                DropdownMenuItem(
                    value: _customRoleOption,
                    child: Text(isRu ? 'Своя роль…' : 'Custom…')),
              ],
              onChanged: (val) {
                if (val != null) {
                  setState(() => _role = val);
                }
              },
            ),
            if (_isCustomRole) ...[
              const SizedBox(height: 16),
              TextField(
                controller: _customRoleController,
                decoration: InputDecoration(
                  labelText: isRu ? 'Название роли' : 'Role name',
                  helperText: isRu
                      ? 'snake_case, латиница: smm_writer, content_editor'
                      : 'snake_case, latin: smm_writer, content_editor',
                  hintText: 'smm_writer',
                  border: const OutlineInputBorder(),
                ),
                onChanged: (_) => setState(() {}),
              ),
            ],
            const SizedBox(height: 16),
            TextField(
              controller: _roleDescController,
              minLines: 2,
              maxLines: 4,
              decoration: InputDecoration(
                labelText:
                    isRu ? 'Описание роли' : 'Role description',
                helperText: isRu
                    ? 'Видит Router при выборе агента. Опишите, что агент делает.'
                    : 'Shown to the Router when it picks an agent. Describe what the agent does.',
                helperMaxLines: 2,
                hintText: isRu
                    ? 'Например: ${_roleExample(true)}'
                    : 'e.g. ${_roleExample(false)}',
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 16),
            TextField(
              controller: _systemPromptController,
              minLines: 2,
              maxLines: 6,
              decoration: InputDecoration(
                labelText: _isCustomRole
                    ? (isRu ? 'Системный промпт *' : 'System prompt *')
                    : (isRu ? 'Системный промпт' : 'System prompt'),
                helperText: _isCustomRole
                    ? (isRu
                        ? 'Обязателен для своей роли: инструкции, что и как делает агент.'
                        : 'Required for a custom role: instructions on what the agent does.')
                    : (isRu
                        ? 'Необязательно. Пусто — берётся дефолтный промпт роли.'
                        : 'Optional. If empty, the role default prompt is used.'),
                helperMaxLines: 2,
                border: const OutlineInputBorder(),
              ),
            ),
            const SizedBox(height: 16),
            DropdownButtonFormField<String>(
              value: _executionKind,
              decoration: InputDecoration(
                labelText: isRu ? 'Тип исполнения' : 'Execution Kind',
                border: const OutlineInputBorder(),
              ),
              items: const [
                DropdownMenuItem(value: 'llm', child: Text('LLM')),
                DropdownMenuItem(value: 'sandbox', child: Text('Sandbox')),
              ],
              onChanged: (val) {
                if (val != null) {
                  setState(() => _executionKind = val);
                }
              },
            ),
            if (_executionKind == 'sandbox') ...[
              const SizedBox(height: 16),
              DropdownButtonFormField<String>(
                value: _codeBackend,
                decoration: InputDecoration(
                  labelText: isRu ? 'Code backend' : 'Code backend',
                  border: const OutlineInputBorder(),
                ),
                items: codeBackends
                    .map((b) => DropdownMenuItem(value: b, child: Text(b)))
                    .toList(),
                onChanged: (val) {
                  if (val != null) {
                    setState(() {
                      _codeBackend = val;
                      // Если выбранный провайдер не подходит новому бекенду — сбрасываем.
                      if (_providerKind != null &&
                          !allowedProviderKindsForBackend(val)
                              .contains(_providerKind)) {
                        _providerKind = null;
                      }
                    });
                  }
                },
              ),
            ],
            const SizedBox(height: 16),
            DropdownButtonFormField<String?>(
              value: providerOptions.contains(_providerKind) ? _providerKind : null,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: isRu ? 'Провайдер' : 'Provider',
                helperText: providerOptions.isEmpty
                    ? (isRu
                        ? 'Нет подключённых провайдеров — настройте в LLM Integrations'
                        : 'No connected providers — set them up in LLM Integrations')
                    : null,
                border: const OutlineInputBorder(),
              ),
              items: [
                DropdownMenuItem<String?>(
                  value: null,
                  child: Text(isRu ? 'Не выбран' : 'Not set'),
                ),
                ...providerOptions.map(
                  (p) => DropdownMenuItem<String?>(value: p, child: Text(p)),
                ),
              ],
              onChanged: (val) {
                setState(() => _applyProviderDefaults(val));
              },
            ),
            const SizedBox(height: 16),
            SearchAnchor(
              searchController: _modelController,
              builder: (context, controller) {
                return TextField(
                  controller: controller,
                  enabled: _providerKind != null,
                  decoration: InputDecoration(
                    labelText: isRu ? 'Модель' : 'Model',
                    helperText: _providerKind == null
                        ? (isRu
                            ? 'Сначала выберите провайдера'
                            : 'Pick a provider first')
                        : (isRu
                            ? 'Для sandbox сохраняется в настройках бекенда.'
                            : 'For sandbox it is stored in the backend settings.'),
                    helperMaxLines: 2,
                    suffixIcon: dynamicModelsAsync.maybeWhen(
                      loading: () => const Padding(
                        padding: EdgeInsets.all(12.0),
                        child: SizedBox(
                          width: 16,
                          height: 16,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                      ),
                      orElse: () => const Icon(Icons.arrow_drop_down),
                    ),
                    border: const OutlineInputBorder(),
                  ),
                  onTap: () {
                    if (_providerKind != null) controller.openView();
                  },
                  onChanged: (_) {
                    if (_providerKind != null) controller.openView();
                  },
                );
              },
              suggestionsBuilder: (context, controller) {
                final text = controller.text;
                final providerKind = _providerKind;
                if (providerKind == null) {
                  return [
                    ListTile(
                      title: Text(isRu
                          ? 'Сначала выберите провайдера'
                          : 'Pick a provider first'),
                    ),
                  ];
                }
                // Динамический каталог приоритетнее; фолбэк — статические подсказки.
                final dynamicModels = dynamicModelsAsync.asData?.value;
                final allSuggestions =
                    (dynamicModels != null && dynamicModels.isNotEmpty)
                        ? dynamicModels
                        : agentModelSuggestions(providerKind);
                final isExactMatch = allSuggestions
                    .any((s) => s.toLowerCase() == text.trim().toLowerCase());
                final filtered = isExactMatch
                    ? allSuggestions
                    : allSuggestions
                        .where((m) =>
                            m.toLowerCase().contains(text.toLowerCase()))
                        .toList();

                final list = <Widget>[];
                if (text.isNotEmpty && !allSuggestions.contains(text)) {
                  list.add(
                    ListTile(
                      leading: const Icon(Icons.add),
                      title: Text(isRu
                          ? 'Использовать свою модель: «$text»'
                          : 'Use custom model: "$text"'),
                      onTap: () => controller.closeView(text),
                    ),
                  );
                }
                list.addAll(filtered.map((model) => ListTile(
                      title: Text(model),
                      onTap: () => controller.closeView(model),
                    )));
                if (list.isEmpty) {
                  list.add(
                    ListTile(
                      title: Text(isRu
                          ? 'Ничего не найдено — нажмите Enter, чтобы оставить введённое.'
                          : 'No matches — press Enter to keep your input.'),
                      onTap: () => controller.closeView(text),
                    ),
                  );
                }
                return list;
              },
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: _isLoading ? null : () => Navigator.of(context).pop(),
          child: Text(isRu ? 'Отмена' : 'Cancel'),
        ),
        FilledButton(
          onPressed: _isLoading ? null : _submit,
          child: _isLoading
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(strokeWidth: 2),
                )
              : Text(isRu ? 'Добавить' : 'Add'),
        ),
      ],
    );
  }
}
