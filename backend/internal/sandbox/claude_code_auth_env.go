package sandbox

// ClaudeCodeAuthEnv — набор переменных окружения для аутентификации в sandbox-контейнере.
// Имя сохранено по историческим причинам (Sprint 15.14), но фактически используется для всех
// поддерживаемых code_backend'ов. Sprint 16 добавил `Extra` для backend-specific env (Hermes
// использует OPENROUTER_API_KEY/NOUS_PORTAL_API_KEY и т.п., не ANTHROPIC_*).
//
// Sprint 15.e2e + Sprint 16: оркестратор формирует env в зависимости от kind провайдера
// агента (см. SandboxAuthEnvResolver):
//   - claude-code:
//   - anthropic              → APIKey
//   - anthropic_oauth        → OAuthToken
//   - deepseek/zhipu/openrouter → BaseURL + AuthToken (native Anthropic endpoint)
//   - hermes: всё через Extra с провайдер-специфичными env (OPENROUTER_API_KEY, …)
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
	// Extra — Sprint 16: backend-specific env-vars (для code_backend != claude-code,
	// у которого свои имена переменных). Ключи — точные имена env, значения — plaintext.
	// При коллизии с типизированными полями выше — Extra перетирает (последняя запись побеждает).
	Extra map[string]string
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
	for k, v := range e.Extra {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

// HasCredential сообщает, есть ли хоть одна форма аутентификации.
// Соответствует проверке в deployment/sandbox/claude/entrypoint.sh и hermes entrypoint.
func (e ClaudeCodeAuthEnv) HasCredential() bool {
	if e.OAuthToken != "" || e.APIKey != "" || e.AuthToken != "" {
		return true
	}
	for _, v := range e.Extra {
		if v != "" {
			return true
		}
	}
	return false
}
