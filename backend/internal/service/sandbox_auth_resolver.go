package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/google/uuid"
)

// SandboxAuthEnvResolver — подбирает аутентификационные env для sandbox-агента
// на основе agent.ProviderKind + per-user creds (Sprint 15.e2e refactor).
//
// Логика по kind:
//
//	anthropic        → user_llm_credentials(owner, anthropic) → ANTHROPIC_API_KEY
//	anthropic_oauth  → claude_code_subscriptions(owner)       → CLAUDE_CODE_OAUTH_TOKEN
//	deepseek         → user_llm_credentials(owner, deepseek)  → ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN
//	zhipu            → user_llm_credentials(owner, zhipu)     → ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN
//	openrouter       → user_llm_credentials(owner, openrouter)→ ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN
//
// Fallback (agent.ProviderKind == nil) — последовательная попытка:
//  1. OAuth-подписка у владельца проекта;
//  2. Статический ANTHROPIC_API_KEY из cfg.LLM.Anthropic.APIKey (backwards compat).
type SandboxAuthEnvResolver interface {
	Resolve(ctx context.Context, project *models.Project, agent *models.Agent) sandbox.ClaudeCodeAuthEnv
}

// UserLLMCredentialResolver — узкий интерфейс, который нужен резолверу: достать plaintext-ключ
// конкретного пользователя для kind. Реализуется service.UserLlmCredentialService.
type UserLLMCredentialResolver interface {
	GetPlaintext(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (string, error)
}

// ClaudeCodeOAuthAccessor — узкий интерфейс, который нужен резолверу: достать
// OAuth-токен подписки Claude Code для sandbox. Полный ClaudeCodeAuthService с
// device-flow/Refresh/Revoke для резолвера избыточен; узкий интерфейс минимизирует
// boilerplate в тестовых стабах и не ломает их при добавлении новых методов сервиса.
type ClaudeCodeOAuthAccessor interface {
	AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error)
}

// sandboxAuthEnvResolver — реализация по умолчанию.
type sandboxAuthEnvResolver struct {
	claudeCodeAuth ClaudeCodeOAuthAccessor
	userCreds      UserLLMCredentialResolver
	fallbackAPIKey string
	logger         *slog.Logger
}

// NewSandboxAuthEnvResolver собирает резолвер.
//   - claudeCodeAuth может быть nil (фича OAuth выключена — kind=anthropic_oauth тогда не работает).
//     Принимается узкий интерфейс ClaudeCodeOAuthAccessor; полный ClaudeCodeAuthService его удовлетворяет.
//   - userCreds может быть nil (тогда kind=anthropic/deepseek/zhipu/openrouter не сработают).
//   - fallbackAPIKey — статический ANTHROPIC_API_KEY (для агентов без ProviderKind).
func NewSandboxAuthEnvResolver(
	claudeCodeAuth ClaudeCodeOAuthAccessor,
	userCreds UserLLMCredentialResolver,
	fallbackAPIKey string,
	logger *slog.Logger,
) SandboxAuthEnvResolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &sandboxAuthEnvResolver{
		claudeCodeAuth: claudeCodeAuth,
		userCreds:      userCreds,
		fallbackAPIKey: fallbackAPIKey,
		logger:         logger.With("component", "sandbox_auth_env_resolver"),
	}
}

func (r *sandboxAuthEnvResolver) Resolve(ctx context.Context, project *models.Project, agent *models.Agent) sandbox.ClaudeCodeAuthEnv {
	env := sandbox.ClaudeCodeAuthEnv{}
	if project == nil || project.UserID == [16]byte{} {
		// Нет владельца — fallback на статический ключ.
		env.APIKey = r.fallbackAPIKey
		return env
	}

	// Sprint 16: Hermes Agent — собственная схема env (OPENROUTER_API_KEY, …),
	// не Anthropic-совместимая. Резолвится отдельным путём.
	if agent != nil && agent.CodeBackend != nil && *agent.CodeBackend == models.CodeBackendHermes {
		return r.resolveHermes(ctx, project, agent)
	}

	// 1) Явный kind на агенте — основной путь после рефакторинга.
	if agent != nil && agent.ProviderKind != nil && agent.ProviderKind.IsValid() {
		return r.resolveByKind(ctx, project, *agent.ProviderKind)
	}

	// 2) Legacy fallback: OAuth-подписка → static API key.
	if r.claudeCodeAuth != nil {
		token, err := r.claudeCodeAuth.AccessTokenForSandbox(ctx, project.UserID)
		if err == nil && token != "" {
			env.OAuthToken = token
			return env
		}
		if err != nil && !errors.Is(err, repository.ErrClaudeCodeSubscriptionNotFound) {
			r.logger.Warn("oauth token lookup failed; falling back to api key",
				"project_id", project.ID.String(),
				"user_id", project.UserID.String(),
				"err", err)
		}
	}
	env.APIKey = r.fallbackAPIKey
	return env
}

