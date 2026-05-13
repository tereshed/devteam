package sandbox

// ClaudeCodeAuthEnv — набор переменных окружения для аутентификации Claude Code в sandbox-контейнере.
// Sprint 15.e2e refactor: оркестратор формирует ClaudeCodeAuthEnv в зависимости от kind провайдера агента
// (см. SandboxAuthEnvResolver):
//   - anthropic              → APIKey
//   - anthropic_oauth        → OAuthToken
//   - deepseek / zhipu /
//     openrouter             → BaseURL + AuthToken (native Anthropic endpoint провайдера)
//
// Результат сливается в SandboxOptions.EnvVars.
type ClaudeCodeAuthEnv struct {
	// OAuthToken — Sprint 15.B: токен подписки Claude Code (приоритет над APIKey).
	OAuthToken string
	// APIKey — классический ANTHROPIC_API_KEY (для kind=anthropic).
	APIKey string
	// BaseURL — Anthropic-совместимый endpoint провайдера (для kind=deepseek/zhipu/openrouter).
	// Выставляется в env как ANTHROPIC_BASE_URL.
	BaseURL string
	// AuthToken — Bearer-токен для BaseURL. Выставляется как ANTHROPIC_AUTH_TOKEN.
	AuthToken string
}

// ToEnv возвращает map env, готовый для SandboxOptions.EnvVars.
// Пустые значения не записываются, чтобы entrypoint мог опираться на наличие переменной.
func (e ClaudeCodeAuthEnv) ToEnv() map[string]string {
	out := map[string]string{}
	if e.BaseURL != "" {
		out[EnvAnthropicBaseURL] = e.BaseURL
	}
	if e.AuthToken != "" {
		out[EnvAnthropicAuthToken] = e.AuthToken
	}
	if e.OAuthToken != "" {
		out[EnvClaudeCodeOAuthToken] = e.OAuthToken
	}
	if e.APIKey != "" {
		out[EnvAnthropicAPIKey] = e.APIKey
	}
	return out
}

// HasCredential сообщает, есть ли хоть одна форма аутентификации.
// Соответствует проверке в deployment/sandbox/claude/entrypoint.sh.
func (e ClaudeCodeAuthEnv) HasCredential() bool {
	return e.OAuthToken != "" || e.APIKey != "" || e.AuthToken != ""
}
