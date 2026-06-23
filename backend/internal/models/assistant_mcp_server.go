package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AssistantMCPServer — удалённый MCP-сервер, привязанный к проекту и доступный
// IN-PROCESS петле ассистента (agentloop поверх OpenRouter/Gemini). Отдельно от
// MCPServerRegistry/MCPServerConfig: те обслуживают sandbox-агентов через .mcp.json,
// здесь же конфиг для прямого MCP-клиента бэкенда.
//
// Remote-only: Transport ∈ {http, sse} (stdio запрещён на уровне валидации и CHECK).
// Headers могут содержать ${secret:NAME} — резолвятся SecretResolver при подключении,
// поэтому секреты в таблице не хранятся. UNIQUE(project_id, name).
type AssistantMCPServer struct {
	ID                  uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID           uuid.UUID      `gorm:"type:uuid;not null;index" json:"project_id"`
	Name                string         `gorm:"type:varchar(255);not null" json:"name"`
	Transport           MCPTransport   `gorm:"type:varchar(16);not null;default:'http'" json:"transport"`
	URL                 string         `gorm:"type:varchar(1024);not null" json:"url"`
	Headers             datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"headers" swaggertype:"object"`
	RequireConfirmation bool           `gorm:"type:boolean;not null;default:true" json:"require_confirmation"`
	IsEnabled           bool           `gorm:"type:boolean;not null;default:true" json:"is_enabled"`
	CreatedAt           time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt           time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (AssistantMCPServer) TableName() string {
	return "assistant_mcp_servers"
}

// BeforeCreate генерирует UUID если не задан.
func (m *AssistantMCPServer) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
