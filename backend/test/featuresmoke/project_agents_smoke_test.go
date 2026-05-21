//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// project_agents_smoke_test.go — P1 контракт автоматического создания team-агентов
// при создании проекта.
//
// Покрытие:
//   - POST /api/v1/projects → автоматически создаёт team с orchestrator + router агентами.
//   - GET /api/v1/projects/:id/team → проверяем наличие обоих ролей.

// TestProjectAgents_CreatedOnProjectCreate — при создании проекта автоматически
// создаются team-агенты с ролями orchestrator и router (см. AgentService.CreateDefaultProjectAgents).
func TestProjectAgents_CreatedOnProjectCreate(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// Создаём проект — backend автоматически создаёт team + агенты.
	p := createLocalProject(t, h, user.AccessToken)
	t.Cleanup(func() {
		// Best-effort удаление проекта после теста.
		req, err := http.NewRequest("DELETE", h.BaseURL+"/api/v1/projects/"+p.ID, nil)
		if err != nil {
			t.Logf("cleanup project %s: build request: %v", p.ID, err)
			return
		}
		req.Header.Set("Authorization", "Bearer "+user.AccessToken)
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Logf("cleanup project %s: %v", p.ID, err)
			return
		}
		defer resp.Body.Close()
	})

	// Получаем team проекта — fetchTeam определён в team_smoke_test.go (тот же пакет).
	team := fetchTeam(t, h, user.AccessToken, p.ID)
	require.NotEmpty(t, team.ID, "team ID should not be empty")
	require.Equal(t, p.ID, team.ProjectID, "team.project_id should match project ID")

	// Проверяем, что среди агентов команды есть orchestrator и router.
	require.GreaterOrEqual(t, len(team.Agents), 2,
		"expected at least 2 agents (orchestrator + router), got %d", len(team.Agents))

	roleSet := make(map[string]bool, len(team.Agents))
	for _, a := range team.Agents {
		roleSet[a.Role] = true
	}

	require.True(t, roleSet["orchestrator"],
		"team should contain an agent with role=orchestrator; roles found: %v", roleSet)
	require.True(t, roleSet["router"],
		"team should contain an agent with role=router; roles found: %v", roleSet)
}
