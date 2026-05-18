//go:build featuresmoke

package featuresmoke

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// dbConnOnce — общий sql.DB на пакет, чтобы не открывать соединение на каждый тест.
// Использование ограничено хелпером attachAgentToTeam, который вставляет агента в
// agents-таблицу с team_id. Без публичного REST-эндпоинта «add agent to team»
// это единственный способ заполнить team_agents для тестирования PATCH happy-path.
var (
	dbConnOnce sync.Once
	dbConn     *sql.DB
	dbConnErr  error
)

func directDB(t *testing.T) *sql.DB {
	t.Helper()
	dbConnOnce.Do(func() {
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			envOr("DB_HOST", "localhost"),
			envOr("DB_PORT", "5433"),
			envOr("DB_USER", "yugabyte"),
			envOr("DB_PASSWORD", "yugabyte"),
			envOr("DB_NAME", "yugabyte"),
			envOr("DB_SSLMODE", "disable"),
		)
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			dbConnErr = fmt.Errorf("open db: %w", err)
			return
		}
		db.SetMaxOpenConns(4) // Тестовый хелпер, нагрузка минимальная.
		db.SetConnMaxLifetime(5 * time.Minute)
		if err := db.PingContext(context.Background()); err != nil {
			dbConnErr = fmt.Errorf("ping db: %w", err)
			return
		}
		dbConn = db
		registerGlobalCleanup(func() {
			_ = db.Close()
		})
	})
	if dbConnErr != nil {
		t.Skipf("featuresmoke: direct DB connection unavailable: %v", dbConnErr)
	}
	return dbConn
}

// attachAgentToTeam напрямую обновляет agents.team_id, привязывая агента (созданного
// через POST /api/v1/agents) к команде проекта. Это единственный путь, т.к.
// публичный REST API не предоставляет «add agent to team» — агенты исторически
// заводились через seed-миграции / внутренние сервисы. Используется только в
// смоук-тестах PatchAgent happy-path.
func attachAgentToTeam(t *testing.T, agentID, teamID string) {
	t.Helper()
	db := directDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := db.ExecContext(ctx,
		`UPDATE agents SET team_id = $1::uuid WHERE id = $2::uuid`,
		teamID, agentID)
	if err != nil {
		t.Fatalf("attachAgentToTeam: UPDATE failed: %v", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		t.Fatalf("attachAgentToTeam: агент %s не найден в таблице agents (RowsAffected=0)", agentID)
	}
	// YugabyteDB пишет асинхронно во вьюшку Preload'а — короткий sleep на eventual
	// consistency между UPDATE и последующим GET /team.
	if os.Getenv("FEATURESMOKE_SKIP_DB_SETTLE") == "" {
		time.Sleep(50 * time.Millisecond)
	}
}

// team_smoke_test.go — P0 /api/v1/projects/:id/team.
//
// Контракт: при создании проекта auto-создаётся team. Проверяем GET / PUT
// и PATCH /team/agents/:agentId. Минимальный набор полей (id, name, agents[*].id/role).

type teamAgent struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Role         string                 `json:"role"`
	Model        *string                `json:"model"`
	CodeBackend  *string                `json:"code_backend"`
	IsActive     bool                   `json:"is_active"`
	ToolBindings []toolBindingResponse `json:"tool_bindings"`
}

type toolBindingResponse struct {
	ToolDefinitionID string `json:"tool_definition_id"`
	Name             string `json:"name"`
	Category         string `json:"category"`
}

type teamResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	ProjectID string      `json:"project_id"`
	Type      string      `json:"type"`
	Agents    []teamAgent `json:"agents"`
}

