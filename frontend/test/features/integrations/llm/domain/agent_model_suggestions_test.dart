import 'package:flutter_test/flutter_test.dart';
import 'package:frontend/features/integrations/llm/domain/agent_model_suggestions.dart';

void main() {
  group('agentModelSuggestions', () {
    test('openrouter включает актуальные deepseek v4 модели (pro+flash)', () {
      final or = agentModelSuggestions('openrouter');
      expect(or, contains('deepseek/deepseek-v4-pro'));
      expect(or, contains('deepseek/deepseek-v4-flash'));
    });

    test('известные провайдеры дают непустой список, неизвестный — пустой', () {
      for (final kind in ['anthropic', 'anthropic_oauth', 'deepseek', 'zhipu', 'antigravity']) {
        expect(agentModelSuggestions(kind), isNotEmpty, reason: kind);
      }
      expect(agentModelSuggestions('totally_unknown'), isEmpty);
    });
  });
}
