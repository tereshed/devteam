package service

import (
	"context"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// --- стабы зависимостей резолвера -------------------------------------------------------

// stubClaudeCodeAuthSvc удовлетворяет ClaudeCodeOAuthAccessor (узкий интерфейс).
// Полный ClaudeCodeAuthService нам не нужен — при добавлении новых методов в сервис
// этот стаб не сломается.
type stubClaudeCodeAuthSvc struct {
	token string
	err   error
}

func (s *stubClaudeCodeAuthSvc) AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error) {
	return s.token, s.err
}

type stubUserCreds struct {
	// Sprint 15.e2e: имитируем sentinel-контракт GetPlaintext — отсутствующий
	// в map ключ возвращает ErrUserLlmCredentialNotFound, а не ("", nil).
	byProvider map[models.UserLLMProvider]string
	err        error
}

func (s *stubUserCreds) GetPlaintext(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	v, ok := s.byProvider[provider]
	if !ok {
		return "", repository.ErrUserLlmCredentialNotFound
	}
	return v, nil
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

// Sprint 15.e2e ревью #2: «не найдено» (ErrUserLlmCredentialNotFound) и «ошибка»
// (любая другая) трактуются одинаково на исход (пустой env), но через разные
// ветки. Тест защищает резолвер от регрессии «error → empty key → silent
// fallback на чужой провайдер».
func TestResolver_DeepSeek_LookupErrorDoesNotLeakToOtherProvider(t *testing.T) {
	user := &stubUserCreds{err: assert.AnError}
	r := NewSandboxAuthEnvResolver(nil, user, "FALLBACK-SHOULD-NOT-LEAK", nil)

	env := r.Resolve(context.Background(), newProject(),
		&models.Agent{ProviderKind: kindPtr(models.AgentProviderKindDeepSeek)})

	assert.False(t, env.HasCredential())
	assert.NotEqual(t, "FALLBACK-SHOULD-NOT-LEAK", env.APIKey)
	assert.Empty(t, env.BaseURL)
}
