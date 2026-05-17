package dto

import (
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Sprint 21 — Assistant Sidebar (docs/tasks/21-assistant-sidebar.md §4).
//
// DTO для глобального ассистента правой панели. Параллель conversation_dto.go,
// но scope=user (без project_id) и сообщения имеют tool-call поля.

// ─────────────────────────────────────────────────────────────────────────────
// Requests
// ─────────────────────────────────────────────────────────────────────────────

// CreateAssistantSessionRequest — пустое тело: сессия создаётся без title,
// title выставляется автоматически по первому ответу модели или через
// UpdateSessionTitle (см. план §2 «UpdateSessionTitle»).
type CreateAssistantSessionRequest struct{}

// SendAssistantMessageRequest — отправка user-сообщения.
//
// client_message_id — UUIDv4 ключ идемпотентности (план §4.1: повторный
// POST с тем же значением → 202 Accepted с тем же message_id, без второй
// агент-петли). Источник правды — уникальный partial-индекс
// `idx_assistant_messages_client_id`.
type SendAssistantMessageRequest struct {
	Content         string `json:"content" binding:"required,max=4096" example:"Покажи мои проекты"`
	ClientMessageID string `json:"client_message_id" example:"3f1b6d4a-2c1e-4b9f-8f1a-1f1c5a9d2b7e"`
}

// ConfirmToolCallRequest — подтверждение/отказ destructive операции
// (план §4.1). approved=false синтезирует `{status:"denied"}` payload без
// исполнения MCP-tool'а.
//
// client_request_id — необязательный, идёт только в лог для трассировки.
// Источник правды идемпотентности — атомарный UPDATE по `tool_result IS NULL`
// внутри ConfirmAndClosePending; client_request_id здесь — лишь crumb для
// постмортемов (не уникальный ключ).
type ConfirmToolCallRequest struct {
	ToolCallID      string `json:"tool_call_id" binding:"required,max=64" example:"call_01H..."`
	Approved        bool   `json:"approved" example:"true"`
	ClientRequestID string `json:"client_request_id,omitempty" example:"4a2b6c5d-7e8f-4a3b-9c1d-2f3e4a5b6c7d"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Responses — sessions
// ─────────────────────────────────────────────────────────────────────────────

// AssistantSessionResponse — детали сессии для UI.
//
// Поля busy/busy_since/pending_tool_call_id отдаются фронту, чтобы он мог
// дизейблить input до прихода `assistant.session_updated busy=false`
// (план §3.1 «UX-контракт»). status='archived' — soft-delete, фронт скрывает
// архивные сессии из основного списка.
type AssistantSessionResponse struct {
	ID                uuid.UUID      `json:"id"`
	UserID            uuid.UUID      `json:"user_id"`
	Title             *string        `json:"title,omitempty"`
	Status            string         `json:"status" example:"active"`
	Busy              bool           `json:"busy"`
	BusySince         *time.Time     `json:"busy_since,omitempty"`
	PendingToolCallID *string        `json:"pending_tool_call_id,omitempty"`
	Metadata          datatypes.JSON `json:"metadata,omitempty" swaggertype:"object"`
	LastMessageAt     *time.Time     `json:"last_message_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// AssistantSessionListResponse — список сессий пользователя.
// Сортировка — last_message_at DESC NULLS LAST (см. репозиторий ListSessionsByUser).
type AssistantSessionListResponse struct {
	Sessions []AssistantSessionResponse `json:"sessions"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Responses — messages
// ─────────────────────────────────────────────────────────────────────────────

// AssistantMessageResponse — одно сообщение сессии. Поле tool_result может
// быть nil при role=tool, если сообщение ещё в pending (ожидает confirm).
type AssistantMessageResponse struct {
	ID              uuid.UUID      `json:"id"`
	SessionID       uuid.UUID      `json:"session_id"`
	Role            string         `json:"role" example:"assistant"`
	Content         *string        `json:"content,omitempty"`
	ToolCallID      *string        `json:"tool_call_id,omitempty"`
	ToolName        *string        `json:"tool_name,omitempty"`
	ToolArguments   datatypes.JSON `json:"tool_arguments,omitempty" swaggertype:"object"`
	ToolResult      datatypes.JSON `json:"tool_result,omitempty" swaggertype:"object"`
	ClientMessageID *string        `json:"client_message_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

// AssistantMessageListResponse — курсорная пагинация (план §2).
//
// Курсор — пара (created_at, id) последнего элемента ответа: фронт передаёт
// их в `before_created_at` + `before_id` следующего запроса. has_more=true,
// если репо вернул ровно `limit` строк (значит есть ещё страница).
//
// Используем DESC по умолчанию (сначала новые), как ожидает UI чата с
// бесконечным scrolling'ом вверх. has_more нужен фронту, чтобы понять,
// показывать ли «Loading older messages…».
type AssistantMessageListResponse struct {
	Messages              []AssistantMessageResponse `json:"messages"`
	Limit                 int                        `json:"limit"`
	HasMore               bool                       `json:"has_more"`
	NextBeforeCreatedAt   *time.Time                 `json:"next_before_created_at,omitempty"`
	NextBeforeID          *uuid.UUID                 `json:"next_before_id,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Responses — active tasks (Tasks-tab правой панели)
// ─────────────────────────────────────────────────────────────────────────────

// AssistantActiveTaskResponse — карточка для Tasks-tab правой панели.
// Фронт показывает project_name · title · state · updated_at и тапом ведёт
// на /projects/:project_id/tasks/:task_id (план §1 frontend).
type AssistantActiveTaskResponse struct {
	TaskID      uuid.UUID `json:"task_id"`
	ProjectID   uuid.UUID `json:"project_id"`
	ProjectName string    `json:"project_name"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AssistantActiveTasksResponse — обёртка для /assistant/active-tasks.
type AssistantActiveTasksResponse struct {
	Tasks []AssistantActiveTaskResponse `json:"tasks"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Responses — confirm / send (202 Accepted)
// ─────────────────────────────────────────────────────────────────────────────

// SendAssistantMessageResponse — 202 Accepted: user-row уже записан, агент-петля
// побежит асинхронно; финальный assistant-ответ придёт через WS
// (assistant.message). При повторе с тем же client_message_id возвращается
// тот же ID без новой петли (идемпотентность).
type SendAssistantMessageResponse struct {
	Message AssistantMessageResponse `json:"message"`
	// Duplicate=true → сервер не запустил петлю заново (idempotent replay).
	// Фронту полезно: можно не показывать typing-индикатор «думаю...».
	Duplicate bool `json:"duplicate"`
}

// ConfirmToolCallResponse — пустой ack-ответ. UI узнает результат через
// WS-события (assistant.tool_result + assistant.message).
type ConfirmToolCallResponse struct {
	Accepted bool `json:"accepted"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Mappers
// ─────────────────────────────────────────────────────────────────────────────

// ToAssistantSessionResponse — models → DTO.
func ToAssistantSessionResponse(s *models.AssistantSession) AssistantSessionResponse {
	if s == nil {
		return AssistantSessionResponse{}
	}
	return AssistantSessionResponse{
		ID:                s.ID,
		UserID:            s.UserID,
		Title:             s.Title,
		Status:            string(s.Status),
		Busy:              s.Busy,
		BusySince:         s.BusySince,
		PendingToolCallID: s.PendingToolCallID,
		Metadata:          s.Metadata,
		LastMessageAt:     s.LastMessageAt,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

// ToAssistantSessionListResponse — пачка models → DTO.
func ToAssistantSessionListResponse(sessions []*models.AssistantSession) AssistantSessionListResponse {
	out := make([]AssistantSessionResponse, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, ToAssistantSessionResponse(s))
	}
	return AssistantSessionListResponse{Sessions: out}
}

// ToAssistantMessageResponse — models → DTO.
func ToAssistantMessageResponse(m *models.AssistantMessage) AssistantMessageResponse {
	if m == nil {
		return AssistantMessageResponse{}
	}
	return AssistantMessageResponse{
		ID:              m.ID,
		SessionID:       m.SessionID,
		Role:            string(m.Role),
		Content:         m.Content,
		ToolCallID:      m.ToolCallID,
		ToolName:        m.ToolName,
		ToolArguments:   m.ToolArguments,
		ToolResult:      m.ToolResult,
		ClientMessageID: m.ClientMessageID,
		CreatedAt:       m.CreatedAt,
	}
}

// ToAssistantMessageListResponse — собирает курсорный ответ.
//
// `limit` — тот лимит, что отправили в репо (после нормализации). HasMore=true
// если ровно limit получили: репо вернёт `limit+0` строк, но в курсоре нет
// total — отсюда heuristic. При нагрузке это работает корректно, потому что
// нестабильности по равному created_at репо уже разрешил вторичным ключом id.
func ToAssistantMessageListResponse(messages []*models.AssistantMessage, limit int) AssistantMessageListResponse {
	out := make([]AssistantMessageResponse, 0, len(messages))
	for _, m := range messages {
		out = append(out, ToAssistantMessageResponse(m))
	}
	resp := AssistantMessageListResponse{
		Messages: out,
		Limit:    limit,
		HasMore:  len(messages) >= limit && limit > 0,
	}
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		t := last.CreatedAt
		id := last.ID
		resp.NextBeforeCreatedAt = &t
		resp.NextBeforeID = &id
	}
	return resp
}

// Маппинг service.ActiveTaskSummary → DTO живёт в handler-слое
// (assistant_handler.go: toActiveTasksResponse), чтобы пакет dto не
// импортировал service. Цикл `service ← dto ← service` запрещён —
// conversation_service.go уже импортирует dto, и обратная ссылка
// сломает компиляцию (см. cmd/api build error).
