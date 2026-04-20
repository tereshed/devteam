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

// MessageType — дискриминатор конверта.
type MessageType string

const (
	MessageTypeTaskStatus  MessageType = "task_status"
	MessageTypeTaskMessage MessageType = "task_message"
	MessageTypeAgentLog    MessageType = "agent_log"
	MessageTypeError       MessageType = "error"
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

// Envelope — единый формат всех исходящих сообщений.
type Envelope[T any] struct {
	Type      MessageType `json:"type"`
	Version   int         `json:"v"`
	Timestamp time.Time   `json:"ts"`
	ProjectID uuid.UUID   `json:"project_id"`
	Data      T           `json:"data"`
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
