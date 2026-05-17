package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/internal/ws"
)

// tools_assistant.go — Sprint 21 §5.
//
// MCP-инструменты «нового» поколения, специфичные для глобального ассистента
// (sidebar). В отличие от `authorized_executor.go` (in-process catalog для
// внутреннего agent loop), здесь — экспозиция тех же tools через HTTP MCP
// для внешних клиентов (Claude Code и т.п.) и для unit-тестируемости.
//
// Каждый tool сам читает user_id/role из ctx через UserIDFromContext /
// UserRoleFromContext (это устанавливает auth middleware) — никаких free-form
// user_id из args, никакого admin-обхода.
//
// Tools:
//   - app_navigate(route)             — публикует WS assistant.navigate user'у.
//   - assistant_active_tasks_count()  — short-query «сколько задач сейчас активно».
//   - whoami()                        — id/email/role текущего юзера.

// ─────────────────────────────────────────────────────────────────────────────
// Params
// ─────────────────────────────────────────────────────────────────────────────

// AppNavigateParams — параметры app_navigate.
//
// jsonschema-tag — голый description-текст (формат google/jsonschema-go v0.4+;
// «WORD=...» больше не принимается). `required` выводится из отсутствия
// `omitempty` в json-теге.
type AppNavigateParams struct {
	Route string `json:"route" jsonschema:"go_router route, e.g. /projects/<uuid> or /dashboard"`
}

// AssistantActiveTasksCountParams — пустой объект; tool работает без аргументов.
type AssistantActiveTasksCountParams struct{}

// WhoAmIParams — пустой объект.
type WhoAmIParams struct{}

// ─────────────────────────────────────────────────────────────────────────────
// Payloads (data в Response)
// ─────────────────────────────────────────────────────────────────────────────

// AppNavigateData — payload tool_result для app_navigate.
type AppNavigateData struct {
	Status string `json:"status"` // "sent"
	Route  string `json:"route"`
}

// AssistantActiveTasksCountData — payload tool_result для assistant_active_tasks_count.
type AssistantActiveTasksCountData struct {
	Count int `json:"count"`
}

