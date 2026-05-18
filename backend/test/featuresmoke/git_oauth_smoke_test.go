//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"strings"
	"testing"
)

// git_oauth_smoke_test.go — P1 контракт OAuth-флоу для git-интеграций и
// Claude Code OAuth.
//
// PR-gate: реальный consent-flow не доступен (нет браузера, нет client_secret'ов),
// поэтому смокинг ограничивается:
//   - init без конфигурации провайдера → 503 oauth_not_configured;
//   - init c конфигурацией → 200 + authorize_url содержит state;
//   - status «без подключения» → 200 + connected=false;
//   - revoke на не-подключённый → корректно отвечает (200 или 404, без 5xx);
//   - callback с битым state → 4xx invalid_state;
//   - все ручки требуют Bearer.

type gitInitResponse struct {
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"state"`
}

type gitStatusResponse struct {
	Provider     string `json:"provider"`
	Connected    bool   `json:"connected"`
	AccountLogin string `json:"account_login,omitempty"`
}

// TestGitOAuth_InitWithoutConfigurationReturnsHandled — без CLIENT_ID/CLIENT_SECRET
// (PR-gate compose их не выставляет) бэкенд должен отдать 503/400 с
// error=oauth_not_configured, но никогда — 5xx.
func TestGitOAuth_GitHubInitReturns200or503(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST", "/api/v1/integrations/github/auth/init", map[string]any{
		"redirect_uri": "http://localhost:3000/oauth/callback",
	}, user.AccessToken)
	switch {
	case resp.Status == http.StatusOK:
		var out gitInitResponse
		resp.JSON(t, &out)
		if out.AuthorizeURL == "" || out.State == "" {
			t.Fatalf("init OK: пустые authorize_url/state: %s", truncBody(resp.Body))
		}
		// authorize_url должен быть валидным URL'ом провайдера, не нашим.
		if !strings.HasPrefix(out.AuthorizeURL, "http") {
			t.Fatalf("init OK: authorize_url=%q не похож на URL", out.AuthorizeURL)
		}
	case resp.Status == http.StatusServiceUnavailable:
		// норма для PR-gate без OAuth-конфига.
	default:
		t.Fatalf("github init: status=%d (ожидали 200 или 503) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestGitOAuth_GitHubStatusReturnsDisconnectedByDefault.
func TestGitOAuth_GitHubStatusReturnsDisconnectedByDefault(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/integrations/github/auth/status", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("github status: status=%d (ожидали 200) body=%s",
			resp.Status, truncBody(resp.Body))
	}
	var st gitStatusResponse
	resp.JSON(t, &st)
	if st.Connected {
		t.Fatalf("github status: новосозданный пользователь connected=true (ожидали false)")
	}
	if st.Provider != "" && st.Provider != "github" {
		t.Fatalf("github status: provider=%q (ожидали 'github' или пусто)", st.Provider)
	}
}

// TestGitOAuth_GitLabStatusReturnsDisconnectedByDefault.
func TestGitOAuth_GitLabStatusReturnsDisconnectedByDefault(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/integrations/gitlab/auth/status", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("gitlab status: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var st gitStatusResponse
	resp.JSON(t, &st)
	if st.Connected {
		t.Fatalf("gitlab status: connected=true до коннекта")
	}
}

// TestGitOAuth_CallbackWithBadStateReturns4xx — callback с произвольным state
// должен вернуть 4xx (invalid_state / expired_state), но не 5xx.
func TestGitOAuth_CallbackWithBadStateReturns4xx(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST", "/api/v1/integrations/github/auth/callback", map[string]any{
		"code":  "totally-fake-code",
		"state": "fake-state-that-was-never-issued",
	}, user.AccessToken)
	if resp.Status >= 500 {
		t.Fatalf("callback bad state: server error status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
	if resp.Status < 400 {
		t.Fatalf("callback bad state: status=%d (ожидали 4xx)", resp.Status)
	}
}

// TestGitOAuth_RevokeOnDisconnectedIsHandled — revoke на нечего отзывать
// должен не падать в 5xx; 200/404/503 — допустимы (503 = oauth_not_configured).
func TestGitOAuth_GitHubRevokeOnDisconnectedIsHandled(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "DELETE", "/api/v1/integrations/github/auth/revoke", nil, user.AccessToken)
	if resp.Status >= 500 && resp.Status != http.StatusServiceUnavailable {
		t.Fatalf("github revoke: server error status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestGitOAuth_AllEndpointsRequireAuth.
func TestGitOAuth_AllEndpointsRequireAuth(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	cases := []struct {
		method string
		path   string
		body   any
	}{
		{"POST", "/api/v1/integrations/github/auth/init", map[string]any{"redirect_uri": "x"}},
		{"GET", "/api/v1/integrations/github/auth/status", nil},
		{"DELETE", "/api/v1/integrations/github/auth/revoke", nil},
		{"POST", "/api/v1/integrations/github/auth/callback", map[string]any{"code": "c", "state": "s"}},
		{"POST", "/api/v1/integrations/gitlab/auth/init", map[string]any{"redirect_uri": "x"}},
		{"GET", "/api/v1/integrations/gitlab/auth/status", nil},
		{"DELETE", "/api/v1/integrations/gitlab/auth/revoke", nil},
	}
	for _, tc := range cases {
		resp := h.Do(t, tc.method, tc.path, tc.body, "")
		if resp.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s without token: status=%d (ожидали 401)",
				tc.method, tc.path, resp.Status)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Claude Code OAuth (POST /claude-code/auth/init|callback, PUT /manual-token,
// GET /status, DELETE /). PR-gate без real provider — проверяем контракт.

// TestClaudeCodeOAuth_StatusReturnsDisconnectedByDefault.
func TestClaudeCodeOAuth_StatusReturnsDisconnectedByDefault(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/claude-code/auth/status", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("claude-code status: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var st struct {
		Connected bool `json:"connected"`
	}
	resp.JSON(t, &st)
	if st.Connected {
		t.Fatalf("claude-code status: новый пользователь connected=true")
	}
}

// TestClaudeCodeOAuth_InitReturnsHandled — device-flow init либо отдаёт код
// (200), либо 503 oauth_not_configured (если CLAUDE_CODE_OAUTH_CLIENT_ID не задан).
// 500+ кроме 503 — баг.
func TestClaudeCodeOAuth_InitReturnsHandled(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST", "/api/v1/claude-code/auth/init", nil, user.AccessToken)
	if resp.Status >= 500 && resp.Status != http.StatusServiceUnavailable {
		t.Fatalf("claude-code init: server error status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestClaudeCodeOAuth_ManualTokenDoesNotLeak — PUT /manual-token с canary'м
// токеном. Ответ не должен содержать сам токен.
func TestClaudeCodeOAuth_ManualTokenDoesNotLeak(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	canary := "sk-ant-oauth-CANARY-feedeadbeef-1234567890-abcdef1234567890"
	resp := h.Do(t, "PUT", "/api/v1/claude-code/auth/manual-token", map[string]any{
		"access_token":  canary,
		"refresh_token": "rt-" + canary,
		"token_type":    "Bearer",
		"scopes":        "read",
	}, user.AccessToken)
	if resp.Status >= 500 && resp.Status != http.StatusServiceUnavailable {
		t.Fatalf("manual-token: server error status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
	if strings.Contains(string(resp.Body), canary) {
		t.Fatalf("manual-token: токен утёк в ответ: %s", truncBody(resp.Body))
	}
}

// TestClaudeCodeOAuth_RevokeIsHandled.
func TestClaudeCodeOAuth_RevokeIsHandled(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "DELETE", "/api/v1/claude-code/auth", nil, user.AccessToken)
	if resp.Status >= 500 && resp.Status != http.StatusServiceUnavailable {
		t.Fatalf("claude-code revoke: server error status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestClaudeCodeOAuth_AllRequireAuth.
func TestClaudeCodeOAuth_AllRequireAuth(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	cases := []struct {
		method string
		path   string
	}{
		{"POST", "/api/v1/claude-code/auth/init"},
		{"POST", "/api/v1/claude-code/auth/callback"},
		{"PUT", "/api/v1/claude-code/auth/manual-token"},
		{"GET", "/api/v1/claude-code/auth/status"},
		{"DELETE", "/api/v1/claude-code/auth"},
	}
	for _, tc := range cases {
		resp := h.Do(t, tc.method, tc.path, nil, "")
		if resp.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s without token: status=%d (ожидали 401)",
				tc.method, tc.path, resp.Status)
		}
	}
}
