//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// agents_smoke_test.go — P1 контракт реестра агентов v2 и их секретов.
//
// Покрытие:
//   - GET /api/v1/agents — 200, items[].
//   - POST /api/v1/agents — create + duplicate name = 409.
//   - GET /api/v1/agents/:id — 200 на свежесозданном, 404 на левом id.
//   - PUT /api/v1/agents/:id — partial update.
//   - POST /api/v1/agents/:id/secrets — успешный set; ответ не содержит plaintext.
//   - DELETE /api/v1/agents/secrets/:secret_id — 204.
//   - 401 без токена.

type agentRecord struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Role          string  `json:"role"`
	ExecutionKind string  `json:"execution_kind"`
	Model         *string `json:"model,omitempty"`
	IsActive      bool    `json:"is_active"`
}

// agentSecretResponse — service.SetSecretOutput. Поля без json-тегов,
// поэтому Go-default-маршалинг отдаёт PascalCase (SecretID, AgentID, KeyName).
// Это отдельный backend-NIT (можно пофиксить тегами в service/agent_service.go),
// но контракт сейчас такой.
type agentSecretResponse struct {
	AgentID  string `json:"AgentID"`
	SecretID string `json:"SecretID"`
	KeyName  string `json:"KeyName"`
}

// createSmokeAgent — helper. Возвращает свежесозданного агента с unique name.
// Для execution_kind="llm" модель обязательна (см. agent_service.go).
//
// КРИТИЧНО (cost-leak prevention): регистрируем `t.Cleanup`, который удаляет
// агента после теста. Без этого записи `ag-<uuid>` накапливаются в таблице
// `agents` (~300 за день прогонов до фикса) и каждая попадает в KAЖДЫЙ router-
// prompt → раздувает input-токены до 7k+ и жжёт деньги на real-pipeline.
// См. cost-leak инцидент Phase 2.
func createSmokeAgent(t *testing.T, h *Harness, token string) agentRecord {
	t.Helper()
	name := "ag-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:24]
	resp := h.Do(t, "POST", "/api/v1/agents", map[string]any{
		"name":           name,
		"role":           "developer",
		"execution_kind": "llm",
		"model":          TestModelAnthropic,
	}, token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("create agent: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var a agentRecord
	resp.JSON(t, &a)
	if a.ID == "" {
		t.Fatalf("create agent: пустой id: %s", truncBody(resp.Body))
	}
	t.Cleanup(func() {
		// h.Do НЕЛЬЗЯ — он использует t.Fatalf на network-ошибках; во время
		// cleanup'а backend уже может закрываться (особенно при killTree из
		// глобального cleanup'а TestMain'а), и t.Fatalf пометит тест как failed
		// постфактум. Используем raw http.Client с коротким timeout'ом и просто
		// логируем результат — cost-leak фиксится best-effort, не блокируя suit.
		deleteAgentBestEffort(t, h.BaseURL, token, a.ID)
	})
	return a
}

// deleteAgentBestEffort — нерейзящий DELETE для использования из t.Cleanup.
func deleteAgentBestEffort(t *testing.T, baseURL, token, agentID string) {
	t.Helper()
	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/agents/"+agentID, nil)
	if err != nil {
		t.Logf("cleanup agent %s: build request: %v", agentID, err)
		return
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// backend уже остановлен — это ок, главное чтобы не было утечек
		// в долгоживущей БД. На повторных прогонах оставшиеся записи будут
		// идемпотентно удалены при следующем cleanup'e.
		t.Logf("cleanup agent %s: %v (likely backend already shutting down)", agentID, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		t.Logf("cleanup agent %s: DELETE returned status=%d", agentID, resp.StatusCode)
	}
}

// TestAgents_ListReturnsItems — список агентов отвечает успешно.
func TestAgents_ListReturnsItems(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/agents", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("list agents: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out struct {
		Total int           `json:"total"`
		Items []agentRecord `json:"items"`
	}
	resp.JSON(t, &out)
	if out.Items == nil {
		t.Fatalf("list agents: items=nil")
	}
}

// TestAgents_CreateAndGet — create + duplicate name = 409.
func TestAgents_CreateAndGet(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	a := createSmokeAgent(t, h, user.AccessToken)
	if a.Role != "developer" {
		t.Fatalf("created agent: role=%q ожидали developer", a.Role)
	}

	// Get by ID.
	getResp := h.Do(t, "GET", "/api/v1/agents/"+a.ID, nil, user.AccessToken)
	if getResp.Status != http.StatusOK {
		t.Fatalf("get agent: status=%d body=%s", getResp.Status, truncBody(getResp.Body))
	}

	// Duplicate name → 409.
	dupResp := h.Do(t, "POST", "/api/v1/agents", map[string]any{
		"name":           a.Name,
		"role":           "developer",
		"execution_kind": "llm",
		"model":          TestModelAnthropic,
	}, user.AccessToken)
	if dupResp.Status != http.StatusConflict {
		t.Fatalf("duplicate agent: status=%d (ожидали 409) body=%s",
			dupResp.Status, truncBody(dupResp.Body))
	}
}

// TestAgents_GetMissingReturns404.
func TestAgents_GetMissingReturns404(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/agents/"+uuid.NewString(), nil, user.AccessToken)
	if resp.Status != http.StatusNotFound {
		t.Fatalf("get missing agent: status=%d (ожидали 404)", resp.Status)
	}
}

// TestAgents_UpdatePartial — partial update is_active.
func TestAgents_UpdatePartial(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	a := createSmokeAgent(t, h, user.AccessToken)

	resp := h.Do(t, "PUT", "/api/v1/agents/"+a.ID, map[string]any{
		"is_active": !a.IsActive,
	}, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("update agent: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	// Read-back.
	getResp := h.Do(t, "GET", "/api/v1/agents/"+a.ID, nil, user.AccessToken)
	if getResp.Status != http.StatusOK {
		t.Fatalf("get after update: status=%d", getResp.Status)
	}
	var updated agentRecord
	getResp.JSON(t, &updated)
	if updated.IsActive == a.IsActive {
		t.Fatalf("update agent: is_active не изменился (%v → %v)", a.IsActive, updated.IsActive)
	}
}

// TestAgents_SetSecretDoesNotLeakAndDelete — secret set + delete.
func TestAgents_SetSecretDoesNotLeakAndDelete(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	a := createSmokeAgent(t, h, user.AccessToken)

	plaintext := "secret-" + strings.ReplaceAll(uuid.NewString(), "-", "") + "-payload"
	setResp := h.Do(t, "POST", "/api/v1/agents/"+a.ID+"/secrets", map[string]any{
		"key_name": "API_TOKEN",
		"value":    plaintext,
	}, user.AccessToken)
	if setResp.Status != http.StatusCreated {
		// 503 если encryptor не сконфигурирован — допустимая ветка в PR-gate.
		if setResp.Status == http.StatusServiceUnavailable {
			t.Skipf("encryption не настроена на сервере (PR-gate): %s",
				truncBody(setResp.Body))
		}
		t.Fatalf("set secret: status=%d body=%s", setResp.Status, truncBody(setResp.Body))
	}
	if strings.Contains(string(setResp.Body), plaintext) {
		t.Fatalf("set secret: plaintext утёк в ответ: %s", truncBody(setResp.Body))
	}
	var out agentSecretResponse
	setResp.JSON(t, &out)
	if out.SecretID == "" {
		t.Fatalf("set secret: пустой secret_id: %s", truncBody(setResp.Body))
	}
	if out.KeyName != "API_TOKEN" {
		t.Fatalf("set secret: key_name=%q ожидали API_TOKEN", out.KeyName)
	}

	// Delete by ID.
	delResp := h.Do(t, "DELETE", "/api/v1/agents/secrets/"+out.SecretID, nil, user.AccessToken)
	if delResp.Status != http.StatusNoContent && delResp.Status != http.StatusOK {
		t.Fatalf("delete secret: status=%d body=%s", delResp.Status, truncBody(delResp.Body))
	}

	// Повторный delete должен дать 404.
	del2 := h.Do(t, "DELETE", "/api/v1/agents/secrets/"+out.SecretID, nil, user.AccessToken)
	if del2.Status != http.StatusNotFound {
		t.Fatalf("delete secret again: status=%d (ожидали 404)", del2.Status)
	}
}

// TestAgents_CreateMissingFieldsReturns400.
func TestAgents_CreateMissingFieldsReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST", "/api/v1/agents", map[string]any{
		"name": "no-role-and-no-kind-" + uuid.NewString(),
	}, user.AccessToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("missing fields: status=%d (ожидали 400) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestAgents_CreateInvalidRoleReturns400.
func TestAgents_CreateInvalidRoleReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST", "/api/v1/agents", map[string]any{
		"name":           "bad-role-" + uuid.NewString(),
		"role":           "not-a-real-role",
		"execution_kind": "llm",
	}, user.AccessToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("invalid role: status=%d (ожидали 400)", resp.Status)
	}
}

// TestAgents_RequiresAuth.
func TestAgents_RequiresAuth(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	cases := []struct {
		method string
		path   string
		body   any
	}{
		{"GET", "/api/v1/agents", nil},
		{"POST", "/api/v1/agents", map[string]any{"name": "x", "role": "developer", "execution_kind": "llm", "model": TestModelAnthropic}},
		{"GET", "/api/v1/agents/" + uuid.NewString(), nil},
		{"PUT", "/api/v1/agents/" + uuid.NewString(), map[string]any{"is_active": false}},
	}
	for _, tc := range cases {
		resp := h.Do(t, tc.method, tc.path, tc.body, "")
		if resp.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s without token: status=%d (ожидали 401)",
				tc.method, tc.path, resp.Status)
		}
	}
}
