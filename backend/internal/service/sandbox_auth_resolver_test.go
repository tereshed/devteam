package service

import (
	"context"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeAgent(backend models.CodeBackend) *models.Agent {
	b := backend
	return &models.Agent{ID: uuid.New(), CodeBackend: &b}
}

func TestSandboxAuthResolver_FallbackToAPIKey(t *testing.T) {
	r := NewSandboxAuthEnvResolver(nil, FreeClaudeProxyAccess{}, "static-api-key", nil)
	env := r.Resolve(context.Background(), &models.Project{ID: uuid.New(), UserID: uuid.New()}, makeAgent(models.CodeBackendClaudeCode))
	assert.Equal(t, "static-api-key", env.APIKey)
	assert.Empty(t, env.OAuthToken)
	assert.Empty(t, env.ProxyBaseURL)
}

func TestSandboxAuthResolver_ProxyMode(t *testing.T) {
	r := NewSandboxAuthEnvResolver(nil, FreeClaudeProxyAccess{
		BaseURL: "http://free-claude-proxy:8787", ServiceToken: "svc-tok",
	}, "static-api-key", nil)
	env := r.Resolve(context.Background(), &models.Project{ID: uuid.New(), UserID: uuid.New()}, makeAgent(models.CodeBackendClaudeCodeViaProxy))
	out := env.ToEnv()
	assert.Equal(t, "http://free-claude-proxy:8787", out[sandbox.EnvAnthropicBaseURL])
	assert.Equal(t, "svc-tok", out[sandbox.EnvAnthropicAuthToken])
	// В proxy-режиме нет ни OAuth, ни статического API-ключа.
	_, hasOAuth := out[sandbox.EnvClaudeCodeOAuthToken]
	_, hasAPIKey := out[sandbox.EnvAnthropicAPIKey]
	assert.False(t, hasOAuth)
	assert.False(t, hasAPIKey)
}

func TestSandboxAuthResolver_OAuthSubscription(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	oauth := &stubOAuthProvider{
		pollFn: func(_ context.Context, _ string) (*ClaudeCodeOAuthToken, error) {
			exp := time.Now().Add(time.Hour)
			return &ClaudeCodeOAuthToken{AccessToken: "sub-token", RefreshToken: "r", TokenType: "Bearer", ExpiresAt: &exp}, nil
		},
	}
	uid := uuid.New()
	authSvc := seedDeviceCode(NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth), uid, "dc")
	_, err := authSvc.CompleteDeviceCode(context.Background(), uid, "dc")
	require.NoError(t, err)

	r := NewSandboxAuthEnvResolver(authSvc, FreeClaudeProxyAccess{}, "static-api-key", nil)
	env := r.Resolve(context.Background(), &models.Project{ID: uuid.New(), UserID: uid}, makeAgent(models.CodeBackendClaudeCode))
	assert.Equal(t, "sub-token", env.OAuthToken)
	assert.Empty(t, env.APIKey, "OAuth должен иметь приоритет над static API key")
}

func TestSandboxAuthResolver_OAuthMissing_FallsBackToAPIKey(t *testing.T) {
	repo := newMockClaudeCodeSubRepo()
	oauth := &stubOAuthProvider{}
	authSvc := NewClaudeCodeAuthService(repo, NoopEncryptor{}, oauth)

	r := NewSandboxAuthEnvResolver(authSvc, FreeClaudeProxyAccess{}, "static-api-key", nil)
	env := r.Resolve(context.Background(), &models.Project{ID: uuid.New(), UserID: uuid.New()}, makeAgent(models.CodeBackendClaudeCode))
	assert.Equal(t, "static-api-key", env.APIKey)
	// убедиться, что ErrClaudeCodeSubscriptionNotFound не превращён в фатал, а просто молча проигнорирован
	_ = repository.ErrClaudeCodeSubscriptionNotFound
}
