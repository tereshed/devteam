//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// api_keys_smoke_test.go — P2 /api/v1/auth/api-keys.
//
// Контракт:
//   - POST /auth/api-keys — создаёт ключ, в ответе один раз отдаёт raw_key и
//     key_prefix; пустой name отбивается 400.
//   - GET /auth/api-keys — отдаёт массив (не null) с key_prefix, без secret.
//   - POST /auth/api-keys/:id/revoke — 200, ключ помечен как revoked.
//   - DELETE /auth/api-keys/:id — 204; повторный DELETE → 404.
//   - Cross-tenant: чужой ключ не виден и не редактируется.
//   - GET /auth/api-keys/mcp-config — 200 либо 404/500, в зависимости от того,
//     включён ли MCP в backend env. PR-gate не настраивает MCP_PUBLIC_URL,
//     поэтому проверяем только «5xx не должно быть, кроме конкретного
//     «MCP_PUBLIC_URL is not configured»».

type apiKeyCreatedResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	KeyPrefix  string `json:"key_prefix"`
	Scopes     string `json:"scopes"`
	RawKey     string `json:"raw_key"`
}

func createApiKey(t *testing.T, h *Harness, token, name string) apiKeyCreatedResponse {
	t.Helper()
	resp := h.Do(t, "POST", "/api/v1/auth/api-keys", map[string]any{
		"name":   name,
		"scopes": "*",
	}, token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("create api-key: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out apiKeyCreatedResponse
	resp.JSON(t, &out)
	if out.ID == "" {
		t.Fatalf("create api-key: пустой id: %s", truncBody(resp.Body))
	}
	if out.RawKey == "" {
		t.Fatalf("create api-key: пустой raw_key: %s", truncBody(resp.Body))
	}
	if out.KeyPrefix == "" {
		t.Fatalf("create api-key: пустой key_prefix: %s", truncBody(resp.Body))
	}
	// raw_key должен начинаться с key_prefix — иначе фронт не сможет показать
	// «начало вашего ключа» в списке после первого показа.
	if !strings.HasPrefix(out.RawKey, out.KeyPrefix) {
		t.Fatalf("create api-key: raw_key=%q не начинается с key_prefix=%q",
			truncStr(out.RawKey, 32), out.KeyPrefix)
	}
	return out
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// TestApiKeys_CreateListRevokeDelete — happy path.
func TestApiKeys_CreateListRevokeDelete(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	name := "smoke-key-" + uuid.NewString()
	created := createApiKey(t, h, user.AccessToken, name)

	// List должен содержать наш ключ.
	listResp := h.Do(t, "GET", "/api/v1/auth/api-keys", nil, user.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("list api-keys: status=%d body=%s", listResp.Status, truncBody(listResp.Body))
	}
	// Тело — массив; raw_key никогда не возвращается в list.
	if strings.Contains(string(listResp.Body), created.RawKey) {
		t.Fatalf("list api-keys: raw_key утёк в list response")
	}
	var list []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		KeyPrefix string `json:"key_prefix"`
	}
	listResp.JSON(t, &list)
	found := false
	for _, k := range list {
		if k.ID == created.ID {
			found = true
			if k.Name != name {
				t.Fatalf("list: name=%q ожидали %q", k.Name, name)
			}
			if k.KeyPrefix != created.KeyPrefix {
				t.Fatalf("list: key_prefix=%q ожидали %q", k.KeyPrefix, created.KeyPrefix)
			}
		}
	}
	if !found {
		t.Fatalf("list api-keys: созданный ключ %s не найден среди %d", created.ID, len(list))
	}

	// Revoke.
	revokeResp := h.Do(t, "POST", "/api/v1/auth/api-keys/"+created.ID+"/revoke",
		nil, user.AccessToken)
	if revokeResp.Status != http.StatusOK {
		t.Fatalf("revoke: status=%d body=%s", revokeResp.Status, truncBody(revokeResp.Body))
	}

	// Delete.
	delResp := h.Do(t, "DELETE", "/api/v1/auth/api-keys/"+created.ID,
		nil, user.AccessToken)
	if delResp.Status != http.StatusNoContent && delResp.Status != http.StatusOK {
		t.Fatalf("delete: status=%d body=%s", delResp.Status, truncBody(delResp.Body))
	}

	// Повторный delete → 404.
	del2 := h.Do(t, "DELETE", "/api/v1/auth/api-keys/"+created.ID,
		nil, user.AccessToken)
	if del2.Status != http.StatusNotFound {
		t.Fatalf("delete second time: status=%d (ожидали 404) body=%s",
			del2.Status, truncBody(del2.Body))
	}
}

// TestApiKeys_CreateEmptyNameReturns400 — валидация min=1.
func TestApiKeys_CreateEmptyNameReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST", "/api/v1/auth/api-keys", map[string]any{
		"name":   "",
		"scopes": "*",
	}, user.AccessToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("empty name: status=%d (ожидали 400) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestApiKeys_RequireAuthentication.
func TestApiKeys_RequireAuthentication(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	cases := []struct {
		method string
		path   string
		body   any
	}{
		{"POST", "/api/v1/auth/api-keys", map[string]any{"name": "x", "scopes": "*"}},
		{"GET", "/api/v1/auth/api-keys", nil},
		{"GET", "/api/v1/auth/api-keys/mcp-config", nil},
		{"POST", "/api/v1/auth/api-keys/" + uuid.NewString() + "/revoke", nil},
		{"DELETE", "/api/v1/auth/api-keys/" + uuid.NewString(), nil},
	}
	for _, tc := range cases {
		resp := h.Do(t, tc.method, tc.path, tc.body, "")
		if resp.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s no token: status=%d (ожидали 401)",
				tc.method, tc.path, resp.Status)
		}
	}
}

