//go:build featuresmoke

package featuresmoke

import (
	"testing"

	"github.com/google/uuid"
)

// prompts_smoke_test.go — P2 контракт админ-only /api/v1/prompts.
//
// Что покрываем (через обычного юзера + без токена; реальный admin-flow живёт
// в e2e_real_test.go, как и для llm-providers):
//   - GET/POST/GET-by-id/PUT/DELETE без токена → 401.
//   - То же самое от обычного пользователя → 403 (AdminOnlyMiddleware).
//   - Mutation-эндпоинты не пускают и неавторизованных, и не-админов
//     (важно: запрещаем CRUD «на всех», даже если в БД остался какой-то prompt).
//
// Идея: PR-gate не валидирует happy-path под админом (нет публичной ручки
// «стать админом», и через DB-инъекцию ломать изоляцию — плохая практика);
// зато явно гарантирует, что админ-маршрут защищён обоими слоями.

// promptsAuthCases — общий набор ручек для AssertRequiresAuth.
// Каждая новая ручка /api/v1/prompts добавляется одной строкой —
// и сразу попадает под контракт 401 без токена.
func promptsAuthCases() []EndpointCase {
	return []EndpointCase{
		{Name: "list", Method: "GET", Path: "/api/v1/prompts"},
		{Name: "create", Method: "POST", Path: "/api/v1/prompts", Body: map[string]any{
			"name":     "p-" + uuid.NewString(),
			"template": "hello",
		}},
		{Name: "get", Method: "GET", Path: "/api/v1/prompts/" + uuid.NewString()},
		{Name: "update", Method: "PUT", Path: "/api/v1/prompts/" + uuid.NewString(), Body: map[string]any{
			"template": "x",
		}},
		{Name: "delete", Method: "DELETE", Path: "/api/v1/prompts/" + uuid.NewString()},
	}
}

// promptsAdminCases — набор ручек, требующих роли администратора (403 от обычного юзера).
func promptsAdminCases() []EndpointCase {
	return []EndpointCase{
		{Name: "create", Method: "POST", Path: "/api/v1/prompts", Body: map[string]any{
			"name":     "p-" + uuid.NewString(),
			"template": "hello",
		}},
		{Name: "update", Method: "PUT", Path: "/api/v1/prompts/" + uuid.NewString(), Body: map[string]any{
			"template": "x",
		}},
		{Name: "delete", Method: "DELETE", Path: "/api/v1/prompts/" + uuid.NewString()},
	}
}

func TestPrompts_RequireAuthentication(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	h.AssertRequiresAuth(t, promptsAuthCases())
}

func TestPrompts_RequireAdminForNonAdminUser(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	h.AssertRequiresAdmin(t, user.AccessToken, promptsAdminCases())
}

// TestPrompts_AdminHappyPath_Skip — happy-path CRUD под админом покрывается
// feature-e2e-real.yml (Phase 5), потому что в PR-gate нет ручки «стать
// админом», а ломать политику через прямой UPDATE users.role — плохой
// прецедент (этот тест будет лгать про реальный API-контракт).
func TestPrompts_AdminHappyPath(t *testing.T) {
	t.Parallel()
	t.Skip("admin CRUD prompts покрывает feature-e2e-real.yml (Phase 5)")
}
