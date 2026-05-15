package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AgentSkillSource — источник Claude Code skill (Sprint 15.5).
type AgentSkillSource string

const (
	AgentSkillSourceBuiltin AgentSkillSource = "builtin"
	AgentSkillSourcePlugin  AgentSkillSource = "plugin"
	AgentSkillSourcePath    AgentSkillSource = "path"
)

// IsValid проверяет, что источник skill — поддерживаемый.
func (s AgentSkillSource) IsValid() bool {
	switch s {
	case AgentSkillSourceBuiltin, AgentSkillSourcePlugin, AgentSkillSourcePath:
		return true
	default:
		return false
	}
}

// AgentSkill — назначение Claude Code skill агенту (agent_skills).
// Уникальность — (agent_id, skill_name).
type AgentSkill struct {
	ID          uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AgentID     uuid.UUID        `gorm:"type:uuid;not null;index:idx_agent_skills_agent_id;uniqueIndex:uq_agent_skills_agent_name,priority:1" json:"agent_id"`
	SkillName   string           `gorm:"type:varchar(255);not null;uniqueIndex:uq_agent_skills_agent_name,priority:2" json:"skill_name"`
	SkillSource AgentSkillSource `gorm:"type:varchar(16);not null" json:"skill_source"`
	ConfigJSON  datatypes.JSON   `gorm:"type:jsonb;not null;default:'{}'" json:"config_json" swaggertype:"object"`
	IsActive    bool             `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time        `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time        `gorm:"type:timestamp with time zone;not null;default:now();autoUpdateTime" json:"updated_at"`
}

func (AgentSkill) TableName() string {
	return "agent_skills"
}

func (s *AgentSkill) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