// WhoAmIData — payload tool_result для whoami.
type WhoAmIData struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email,omitempty"`
	Role          string `json:"role"`
	EmailVerified bool   `json:"email_verified"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

// UserNotifier — узкий контракт для WS-публикации, требуемый app_navigate.
// Hub из internal/ws удовлетворяет его (SendToUser неблокирующая, см.
// hub.go). Введён намеренно отдельным интерфейсом для unit-тестов и для того,
// чтобы tools_assistant.go не таскал за собой весь Hub.
type UserNotifier interface {
	SendToUser(userID, msgType string, payload []byte) error
}

// compile-time check: *ws.Hub удовлетворяет UserNotifier.
var _ UserNotifier = (*ws.Hub)(nil)

// AssistantToolsDeps — DI для регистрации.
//
// Все поля опциональны: nil-сервис → соответствующий tool не регистрируется.
// Это позволяет включать поверхность ассистента по мере готовности (например,
// app_navigate доступен только если Notifier передан).
type AssistantToolsDeps struct {
	// Notifier — нужен для app_navigate (WS fan-out пользователю).
	Notifier UserNotifier
	// TaskService — нужен для assistant_active_tasks_count.
	TaskService service.TaskService
	// UserRepo — нужен для whoami (email/role lookup).
	UserRepo repository.UserRepository
}

// RegisterAssistantTools регистрирует ассистент-специфичные MCP-инструменты.
//
// Контракт: tools читают user_id/role из ctx (через UserIDFromContext /
// UserRoleFromContext, установленные auth middleware'ом). Если auth не задана —
// возвращается ValidationErr "authentication required".
func RegisterAssistantTools(server *mcp.Server, deps AssistantToolsDeps) {
	if deps.Notifier != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "app_navigate",
			Description: "Просит фронт перейти на указанный go_router маршрут (например '/projects/<uuid>'). Side-effect: WS-событие assistant.navigate пользователю.",
		}, makeAppNavigateHandler(deps.Notifier))
	}

	if deps.TaskService != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "assistant_active_tasks_count",
			Description: "Возвращает количество задач пользователя в state=active по всем проектам. Короткий запрос для быстрых LLM-ответов «сколько задач в работе».",
		}, makeAssistantActiveTasksCountHandler(deps.TaskService))
	}

	if deps.UserRepo != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "whoami",
			Description: "Возвращает информацию о текущем пользователе (id, email, role).",
		}, makeWhoAmIHandler(deps.UserRepo))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// app_navigate
// ─────────────────────────────────────────────────────────────────────────────

// wsTypeAssistantNavigate — тот же тип события, что эмитит AssistantService
// (см. service/assistant_service.go). Дублируется здесь намеренно: MCP-пакет
// не должен импортировать service-пакет ради одной строки-константы.
const wsTypeAssistantNavigate = "assistant.navigate"

func makeAppNavigateHandler(notifier UserNotifier) func(ctx context.Context, req *mcp.CallToolRequest, params *AppNavigateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AppNavigateParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		if params == nil || params.Route == "" {
			return ValidationErr("route is required")
		}

		// Минимальный sanity-check: маршрут должен начинаться со слэша.
		// Это не полная валидация go_router (фронт сам отрулит 404), но
		// блокирует очевидные ошибки LLM («открой projects», без слэша).
		if params.Route[0] != '/' {
			return ValidationErr("route must start with '/'")
		}

		payload, err := json.Marshal(map[string]string{
			"type":  wsTypeAssistantNavigate,
			"route": params.Route,
		})
		if err != nil {
			return Err("internal serialization error", err)
		}
		if err := notifier.SendToUser(uid.String(), wsTypeAssistantNavigate, payload); err != nil {
			// Hub.SendToUser возвращает ошибку только при пустом userID.
			// При переполнении канала сообщение дропается (silent drop) и возвращается nil.
			return Err(fmt.Sprintf("failed to publish navigate event: %v", err), err)
		}
		return OK("navigate event sent", AppNavigateData{Status: "sent", Route: params.Route})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// assistant_active_tasks_count
// ─────────────────────────────────────────────────────────────────────────────

// activeTasksCountLimit — soft-cap: ListActiveByUser возвращает не больше
// этого числа задач, count = len(slice). Если у пользователя 1000 активных
// задач — мы вернём 1000 (а не точное число), что для UX «сколько в работе»
// более чем достаточно. Magic 1000 здесь — sentinel, см. сервисный метод.
const activeTasksCountLimit = 1000

func makeAssistantActiveTasksCountHandler(taskSvc service.TaskService) func(ctx context.Context, req *mcp.CallToolRequest, params *AssistantActiveTasksCountParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *AssistantActiveTasksCountParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		rows, err := taskSvc.ListActiveByUser(ctx, uid, []models.TaskState{models.TaskStateActive}, activeTasksCountLimit)
		if err != nil {
			return Err("failed to list active tasks", err)
		}
		return OK(fmt.Sprintf("found %d active tasks", len(rows)), AssistantActiveTasksCountData{Count: len(rows)})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// whoami
// ─────────────────────────────────────────────────────────────────────────────

func makeWhoAmIHandler(userRepo repository.UserRepository) func(ctx context.Context, req *mcp.CallToolRequest, params *WhoAmIParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ *WhoAmIParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		// Lookup юзера — нам нужны email/role. Role в ctx уже есть, но email — нет.
		u, err := userRepo.GetByID(ctx, uid)
		if err != nil {
			if errors.Is(err, repository.ErrUserNotFound) {
				// Странный кейс: middleware прошло, но юзер удалён. Возвращаем
				// минимум — то, что точно есть в ctx.
				role, _ := UserRoleFromContext(ctx)
				return OK("user not found in db (deleted?), returning ctx data only", WhoAmIData{
					UserID: uid.String(),
					Role:   string(role),
				})
			}
			return Err("failed to load user", err)
		}
		return OK("ok", WhoAmIData{
			UserID:        u.ID.String(),
			Email:         u.Email,
			Role:          string(u.Role),
			EmailVerified: u.EmailVerified,
		})
	}
}

