package models

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// UUIDSlice маппит PostgreSQL uuid[] через pq.Array (в lib/pq нет отдельного UUIDArray).
type UUIDSlice []uuid.UUID

// Value реализует driver.Valuer
func (a UUIDSlice) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	return pq.Array([]uuid.UUID(a)).Value()
}

// Scan реализует sql.Scanner
func (a *UUIDSlice) Scan(src interface{}) error {
	if a == nil {
		return fmt.Errorf("UUIDSlice: Scan on nil pointer")
	}
	var s []uuid.UUID
	if err := pq.Array(&s).Scan(src); err != nil {
		return err
	}
	*a = UUIDSlice(s)
	return nil
}

// ConversationStatus статус чата пользователя с системой
type ConversationStatus string

const (
	ConversationStatusActive    ConversationStatus = "active"
	ConversationStatusCompleted ConversationStatus = "completed"
	ConversationStatusArchived  ConversationStatus = "archived"
)

// IsValid проверяет валидность статуса чата
func (s ConversationStatus) IsValid() bool {
	switch s {
	case ConversationStatusActive, ConversationStatusCompleted, ConversationStatusArchived:
		return true
	default:
		return false
	}
}

// Conversation чат пользователя с DevTeam в рамках проекта
type Conversation struct {
	ID        uuid.UUID              `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID              `gorm:"type:uuid;not null" json:"project_id"`
	Project   *Project               `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	UserID    uuid.UUID              `gorm:"type:uuid;not null" json:"user_id"`
	User      *User                  `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Title     string                 `gorm:"type:varchar(500);not null;default:''" json:"title"`
	Status    ConversationStatus   `gorm:"type:varchar(50);not null;default:'active'" json:"status"`
	Messages  []ConversationMessage `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
	CreatedAt time.Time              `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt time.Time              `gorm:"type:timestamp with time zone;not null;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы
func (Conversation) TableName() string {
	return "conversations"
}

// BeforeCreate генерирует UUID если не задан
func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// ConversationRole роль автора сообщения в чате
type ConversationRole string

const (
	ConversationRoleUser      ConversationRole = "user"
	ConversationRoleAssistant ConversationRole = "assistant"
	ConversationRoleSystem    ConversationRole = "system"
)

// IsValid проверяет валидность роли сообщения
func (r ConversationRole) IsValid() bool {
	switch r {
	case ConversationRoleUser, ConversationRoleAssistant, ConversationRoleSystem:
		return true
	default:
		return false
	}
}

// ConversationMessage сообщение внутри чата
type ConversationMessage struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ConversationID uuid.UUID        `gorm:"type:uuid;not null" json:"conversation_id"`
	Conversation   *Conversation    `gorm:"foreignKey:ConversationID" json:"conversation,omitempty"`
	Role           ConversationRole `gorm:"type:varchar(50);not null" json:"role"`
	Content        string           `gorm:"type:text;not null" json:"content"`
	LinkedTaskIDs  UUIDSlice        `gorm:"type:uuid[]" json:"linked_task_ids"`
	Metadata       datatypes.JSON   `gorm:"type:jsonb;not null;default:'{}'" json:"metadata" swaggertype:"object"`
	CreatedAt      time.Time        `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
}

// TableName возвращает имя таблицы
func (ConversationMessage) TableName() string {
	return "conversation_messages"
}

// BeforeCreate генерирует UUID если не задан
func (m *ConversationMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
