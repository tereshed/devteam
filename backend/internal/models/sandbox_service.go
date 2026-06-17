package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SandboxServiceKind — тип эфемерного сервис-сайдкара (пока только postgres).
type SandboxServiceKind string

const (
	SandboxServiceKindPostgres SandboxServiceKind = "postgres"
)

// IsValid — известный ли тип сервиса.
func (k SandboxServiceKind) IsValid() bool {
	return k == SandboxServiceKindPostgres
}

// SandboxServiceSeedKind — источник сида БД сервис-контейнера.
type SandboxServiceSeedKind string

const (
	// SandboxSeedNone — без сида (схему строит сам тест/миграции проекта).
	SandboxSeedNone SandboxServiceSeedKind = "none"
	// SandboxSeedRepoFile — путь к .sql внутри репозитория (резолвится агентом).
	SandboxSeedRepoFile SandboxServiceSeedKind = "repo_file"
	// SandboxSeedInline — SQL прямо в seed_value (кладётся в /docker-entrypoint-initdb.d).
	SandboxSeedInline SandboxServiceSeedKind = "inline"
)

// IsValid — известный ли источник сида.
func (k SandboxServiceSeedKind) IsValid() bool {
	switch k {
	case SandboxSeedNone, SandboxSeedRepoFile, SandboxSeedInline:
		return true
	}
	return false
}

// SandboxServiceConfig — per-project декларация эфемерного сервис-сайдкара
// (см. db/migrations/091_sandbox_service_configs.sql). Пароль БД не хранится —
// генерится случайно на каждый прогон в момент диспатча.
type SandboxServiceConfig struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;index" json:"project_id"`
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`

	IsEnabled bool `gorm:"type:boolean;not null;default:false" json:"is_enabled"`
	// Kind — тип сервиса (postgres).
	Kind SandboxServiceKind `gorm:"type:varchar(32);not null;default:'postgres'" json:"kind"`
	// Alias — сетевой alias/hostname в bridge-сети прогона (агент: alias:port).
	Alias string `gorm:"type:varchar(63);not null;default:'db'" json:"alias"`
	// Image — docker-образ; сверяется с allowlist раннера.
	Image string `gorm:"type:varchar(255);not null;default:'postgres:16-alpine'" json:"image"`
	// DBName / DBUser — имя БД и суперюзера сервис-контейнера (пароль НЕ хранится).
	DBName string `gorm:"type:varchar(255);not null;default:'app'" json:"db_name"`
	DBUser string `gorm:"type:varchar(255);not null;default:'postgres'" json:"db_user"`
	Port   int    `gorm:"type:integer;not null;default:5432" json:"port"`
	// SeedKind / SeedValue — источник сида и его значение (путь в репо или inline SQL).
	SeedKind  SandboxServiceSeedKind `gorm:"type:varchar(16);not null;default:'none'" json:"seed_kind"`
	SeedValue string                 `gorm:"type:text;not null;default:''" json:"seed_value"`
	// ReadyTimeoutSeconds — потолок ожидания готовности сервиса в entrypoint.
	ReadyTimeoutSeconds int `gorm:"type:integer;not null;default:60" json:"ready_timeout_seconds"`

	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (SandboxServiceConfig) TableName() string {
	return "sandbox_service_configs"
}

// BeforeCreate генерирует UUID если не задан.
func (c *SandboxServiceConfig) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
