package freeclaudeproxy

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
	var capturedAuth, capturedVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedAuth = r.Header.Get("Authorization")
		capturedVersion = r.Header.Get("anthropic-version")

		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("body: %v", err)
		}
		if parsed["system"] != "you are tester" {
			t.Fatalf("system not propagated: %v", parsed["system"])
		}
		_, _ = w.Write([]byte(`{
            "content":[{"type":"text","text":"pong"}],
            "usage":{"input_tokens":3,"output_tokens":5}
        }`))
	}))
	defer srv.Close()

	c, err := NewClient(llm.Config{APIKey: "svc-token", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := c.Generate(context.Background(), llm.Request{
		SystemPrompt: "you are tester",
		Messages:     []llm.Message{{Role: llm.RoleUser, Content: "ping"}},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if resp.Content != "pong" {
		t.Fatalf("expected pong, got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 8 {
		t.Fatalf("expected total 8, got %d", resp.Usage.TotalTokens)
	}
	if capturedAuth != "Bearer svc-token" {
		t.Fatalf("expected Bearer auth, got %q", capturedAuth)
	}
	if capturedVersion != "2023-06-01" {
		t.Fatalf("expected anthropic-version header, got %q", capturedVersion)
	}
}

func TestClient_HealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c, _ := NewClient(llm.Config{APIKey: "t", BaseURL: srv.URL})
	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
}

func TestClient_HealthCheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c, _ := NewClient(llm.Config{APIKey: "t", BaseURL: srv.URL})
	err := c.HealthCheck(context.Background())
	if err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("expected status 503 error, got %v", err)
	}
}

func TestClient_DefaultBaseURL(t *testing.T) {
	c, _ := NewClient(llm.Config{APIKey: "t"})
	if c.BaseURL() != DefaultBaseURL {
		t.Fatalf("expected default base URL, got %q", c.BaseURL())
	}
}
