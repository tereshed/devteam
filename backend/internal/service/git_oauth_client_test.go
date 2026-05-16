package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubOAuthClient_ExchangeCode_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Fatalf("expected Accept=application/json, got %q", r.Header.Get("Accept"))
		}
		_, _ = w.Write([]byte(`{"access_token":"ghp_abc","scope":"repo","token_type":"bearer"}`))
	}))
	defer srv.Close()

	c := NewGitHubOAuthClient(GitHubOAuthConfig{
		ClientID:     "cid",
		ClientSecret: "csec",
		TokenURL:     srv.URL + "/oauth/token",
	})
	tok, err := c.ExchangeCode(context.Background(), "code", "https://app/cb")
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if tok.AccessToken != "ghp_abc" || tok.TokenType != "bearer" {
		t.Fatalf("bad token: %+v", tok)
	}
}

func TestGitHubOAuthClient_ExchangeCode_AccessDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":"access_denied"}`))
	}))
	defer srv.Close()

	c := NewGitHubOAuthClient(GitHubOAuthConfig{
		ClientID: "cid", ClientSecret: "csec", TokenURL: srv.URL,
	})
	_, err := c.ExchangeCode(context.Background(), "code", "cb")
	if !errors.Is(err, ErrGitOAuthUserCancelled) {
		t.Fatalf("expected user-cancelled, got %v", err)
	}
}

func TestGitHubOAuthClient_ExchangeCode_BadCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":"bad_verification_code"}`))
	}))
	defer srv.Close()

	c := NewGitHubOAuthClient(GitHubOAuthConfig{
		ClientID: "cid", ClientSecret: "csec", TokenURL: srv.URL,
	})
	_, err := c.ExchangeCode(context.Background(), "code", "cb")
	if !errors.Is(err, ErrGitOAuthInvalidGrant) {
		t.Fatalf("expected invalid-grant, got %v", err)
	}
}

func TestGitHubOAuthClient_ExchangeCode_5xxUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := NewGitHubOAuthClient(GitHubOAuthConfig{
		ClientID: "cid", ClientSecret: "csec", TokenURL: srv.URL,
	})
	_, err := c.ExchangeCode(context.Background(), "code", "cb")
	if !errors.Is(err, ErrGitOAuthProviderUnreachable) {
		t.Fatalf("expected unreachable, got %v", err)
	}
}

func TestGitHubOAuthClient_Revoke_BasicAuth(t *testing.T) {
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if !strings.HasPrefix(sawAuth, "Basic ") {
			t.Fatalf("missing Basic auth: %q", sawAuth)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewGitHubOAuthClient(GitHubOAuthConfig{
		ClientID: "cid", ClientSecret: "csec",
		TokenURL: srv.URL + "/token", APIBaseURL: srv.URL,
	})
	if err := c.Revoke(context.Background(), "ghp_abc"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
}

func TestGitLabOAuthClient_Revoke_FormBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/revoke" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.PostForm.Get("token") == "" || r.PostForm.Get("client_id") == "" || r.PostForm.Get("client_secret") == "" {
			t.Fatalf("missing form fields: %v", r.PostForm)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewGitLabOAuthClient(GitLabOAuthConfig{
		ClientID: "cid", ClientSecret: "csec", BaseURL: srv.URL,
	})
	if err := c.Revoke(context.Background(), "glat_x"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
}

func TestGitOAuthClient_Unconfigured(t *testing.T) {
	c := NewGitHubOAuthClient(GitHubOAuthConfig{})
	if _, err := c.ExchangeCode(context.Background(), "x", "y"); !errors.Is(err, ErrGitOAuthNotConfigured) {
		t.Fatalf("expected not-configured: %v", err)
	}
}
