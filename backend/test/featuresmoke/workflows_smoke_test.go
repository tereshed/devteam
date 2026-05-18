//go:build featuresmoke

package featuresmoke

import (
	"testing"

	"github.com/google/uuid"
)

// workflows_smoke_test.go — P2 контракт админ-only /api/v1/workflows и /executions.
//
// Аналогично prompts_smoke_test.go: PR-gate тестирует authn/authz контур.
// Реальный happy-path запуска воркфлоу под админом — feature-e2e-real.yml.

// workflowsAuthCases — общий набор ручек /workflows + /executions. Новые
// admin-only маршруты добавляются одной строкой и сразу под обоими
// контрактами (401/403). См. AssertRequiresAuth/AssertRequiresAdmin в harness.go.
func workflowsAuthCases() []EndpointCase {
	return []EndpointCase{
		{Name: "list_workflows", Method: "GET", Path: "/api/v1/workflows"},
		{Name: "start_workflow", Method: "POST", Path: "/api/v1/workflows/anything/start", Body: map[string]any{
			"input": map[string]any{},
		}},
		{Name: "list_executions", Method: "GET", Path: "/api/v1/executions"},
		{Name: "get_execution", Method: "GET", Path: "/api/v1/executions/" + uuid.NewString()},
		{Name: "execution_steps", Method: "GET", Path: "/api/v1/executions/" + uuid.NewString() + "/steps"},
	}
}

func TestWorkflows_RequireAuthentication(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	h.AssertRequiresAuth(t, workflowsAuthCases())
}

func TestWorkflows_RequireAdminForNonAdminUser(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	h.AssertRequiresAdmin(t, user.AccessToken, workflowsAuthCases())
}

// TestWorkflows_AdminHappyPath_Skip — list/start под админом покрывается
// feature-e2e-real.yml (Phase 5), как и аналогичный admin-flow в prompts.
func TestWorkflows_AdminHappyPath(t *testing.T) {
	t.Parallel()
	t.Skip("admin list/start workflows покрывает feature-e2e-real.yml (Phase 5)")
}
