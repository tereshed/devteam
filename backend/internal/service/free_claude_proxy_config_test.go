package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimal mock of LLMProviderRepository for tests
type mockLLMProvidersForProxy struct {
	mu   sync.Mutex
	rows []models.LLMProvider
}

func (m *mockLLMProvidersForProxy) Create(context.Context, *models.LLMProvider) error { return nil }
func (m *mockLLMProvidersForProxy) GetByID(_ context.Context, id uuid.UUID) (*models.LLMProvider, error) {
	for i := range m.rows {
		if m.rows[i].ID == id {
			c := m.rows[i]
			return &c, nil
		}
	}
	return nil, repository.ErrLLMProviderNotFound
}
func (m *mockLLMProvidersForProxy) GetByName(context.Context, string) (*models.LLMProvider, error) {
	return nil, repository.ErrLLMProviderNotFound
}
func (m *mockLLMProvidersForProxy) List(_ context.Context, _ bool) ([]models.LLMProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	clone := make([]models.LLMProvider, len(m.rows))
	copy(clone, m.rows)
	return clone, nil
}
func (m *mockLLMProvidersForProxy) Update(context.Context, *models.LLMProvider) error { return nil }
func (m *mockLLMProvidersForProxy) Delete(context.Context, uuid.UUID) error            { return nil }

type staticSecrets struct{ key string }

func (s staticSecrets) ResolveCredentials(_ context.Context, _ *models.LLMProvider) (string, error) {
	return s.key, nil
}

func TestFreeClaudeProxyConfig_Build_SkipsAnthropicAndSelf(t *testing.T) {
	repo := &mockLLMProvidersForProxy{rows: []models.LLMProvider{
		{ID: uuid.New(), Name: "anthropic-main", Kind: models.LLMProviderKindAnthropic, Enabled: true},
		{ID: uuid.New(), Name: "self-proxy", Kind: models.LLMProviderKindFreeClaudeProxy, Enabled: true},
		{ID: uuid.New(), Name: "openrouter", Kind: models.LLMProviderKindOpenRouter, BaseURL: "https://openrouter.ai/api/v1", DefaultModel: "openrouter/auto", Enabled: true},
		{ID: uuid.New(), Name: "deepseek", Kind: models.LLMProviderKindDeepSeek, BaseURL: "https://api.deepseek.com/v1", DefaultModel: "deepseek-chat", Enabled: true},
	}}
	b := NewFreeClaudeProxyConfigBuilder(repo, staticSecrets{key: "k"}, 8787, "svc-token")
	cfg, err := b.Build(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 8787, cfg.Port)
	assert.Equal(t, "svc-token", cfg.ServiceToken)
	require.Len(t, cfg.Providers, 2, "anthropic and self should be skipped")
	names := []string{cfg.Providers[0].Name, cfg.Providers[1].Name}
	assert.Contains(t, names, "openrouter")
	assert.Contains(t, names, "deepseek")
	// Все API-ключи из secrets-резолвера.
	for _, p := range cfg.Providers {
		assert.Equal(t, "k", p.APIKey)
	}
	// Routing проложен на первого по алфавиту провайдера.
	first := cfg.Providers[0].Name
	assert.Equal(t, first, cfg.Routes["claude-3-5-sonnet-20240620"])
}

func TestFreeClaudeProxyConfig_WriteFile_AtomicRename(t *testing.T) {
	repo := &mockLLMProvidersForProxy{rows: []models.LLMProvider{
		{ID: uuid.New(), Name: "openrouter", Kind: models.LLMProviderKindOpenRouter, BaseURL: "https://openrouter.ai/api/v1", Enabled: true},
	}}
	b := NewFreeClaudeProxyConfigBuilder(repo, staticSecrets{key: "k"}, 0, "svc")
	dir := t.TempDir()
	target := filepath.Join(dir, "subdir", "config.yaml")

	require.NoError(t, b.WriteFile(context.Background(), target))
	data, err := os.ReadFile(target)
	require.NoError(t, err)
	str := string(data)
	assert.True(t, strings.Contains(str, "openrouter"))
	assert.True(t, strings.Contains(str, "service_token: svc"))
	// Не должно быть .tmp файлов рядом.
	entries, _ := os.ReadDir(filepath.Dir(target))
	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"), "stale .tmp left: %s", e.Name())
	}
}
