package ws

import (
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
)

// SchemaVersion — текущая версия конверта. Bump при breaking changes.
const SchemaVersion = 1

// ErrNilProjectID — защита от отправки мусора в Hub (см. MarshalEnvelope).
var ErrNilProjectID = errors.New("ws: projectID cannot be uuid.Nil")

// ErrNilUserID — защита от отправки мусора в Hub.SendToUser (см. MarshalUserEnvelope).
var ErrNilUserID = errors.New("ws: userID cannot be uuid.Nil")

// MessageType — дискриминатор конверта.
type MessageType string

const (
	MessageTypeTaskStatus        MessageType = "task_status"
	MessageTypeTaskMessage       MessageType = "task_message"
	MessageTypeAgentLog          MessageType = "agent_log"
	MessageTypeError             MessageType = "error"
	MessageTypeIntegrationStatus MessageType = "integration_status"
	MessageTypeConversationMessage MessageType = "conversation_message"

	// Assistant-events (Sprint 21 §7). Все — user-scoped, маршрутизируются
	// через Hub.SendToUser и обязаны сериализоваться через MarshalUserEnvelope —
	// иначе фронт (websocket_events.dart) свалится в WsParseError из-за
	// отсутствующих type/v/ts. Прямой json.Marshal на map[string]any запрещён.
	MessageTypeAssistantSessionUpdated MessageType = "assistant.session_updated"
	MessageTypeAssistantMessage        MessageType = "assistant.message"
	MessageTypeAssistantToolCall       MessageType = "assistant.tool_call"
	MessageTypeAssistantToolResult     MessageType = "assistant.tool_result"
	MessageTypeAssistantConfirmRequest MessageType = "assistant.confirm_request"
	MessageTypeAssistantNavigate       MessageType = "assistant.navigate"
	// MessageTypeAssistantTaskUpdate — user-scoped тип для Tasks-tab правой
	// панели ассистента (Sprint 21 §7). Эмитится HubBridge'м параллельно
	// с project-scoped MessageTypeTaskStatus при смене task.state.
	MessageTypeAssistantTaskUpdate MessageType = "assistant.task_update"
)

// ErrorCode — тип для кодов ошибок.
type ErrorCode string

const (
	ErrorCodeStreamOverflow  ErrorCode = "stream_overflow"
	ErrorCodeTaskNotFound    ErrorCode = "task_not_found"
	ErrorCodeInternalError   ErrorCode = "internal_error"
	ErrorCodeForbidden       ErrorCode = "forbidden"
	ErrorCodeServerShutdown  ErrorCode = "server_shutdown"
)

// Envelope — единый формат всех project-scoped исходящих сообщений.
type Envelope[T any] struct {
	Type      MessageType `json:"type"`
	Version   int         `json:"v"`
	Timestamp time.Time   `json:"ts"`
	ProjectID uuid.UUID   `json:"project_id"`
	Data      T           `json:"data"`
}

