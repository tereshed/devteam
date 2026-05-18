//go:build featuresmoke

package featuresmoke

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// projects_smoke_test.go — P0 CRUD по /api/v1/projects + cross-tenant guard.
//
// Стратегия: на каждый тест — отдельный User (через harness.NewUser). Все проекты
// создаются с git_provider="local" (без git_url), чтобы исключить попытку
// валидации remote-репо в Create-handler'е (см. projectService.Create).

type projectResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	GitProvider      string `json:"git_provider"`
	GitURL           string `json:"git_url"`
	GitDefaultBranch string `json:"git_default_branch"`
	Status           string `json:"status"`
}

type projectListResponse struct {
	Projects []projectResponse `json:"projects"`
	Total    int64             `json:"total"`
	Limit    int               `json:"limit"`
	Offset   int               `json:"offset"`
}

// createLocalProject — helper, возвращает свежий проект с unique именем.
func createLocalProject(t *testing.T, h *Harness, token string) projectResponse {
	t.Helper()
	name := "p-" + uuid.NewString()
	resp := h.Do(t, "POST", "/api/v1/projects", map[string]any{
		"name":         name,
		"description":  "featuresmoke project",
		"git_provider": "local",
	}, token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("createProject: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out projectResponse
	resp.JSON(t, &out)
	if out.ID == "" {
		t.Fatalf("createProject: пустой id в ответе: %s", truncBody(resp.Body))
	}
	return out
}

// TestProjects_CreateReadUpdateDelete — основной happy path.
func TestProjects_CreateReadUpdateDelete(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// 1. Create.
	created := createLocalProject(t, h, user.AccessToken)
	if created.GitProvider != "local" {
		t.Fatalf("create: git_provider=%q ожидали local", created.GitProvider)
	}

	// 2. GetByID — что положили, то и достали.
	getResp := h.Do(t, "GET", "/api/v1/projects/"+created.ID, nil, user.AccessToken)
	if getResp.Status != http.StatusOK {
		t.Fatalf("get: status=%d body=%s", getResp.Status, truncBody(getResp.Body))
	}
	var got projectResponse
	getResp.JSON(t, &got)
	if got.Name != created.Name {
		t.Fatalf("get: name=%q ожидали %q", got.Name, created.Name)
	}

	// 3. Update — меняем description.
	newDesc := "updated " + uuid.NewString()
	upResp := h.Do(t, "PUT", "/api/v1/projects/"+created.ID, map[string]any{
		"description": newDesc,
	}, user.AccessToken)
	if upResp.Status != http.StatusOK {
		t.Fatalf("update: status=%d body=%s", upResp.Status, truncBody(upResp.Body))
	}
	var updated projectResponse
	upResp.JSON(t, &updated)
	if updated.Description != newDesc {
		t.Fatalf("update: description=%q ожидали %q", updated.Description, newDesc)
	}

	// 4. List — наш проект в списке (фильтр по поиску для стабильности).
	listResp := h.Do(t, "GET",
		"/api/v1/projects?search="+created.Name, nil, user.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("list: status=%d", listResp.Status)
	}
	var list projectListResponse
	listResp.JSON(t, &list)
	found := false
	for _, p := range list.Projects {
		if p.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list: созданный проект %s не найден среди %d проектов", created.ID, list.Total)
	}

	// 5. Delete.
	delResp := h.Do(t, "DELETE", "/api/v1/projects/"+created.ID, nil, user.AccessToken)
	if delResp.Status != http.StatusNoContent && delResp.Status != http.StatusOK {
		t.Fatalf("delete: status=%d body=%s", delResp.Status, truncBody(delResp.Body))
	}

	// 6. После Delete — Get отдаёт 404.
	getAfter := h.Do(t, "GET", "/api/v1/projects/"+created.ID, nil, user.AccessToken)
	if getAfter.Status != http.StatusNotFound {
		t.Fatalf("get after delete: status=%d (ожидали 404)", getAfter.Status)
	}
}

// TestProjects_ReindexLocalReturnsAccepted — POST /reindex на local-провайдере.
// Для local-проектов индексация может быть no-op, но эндпоинт обязан существовать
// и не возвращать 5xx.
func TestProjects_ReindexLocalReturnsHandled(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	resp := h.Do(t, "POST", "/api/v1/projects/"+p.ID+"/reindex", nil, user.AccessToken)
	// Допустимые исходы:
	//   200 OK              — индексация запущена;
	//   202 Accepted        — async-режим;
	//   400 Bad Request     — local + нет git_url (валидно: реиндекс невозможен);
	//   409 Conflict        — уже идёт.
	// 5xx — баг.
	if resp.Status >= 500 {
		t.Fatalf("reindex: server error status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
}

// TestProjects_CreateRequiresAuth — без токена = 401.
func TestProjects_CreateRequiresAuth(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	resp := h.Do(t, "POST", "/api/v1/projects", map[string]any{
		"name": "should-not-create-" + uuid.NewString(),
	}, "")
	if resp.Status != http.StatusUnauthorized {
		t.Fatalf("create without token: status=%d (ожидали 401)", resp.Status)
	}
}

// TestProjects_CrossTenantIsolation — чужой проект не виден и не редактируется.
func TestProjects_CrossTenantIsolation(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)

	p := createLocalProject(t, h, alice.AccessToken)

	// Bob не должен видеть Alice'ин проект.
	bobGet := h.Do(t, "GET", "/api/v1/projects/"+p.ID, nil, bob.AccessToken)
	if bobGet.Status != http.StatusNotFound && bobGet.Status != http.StatusForbidden {
		t.Fatalf("cross-tenant get: status=%d (ожидали 404 или 403) body=%s",
			bobGet.Status, truncBody(bobGet.Body))
	}

	// Bob не должен иметь возможность удалить.
	bobDel := h.Do(t, "DELETE", "/api/v1/projects/"+p.ID, nil, bob.AccessToken)
	if bobDel.Status != http.StatusNotFound && bobDel.Status != http.StatusForbidden {
		t.Fatalf("cross-tenant delete: status=%d (ожидали 404 или 403)", bobDel.Status)
	}

	// Проект всё ещё доступен Alice.
	aliceGet := h.Do(t, "GET", "/api/v1/projects/"+p.ID, nil, alice.AccessToken)
	if aliceGet.Status != http.StatusOK {
		t.Fatalf("alice get after bob attack: status=%d", aliceGet.Status)
	}
}

// TestProjects_InvalidUUIDReturns400 — гарбидж в :id = 400.
func TestProjects_InvalidUUIDReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/projects/not-a-uuid", nil, user.AccessToken)
	if resp.Status != http.StatusBadRequest && resp.Status != http.StatusNotFound {
		t.Fatalf("get invalid uuid: status=%d (ожидали 400/404)", resp.Status)
	}
}

// TestProjects_CreateInvalidProviderReturns400.
func TestProjects_CreateInvalidProviderReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "POST", "/api/v1/projects", map[string]any{
		"name":         "p-" + uuid.NewString(),
		"git_provider": fmt.Sprintf("totally-fake-%s", uuid.NewString()),
	}, user.AccessToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("create invalid provider: status=%d (ожидали 400) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}
