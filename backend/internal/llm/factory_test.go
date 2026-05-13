package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

func staticSecret(secret string) SecretsResolver {
	return SecretsResolverFunc(func(ctx context.Context, _ *models.LLMProvider) (string, error) {
		return secret, nil
	})
}

func TestNewLLMClient_DisabledProvider(t *testing.T) {
	p := &models.LLMProvider{ID: uuid.New(), Kind: models.LLMProviderKindOpenRouter, Enabled: false}
	_, err := NewLLMClient(context.Background(), p, staticSecret("k"), nil)
	if !errors.Is(err, ErrProviderDisabled) {
		t.Fatalf("expected ErrProviderDisabled, got %v", err)
	}
}

func TestNewLLMClient_UnsupportedKind(t *testing.T) {
	p := &models.LLMProvider{ID: uuid.New(), Kind: "wat", Enabled: true, AuthType: models.LLMProviderAuthNone}
	_, err := NewLLMClient(context.Background(), p, staticSecret(""), nil)
	if !errors.Is(err, ErrUnsupportedKind) {
		t.Fatalf("expected ErrUnsupportedKind, got %v", err)
	}
}

func TestNewLLMClient_OpenRouter_HealthCheck(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/models" {
			t.Fatalf("expected /models, got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	p := &models.LLMProvider{
		ID:                   uuid.New(),
		Kind:                 models.LLMProviderKindOpenRouter,
		BaseURL:              srv.URL,
		AuthType:             models.LLMProviderAuthAPIKey,
		Enabled:              true,
		CredentialsEncrypted: []byte("non-empty"),
	}

	c, err := NewLLMClient(context.Background(), p, staticSecret("openrouter-key"), nil)
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}
	if c.ResolveBaseURL() != srv.URL {
		t.Fatalf("expected BaseURL=%s, got %s", srv.URL, c.ResolveBaseURL())
	}
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if receivedAuth != "Bearer openrouter-key" {
		t.Fatalf("expected resolved secret in auth header, got %q", receivedAuth)
	}
}

func TestNewLLMClient_FreeClaudeProxy_HealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := &models.LLMProvider{
		ID:                   uuid.New(),
		Kind:                 models.LLMProviderKindFreeClaudeProxy,
		BaseURL:              srv.URL,
		AuthType:             models.LLMProviderAuthBearer,
		Enabled:              true,
		CredentialsEncrypted: []byte("blob"),
	}
	c, err := NewLLMClient(context.Background(), p, staticSecret("svc-token"), nil)
	if err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestNewLLMClient_AuthNoneSkipsResolver(t *testing.T) {
	calls := 0
	resolver := SecretsResolverFunc(func(ctx context.Context, _ *models.LLMProvider) (string, error) {
		calls++
		return "should-not-be-used", nil
	})
	p := &models.LLMProvider{
		ID:       uuid.New(),
		Kind:     models.LLMProviderKindOllama,
		BaseURL:  "http://example.invalid",
		AuthType: models.LLMProviderAuthNone,
		Enabled:  true,
	}
	if _, err := NewLLMClient(context.Background(), p, resolver, nil); err != nil {
		t.Fatalf("NewLLMClient: %v", err)
	}
	if calls != 0 {
		t.Fatalf("resolver must not be called when auth_type=none, called %d times", calls)
	}
}
