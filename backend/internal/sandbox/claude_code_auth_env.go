package sandbox

// ClaudeCodeAuthEnv — набор переменных окружения для аутентификации Claude Code в sandbox-контейнере.
// Sprint 15.14: оркестратор формирует ClaudeCodeAuthEnv в зависимости от настроек агента
// (OAuth-подписка → OAuthToken; free-claude-proxy → ProxyBaseURL + ProxyAuthToken; иначе → APIKey)
// и сливает результат в SandboxOptions.EnvVars.
type ClaudeCodeAuthEnv struct {
	// OAuthToken — Sprint 15.B: токен подписки Claude Code (приоритет над APIKey).
	OAuthToken string
	// APIKey — классический ANTHROPIC_API_KEY (legacy/fallback).
	APIKey string
	// ProxyBaseURL — Sprint 15.18: URL free-claude-proxy. Если задан, Claude Code ходит туда.
	ProxyBaseURL string
	// ProxyAuthToken — Bearer-токен для free-claude-proxy.
	ProxyAuthToken string
}

// ToEnv возвращает map env, готовый для SandboxOptions.EnvVars.
// Пустые значения не записываются, чтобы entrypoint мог опираться на наличие переменной.
func (e ClaudeCodeAuthEnv) ToEnv() map[string]string {
	out := map[string]string{}
	if e.ProxyBaseURL != "" {
		out[EnvAnthropicBaseURL] = e.ProxyBaseURL
	}
	if e.ProxyAuthToken != "" {
		out[EnvAnthropicAuthToken] = e.ProxyAuthToken
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
	return e.OAuthToken != "" || e.APIKey != "" || e.ProxyAuthToken != ""
}
