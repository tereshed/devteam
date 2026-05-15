package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SenderType тип отправителя сообщения задачи (полиморфная связь sender_id)
type SenderType string

const (
	SenderTypeUser  SenderType = "user"
	SenderTypeAgent SenderType = "agent"
)

// IsValid проверяет валидность типа отправителя
func (s SenderType) IsValid() bool {
	switch s {
	case SenderTypeUser, SenderTypeAgent:
		return true
	default:
		return false
	}
}

// MessageType тип содержимого сообщения в контексте задачи
type MessageType string

const (
	MessageTypeInstruction MessageType = "instruction"
	MessageTypeResult      MessageType = "result"
	MessageTypeQuestion    MessageType = "question"
	MessageTypeFeedback    MessageType = "feedback"
	MessageTypeError       MessageType = "error"
	MessageTypeComment     MessageType = "comment"
	MessageTypeSummary     MessageType = "summary"
)

// IsValid проверяет валидность типа сообщения
func (m MessageType) IsValid() bool {
	switch m {
	case MessageTypeInstruction, MessageTypeResult, MessageTypeQuestion,
		MessageTypeFeedback, MessageTypeError, MessageTypeComment, MessageTypeSummary:
		return true
	default:
		return false
	}
}

// TaskMessage лог коммуникации по задаче (агенты / пользователь)
type TaskMessage struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TaskID      uuid.UUID       `gorm:"type:uuid;not null" json:"task_id"`
	Task        *Task           `gorm:"foreignKey:TaskID" json:"task,omitempty"`
	SenderType  SenderType      `gorm:"type:varchar(50);not null" json:"sender_type"`
	SenderID    uuid.UUID       `gorm:"type:uuid;not null" json:"sender_id"`
	Content     string          `gorm:"type:text;not null" json:"content"`
	MessageType MessageType `gorm:"type:varchar(50);not null" json:"message_type"`
	Metadata    datatypes.JSON  `gorm:"type:jsonb;not null;default:'{}'" json:"metadata" swaggertype:"object"`
	CreatedAt   time.Time       `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
}

// TableName возвращает имя таблицы
func (TaskMessage) TableName() string {
	return "task_messages"
}

// BeforeCreate генерирует UUID если не задан
func (m *TaskMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
