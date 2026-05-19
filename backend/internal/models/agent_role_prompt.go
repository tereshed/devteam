package models

import (
	"time"

	"github.com/google/uuid"
)

type AgentRolePrompt struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Role        string     `gorm:"type:varchar(50);not null;uniqueIndex" json:"role"`
	Content     string     `gorm:"type:text;not null" json:"content"`
	Description *string    `gorm:"type:text" json:"description,omitempty"`
	UpdatedAt   time.Time  `gorm:"type:timestamp with time zone;not null;default:now()" json:"updated_at"`
	UpdatedBy   *uuid.UUID `gorm:"type:uuid" json:"updated_by,omitempty"`
}

func (AgentRolePrompt) TableName() string {
	return "agent_role_prompts"
}
