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
//   anthropic        → user_llm_credentials(owner, anthropic) → ANTHROPIC_API_KEY
//   anthropic_oauth  → claude_code_subscriptions(owner)       → CLAUDE_CODE_OAUTH_TOKEN
//   deepseek         → user_llm_credentials(owner, deepseek)  → ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN
//   zhipu            → user_llm_credentials(owner, zhipu)     → ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN
//   openrouter       → user_llm_credentials(owner, openrouter)→ ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN
//
// Fallback (agent.ProviderKind == nil) — последовательная попытка:
//   1) OAuth-подписка у владельца проекта;
//   2) Статический ANTHROPIC_API_KEY из cfg.LLM.Anthropic.APIKey (backwards compat).
type SandboxAuthEnvResolver interface {
	Resolve(ctx context.Context, project *models.Project, agent *models.Agent) sandbox.ClaudeCodeAuthEnv
}

// UserLLMCredentialResolver — узкий интерфейс, который нужен резолверу: достать plaintext-ключ
// конкретного пользователя для kind. Реализуется service.UserLlmCredentialService.
type UserLLMCredentialResolver interface {
	GetPlaintext(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (string, error)
}

// sandboxAuthEnvResolver — реализация по умолчанию.
type sandboxAuthEnvResolver struct {
	claudeCodeAuth ClaudeCodeAuthService
	userCreds      UserLLMCredentialResolver
	fallbackAPIKey string
	logger         *slog.Logger
}

// NewSandboxAuthEnvResolver собирает резолвер.
//   - claudeCodeAuth может быть nil (фича OAuth выключена — kind=anthropic_oauth тогда не работает).
//   - userCreds может быть nil (тогда kind=anthropic/deepseek/zhipu/openrouter не сработают).
//   - fallbackAPIKey — статический ANTHROPIC_API_KEY (для агентов без ProviderKind).
func NewSandboxAuthEnvResolver(
	claudeCodeAuth ClaudeCodeAuthService,
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
		if err != nil {
			logger.Warn("user credential lookup failed", "user_provider", string(userProvider), "err", err)
			return env
		}
		if key == "" {
			logger.Warn("user has no credential for provider", "user_provider", string(userProvider))
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
