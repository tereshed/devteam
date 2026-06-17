package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ScoutConfig — per-project конфиг агента-разведчика. Одна строка на проект.
// Разведчик — headless sandbox-прогон на подписке, который диспатчит проектный
// ассистент для сбора контекста (см. db/migrations/088_scout_configs.sql).
type ScoutConfig struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"project_id"`
	// CreatedBy — владелец конфига: от его имени резолвятся подписка и ключи.
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`

	IsEnabled bool `gorm:"type:boolean;not null;default:false" json:"is_enabled"`
	// Prompt — редактируемый промпт разведчика; пусто → встроенный дефолт.
	Prompt string `gorm:"type:text;not null;default:''" json:"prompt"`
	// CodeBackend — CLI внутри sandbox-образа (claude-code/hermes/antigravity).
	CodeBackend CodeBackend `gorm:"type:varchar(32);not null;default:'claude-code'" json:"code_backend"`
	// Агентная настройка (зеркалит models.Agent; всегда sandbox). При диспатче из
	// этих полей собирается временный Agent → тот же auth-резолвер + BuildSandboxBundle.
	// ProviderKind — для аутентификации/hermes (anthropic_oauth = подписка).
	ProviderKind *AgentProviderKind `gorm:"type:varchar(32)" json:"provider_kind,omitempty"`
	// Temperature — параметр LLM (nil — не задан).
	Temperature *float64 `gorm:"type:numeric(4,3)" json:"temperature,omitempty"`
	// CodeBackendSettings — model/mcp_servers/skills/hermes-блок (как у агента).
	CodeBackendSettings datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"code_backend_settings" swaggertype:"object"`
	// SandboxPermissions — allow/deny/ask/defaultMode для Claude Code в sandbox.
	SandboxPermissions datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"sandbox_permissions" swaggertype:"object"`
	// SubscriptionID — выбранная Claude-подписка; nil → дефолтная подписка владельца.
	SubscriptionID *uuid.UUID `gorm:"type:uuid" json:"subscription_id,omitempty"`
	// TimeoutSeconds — жёсткий потолок прогона разведчика в sandbox.
	TimeoutSeconds int `gorm:"type:integer;not null;default:600" json:"timeout_seconds"`

	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (ScoutConfig) TableName() string {
	return "scout_configs"
}

// BeforeCreate генерирует UUID если не задан.
func (c *ScoutConfig) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// ScoutRunStatus — состояние прогона разведчика.
type ScoutRunStatus string

const (
	ScoutRunStatusRunning ScoutRunStatus = "running"
	ScoutRunStatusDone    ScoutRunStatus = "done"
	ScoutRunStatusFailed  ScoutRunStatus = "failed"
)

// ScoutRun — один прогон разведчика: headless sandbox-исполнение на подписке,
// читает репозитории проекта и собирает досье (см. 089_scout_runs.sql).
type ScoutRun struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null" json:"project_id"`
	// CreatedBy — инициатор прогона (владелец подписки для прогона).
	CreatedBy *uuid.UUID `gorm:"type:uuid" json:"created_by,omitempty"`
	// SessionID / ToolCallID — фаза 2: распарканная сессия ассистента и tool_call,
	// который закрывается досье при завершении (wake-up). В фазе 1 nil.
	SessionID  *uuid.UUID `gorm:"type:uuid" json:"session_id,omitempty"`
	ToolCallID *string    `gorm:"type:varchar(128)" json:"tool_call_id,omitempty"`

	Status      ScoutRunStatus `gorm:"type:varchar(16);not null;default:'running'" json:"status"`
	CodeBackend CodeBackend    `gorm:"type:varchar(32);not null;default:'claude-code'" json:"code_backend"`
	// Problem — постановка проблемы (вход разведки).
	Problem string `gorm:"type:text;not null;default:''" json:"problem"`
	// Dossier — собранное досье (выход разведки).
	Dossier           string `gorm:"type:text;not null;default:''" json:"dossier"`
	Error             string `gorm:"type:text;not null;default:''" json:"error,omitempty"`
	SandboxInstanceID string `gorm:"type:varchar(128);not null;default:''" json:"sandbox_instance_id,omitempty"`

	StartedAt  time.Time  `gorm:"type:timestamp with time zone;default:now()" json:"started_at"`
	FinishedAt *time.Time `gorm:"type:timestamp with time zone" json:"finished_at,omitempty"`
	CreatedAt  time.Time  `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (ScoutRun) TableName() string {
	return "scout_runs"
}

// BeforeCreate генерирует UUID если не задан.
func (r *ScoutRun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
