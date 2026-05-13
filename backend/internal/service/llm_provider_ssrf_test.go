package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sprint 15.N8 — SSRF-safe http.Client должен использоваться provider'ом при HealthCheck/TestConnection,
// не только validateBaseURLForProvider. Эти тесты доказывают, что (а) пре-валидация рубит loopback
// для non-local kind, (б) даже если URL прошёл pre-check, post-resolve guard в Dialer ловит 30x на
// private/metadata host.

func TestLLMProviderService_TestConnection_BlocksLoopbackForNonLocalKind(t *testing.T) {
	// httptest сервер на loopback. kind=openrouter — loopback запрещён.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{})
	err := svc.TestConnection(context.Background(), LLMProviderInput{
		Name: "Bad", Kind: models.LLMProviderKindOpenRouter,
		BaseURL: srv.URL, AuthType: models.LLMProviderAuthAPIKey,
		Credential: "k", Enabled: true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInsecureBaseURL),
		"pre-validate guard must reject http://loopback for openrouter, got %v", err)
}

// Дополнительно проверяем что real HTTP client из HTTPClientFactory тоже SSRF-safe:
// если кто-нибудь обойдёт pre-validate (например через DNS rebinding к private IP),
// сам Dialer.Control откажет в коннекте.
func TestLLMProviderService_HTTPClient_DialControl_BlocksRFC1918(t *testing.T) {
	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{}).(*llmProviderService)
	client := svc.HTTPClient(&models.LLMProvider{Kind: models.LLMProviderKindOpenRouter})

	// 10.0.0.1 — RFC1918. Dial должен зарезать.
	req, _ := http.NewRequest(http.MethodGet, "http://10.0.0.1/", nil)
	_, err := client.Do(req)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "blocked") ||
		strings.Contains(err.Error(), "insecure"),
		"expected SSRF-block error, got: %v", err)
}

func TestLLMProviderService_HTTPClient_LoopbackAllowedForOllama(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`ok`))
	}))
	defer srv.Close()
	svc := NewLLMProviderService(newMockLLMProviderRepo(), NoopEncryptor{}).(*llmProviderService)
	client := svc.HTTPClient(&models.LLMProvider{Kind: models.LLMProviderKindOllama})
	resp, err := client.Get(srv.URL)
	require.NoError(t, err, "ollama kind must allow loopback")
	resp.Body.Close()
}