func (r *sandboxAuthEnvResolver) resolveByKind(ctx context.Context, project *models.Project, kind models.AgentProviderKind) sandbox.ClaudeCodeAuthEnv {
	env := sandbox.ClaudeCodeAuthEnv{}
	logger := r.logger.With(
		"project_id", project.ID.String(),
		"user_id", project.UserID.String(),
		"provider_kind", string(kind),
	)

	switch kind {
	case models.AgentProviderKindAnthropicOAuth:
		if r.claudeCodeAuth == nil {
			logger.Warn("agent has kind=anthropic_oauth but claude code auth service is disabled")
			return env
		}
		token, err := r.claudeCodeAuth.AccessTokenForSandbox(ctx, project.UserID)
		if err != nil {
			logger.Warn("anthropic_oauth: failed to resolve subscription token", "err", err)
			return env
		}
		env.OAuthToken = token
		return env

	case models.AgentProviderKindAnthropic,
		models.AgentProviderKindDeepSeek,
		models.AgentProviderKindZhipu,
		models.AgentProviderKindOpenRouter:
		if r.userCreds == nil {
			logger.Warn("user credentials resolver is nil; cannot resolve per-user key")
			return env
		}
		userProvider := kind.UserLLMProvider()
		if userProvider == "" {
			logger.Warn("kind has no user_llm_credentials mapping")
			return env
		}
		key, err := r.userCreds.GetPlaintext(ctx, project.UserID, userProvider)
		if errors.Is(err, repository.ErrUserLlmCredentialNotFound) {
			// Пользователь не настроил ключ для выбранного kind — это не системный
			// сбой, а ожидаемое состояние. Возвращаем пустой env: sandbox упадёт на
			// fast-fail "no credentials" в entrypoint вместо тихого fallback на
			// чужого провайдера (см. TestResolver_DeepSeek_UserHasNoKey_ReturnsEmpty).
			logger.Warn("user has no credential for provider", "user_provider", string(userProvider))
			return env
		}
		if err != nil {
			logger.Warn("user credential lookup failed", "user_provider", string(userProvider), "err", err)
			return env
		}
		if kind == models.AgentProviderKindAnthropic {
			// Anthropic API key: классический ANTHROPIC_API_KEY, без BASE_URL (CLI пойдёт по дефолту).
			env.APIKey = key
			return env
		}
		// Не-anthropic kind с native Anthropic-endpoint'ом.
		env.BaseURL = kind.AnthropicBaseURL()
		env.AuthToken = key
		return env
	}

	logger.Warn("unknown provider_kind")
	return env
}

// resolveHermes — Sprint 16: env для Hermes Agent sandbox.
// Hermes сам выбирает провайдера по `model: "<provider>/<name>"` (см.
// HermesModelString); ключ читает из env'а с провайдер-специфичным именем
// (HermesEnvVar для kind). Контракт «не найдено != ошибка» из user_llm_credentials
// сохраняется: при отсутствии ключа возвращаем пустой env, sandbox-entrypoint
// упадёт fast-fail вместо тихого fallback.
func (r *sandboxAuthEnvResolver) resolveHermes(ctx context.Context, project *models.Project, agent *models.Agent) sandbox.ClaudeCodeAuthEnv {
	env := sandbox.ClaudeCodeAuthEnv{Extra: map[string]string{}}
	logger := r.logger.With(
		"project_id", project.ID.String(),
		"user_id", project.UserID.String(),
		"code_backend", "hermes",
	)
	if agent == nil || agent.ProviderKind == nil || !agent.ProviderKind.IsValid() {
		logger.Warn("hermes: provider_kind required for this code_backend")
		return env
	}
	kind := *agent.ProviderKind
	logger = logger.With("provider_kind", string(kind))

	envName := kind.HermesEnvVar()
	if envName == "" {
		logger.Warn("hermes: kind has no env-var mapping (provider not supported by hermes directly)")
		return env
	}
	userProvider := kind.UserLLMProvider()
	if userProvider == "" {
		logger.Warn("hermes: kind has no user_llm_credentials mapping")
		return env
	}
	if r.userCreds == nil {
		logger.Warn("hermes: user credentials resolver is nil")
		return env
	}
	key, err := r.userCreds.GetPlaintext(ctx, project.UserID, userProvider)
	if errors.Is(err, repository.ErrUserLlmCredentialNotFound) {
		logger.Warn("hermes: user has no credential for provider", "user_provider", string(userProvider))
		return env
	}
	if err != nil {
		logger.Warn("hermes: user credential lookup failed", "user_provider", string(userProvider), "err", err)
		return env
	}
	env.Extra[envName] = key
	// Sprint 16: имя hermes-провайдера в отдельной env-переменной, которую entrypoint
	// передаёт как `hermes chat --provider $DEVTEAM_HERMES_PROVIDER`. DEVTEAM_AGENT_MODEL
	// должен оставаться чистым именем модели (например `anthropic/claude-3.5-haiku`),
	// без openrouter/ префикса — иначе OpenRouter режектит запрос.
	if provName := kind.HermesProviderName(); provName != "" {
		env.Extra["DEVTEAM_HERMES_PROVIDER"] = provName
	}
	return env
}
