package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
)

// SandboxAuthEnvResolver — подбирает аутентификационные env для sandbox-агента (Sprint 15.18).
//
// Логика по приоритету (от агента к проекту):
//   1) Agent.CodeBackend == claude-code-via-proxy → ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN
//      (URL и сервис-токен берутся из FreeClaudeProxyConfig).
//   2) У владельца проекта есть OAuth-подписка Claude Code → CLAUDE_CODE_OAUTH_TOKEN.
//   3) Fallback на ANTHROPIC_API_KEY из статических sandboxSecrets.
type SandboxAuthEnvResolver interface {
	Resolve(ctx context.Context, project *models.Project, agent *models.Agent) sandbox.ClaudeCodeAuthEnv
}

// FreeClaudeProxyAccess — параметры подключения sandbox-а к free-claude-proxy (Sprint 15.18).
type FreeClaudeProxyAccess struct {
	BaseURL      string
	ServiceToken string
}

// sandboxAuthEnvResolver — реализация по умолчанию.
type sandboxAuthEnvResolver struct {
	claudeCodeAuth ClaudeCodeAuthService
	proxyAccess    FreeClaudeProxyAccess
	fallbackAPIKey string
	logger         *slog.Logger
}

// NewSandboxAuthEnvResolver собирает резолвер. claudeCodeAuth может быть nil (фича выключена).
// proxyAccess может быть пустым (если прокси не настроен).
// fallbackAPIKey — статический ANTHROPIC_API_KEY из cfg.LLM.Anthropic.APIKey.
func NewSandboxAuthEnvResolver(
	claudeCodeAuth ClaudeCodeAuthService,
	proxyAccess FreeClaudeProxyAccess,
	fallbackAPIKey string,
	logger *slog.Logger,
) SandboxAuthEnvResolver {
	if logger == nil {
		logger = slog.Default()
	}
	return &sandboxAuthEnvResolver{
		claudeCodeAuth: claudeCodeAuth,
		proxyAccess:    proxyAccess,
		fallbackAPIKey: fallbackAPIKey,
		logger:         logger.With("component", "sandbox_auth_env_resolver"),
	}
}

func (r *sandboxAuthEnvResolver) Resolve(ctx context.Context, project *models.Project, agent *models.Agent) sandbox.ClaudeCodeAuthEnv {
	env := sandbox.ClaudeCodeAuthEnv{}

	// 1) Если агент явно настроен через прокси — выставляем proxy URL+token.
	if agent != nil && agent.CodeBackend != nil && *agent.CodeBackend == models.CodeBackendClaudeCodeViaProxy {
		env.ProxyBaseURL = r.proxyAccess.BaseURL
		env.ProxyAuthToken = r.proxyAccess.ServiceToken
		return env
	}

	// 2) OAuth-подписка владельца проекта (если фича включена).
	if r.claudeCodeAuth != nil && project != nil && project.UserID != [16]byte{} {
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

	// 3) Fallback на классический API-ключ.
	env.APIKey = r.fallbackAPIKey
	return env
}
