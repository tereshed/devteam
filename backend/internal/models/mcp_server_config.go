package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MCPAuthType тип аутентификации MCP-сервера
type MCPAuthType string

const (
	MCPAuthNone   MCPAuthType = "none"
	MCPAuthAPIKey MCPAuthType = "api_key"
	MCPAuthOAuth  MCPAuthType = "oauth"
	MCPAuthBearer MCPAuthType = "bearer"
)

// IsValid проверяет валидность типа аутентификации
func (a MCPAuthType) IsValid() bool {
	switch a {
	case MCPAuthNone, MCPAuthAPIKey, MCPAuthOAuth, MCPAuthBearer:
		return true
	default:
		return false
	}
}

// MCPServerConfig — конфигурация MCP-сервера, привязанная к проекту.
// Credentials хранятся зашифрованными (AES-256-GCM).
// UNIQUE constraint: (project_id, name).
type MCPServerConfig struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID            uuid.UUID      `gorm:"type:uuid;not null" json:"project_id"`
	Project              *Project       `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Name                 string         `gorm:"type:varchar(255);not null" json:"name"`
	URL                  string         `gorm:"type:varchar(1024);not null" json:"url"`
	AuthType             MCPAuthType    `gorm:"type:varchar(50);not null;default:'none'" json:"auth_type"`
	EncryptedCredentials []byte         `gorm:"type:bytea" json:"-"`
	Settings             datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	IsActive             bool           `gorm:"not null;default:true" json:"is_active"`
	CreatedAt            time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt            time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

func (MCPServerConfig) TableName() string {
	return "mcp_server_configs"
}

func (m *MCPServerConfig) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// AgentMCPBinding — связь M:N между Agent и MCPServerConfig.
// Composite PK: (agent_id, mcp_server_config_id).
type AgentMCPBinding struct {
	AgentID            uuid.UUID      `gorm:"type:uuid;primaryKey" json:"agent_id"`
	MCPServerConfigID  uuid.UUID      `gorm:"type:uuid;primaryKey" json:"mcp_server_config_id"`
	MCPServerConfig    *MCPServerConfig `gorm:"foreignKey:MCPServerConfigID" json:"mcp_server_config,omitempty"`
	Settings           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	CreatedAt          time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

func (AgentMCPBinding) TableName() string {
	return "agent_mcp_bindings"
}
