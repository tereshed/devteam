/// Единый источник статических подсказок моделей по provider kind — общий для настройки
/// командных агентов (agent_edit_dialog) и персонального ассистента (assistant_settings_card),
/// чтобы списки не расходились. Используется как фоллбек, когда динамический каталог моделей
/// (availableModelsProvider) ещё не загрузился или пуст. Кастомную модель всегда можно ввести
/// вручную в поле поиска.
///
/// providerKind — канонический ключ провайдера ('openrouter', 'anthropic', 'anthropic_oauth',
/// 'deepseek', 'zhipu', 'antigravity', 'antigravity_oauth', ...).
List<String> agentModelSuggestions(String providerKind) {
  switch (providerKind) {
    case 'openrouter':
      return const [
        'deepseek/deepseek-v4-pro',
        'deepseek/deepseek-v4-flash',
        'deepseek/deepseek-r1',
        'anthropic/claude-3.5-sonnet',
        'anthropic/claude-3.5-haiku',
        'google/gemini-2.5-flash',
        'openai/gpt-4o',
        'openai/gpt-4o-mini',
        'meta-llama/llama-3.3-70b-instruct',
      ];
    case 'anthropic':
      return const [
        'claude-3-5-sonnet-latest',
        'claude-3-5-haiku-latest',
        'claude-haiku-4-5-20251001',
      ];
    case 'anthropic_oauth':
      return const [
        'claude-3-5-sonnet-latest',
        'claude-haiku-4-5-20251001',
      ];
    case 'deepseek':
      return const [
        'deepseek-chat',
        'deepseek-reasoner',
      ];
    case 'zhipu':
      return const [
        'glm-4',
        'glm-4-flash',
      ];
    case 'antigravity':
    case 'antigravity_oauth':
      return const [
        'antigravity-default',
      ];
    default:
      return const [];
  }
}
