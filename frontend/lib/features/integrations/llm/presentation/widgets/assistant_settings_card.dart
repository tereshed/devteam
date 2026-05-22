import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:frontend/features/admin/agents_v2/domain/agent_v2_model.dart';
import 'package:frontend/features/assistant/data/assistant_providers.dart';
import 'package:frontend/features/integrations/llm/data/llm_integrations_providers.dart';
import 'package:frontend/features/integrations/llm/domain/llm_provider_model.dart';
import 'package:frontend/features/onboarding/data/my_agents_providers.dart';

class AssistantSettingsCard extends ConsumerStatefulWidget {
  const AssistantSettingsCard({super.key});

  @override
  ConsumerState<AssistantSettingsCard> createState() => _AssistantSettingsCardState();
}

class _AssistantSettingsCardState extends ConsumerState<AssistantSettingsCard> {
  final _formKey = GlobalKey<FormState>();
  final _modelSearchController = SearchController();
  
  AgentV2? _lastAgent;
  LlmIntegrationProvider? _selectedProvider;
  bool _isSaving = false;
  String? _errorMessage;

  static const _supportedProviders = [
    LlmIntegrationProvider.claudeCodeOAuth,
    LlmIntegrationProvider.antigravityOAuth,
    LlmIntegrationProvider.antigravity,
    LlmIntegrationProvider.anthropic,
    LlmIntegrationProvider.deepseek,
    LlmIntegrationProvider.zhipu,
    LlmIntegrationProvider.openrouter,
  ];

  @override
  void dispose() {
    _modelSearchController.dispose();
    super.dispose();
  }

  String _providerToKind(LlmIntegrationProvider provider) {
    switch (provider) {
      case LlmIntegrationProvider.claudeCodeOAuth:
        return 'anthropic_oauth';
      case LlmIntegrationProvider.antigravityOAuth:
        return 'antigravity_oauth';
      case LlmIntegrationProvider.antigravity:
        return 'antigravity';
      case LlmIntegrationProvider.anthropic:
        return 'anthropic';
      case LlmIntegrationProvider.deepseek:
        return 'deepseek';
      case LlmIntegrationProvider.zhipu:
        return 'zhipu';
      case LlmIntegrationProvider.openrouter:
        return 'openrouter';
      default:
        throw ArgumentError('Unsupported assistant provider: $provider');
    }
  }

  LlmIntegrationProvider? _kindToProvider(String kind) {
    switch (kind) {
      case 'anthropic_oauth':
        return LlmIntegrationProvider.claudeCodeOAuth;
      case 'antigravity_oauth':
        return LlmIntegrationProvider.antigravityOAuth;
      case 'antigravity':
        return LlmIntegrationProvider.antigravity;
      case 'anthropic':
        return LlmIntegrationProvider.anthropic;
      case 'deepseek':
        return LlmIntegrationProvider.deepseek;
      case 'zhipu':
        return LlmIntegrationProvider.zhipu;
      case 'openrouter':
        return LlmIntegrationProvider.openrouter;
      default:
        return null;
    }
  }

  String _getProviderName(LlmIntegrationProvider provider) {
    switch (provider) {
      case LlmIntegrationProvider.claudeCodeOAuth:
        return 'Claude Code (OAuth)';
      case LlmIntegrationProvider.antigravityOAuth:
        return 'Antigravity (OAuth)';
      case LlmIntegrationProvider.antigravity:
        return 'Antigravity';
      case LlmIntegrationProvider.anthropic:
        return 'Anthropic Claude';
      case LlmIntegrationProvider.deepseek:
        return 'DeepSeek';
      case LlmIntegrationProvider.zhipu:
        return 'Zhipu AI';
      case LlmIntegrationProvider.openrouter:
        return 'OpenRouter';
      default:
        return provider.name;
    }
  }

  List<String> _suggestionsFor(LlmIntegrationProvider provider) {
    switch (provider) {
      case LlmIntegrationProvider.openrouter:
        return [
          'deepseek/deepseek-r1',
          'anthropic/claude-3.5-sonnet',
          'google/gemini-2.5-flash',
          'openai/gpt-4o',
          'openai/gpt-4o-mini',
          'meta-llama/llama-3.3-70b-instruct',
          'deepseek/deepseek-v4-flash',
          'anthropic/claude-3.5-haiku',
        ];
      case LlmIntegrationProvider.anthropic:
        return [
          'claude-3-5-sonnet-latest',
          'claude-3-5-haiku-latest',
          'claude-haiku-4-5-20251001',
        ];
      case LlmIntegrationProvider.claudeCodeOAuth:
        return [
          'claude-3-5-sonnet-latest',
          'claude-haiku-4-5-20251001',
        ];
      case LlmIntegrationProvider.deepseek:
        return [
          'deepseek-chat',
          'deepseek-reasoner',
        ];
      case LlmIntegrationProvider.zhipu:
        return [
          'glm-4',
          'glm-4-flash',
        ];
      case LlmIntegrationProvider.antigravity:
      case LlmIntegrationProvider.antigravityOAuth:
        return [
          'antigravity-default',
        ];
      default:
        return const [];
    }
  }