func fetchTeam(t *testing.T, h *Harness, token, projectID string) teamResponse {
	t.Helper()
	resp := h.Do(t, "GET", "/api/v1/projects/"+projectID+"/team", nil, token)
	if resp.Status != http.StatusOK {
		t.Fatalf("get team: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var team teamResponse
	resp.JSON(t, &team)
	if team.ID == "" {
		t.Fatalf("get team: пустой id команды: %s", truncBody(resp.Body))
	}
	if team.ProjectID != projectID {
		t.Fatalf("get team: project_id=%q ожидали %q", team.ProjectID, projectID)
	}
	return team
}

// TestTeam_GetAutoCreatedOnProject — после создания проекта команда уже есть.
func TestTeam_GetAutoCreatedOnProject(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	team := fetchTeam(t, h, user.AccessToken, p.ID)
	if team.Name == "" {
		t.Fatalf("team.name пустое: %+v", team)
	}
}

// TestTeam_UpdateName — переименование команды.
func TestTeam_UpdateName(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	newName := "renamed-" + uuid.NewString()
	resp := h.Do(t, "PUT", "/api/v1/projects/"+p.ID+"/team",
		map[string]any{"name": newName}, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("update team: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var updated teamResponse
	resp.JSON(t, &updated)
	if updated.Name != newName {
		t.Fatalf("update team: name=%q ожидали %q", updated.Name, newName)
	}

	// И GET тоже отдаёт новое имя.
	again := fetchTeam(t, h, user.AccessToken, p.ID)
	if again.Name != newName {
		t.Fatalf("get after rename: name=%q ожидали %q", again.Name, newName)
	}
}

// TestTeam_PatchAgent_DoesNotErrorWhenAgentsEmpty — гард против падения PATCH
// на отсутствующем агенте: PATCH с заведомо несуществующим id должен вернуть
// 404 (или 400), но не 5xx.
func TestTeam_PatchAgentMissingReturns4xx(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	fakeAgentID := uuid.NewString()
	resp := h.Do(t, "PATCH",
		"/api/v1/projects/"+p.ID+"/team/agents/"+fakeAgentID,
		map[string]any{"is_active": false}, user.AccessToken)
	if resp.Status >= 500 {
		t.Fatalf("patch agent missing: server error status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
	if resp.Status < 400 {
		t.Fatalf("patch agent missing: status=%d (ожидали 4xx) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestTeam_PatchAgentHappyPath — полноценный Happy Path PATCH'а агента,
// БЕЗ зависимости от seed. Сценарий:
//   1. Создаём проект → backend заводит пустую team (без агентов).
//   2. Создаём v2-агента через POST /api/v1/agents (он лежит в реестре, team_id=NULL).
//   3. Через прямой UPDATE прикрепляем агента к команде проекта (публичного REST для
//      этого не существует — см. attachAgentToTeam).
//   4. PATCH /team/agents/:id { is_active: false } → 200.
//   5. GET /team — agent.is_active=false, агент остался в списке.
func TestTeam_PatchAgentHappyPath(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	team := fetchTeam(t, h, user.AccessToken, p.ID)

	// Создаём v2-агента: createSmokeAgent живёт в agents_smoke_test.go (общий
	// файл пакета). is_active по умолчанию = true.
	created := createSmokeAgent(t, h, user.AccessToken)
	attachAgentToTeam(t, created.ID, team.ID)

	// Убедимся, что агент теперь виден в /team.
	teamAfter := fetchTeam(t, h, user.AccessToken, p.ID)
	var target *teamAgent
	for i := range teamAfter.Agents {
		if teamAfter.Agents[i].ID == created.ID {
			target = &teamAfter.Agents[i]
			break
		}
	}
	if target == nil {
		t.Fatalf("attachAgentToTeam: агент %s не появился в /team (агентов: %d)",
			created.ID, len(teamAfter.Agents))
	}
	if !target.IsActive {
		t.Fatalf("новый агент должен быть is_active=true, получили false")
	}

	// PATCH: переключаем is_active=false.
	resp := h.Do(t, "PATCH",
		"/api/v1/projects/"+p.ID+"/team/agents/"+target.ID,
		map[string]any{"is_active": false}, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("patch agent: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}

	// GET /team — is_active действительно сменилось.
	after := fetchTeam(t, h, user.AccessToken, p.ID)
	for _, a := range after.Agents {
		if a.ID == target.ID {
			if a.IsActive {
				t.Fatalf("patch agent: is_active=true после PATCH (ожидали false)")
			}
			return
		}
	}
	t.Fatalf("patch agent: целевой агент %s исчез из команды после PATCH", target.ID)
}

// TestTeam_CrossTenantForbidden — чужая команда не доступна.
func TestTeam_CrossTenantForbidden(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)
	p := createLocalProject(t, h, alice.AccessToken)

	resp := h.Do(t, "GET", "/api/v1/projects/"+p.ID+"/team", nil, bob.AccessToken)
	if resp.Status != http.StatusNotFound && resp.Status != http.StatusForbidden {
		t.Fatalf("cross-tenant team get: status=%d (ожидали 404/403)", resp.Status)
	}
}
