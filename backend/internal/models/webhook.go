package models

import (
	"time"

	"github.com/google/uuid"
)

// WebhookTrigger представляет webhook-триггер для запуска workflow
type WebhookTrigger struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name         string    `gorm:"uniqueIndex;not null"`
	WorkflowName string    `gorm:"not null"` // Имя workflow для запуска
	Secret       string    `gorm:"not null"` // Секретный ключ для валидации
	Description  string    `gorm:"type:text"`

	// Настройки обработки входных данных
	InputJSONPath string `gorm:"type:text"` // JSONPath для извлечения input из тела запроса
	InputTemplate string `gorm:"type:text"` // Шаблон для формирования input

	// Настройки безопасности
	AllowedIPs    string `gorm:"type:text"` // Список разрешённых IP (через запятую)
	RequireSecret bool   `gorm:"default:true"`

	// Статистика
	TriggerCount  int64 `gorm:"default:0"`
	LastTriggered *time.Time

	IsActive  bool `gorm:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WebhookLog лог вызовов webhook
type WebhookLog struct {
	ID          uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WebhookID   uuid.UUID  `gorm:"type:uuid;not null"`
	Webhook     WebhookTrigger `gorm:"foreignKey:WebhookID"`
	ExecutionID *uuid.UUID `gorm:"type:uuid"` // Может быть nil если webhook не запустил workflow
	
	// Информация о запросе
	SourceIP    string `gorm:"not null"`
	Method      string `gorm:"not null"`
	Headers     string `gorm:"type:text"` // JSON
	Body        string `gorm:"type:text"`
	
	// Результат
	Success      bool   `gorm:"default:false"`
	ErrorMessage string `gorm:"type:text"`
	ResponseCode int    `gorm:"default:0"`
	
	CreatedAt time.Time
}

