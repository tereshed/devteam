package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/devteam/backend/pkg/agentsloader"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// Sprint 15.35 — E2E (provider): агент с DeepSeek через free-claude-proxy
// получает только ANTHROPIC_BASE_URL + ANTHROPIC_AUTH_TOKEN; ни ANTHROPIC_API_KEY,
// ни CLAUDE_CODE_OAUTH_TOKEN не должны попасть в env sandbox-а.
//
// Тест проверяет ContextBuilder.Build() + SandboxAuthEnvResolver конец-в-конец
// (без реального Docker — это покрывается sandbox/sandbox_real_test.go).
func TestSprint15_35_SandboxEnv_FreeClaudeProxy_Path(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	oauth := &stubOAuthProvider{} // подписки нет
	authSvc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)

	resolver := NewSandboxAuthEnvResolver(
		authSvc,
		FreeClaudeProxyAccess{
			BaseURL:      "http://free-claude-proxy:8787",
			ServiceToken: "svc-tok-1",
		},
		"static-anthropic-key-should-not-leak",
		nil,
	)

	// Контекст-билдер с резолвером (без taskMsgRepo/composer/cache — они не критичны для проверки env).
	cb := NewContextBuilderFull(NoopEncryptor{}, nil, nil, nil, nil)
	cb = WithSandboxAuthResolver(cb, resolver)

	deepseekProviderID := uuid.New()
	codeBackend := models.CodeBackendClaudeCodeViaProxy
	role := models.AgentRoleDeveloper
	a := &models.Agent{
		ID:            uuid.New(),
		Role:          role,
		LLMProviderID: &deepseekProviderID,
		CodeBackend:   &codeBackend,
	}
	p := &models.Project{ID: uuid.New(), UserID: uuid.New()}
	task := &models.Task{ID: uuid.New(), ProjectID: p.ID, Title: "t", Description: "d"}

	in, err := cb.Build(context.Background(), task, a, p)
	require.NoError(t, err)
	require.NotNil(t, in)

	assert.Equal(t, "http://free-claude-proxy:8787",
		in.EnvSecrets[sandbox.EnvAnthropicBaseURL])
	assert.Equal(t, "svc-tok-1",
		in.EnvSecrets[sandbox.EnvAnthropicAuthToken])
	_, hasAPIKey := in.EnvSecrets[sandbox.EnvAnthropicAPIKey]
	_, hasOAuth := in.EnvSecrets[sandbox.EnvClaudeCodeOAuthToken]
	assert.False(t, hasAPIKey, "ANTHROPIC_API_KEY must NOT be present in proxy mode")
	assert.False(t, hasOAuth, "CLAUDE_CODE_OAUTH_TOKEN must NOT be present in proxy mode")
}

// Дополнительный smoke на orchestrator.Start → fail-fast health-check (Sprint 15.19).
func TestSprint15_35_OrchestratorFailsFastWhenProxyDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	checker := NewFreeClaudeProxyHealthCheck(srv.URL, "")
	require.Error(t, checker.Check(context.Background()),
		"unhealthy proxy must surface as error so orchestrator can fail-fast")
}

// Заглушки, чтобы тесту не нужен был полностью смонтированный кэш агентов и т.п.
var _ = []interface{}{
	&agentsloader.AgentConfig{},
	(*agent.ExecutionInput)(nil),
	(*repository.TaskMessageRepository)(nil),
	datatypes.JSON{},
}