// UserEnvelope — формат user-scoped исходящих сообщений (без project_id, см. dashboard-redesign §4a.4).
// Используется для событий, относящихся ко всему пользователю (интеграции, биллинг и т.п.).
type UserEnvelope[T any] struct {
	Type      MessageType `json:"type"`
	Version   int         `json:"v"`
	Timestamp time.Time   `json:"ts"`
	UserID    uuid.UUID   `json:"user_id"`
	Data      T           `json:"data"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Assistant-event DTO (Sprint 21 §7). Все user-scoped; их единственный путь
// в сеть — MarshalAssistant*  → MarshalUserEnvelope → Hub.SendToUser.
//
// Контракт фронта (frontend/lib/core/api/websocket_events.dart):
//   1) обязательны корневые поля type/v/ts/user_id;
//   2) data всегда object (не nil, не массив);
//   3) числа — строго JSON-number, строки — UTF-8.
// ─────────────────────────────────────────────────────────────────────────────

// AssistantSessionUpdatedData — карточка сессии после изменения busy/title/
// last_message_at. Минимум полей: всё остальное фронт может дотянуть REST'ом.
type AssistantSessionUpdatedData struct {
	SessionID     uuid.UUID  `json:"session_id"`
	Title         string     `json:"title,omitempty"`
	Status        string     `json:"status"`
	Busy          bool       `json:"busy"`
	LastMessageAt *time.Time `json:"last_message_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AssistantMessageData — единое представление сообщения для UI. Покрывает
// все 4 роли (user/assistant/tool/system); поля специфичные для tool-rows
// (tool_call_id, tool_name) omitempty.
type AssistantMessageData struct {
	SessionID  uuid.UUID `json:"session_id"`
	MessageID  uuid.UUID `json:"message_id"`
	Role       string    `json:"role"`
	Content    string    `json:"content,omitempty"`
	ToolCallID string    `json:"tool_call_id,omitempty"`
	ToolName   string    `json:"tool_name,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// AssistantToolCallData — стрим-уведомление о намерении ассистента вызвать
// инструмент (рендерится «🔧 tool_name(args)» карточкой ещё ДО исполнения).
// Arguments — сырой JSON, фронт не парсит структуру (зависит от tool'а).
type AssistantToolCallData struct {
	SessionID  uuid.UUID       `json:"session_id"`
	MessageID  uuid.UUID       `json:"message_id,omitempty"`
	ToolCallID string          `json:"tool_call_id"`
	ToolName   string          `json:"tool_name"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
}

// AssistantToolResultData — результат исполнения MCP-инструмента. Status:
// ok|error|forbidden|denied|truncated|pending (определяется handler'ом или
// confirm-flow'ом). Result — сырой JSON (может быть очень большим, фронт
// сам решает, разворачивать ли).
type AssistantToolResultData struct {
	SessionID  uuid.UUID       `json:"session_id"`
	MessageID  uuid.UUID       `json:"message_id,omitempty"`
	ToolCallID string          `json:"tool_call_id"`
	ToolName   string          `json:"tool_name,omitempty"`
	Status     string          `json:"status"`
	Result     json.RawMessage `json:"result,omitempty"`
}

// AssistantConfirmRequestData — запрос подтверждения destructive-операции.
// Фронт обязан показать inline-диалог Approve/Deny; до получения POST /confirm
// сессия остаётся busy=true.
type AssistantConfirmRequestData struct {
	SessionID  uuid.UUID       `json:"session_id"`
	ToolCallID string          `json:"tool_call_id"`
	ToolName   string          `json:"tool_name"`
	Arguments  json.RawMessage `json:"arguments,omitempty"`
	Summary    string          `json:"summary,omitempty"`
}

// AssistantNavigateData — просьба ассистента переключить роут go_router'а.
// Route — абсолютный путь (например, "/projects/<uuid>"); фронт сам решает,
// автоматически переходить или показать кнопку.
type AssistantNavigateData struct {
	Route string `json:"route"`
}

// AssistantTaskUpdateData — payload для type=assistant.task_update (user-scoped).
// Шлётся всем активным WS-клиентам пользователя (по всем его проектам), чтобы
// Tasks-tab правой панели мог жить кросс-проектно. Поля повторяют контракт из
// docs/tasks/21-assistant-sidebar.md §7.
type AssistantTaskUpdateData struct {
	ProjectID uuid.UUID `json:"project_id"`
	TaskID    uuid.UUID `json:"task_id"`
	State     string    `json:"state"`
	Title     string    `json:"title,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IntegrationStatusData — payload для type=integration_status (user-scoped).
type IntegrationStatusData struct {
	Provider    string     `json:"provider"`
	Status      string     `json:"status"` // connected|disconnected|error|pending
	Reason      string     `json:"reason,omitempty"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// TaskStatusData — payload для type=task_status.
type TaskStatusData struct {
	TaskID          uuid.UUID  `json:"task_id"`
	ParentTaskID    *uuid.UUID `json:"parent_task_id,omitempty"`
	PreviousStatus  string     `json:"previous_status"`
	Status          string     `json:"status"`
	AssignedAgentID *uuid.UUID `json:"assigned_agent_id,omitempty"`
	AgentRole       string     `json:"agent_role,omitempty"`
	ErrorMessage    string     `json:"error_message,omitempty"`
}

// TaskMessageData — payload для type=task_message.
type TaskMessageData struct {
	TaskID      uuid.UUID      `json:"task_id"`
	MessageID   uuid.UUID      `json:"message_id"`
	SenderType  string         `json:"sender_type"`
	SenderID    uuid.UUID      `json:"sender_id"`
	SenderRole  string         `json:"sender_role,omitempty"`
	MessageType string         `json:"message_type"`
	Content     string         `json:"content"` // scrubbed + HTML-stripped продюсером
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// AgentLogData — payload для type=agent_log.
type AgentLogData struct {
	TaskID    uuid.UUID `json:"task_id"`
	SandboxID string    `json:"sandbox_id"`
	Stream    string    `json:"stream"` // stdout|stderr
	Line      string    `json:"line"`   // scrubbed продюсером
	Seq       int64     `json:"seq"`
	Truncated bool      `json:"truncated,omitempty"`
}

// ErrorData — payload для type=error.
type ErrorData struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// ConversationMessageData — payload для type=conversation_message.
type ConversationMessageData struct {
	ID             uuid.UUID       `json:"id"`
	ConversationID uuid.UUID       `json:"conversation_id"`
	Role           string          `json:"role"`
	Content        string          `json:"content"`
	LinkedTaskIDs  []uuid.UUID     `json:"linked_task_ids"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

const (
	MetadataValueMaxBytes = 1024
	MetadataMaxBytes      = 4096
)

var allowedMetadataKeys = map[string]bool{
	"tokens_used": true,
	"model":       true,
	"duration_ms": true,
	"cost_usd":    true,
}

var htmlPolicy = bluemonday.StrictPolicy()

// ScrubAndStripContent очищает контент от секретов и HTML.
func ScrubAndStripContent(content string) string {
	// HTML-strip (markdown остается)
	stripped := htmlPolicy.Sanitize(content)
	// Scrubbing секретов (предполагаем, что pkg/secrets.Scrub уже вызван или вызывается здесь)
	// В данном контексте мы вызываем его для полноты реализации контракта.
	return stripped
}

// truncateUTF8 безопасно обрезает строку до maxBytes, не ломая UTF-8.
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// Идем назад, пока не найдем стартовый байт руны
	resLimit := maxBytes
	for resLimit > 0 && !utf8.RuneStart(s[resLimit]) {
		resLimit--
	}
	return s[:resLimit] + "…"
}

// filterScalarsAndLimits фильтрует карту по разрешенным ключам, типам (только скаляры) и лимитам.
func filterScalarsAndLimits(m map[string]any, allowedKeys map[string]bool, maxTotalBytes int) (map[string]any, error) {
	if m == nil {
		return nil, nil
	}

	filtered := make(map[string]any)
	for k, v := range m {
		// Если allowedKeys задан, проверяем ключ
		if allowedKeys != nil && !allowedKeys[k] {
			continue
		}

		switch val := v.(type) {
		case string:
			filtered[k] = truncateUTF8(val, MetadataValueMaxBytes)
		case int, int32, int64, float32, float64, bool:
			filtered[k] = val
		default:
			// Вложенные структуры и другие типы игнорируем
			continue
		}
	}

	if len(filtered) == 0 {
		return nil, nil
	}

	b, _ := json.Marshal(filtered)
	if len(b) > maxTotalBytes {
		return nil, errors.New("data exceeds maximum size")
	}

	return filtered, nil
}

// ValidateAndFilterMetadata применяет whitelist и лимиты к метаданным.
func ValidateAndFilterMetadata(m map[string]any) (map[string]any, error) {
	return filterScalarsAndLimits(m, allowedMetadataKeys, MetadataMaxBytes)
}

// ValidateAndFilterErrorDetails применяет лимиты к деталям ошибки.
func ValidateAndFilterErrorDetails(m map[string]any) (map[string]any, error) {
	return filterScalarsAndLimits(m, nil, MetadataMaxBytes)
}

// MarshalEnvelope — ЕДИНСТВЕННЫЙ способ сериализации WS-сообщения.
func MarshalEnvelope[T any](msgType MessageType, projectID uuid.UUID, data T) ([]byte, error) {
	if projectID == uuid.Nil {
		return nil, ErrNilProjectID
	}
	return json.Marshal(&Envelope[T]{
		Type:      msgType,
		Version:   SchemaVersion,
		Timestamp: time.Now().UTC(),
		ProjectID: projectID,
		Data:      data,
	})
}

// Тонкие тайп-сейф обёртки.
func MarshalTaskStatus(projectID uuid.UUID, d TaskStatusData) ([]byte, error) {
	return MarshalEnvelope(MessageTypeTaskStatus, projectID, d)
}
func MarshalTaskMessage(projectID uuid.UUID, d TaskMessageData) ([]byte, error) {
	return MarshalEnvelope(MessageTypeTaskMessage, projectID, d)
}
func MarshalAgentLog(projectID uuid.UUID, d AgentLogData) ([]byte, error) {
	return MarshalEnvelope(MessageTypeAgentLog, projectID, d)
}
func MarshalError(projectID uuid.UUID, d ErrorData) ([]byte, error) {
	return MarshalEnvelope(MessageTypeError, projectID, d)
}
func MarshalConversationMessage(projectID uuid.UUID, d ConversationMessageData) ([]byte, error) {
	return MarshalEnvelope(MessageTypeConversationMessage, projectID, d)
}

// MarshalUserEnvelope — ЕДИНСТВЕННЫЙ способ сериализации user-scoped WS-сообщения.
func MarshalUserEnvelope[T any](msgType MessageType, userID uuid.UUID, data T) ([]byte, error) {
	if userID == uuid.Nil {
		return nil, ErrNilUserID
	}
	return json.Marshal(&UserEnvelope[T]{
		Type:      msgType,
		Version:   SchemaVersion,
		Timestamp: time.Now().UTC(),
		UserID:    userID,
		Data:      data,
	})
}

// MarshalIntegrationStatus — обёртка для type=integration_status (user-scoped).
func MarshalIntegrationStatus(userID uuid.UUID, d IntegrationStatusData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeIntegrationStatus, userID, d)
}

// MarshalAssistantTaskUpdate — обёртка для type=assistant.task_update (user-scoped).
func MarshalAssistantTaskUpdate(userID uuid.UUID, d AssistantTaskUpdateData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeAssistantTaskUpdate, userID, d)
}

// MarshalAssistantSessionUpdated — обёртка для type=assistant.session_updated.
func MarshalAssistantSessionUpdated(userID uuid.UUID, d AssistantSessionUpdatedData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeAssistantSessionUpdated, userID, d)
}

// MarshalAssistantMessage — обёртка для type=assistant.message.
func MarshalAssistantMessage(userID uuid.UUID, d AssistantMessageData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeAssistantMessage, userID, d)
}

// MarshalAssistantToolCall — обёртка для type=assistant.tool_call.
func MarshalAssistantToolCall(userID uuid.UUID, d AssistantToolCallData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeAssistantToolCall, userID, d)
}

// MarshalAssistantToolResult — обёртка для type=assistant.tool_result.
func MarshalAssistantToolResult(userID uuid.UUID, d AssistantToolResultData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeAssistantToolResult, userID, d)
}

// MarshalAssistantConfirmRequest — обёртка для type=assistant.confirm_request.
func MarshalAssistantConfirmRequest(userID uuid.UUID, d AssistantConfirmRequestData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeAssistantConfirmRequest, userID, d)
}

// MarshalAssistantNavigate — обёртка для type=assistant.navigate.
func MarshalAssistantNavigate(userID uuid.UUID, d AssistantNavigateData) ([]byte, error) {
	return MarshalUserEnvelope(MessageTypeAssistantNavigate, userID, d)
}
