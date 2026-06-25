package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ProjectSecret struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID      uuid.UUID `gorm:"type:uuid;not null" json:"project_id"`
	KeyName        string    `gorm:"type:varchar(128);not null" json:"key_name"`
	EncryptedValue []byte    `gorm:"type:bytea;not null" json:"-"`
	// InjectAsEnv — opt-in: класть значение в env песочницы как обычную переменную
	// окружения (и упоминать её имя в промпте «доступные переменные»). Дефолт false —
	// без него секрет доступен только через ${secret:NAME} в конфигах MCP-серверов.
	InjectAsEnv bool `gorm:"column:inject_as_env;not null;default:false" json:"inject_as_env"`
	// Description — необязательная подсказка агенту (что это за переменная). Подставляется
	// в промпт рядом с именем; значение секрета в промпт не попадает никогда.
	Description string    `gorm:"column:description;type:varchar(255);not null;default:''" json:"description"`
	CreatedAt   time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

func (ProjectSecret) TableName() string {
	return "project_secrets"
}

func (s *ProjectSecret) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
