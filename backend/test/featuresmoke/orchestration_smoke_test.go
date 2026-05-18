//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// orchestration_smoke_test.go — P1 read-only API оркестрации v2:
// artifacts, router-decisions, worktrees.
//
// В PR-gate реальный pipeline не запускается (нет LLM, нет sandbox), поэтому
// списки артефактов / решений / worktree'ев пустые. Это покрытие сосредоточено
// на контракте ответов и на access-policy (cross-tenant + admin-only).

type itemsResponse[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}

// TestOrchestration_ListArtifactsForFreshTaskIsEmpty.
func TestOrchestration_ListArtifactsForFreshTaskIsEmpty(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "art-"+uuid.NewString())

	resp := h.Do(t, "GET", "/api/v1/tasks/"+task.ID+"/artifacts", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("list artifacts: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out itemsResponse[map[string]any]
	resp.JSON(t, &out)
	if len(out.Items) != 0 {
		t.Fatalf("list artifacts: для свежесозданной таски items=%d (ожидали 0)", len(out.Items))
	}
}

// TestOrchestration_GetArtifactByIDMissingReturns404.
func TestOrchestration_GetArtifactByIDMissingReturns404(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "art-mis-"+uuid.NewString())

	resp := h.Do(t, "GET",
		"/api/v1/tasks/"+task.ID+"/artifacts/"+uuid.NewString(),
		nil, user.AccessToken)
	if resp.Status != http.StatusNotFound {
		t.Fatalf("get missing artifact: status=%d (ожидали 404) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestOrchestration_ListRouterDecisionsForFreshTaskIsEmpty.
func TestOrchestration_ListRouterDecisionsForFreshTaskIsEmpty(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "rd-"+uuid.NewString())

	resp := h.Do(t, "GET", "/api/v1/tasks/"+task.ID+"/router-decisions", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("list router-decisions: status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
	var out itemsResponse[map[string]any]
	resp.JSON(t, &out)
	if len(out.Items) != 0 {
		t.Fatalf("list router-decisions: items=%d (ожидали 0)", len(out.Items))
	}
}

// TestOrchestration_RouterDecisionsDoesNotLeakRawResponse — гарант контракта
// (модели обещают, что encrypted_raw_response никогда не сериализуется).
// Здесь проверяем, что в JSON-ответе нет ни поля, ни ключа "encrypted_raw".
func TestOrchestration_RouterDecisionsResponseHasNoEncryptedFields(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "rd-noleak-"+uuid.NewString())

	resp := h.Do(t, "GET", "/api/v1/tasks/"+task.ID+"/router-decisions", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("list router-decisions: status=%d", resp.Status)
	}
	body := string(resp.Body)
	for _, leak := range []string{"encrypted_raw_response", "encrypted_raw", "raw_response"} {
		if contains(body, leak) {
			t.Fatalf("router-decisions ответ содержит подозрительное поле %q: %s",
				leak, truncBody(resp.Body))
		}
	}
}

// contains — local helper, чтобы не тянуть strings ради одного вхождения.
func contains(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	return indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	n, m := len(s), len(sub)
	if m == 0 || m > n {
		return -1
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == sub {
			return i
		}
	}
	return -1
}

// TestOrchestration_ListWorktreesGlobalRequiresAdmin — без task_id обычный
// пользователь получает 403.
func TestOrchestration_ListWorktreesGlobalRequiresAdmin(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/worktrees", nil, user.AccessToken)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("list worktrees global: status=%d (ожидали 403) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestOrchestration_ListWorktreesByTaskOwnedReturnsEmpty — обычный пользователь
// видит worktree'ы своей задачи (даже если их 0 — endpoint не должен 403).
func TestOrchestration_ListWorktreesByTaskOwnedReturnsEmpty(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "wt-"+uuid.NewString())

	resp := h.Do(t, "GET", "/api/v1/worktrees?task_id="+task.ID, nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("list worktrees by task: status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
	var out itemsResponse[map[string]any]
	resp.JSON(t, &out)
	if len(out.Items) != 0 {
		t.Fatalf("worktrees for fresh task: items=%d (ожидали 0)", len(out.Items))
	}
}

// TestOrchestration_ListWorktreesInvalidStateReturns400.
func TestOrchestration_ListWorktreesInvalidStateReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "wt-bad-"+uuid.NewString())

	resp := h.Do(t, "GET",
		"/api/v1/worktrees?task_id="+task.ID+"&state=not-a-real-state",
		nil, user.AccessToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("invalid state: status=%d (ожидали 400) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestOrchestration_ReleaseWorktreeRequiresAdmin — POST /worktrees/:id/release
// admin-only. Обычный пользователь получает 403, никогда не 5xx.
func TestOrchestration_ReleaseWorktreeRequiresAdmin(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST",
		"/api/v1/worktrees/"+uuid.NewString()+"/release",
		nil, user.AccessToken)
	if resp.Status != http.StatusForbidden {
		t.Fatalf("release worktree as non-admin: status=%d (ожидали 403)", resp.Status)
	}
}

// TestOrchestration_CrossTenantTaskArtifactsForbidden — owner-only доступ к
// orchestration v2 read-only ручкам: Bob с UUID Alice'иной задачи получает 403/404,
// никаких артефактов/решений увидеть не может. Регрессия в orchestration_v2_handler
// (закрыта в Sprint 17 review #2) — если эти ассерты снова получают 200, проверка
// доступа исчезла из ListArtifacts / GetArtifact / ListRouterDecisions.
func TestOrchestration_CrossTenantTaskArtifactsForbidden(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)
	p := createLocalProject(t, h, alice.AccessToken)
	task := createTask(t, h, alice.AccessToken, p.ID, "private-"+uuid.NewString())

	for _, suffix := range []string{"/artifacts", "/router-decisions"} {
		resp := h.Do(t, "GET",
			"/api/v1/tasks/"+task.ID+suffix, nil, bob.AccessToken)
		if resp.Status != http.StatusForbidden && resp.Status != http.StatusNotFound {
			t.Fatalf("cross-tenant %s: status=%d (ожидали 403/404) body=%s",
				suffix, resp.Status, truncBody(resp.Body))
		}
	}
}

// TestOrchestration_AllRequireAuth.
func TestOrchestration_AllRequireAuth(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	taskID := uuid.NewString()
	cases := []struct {
		method, path string
	}{
		{"GET", "/api/v1/tasks/" + taskID + "/artifacts"},
		{"GET", "/api/v1/tasks/" + taskID + "/router-decisions"},
		{"GET", "/api/v1/worktrees"},
		{"GET", "/api/v1/worktrees?task_id=" + taskID},
		{"POST", "/api/v1/worktrees/" + uuid.NewString() + "/release"},
	}
	for _, tc := range cases {
		resp := h.Do(t, tc.method, tc.path, nil, "")
		if resp.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s without token: status=%d (ожидали 401)",
				tc.method, tc.path, resp.Status)
		}
	}
}
