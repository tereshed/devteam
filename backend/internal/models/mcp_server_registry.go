package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// MCPTransport — транспорт MCP-сервера (Sprint 15.4).
type MCPTransport string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportHTTP  MCPTransport = "http"
	MCPTransportSSE   MCPTransport = "sse"
)

// IsValid проверяет поддерживаемость транспорта.
func (t MCPTransport) IsValid() bool {
	switch t {
	case MCPTransportStdio, MCPTransportHTTP, MCPTransportSSE:
		return true
	default:
		return false
	}
}

// MCPScope — область видимости MCP-сервера в реестре.
type MCPScope string

const (
	MCPScopeGlobal  MCPScope = "global"
	MCPScopeProject MCPScope = "project"
	MCPScopeAgent   MCPScope = "agent"
)

// IsValid проверяет поддерживаемость scope.
func (s MCPScope) IsValid() bool {
	switch s {
	case MCPScopeGlobal, MCPScopeProject, MCPScopeAgent:
		return true
	default:
		return false
	}
}

// MCPServerRegistry — глобальный каталог MCP-серверов (mcp_servers_registry).
// В отличие от MCPServerConfig (привязан к project_id), здесь хранятся шаблоны/глобальные серверы,
// которые далее ссылаются из per-agent settings.
type MCPServerRegistry struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"type:varchar(255);not null;uniqueIndex:uq_mcp_registry_name" json:"name"`
	Description string         `gorm:"type:text;not null;default:''" json:"description"`
	Transport   MCPTransport   `gorm:"type:varchar(16);not null" json:"transport"`
	Command     string         `gorm:"type:varchar(1024);not null;default:''" json:"command"`
	Args        datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"args" swaggertype:"object"`
	URL         string         `gorm:"type:varchar(1024);not null;default:''" json:"url"`
	EnvTemplate datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"env_template" swaggertype:"object"`
	Scope       MCPScope       `gorm:"type:varchar(16);not null;default:'global'" json:"scope"`
	IsActive    bool           `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time      `gorm:"type:timestamp with time zone;not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp with time zone;not null;default:now();autoUpdateTime" json:"updated_at"`
}

func (MCPServerRegistry) TableName() string {
	return "mcp_servers_registry"
}

func (m *MCPServerRegistry) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
