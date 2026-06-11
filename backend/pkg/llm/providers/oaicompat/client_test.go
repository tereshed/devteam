package oaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/pkg/llm"
)

func TestClient_Generate_OK(t *testing.T) {
	var capturedAuth, capturedReferer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")
		capturedReferer = r.Header.Get("HTTP-Referer")

		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("body parse: %v", err)
		}
		if parsed["model"] != "test-model" {
			t.Fatalf("expected default model passed, got %v", parsed["model"])
		}
		_, _ = w.Write([]byte(`{
            "choices": [{
                "message": {"role":"assistant","content":"hi"},
                "finish_reason":"stop"
            }],
            "usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
        }`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		APIKey:       "secret",
		BaseURL:      srv.URL,
		DefaultModel: "test-model",
		ExtraHeaders: map[string]string{"HTTP-Referer": "https://example.com"},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := c.Generate(context.Background(), llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
	}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Content != "hi" {
		t.Fatalf("expected content 'hi', got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 3 {
		t.Fatalf("expected total_tokens=3, got %d", resp.Usage.TotalTokens)
	}
	if capturedAuth != "Bearer secret" {
		t.Fatalf("expected Bearer secret auth, got %q", capturedAuth)
	}
	if capturedReferer != "https://example.com" {
		t.Fatalf("expected HTTP-Referer header to propagate, got %q", capturedReferer)
	}
}

func TestClient_Generate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()

	c, _ := NewClient(Config{APIKey: "k", BaseURL: srv.URL, DefaultModel: "m"})
	_, err := c.Generate(context.Background(), llm.Request{Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}})
	if err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("expected status 401 error, got %v", err)
	}
}

func TestClient_HealthCheck_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	c, _ := NewClient(Config{APIKey: "k", BaseURL: srv.URL, DefaultModel: "m"})
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestClient_HealthCheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c, _ := NewClient(Config{APIKey: "k", BaseURL: srv.URL, DefaultModel: "m"})
	err := c.HealthCheck(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("expected status 502 error, got %v", err)
	}
}

func TestNewClient_RequiresBaseURL(t *testing.T) {
	if _, err := NewClient(Config{APIKey: "k"}); err == nil {
		t.Fatalf("expected error when BaseURL is empty")
	}
}

// Sprint 15.M8 regression — retry на 503 + finally 200.
func TestClient_Generate_RetriesOn503(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{
            "choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
            "usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
        }`))
	}))
	defer srv.Close()

	// Подменяем http.Client с коротким таймаутом, чтобы тест шёл быстро.
	c, _ := NewClient(Config{
		APIKey: "k", BaseURL: srv.URL, DefaultModel: "m",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	})

	resp, err := c.Generate(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}},
	})
	if err != nil {
		t.Fatalf("Generate after retries: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if calls < 3 {
		t.Fatalf("expected ≥3 calls, got %d", calls)
	}
}

// Sprint 15.M8 — после maxRetries попыток возвращаем последнюю ошибку.
func TestClient_Generate_GivesUpAfterMaxRetries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c, _ := NewClient(Config{
		APIKey: "k", BaseURL: srv.URL, DefaultModel: "m",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	})
	_, err := c.Generate(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}},
	})
	if err == nil || !strings.Contains(err.Error(), "status 429") {
		t.Fatalf("expected status 429 after retries, got %v", err)
	}
}

// TestClient_Generate_ServerToolsAndCitations — server-side тулы (openrouter:web_search)
// сериализуются в tools как есть рядом с function-тулами, а annotations/url_citation
// из ответа дописываются к контенту блоком «Источники» (дубли URL схлопываются).
func TestClient_Generate_ServerToolsAndCitations(t *testing.T) {
	var capturedTools []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("body parse: %v", err)
		}
		capturedTools, _ = parsed["tools"].([]any)
		_, _ = w.Write([]byte(`{
            "choices": [{
                "message": {
                    "role": "assistant",
                    "content": "ответ с учётом поиска",
                    "annotations": [
                        {"type":"url_citation","url_citation":{"url":"https://a.example/doc","title":"Doc A"}},
                        {"type":"url_citation","url_citation":{"url":"https://a.example/doc","title":"Doc A dup"}},
                        {"type":"url_citation","url_citation":{"url":"https://b.example","title":""}}
                    ]
                },
                "finish_reason": "stop"
            }],
            "usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}
        }`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{APIKey: "k", BaseURL: srv.URL, DefaultModel: "m"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := c.Generate(context.Background(), llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "q"}},
		Tools: []llm.Tool{
			{Name: "task_create", Description: "create task", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
		ServerTools: []map[string]any{{"type": "openrouter:web_search"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if len(capturedTools) != 2 {
		t.Fatalf("expected 2 tools (function + server), got %d: %v", len(capturedTools), capturedTools)
	}
	first, _ := capturedTools[0].(map[string]any)
	second, _ := capturedTools[1].(map[string]any)
	if first["type"] != "function" {
		t.Fatalf("first tool must be function, got %v", first["type"])
	}
	if second["type"] != "openrouter:web_search" {
		t.Fatalf("server tool must pass through as-is, got %v", second["type"])
	}
	if _, hasFn := second["function"]; hasFn {
		t.Fatalf("server tool must not gain a function wrapper: %v", second)
	}

	if !strings.Contains(resp.Content, "Источники:") {
		t.Fatalf("citations block missing: %q", resp.Content)
	}
	if strings.Count(resp.Content, "https://a.example/doc") != 1 {
		t.Fatalf("duplicate URL must be collapsed: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "[https://b.example](https://b.example)") {
		t.Fatalf("empty title must fall back to URL: %q", resp.Content)
	}
}

// TestClient_Generate_NoAnnotations_NoSourcesBlock — без аннотаций контент не трогаем.
func TestClient_Generate_NoAnnotations_NoSourcesBlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
            "choices": [{"message": {"role":"assistant","content":"plain"}, "finish_reason":"stop"}],
            "usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
        }`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{APIKey: "k", BaseURL: srv.URL, DefaultModel: "m"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	resp, err := c.Generate(context.Background(), llm.Request{Messages: []llm.Message{{Role: llm.RoleUser, Content: "q"}}})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Content != "plain" {
		t.Fatalf("content must be untouched, got %q", resp.Content)
	}
}
