import 'package:freezed_annotation/freezed_annotation.dart';

part 'llm_provider_model.freezed.dart';
part 'llm_provider_model.g.dart';

/// Sprint 15.28 — модель LLM-провайдера из таблицы llm_providers.
///
/// `credentials_encrypted` никогда не приходит на фронт (поле скрыто `json:"-"` на бэке).
/// При создании/обновлении пользователь вводит plaintext-credential, который бэк шифрует.
@freezed
abstract class LLMProviderModel with _$LLMProviderModel {
  const factory LLMProviderModel({
    required String id,
    required String name,
    required String kind,
    @JsonKey(name: 'base_url') @Default('') String baseURL,
    @JsonKey(name: 'auth_type') @Default('api_key') String authType,
    @JsonKey(name: 'default_model') @Default('') String defaultModel,
    @Default(true) bool enabled,
  }) = _LLMProviderModel;

  factory LLMProviderModel.fromJson(Map<String, dynamic> json) =>
      _$LLMProviderModelFromJson(json);
}

/// Все поддерживаемые kind'ы провайдеров (синхронизировано с backend models.LLMProviderKind).
/// Sprint 15.e2e: kind=`free_claude_proxy` удалён — sidecar заменён на native
/// Anthropic endpoint провайдера + per-user creds (см. user_llm_credentials).
const List<String> kSupportedLLMProviderKinds = [
  'anthropic',
  'anthropic_oauth',
  'openai',
  'gemini',
  'deepseek',
  'qwen',
  'openrouter',
  'moonshot',
  'ollama',
  'zhipu',
];
