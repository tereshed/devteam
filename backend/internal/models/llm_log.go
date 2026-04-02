package models

import (
	"time"

	"github.com/google/uuid"
)

// LLMLog представляет запись о запросе к LLM
type LLMLog struct {
	ID                  uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Provider            string    `gorm:"not null"`
	Model               string    `gorm:"not null"`
	InputTokens         int       `gorm:"default:0"`
	OutputTokens        int       `gorm:"default:0"`
	CachedTokens        int       `gorm:"default:0"`
	TotalTokens         int       `gorm:"default:0"`
	PromptSnapshot      string    `gorm:"type:jsonb;default:'{}'"`
	ResponseSnapshot    string    `gorm:"type:jsonb;default:'{}'"`
	DurationMs          int       `gorm:"default:0"`
	Cost                float64   `gorm:"type:numeric(20,10);default:0"`
	WorkflowExecutionID *uuid.UUID `gorm:"type:uuid"`
	StepID              string
	AgentID             *uuid.UUID `gorm:"type:uuid"`
	ErrorMessage        string     `gorm:"type:text"`
	CreatedAt           time.Time
}

