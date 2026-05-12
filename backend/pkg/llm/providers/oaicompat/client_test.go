package oaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
