//go:build featuresmoke

package featuresmoke

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// agents_me_smoke_test.go — P1 контракт /api/v1/me/agents (user-level агенты).
//
// Покрытие:
//   - GET /api/v1/me/agents — assistant создаётся при регистрации.
//   - GET /api/v1/me/agents/:id — полные детали.
//   - PUT /api/v1/me/agents/:id — partial update (system_prompt, internal_mcp_enabled).
//   - ABAC: чужой пользователь не видит агента → 403.
//   - PUT с запрещёнными полями (team_id, role) → 400.

// myAgentRecord — минимальный shape ответа для /me/agents.
type myAgentRecord struct {
	ID                 string  `json:"id"`
	Name               string  `json:"name"`
	Role               string  `json:"role"`
	Model              *string `json:"model"`
	SystemPrompt       *string `json:"system_prompt"`
	IsActive           bool    `json:"is_active"`
	InternalMCPEnabled bool    `json:"internal_mcp_enabled"`
}

// TestMyAgents_CreatedOnRegistration — при регистрации пользователя автоматически
// создаётся один agent с role=assistant, model=nil (unconfigured), system_prompt != "".
func TestMyAgents_CreatedOnRegistration(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/me/agents", nil, user.AccessToken)
	require.Equal(t, http.StatusOK, resp.Status, "list my agents: body=%s", truncBody(resp.Body))

	var out struct {
		Total int             `json:"total"`
		Items []myAgentRecord `json:"items"`
	}
	resp.JSON(t, &out)

	require.Equal(t, 1, len(out.Items), "expected exactly 1 user-level agent after registration, got %d", len(out.Items))

	agent := out.Items[0]
	require.Equal(t, "assistant", agent.Role, "auto-created agent role")
	require.Nil(t, agent.Model, "auto-created agent model should be nil (unconfigured)")
	require.NotNil(t, agent.SystemPrompt, "auto-created agent system_prompt should be set")
	require.NotEmpty(t, *agent.SystemPrompt, "auto-created agent system_prompt should be non-empty")
}

// TestMyAgents_GetByID — получение агента по ID через /me/agents/:id.
func TestMyAgents_GetByID(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// Получаем список, чтобы узнать ID assistant'а.
	listResp := h.Do(t, "GET", "/api/v1/me/agents", nil, user.AccessToken)
	require.Equal(t, http.StatusOK, listResp.Status, "list: body=%s", truncBody(listResp.Body))

	var list struct {
		Items []myAgentRecord `json:"items"`
	}
	listResp.JSON(t, &list)
	require.NotEmpty(t, list.Items, "expected at least 1 agent in list")

	agentID := list.Items[0].ID
	require.NotEmpty(t, agentID, "agent ID should not be empty")

	// GET by ID.
	getResp := h.Do(t, "GET", "/api/v1/me/agents/"+agentID, nil, user.AccessToken)
	require.Equal(t, http.StatusOK, getResp.Status, "get by id: body=%s", truncBody(getResp.Body))

	var agent myAgentRecord
	getResp.JSON(t, &agent)
	require.Equal(t, agentID, agent.ID, "returned agent ID must match requested")
	require.Equal(t, "assistant", agent.Role, "agent role")
}

// TestMyAgents_Update — PUT /me/agents/:id обновляет system_prompt и internal_mcp_enabled.
func TestMyAgents_Update(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// Получаем ID assistant'а.
	listResp := h.Do(t, "GET", "/api/v1/me/agents", nil, user.AccessToken)
	require.Equal(t, http.StatusOK, listResp.Status)

	var list struct {
		Items []myAgentRecord `json:"items"`
	}
	listResp.JSON(t, &list)
	require.NotEmpty(t, list.Items)

	agentID := list.Items[0].ID

	// PUT с новыми значениями.
	customPrompt := "Custom prompt for smoke test " + uuid.NewString()
	updateResp := h.Do(t, "PUT", "/api/v1/me/agents/"+agentID, map[string]any{
		"system_prompt":       customPrompt,
		"internal_mcp_enabled": true,
	}, user.AccessToken)
	require.Equal(t, http.StatusOK, updateResp.Status,
		"update agent: body=%s", truncBody(updateResp.Body))

	var updated myAgentRecord
	updateResp.JSON(t, &updated)
	require.NotNil(t, updated.SystemPrompt, "updated system_prompt should not be nil")
	require.Equal(t, customPrompt, *updated.SystemPrompt, "system_prompt should match sent value")
	require.True(t, updated.InternalMCPEnabled, "internal_mcp_enabled should be true after update")
}

// TestMyAgents_ABAC_ForbidsOtherUser — user B не может получить agent user A через /me/agents/:id.
func TestMyAgents_ABAC_ForbidsOtherUser(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)

	// Получаем ID assistant'а Alice.
	listResp := h.Do(t, "GET", "/api/v1/me/agents", nil, alice.AccessToken)
	require.Equal(t, http.StatusOK, listResp.Status)

	var list struct {
		Items []myAgentRecord `json:"items"`
	}
	listResp.JSON(t, &list)
	require.NotEmpty(t, list.Items)

	aliceAgentID := list.Items[0].ID

	// Bob пытается получить agent Alice.
	bobResp := h.Do(t, "GET", "/api/v1/me/agents/"+aliceAgentID, nil, bob.AccessToken)
	require.Equal(t, http.StatusForbidden, bobResp.Status,
		"cross-user access should be 403: body=%s", truncBody(bobResp.Body))
}

// TestMyAgents_Update_RejectsTeamID — PUT с team_id → 400.
func TestMyAgents_Update_RejectsTeamID(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// Получаем ID assistant'а.
	listResp := h.Do(t, "GET", "/api/v1/me/agents", nil, user.AccessToken)
	require.Equal(t, http.StatusOK, listResp.Status)

	var list struct {
		Items []myAgentRecord `json:"items"`
	}
	listResp.JSON(t, &list)
	require.NotEmpty(t, list.Items)

	agentID := list.Items[0].ID

	resp := h.Do(t, "PUT", "/api/v1/me/agents/"+agentID, map[string]any{
		"team_id": uuid.NewString(),
	}, user.AccessToken)
	require.Equal(t, http.StatusBadRequest, resp.Status,
		"team_id should be rejected with 400: body=%s", truncBody(resp.Body))
}

// TestMyAgents_Update_RejectsRoleChange — PUT с role → 400.
func TestMyAgents_Update_RejectsRoleChange(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// Получаем ID assistant'а.
	listResp := h.Do(t, "GET", "/api/v1/me/agents", nil, user.AccessToken)
	require.Equal(t, http.StatusOK, listResp.Status)

	var list struct {
		Items []myAgentRecord `json:"items"`
	}
	listResp.JSON(t, &list)
	require.NotEmpty(t, list.Items)

	agentID := list.Items[0].ID

	resp := h.Do(t, "PUT", fmt.Sprintf("/api/v1/me/agents/%s", agentID), map[string]any{
		"role": "developer",
	}, user.AccessToken)
	require.Equal(t, http.StatusBadRequest, resp.Status,
		"role change should be rejected with 400: body=%s", truncBody(resp.Body))
}
