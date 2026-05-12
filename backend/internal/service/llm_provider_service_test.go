package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- in-memory mock LLMProviderRepository ---

type mockLLMProviderRepo struct {
	mu      sync.Mutex
	byID    map[uuid.UUID]*models.LLMProvider
	createErr error
}

func newMockLLMProviderRepo() *mockLLMProviderRepo {
	return &mockLLMProviderRepo{byID: map[uuid.UUID]*models.LLMProvider{}}
}

func (m *mockLLMProviderRepo) Create(_ context.Context, p *models.LLMProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	for _, existing := range m.byID {
		if existing.Name == p.Name {
			return repository.ErrLLMProviderNameExists
		}
	}
	clone := *p
	m.byID[p.ID] = &clone
	return nil
}
func (m *mockLLMProviderRepo) GetByID(_ context.Context, id uuid.UUID) (*models.LLMProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.byID[id]
	if !ok {
		return nil, repository.ErrLLMProviderNotFound
	}
	clone := *p
	return &clone, nil
}
func (m *mockLLMProviderRepo) GetByName(_ context.Context, name string) (*models.LLMProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.byID {
		if p.Name == name {
			clone := *p
			return &clone, nil
		}
	}
	return nil, repository.ErrLLMProviderNotFound
}
func (m *mockLLMProviderRepo) List(_ context.Context, onlyEnabled bool) ([]models.LLMProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]models.LLMProvider, 0, len(m.byID))
	for _, p := range m.byID {
		if onlyEnabled && !p.Enabled {
			continue
		}
		out = append(out, *p)
	}
	return out, nil
}
func (m *mockLLMProviderRepo) Update(_ context.Context, p *models.LLMProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[p.ID]; !ok {
		return repository.ErrLLMProviderNotFound
	}
	clone := *p
	m.byID[p.ID] = &clone
	return nil
}
func (m *mockLLMProviderRepo) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[id]; !ok {
		return repository.ErrLLMProviderNotFound
	}
	delete(m.byID, id)
	return nil
}

// --- tests ---

func TestLLMProviderService_Create_EncryptsCredentials(t *testing.T) {
	repo := newMockLLMProviderRepo()
	svc := NewLLMProviderService(repo, NoopEncryptor{})

	in := LLMProviderInput{
		Name:         "OpenRouter prod",
		Kind:         models.LLMProviderKindOpenRouter,
		BaseURL:      "https://openrouter.ai/api/v1",
		AuthType:     models.LLMProviderAuthAPIKey,
		Credential:   "sk-test",
		DefaultModel: "openrouter/auto",
		Enabled:      true,
	}
	p, err := svc.Create(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, p)
	// NoopEncryptor сохраняет plaintext в credentials_encrypted — этого достаточно для проверки,
	// что Create вызывает шифрование и записывает blob, а Update подхватывает поле.
	assert.Equal(t, []byte("sk-test"), p.CredentialsEncrypted)
}

func TestLLMProviderService_Create_ValidationErrors(t *testing.T) {
	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{})

	_, err := svc.Create(context.Background(), LLMProviderInput{
		Kind: models.LLMProviderKindOpenRouter, AuthType: models.LLMProviderAuthAPIKey,
	})
	assert.ErrorIs(t, err, ErrLLMProviderInvalid)

	_, err = svc.Create(context.Background(), LLMProviderInput{Name: "x", Kind: "bogus", AuthType: models.LLMProviderAuthAPIKey})
	assert.ErrorIs(t, err, ErrLLMProviderInvalid)

	_, err = svc.Create(context.Background(), LLMProviderInput{Name: "x", Kind: models.LLMProviderKindOpenAI, AuthType: "fake"})
	assert.ErrorIs(t, err, ErrLLMProviderInvalid)
}

func TestLLMProviderService_ResolveCredentials_RoundTrip(t *testing.T) {
	repo := newMockLLMProviderRepo()
	svc := NewLLMProviderService(repo, NoopEncryptor{})

	p, err := svc.Create(context.Background(), LLMProviderInput{
		Name: "OR", Kind: models.LLMProviderKindOpenRouter,
		BaseURL: "https://example.com", AuthType: models.LLMProviderAuthAPIKey,
		Credential: "secret", DefaultModel: "m", Enabled: true,
	})
	require.NoError(t, err)

	got, err := svc.ResolveCredentials(context.Background(), p)
	require.NoError(t, err)
	assert.Equal(t, "secret", got)
}

func TestLLMProviderService_HealthCheck_CallsProvider(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/models") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		hits++
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	repo := newMockLLMProviderRepo()
	svc := NewLLMProviderService(repo, NoopEncryptor{})

	p, err := svc.Create(context.Background(), LLMProviderInput{
		Name: "OR", Kind: models.LLMProviderKindOpenRouter,
		BaseURL: srv.URL, AuthType: models.LLMProviderAuthAPIKey,
		Credential: "k", DefaultModel: "m", Enabled: true,
	})
	require.NoError(t, err)
	require.NoError(t, svc.HealthCheck(context.Background(), p.ID))
	assert.Equal(t, 1, hits)
}

func TestLLMProviderService_HealthCheck_ProviderDisabled(t *testing.T) {
	repo := newMockLLMProviderRepo()
	svc := NewLLMProviderService(repo, NoopEncryptor{})
	p, err := svc.Create(context.Background(), LLMProviderInput{
		Name: "Ollama", Kind: models.LLMProviderKindOllama,
		BaseURL: "http://example.invalid", AuthType: models.LLMProviderAuthNone, Enabled: false,
	})
	require.NoError(t, err)
	err = svc.HealthCheck(context.Background(), p.ID)
	require.Error(t, err)
}

func TestLLMProviderService_TestConnection_UsesProvidedCredential(t *testing.T) {
	var authSeen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authSeen = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{})
	err := svc.TestConnection(context.Background(), LLMProviderInput{
		Name: "OR", Kind: models.LLMProviderKindOpenRouter,
		BaseURL: srv.URL, AuthType: models.LLMProviderAuthAPIKey,
		Credential: "live-key", DefaultModel: "m", Enabled: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "Bearer live-key", authSeen)
}

func TestLLMProviderService_Update_NotFound(t *testing.T) {
	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{})
	_, err := svc.Update(context.Background(), uuid.New(), LLMProviderInput{
		Name: "x", Kind: models.LLMProviderKindOpenAI, AuthType: models.LLMProviderAuthAPIKey,
	})
	require.True(t, errors.Is(err, repository.ErrLLMProviderNotFound), "got %v", err)
}
