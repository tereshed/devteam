package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ToolDefinition — запись в реестре доступных инструментов.
// Встроенные (is_builtin=true) загружаются из YAML при старте приложения.
type ToolDefinition struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name             string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Description      string         `gorm:"type:text;not null" json:"description"`
	Category         string         `gorm:"type:varchar(100);not null" json:"category"`
	ParametersSchema datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"parameters_schema" swaggertype:"object"`
	IsBuiltin        bool           `gorm:"not null;default:true" json:"is_builtin"`
	IsActive         bool           `gorm:"not null;default:true" json:"is_active"`
	CreatedAt        time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt        time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

func (ToolDefinition) TableName() string {
	return "tool_definitions"
}

func (td *ToolDefinition) BeforeCreate(tx *gorm.DB) error {
	if td.ID == uuid.Nil {
		td.ID = uuid.New()
	}
	return nil
}

// AgentToolBinding — связь M:N между Agent и ToolDefinition.
// Composite PK: (agent_id, tool_definition_id).
type AgentToolBinding struct {
	AgentID          uuid.UUID      `gorm:"type:uuid;primaryKey" json:"agent_id"`
	ToolDefinitionID uuid.UUID      `gorm:"type:uuid;primaryKey" json:"tool_definition_id"`
	ToolDefinition   *ToolDefinition `gorm:"foreignKey:ToolDefinitionID" json:"tool_definition,omitempty"`
	Config           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"config" swaggertype:"object"`
	CreatedAt        time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

func (AgentToolBinding) TableName() string {
	return "agent_tool_bindings"
}
