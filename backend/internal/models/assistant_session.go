package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AssistantSessionStatus — состояние ассистент-сессии в правой панели.
// См. docs/tasks/21-assistant-sidebar.md §1.
type AssistantSessionStatus string

const (
	AssistantSessionStatusActive   AssistantSessionStatus = "active"
	AssistantSessionStatusArchived AssistantSessionStatus = "archived"
)

// IsValid проверяет допустимость статуса.
func (s AssistantSessionStatus) IsValid() bool {
	switch s {
	case AssistantSessionStatusActive, AssistantSessionStatusArchived:
		return true
	default:
		return false
	}
}

// AssistantMessageRole — роль автора сообщения в assistant-сессии.
// LLM tool-calling раскладывается на пары:
//
//	role=assistant с tool_call_id+tool_name+tool_arguments
//	role=tool      с тем же tool_call_id и tool_result после исполнения.
//
// Контракт миграции 045 (см. partial UNIQUE по tool_call_id WHERE role='tool').
type AssistantMessageRole string

const (
	AssistantMessageRoleUser      AssistantMessageRole = "user"
	AssistantMessageRoleAssistant AssistantMessageRole = "assistant"
	AssistantMessageRoleTool      AssistantMessageRole = "tool"
	AssistantMessageRoleSystem    AssistantMessageRole = "system"
)

// IsValid проверяет допустимость роли.
func (r AssistantMessageRole) IsValid() bool {
	switch r {
	case AssistantMessageRoleUser, AssistantMessageRoleAssistant,
		AssistantMessageRoleTool, AssistantMessageRoleSystem:
		return true
	default:
		return false
	}
}

// AssistantSession — глобальный ассистент пользователя (scope=user, без project_id).
//
// Инварианты сериализации agent-loop (см. план §3.1):
//   - Busy=true ровно тогда, когда активна агент-петля (≤ 1 на сессию).
//   - BusySince выставлен ровно тогда же (CHECK constraint в БД).
//   - PendingToolCallID не nil только при «припаркованной» петле, ожидающей confirm.
type AssistantSession struct {
	ID                uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID            uuid.UUID              `gorm:"type:uuid;not null" json:"user_id"`
	Title             *string                `gorm:"type:varchar(255)" json:"title,omitempty"`
	Status            AssistantSessionStatus `gorm:"type:varchar(32);not null;default:'active'" json:"status"`
	Busy              bool                   `gorm:"type:boolean;not null;default:false" json:"busy"`
	BusySince         *time.Time             `gorm:"type:timestamp with time zone" json:"busy_since,omitempty"`
	PendingToolCallID *string                `gorm:"type:varchar(64);column:pending_tool_call_id" json:"pending_tool_call_id,omitempty"`
	Metadata          datatypes.JSON         `gorm:"type:jsonb" json:"metadata,omitempty" swaggertype:"object"`
	LastMessageAt     *time.Time             `gorm:"type:timestamp with time zone" json:"last_message_at,omitempty"`
	CreatedAt         time.Time              `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt         time.Time              `gorm:"type:timestamp with time zone;not null;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (AssistantSession) TableName() string {
	return "assistant_sessions"
}

// BeforeCreate генерирует UUID если не задан.
func (s *AssistantSession) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// AssistantMessage — сообщение сессии (user / assistant / tool / system).
//
// Для role=assistant с tool_call: ToolCallID/ToolName/ToolArguments заполнены,
// Content может быть пустым, ToolResult всегда nil.
// Для role=tool: ToolCallID ссылается на парный assistant-row; ToolResult
// заполнен после исполнения MCP-вызова (или synthetic deny при отказе confirm).
// До прихода ConfirmToolCall возможна pending-строка с ToolResult=nil.
type AssistantMessage struct {
	ID              uuid.UUID            `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SessionID       uuid.UUID            `gorm:"type:uuid;not null" json:"session_id"`
	Role            AssistantMessageRole `gorm:"type:varchar(16);not null" json:"role"`
	Content         *string              `gorm:"type:text" json:"content,omitempty"`
	ToolCallID      *string              `gorm:"type:varchar(64);column:tool_call_id" json:"tool_call_id,omitempty"`
	ToolName        *string              `gorm:"type:varchar(128)" json:"tool_name,omitempty"`
	ToolArguments   datatypes.JSON       `gorm:"type:jsonb" json:"tool_arguments,omitempty" swaggertype:"object"`
	ToolResult      datatypes.JSON       `gorm:"type:jsonb" json:"tool_result,omitempty" swaggertype:"object"`
	ClientMessageID *string              `gorm:"type:varchar(64);column:client_message_id" json:"client_message_id,omitempty"`
	CreatedAt       time.Time            `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt       time.Time            `gorm:"type:timestamp with time zone;not null;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (AssistantMessage) TableName() string {
	return "assistant_messages"
}

// BeforeCreate генерирует UUID если не задан.
func (m *AssistantMessage) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
