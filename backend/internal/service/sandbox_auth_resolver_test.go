package service

import (
	"context"
	"errors"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// --- стабы зависимостей резолвера -------------------------------------------------------

type stubClaudeCodeAuthSvc struct {
	token string
	err   error
}

func (s *stubClaudeCodeAuthSvc) InitDeviceCode(ctx context.Context, userID uuid.UUID) (*ClaudeCodeDeviceInit, error) {
	return nil, errors.New("not used")
}
func (s *stubClaudeCodeAuthSvc) CompleteDeviceCode(ctx context.Context, userID uuid.UUID, deviceCode string) (*ClaudeCodeAuthStatus, error) {
	return nil, errors.New("not used")
}
func (s *stubClaudeCodeAuthSvc) Status(ctx context.Context, userID uuid.UUID) (*ClaudeCodeAuthStatus, error) {
	return nil, errors.New("not used")
}
func (s *stubClaudeCodeAuthSvc) Revoke(ctx context.Context, userID uuid.UUID) error {
	return errors.New("not used")
}
func (s *stubClaudeCodeAuthSvc) AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.token, s.err
}
func (s *stubClaudeCodeAuthSvc) RefreshOne(ctx context.Context, sub *models.ClaudeCodeSubscription) error {
	return errors.New("not used")
}

type stubUserCreds struct {
	byProvider map[models.UserLLMProvider]string
	err        error
}

func (s *stubUserCreds) GetPlaintext(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	return s.byProvider[provider], nil
}

func newProject() *models.Project {
	return &models.Project{ID: uuid.New(), UserID: uuid.New()}
}

func kindPtr(k models.AgentProviderKind) *models.AgentProviderKind { return &k }

// --- тесты ---------------------------------------------------------------------------------

func TestResolver_Anthropic_UsesUserKey(t *testing.T) {
	user := &stubUserCreds{byProvider: map[models.UserLLMProvider]string{
		models.UserLLMProviderAnthropic: "sk-ant-api03-USER",
	}}
	r := NewSandboxAuthEnvResolver(nil, user, "FALLBACK", nil)

	env := r.Resolve(context.Background(), newProject(),
		&models.Agent{ProviderKind: kindPtr(models.AgentProviderKindAnthropic)})

	assert.Equal(t, "sk-ant-api03-USER", env.APIKey)
	assert.Empty(t, env.BaseURL)
	assert.Empty(t, env.AuthToken)
	assert.Empty(t, env.OAuthToken)
}

func TestResolver_AnthropicOAuth_UsesSubscriptionToken(t *testing.T) {
	auth := &stubClaudeCodeAuthSvc{token: "sk-ant-oat01-XYZ"}
	r := NewSandboxAuthEnvResolver(auth, nil, "FALLBACK", nil)

	env := r.Resolve(context.Background(), newProject(),
		&models.Agent{ProviderKind: kindPtr(models.AgentProviderKindAnthropicOAuth)})

	assert.Equal(t, "sk-ant-oat01-XYZ", env.OAuthToken)
	assert.Empty(t, env.APIKey)
	assert.Empty(t, env.BaseURL)
	assert.Empty(t, env.AuthToken)
}

func TestResolver_DeepSeek_UsesNativeAnthropicEndpoint(t *testing.T) {
	user := &stubUserCreds{byProvider: map[models.UserLLMProvider]string{
		models.UserLLMProviderDeepSeek: "sk-deepseek-USER",
	}}
	r := NewSandboxAuthEnvResolver(nil, user, "FALLBACK", nil)

	env := r.Resolve(context.Background(), newProject(),
		&models.Agent{ProviderKind: kindPtr(models.AgentProviderKindDeepSeek)})

	assert.Equal(t, "https://api.deepseek.com/anthropic", env.BaseURL)
	assert.Equal(t, "sk-deepseek-USER", env.AuthToken)
	assert.Empty(t, env.APIKey, "ANTHROPIC_API_KEY must NOT leak when using DeepSeek native endpoint")
	assert.Empty(t, env.OAuthToken)
}

func TestResolver_Zhipu_UsesNativeAnthropicEndpoint(t *testing.T) {
	user := &stubUserCreds{byProvider: map[models.UserLLMProvider]string{
		models.UserLLMProviderZhipu: "glm-key-USER",
	}}
	r := NewSandboxAuthEnvResolver(nil, user, "FALLBACK", nil)

	env := r.Resolve(context.Background(), newProject(),
		&models.Agent{ProviderKind: kindPtr(models.AgentProviderKindZhipu)})

	assert.Equal(t, "https://open.bigmodel.cn/api/anthropic", env.BaseURL)
	assert.Equal(t, "glm-key-USER", env.AuthToken)
	assert.Empty(t, env.APIKey)
}

func TestResolver_NoKind_FallbackToOAuthThenAPIKey(t *testing.T) {
	// OAuth есть → OAuth.
	auth := &stubClaudeCodeAuthSvc{token: "sk-ant-oat01-FROM-SUB"}
	r := NewSandboxAuthEnvResolver(auth, nil, "STATIC", nil)
	env := r.Resolve(context.Background(), newProject(), &models.Agent{})
	assert.Equal(t, "sk-ant-oat01-FROM-SUB", env.OAuthToken)
	assert.Empty(t, env.APIKey)

	// OAuth выключен → static API key.
	r = NewSandboxAuthEnvResolver(nil, nil, "STATIC", nil)
	env = r.Resolve(context.Background(), newProject(), &models.Agent{})
	assert.Equal(t, "STATIC", env.APIKey)
	assert.Empty(t, env.OAuthToken)
}

func TestResolver_DeepSeek_UserHasNoKey_ReturnsEmpty(t *testing.T) {
	user := &stubUserCreds{byProvider: map[models.UserLLMProvider]string{}}
	r := NewSandboxAuthEnvResolver(nil, user, "FALLBACK", nil)

	env := r.Resolve(context.Background(), newProject(),
		&models.Agent{ProviderKind: kindPtr(models.AgentProviderKindDeepSeek)})

	// Никакого fallback на ANTHROPIC_API_KEY: kind=deepseek без ключа → пустой env,
	// sandbox получит «нет креденшелов» и упадёт явно, а не позовёт чужой провайдер.
	assert.False(t, env.HasCredential())
}