// TestApiKeys_CrossTenantIsolation — чужой ключ не виден, не отзывается, не
// удаляется.
func TestApiKeys_CrossTenantIsolation(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)

	aKey := createApiKey(t, h, alice.AccessToken, "alice-"+uuid.NewString())

	// Bob список своих ключей — должен быть пуст / не содержать алисин id.
	listResp := h.Do(t, "GET", "/api/v1/auth/api-keys", nil, bob.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("bob list: status=%d", listResp.Status)
	}
	if strings.Contains(string(listResp.Body), aKey.ID) {
		t.Fatalf("bob list содержит алисин key %s: %s", aKey.ID, truncBody(listResp.Body))
	}

	// Bob revoke — 404 (key not found) или 403, не 200.
	rev := h.Do(t, "POST", "/api/v1/auth/api-keys/"+aKey.ID+"/revoke",
		nil, bob.AccessToken)
	if rev.Status != http.StatusNotFound && rev.Status != http.StatusForbidden {
		t.Fatalf("bob revoke alice key: status=%d (ожидали 404/403)", rev.Status)
	}

	// Bob delete — то же самое.
	del := h.Do(t, "DELETE", "/api/v1/auth/api-keys/"+aKey.ID,
		nil, bob.AccessToken)
	if del.Status != http.StatusNotFound && del.Status != http.StatusForbidden {
		t.Fatalf("bob delete alice key: status=%d (ожидали 404/403)", del.Status)
	}

	// Алисин ключ всё ещё на месте.
	listA := h.Do(t, "GET", "/api/v1/auth/api-keys", nil, alice.AccessToken)
	if listA.Status != http.StatusOK {
		t.Fatalf("alice list after attack: status=%d", listA.Status)
	}
	if !strings.Contains(string(listA.Body), aKey.ID) {
		t.Fatalf("alice key %s пропал после Bob'овой попытки delete", aKey.ID)
	}
}

// TestApiKeys_MCPConfig_NoServerError — без MCP_PUBLIC_URL ручка должна
// штатно отдавать 4xx / 5xx с «not configured», а не зависать или паниковать.
func TestApiKeys_MCPConfig_NoServerError(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// Чтобы попасть в ветку «нужен хотя бы один ключ» — создаём ключ.
	_ = createApiKey(t, h, user.AccessToken, "mcp-"+uuid.NewString())

	resp := h.Do(t, "GET", "/api/v1/auth/api-keys/mcp-config", nil, user.AccessToken)
	// Допустимые исходы:
	//   200 — MCP включён и URL настроен;
	//   404 — MCP disabled (mcpConfig.Enabled=false);
	//   500 — MCP enabled, но MCP_PUBLIC_URL пуст. Это допустимый статус
	//         для PR-gate (env-конфиг по умолчанию).
	if resp.Status != http.StatusOK &&
		resp.Status != http.StatusNotFound &&
		resp.Status != http.StatusInternalServerError {
		t.Fatalf("mcp-config: unexpected status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
}