  Future<void> _save(String agentId) async {
    if (_isSaving) {
      return;
    }
    if (_selectedProvider == null) {
      setState(() {
        _errorMessage = 'Пожалуйста, выберите провайдера';
      });
      return;
    }
    if (_formKey.currentState?.validate() != true) {
      return;
    }

    setState(() {
      _isSaving = true;
      _errorMessage = null;
    });

    try {
      final repo = ref.read(myAgentsRepositoryProvider);
      final kind = _providerToKind(_selectedProvider!);
      final model = _modelSearchController.text.trim();

      await repo.update(
        agentId,
        providerKind: kind,
        model: model,
      );

      // Инвалидируем провайдеры для обновления UI и статуса ассистента
      ref.invalidate(myAgentsListProvider);
      ref.invalidate(assistantStatusProvider);

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Настройки ассистента успешно сохранены'),
            backgroundColor: Colors.green,
          ),
        );
      }
    } catch (e) {
      setState(() {
        _errorMessage = 'Ошибка при сохранении: $e';
      });
    } finally {
      if (mounted) {
        setState(() {
          _isSaving = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final myAgentsAsync = ref.watch(myAgentsListProvider);

    final pk = _selectedProvider != null ? _providerToKind(_selectedProvider!) : null;
    final AsyncValue<List<String>> dynamicModelsAsync = pk != null
        ? ref.watch(availableModelsProvider(pk))
        : const AsyncValue.data([]);

    return myAgentsAsync.when(
      loading: () => const Center(
        child: Padding(
          padding: EdgeInsets.all(24.0),
          child: CircularProgressIndicator(),
        ),
      ),
      error: (error, stack) => Center(
        child: Padding(
          padding: const EdgeInsets.all(24.0),
          child: Text(
            'Ошибка загрузки настроек ассистента: $error',
            style: TextStyle(color: theme.colorScheme.error),
          ),
        ),
      ),
      data: (page) {
        final assistant = page.items
            .where((a) => a.role == 'assistant')
            .firstOrNull;

        if (assistant == null) {
          return const Card(
            child: Padding(
              padding: EdgeInsets.all(24.0),
              child: Text(
                'Персональный ассистент не найден. Обратитесь к администратору.',
                style: TextStyle(fontWeight: FontWeight.w600),
              ),
            ),
          );
        }

        // Синхронизируем начальное состояние при первой загрузке или изменении ассистента
        if (assistant != _lastAgent) {
          _lastAgent = assistant;
          _selectedProvider = assistant.providerKind != null
              ? _kindToProvider(assistant.providerKind!)
              : null;
          _modelSearchController.text = assistant.model ?? '';
        }

        // Получаем текущие подключения LLM-провайдеров
        final integrationsController = ref.watch(llmIntegrationsControllerProvider);
        final integrationsState = integrationsController.state;

        // Фильтруем подключенные из поддерживаемых
        final connectedProviders = _supportedProviders.where((p) {
          return integrationsState.connections[p]?.status ==
              LlmProviderConnectionStatus.connected;
        }).toList();

        // Формируем список опций для выпадающего списка.
        // Добавляем текущего провайдера, даже если он отключен, чтобы избежать ошибок отображения.
        final dropdownOptions = <LlmIntegrationProvider>[];
        for (final p in connectedProviders) {
          if (!dropdownOptions.contains(p)) {
            dropdownOptions.add(p);
          }
        }
        if (_selectedProvider != null && !dropdownOptions.contains(_selectedProvider)) {
          dropdownOptions.add(_selectedProvider!);
        }

        if (dropdownOptions.isEmpty) {
          return Card(
            elevation: 1,
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12),
              side: BorderSide(
                color: theme.colorScheme.outlineVariant.withValues(alpha: 0.5),
              ),
            ),
            child: Padding(
              padding: const EdgeInsets.all(24.0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Настройки ассистента',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 12),
                  Text(
                    'Для настройки ассистента сначала подключите хотя бы один из поддерживаемых LLM-провайдеров выше (Claude Code, Anthropic, DeepSeek, Zhipu, OpenRouter).',
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                ],
              ),
            ),
          );
        }
        return Card(
          elevation: 1,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
            side: BorderSide(
              color: theme.colorScheme.outlineVariant.withValues(alpha: 0.5),
            ),
          ),
          child: Padding(
            padding: const EdgeInsets.all(24.0),
            child: Form(
              key: _formKey,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.settings_suggest_outlined,
                        color: theme.colorScheme.primary,
                        size: 24,
                      ),
                      const SizedBox(width: 12),
                      Text(
                        'Настройки ассистента',
                        style: theme.textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Выберите провайдера и укажите модель, которую будет использовать ваш персональный ассистент.',
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                    ),
                  ),
                  const SizedBox(height: 24),
                  
                  // Выпадающий список провайдеров
                  DropdownButtonFormField<LlmIntegrationProvider>(
                    key: ValueKey(_selectedProvider),
                    initialValue: _selectedProvider,
                    decoration: InputDecoration(
                      labelText: 'LLM Провайдер',
                      prefixIcon: const Icon(Icons.hub_outlined),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8),
                      ),
                    ),
                    items: dropdownOptions.map((provider) {
                      final isConnected = connectedProviders.contains(provider);
                      final displayName = _getProviderName(provider);
                      final label = isConnected ? displayName : '$displayName (Не подключен)';
                      
                      return DropdownMenuItem<LlmIntegrationProvider>(
                        value: provider,
                        enabled: isConnected,
                        child: Text(
                          label,
                          style: TextStyle(
                            color: isConnected 
                                ? theme.colorScheme.onSurface 
                                : theme.colorScheme.onSurfaceVariant.withValues(alpha: 0.6),
                          ),
                        ),
                      );
                    }).toList(),
                    onChanged: _isSaving
                        ? null
                        : (val) {
                            setState(() {
                              _selectedProvider = val;
                              // Очищаем или устанавливаем дефолтную модель при смене провайдера, если текущая не подходит
                              final newSuggestions = val != null ? _suggestionsFor(val) : const <String>[];
                              if (newSuggestions.isNotEmpty && !_suggestionsFor(val!).contains(_modelSearchController.text)) {
                                _modelSearchController.text = newSuggestions.first;
                              }
                            });
                          },
                    validator: (val) {
                      if (val == null) {
                        return 'Выберите провайдера';
                      }
                      if (!connectedProviders.contains(val)) {
                        return 'Этот провайдер не подключен. Подключите его в списке выше.';
                      }
                      return null;
                    },
                  ),
                  const SizedBox(height: 20),

                  // Поле выбора модели со строкой поиска
                  SearchAnchor(
                    searchController: _modelSearchController,
                    builder: (BuildContext context, SearchController controller) {
                      return TextFormField(
                        controller: controller,
                        enabled: !_isSaving && _selectedProvider != null,
                        decoration: InputDecoration(
                          labelText: 'Модель',
                          prefixIcon: const Icon(Icons.psychology_outlined),
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
                          helperText: 'Введите название модели или выберите из списка',
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(8),
                          ),
                        ),
                        onTap: () {
                          controller.openView();
                        },
                        onChanged: (_) {
                          controller.openView();
                        },
                        validator: (val) {
                          if (val == null || val.trim().isEmpty) {
                            return 'Укажите модель';
                          }
                          return null;
                        },
                      );
                    },
                    suggestionsBuilder: (BuildContext context, SearchController controller) {
                      final text = controller.text;
                      final provider = _selectedProvider;
                      if (provider == null) {
                        return const [
                          ListTile(
                            title: Text('Сначала выберите провайдера'),
                          ),
                        ];
                      }
                      
                      final List<String> allSuggestions;
                      final dynamicModels = dynamicModelsAsync.asData?.value;
                      if (dynamicModels != null && dynamicModels.isNotEmpty) {
                        allSuggestions = dynamicModels;
                      } else {
                        allSuggestions = _suggestionsFor(provider);
                      }
                      final isExactMatch = allSuggestions.any((s) => s.toLowerCase() == text.trim().toLowerCase());
                      final filtered = isExactMatch
                          ? allSuggestions
                          : allSuggestions
                              .where((m) => m.toLowerCase().contains(text.toLowerCase()))
                              .toList();

                      final List<Widget> list = [];

                      // Если введённый текст не пустой и его нет в списке подсказок,
                      // даём пользователю возможность выбрать именно его (как кастомную модель).
                      if (text.isNotEmpty && !allSuggestions.contains(text)) {
                        list.add(
                          ListTile(
                            leading: const Icon(Icons.add),
                            title: Text('Использовать кастомную модель: "$text"'),
                            onTap: () {
                              controller.closeView(text);
                            },
                          ),
                        );
                      }

                      list.addAll(
                        filtered.map((model) => ListTile(
                          title: Text(model),
                          onTap: () {
                            controller.closeView(model);
                          },
                        )),
                      );

                      if (list.isEmpty) {
                        list.add(
                          ListTile(
                            title: const Text('Модели не найдены. Нажмите Enter, чтобы использовать введенный текст.'),
                            onTap: () {
                              controller.closeView(text);
                            },
                          ),
                        );
                      }

                      return list;
                    },
                  ),
                  const SizedBox(height: 24),

                  if (_errorMessage != null) ...[
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(
                        color: theme.colorScheme.errorContainer,
                        borderRadius: BorderRadius.circular(8),
                      ),
                      child: Row(
                        children: [
                          Icon(Icons.error_outline, color: theme.colorScheme.error),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Text(
                              _errorMessage!,
                              style: TextStyle(color: theme.colorScheme.onErrorContainer),
                            ),
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(height: 16),
                  ],

                  // Кнопка сохранения
                  SizedBox(
                    width: double.infinity,
                    height: 48,
                    child: FilledButton.icon(
                      onPressed: _isSaving ? null : () => _save(assistant.id),
                      icon: _isSaving
                          ? const SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                strokeWidth: 2,
                                color: Colors.white,
                              ),
                            )
                          : const Icon(Icons.save_outlined),
                      label: Text(_isSaving ? 'Сохранение...' : 'Сохранить настройки'),
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
