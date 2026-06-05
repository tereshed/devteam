package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ScheduledTask — регулярная задача проекта, создаваемая по cron-расписанию.
// Leader-gated раннер периодически выбирает «созревшие» строки (is_active &&
// next_run_at <= now()) и порождает обычную models.Task в проекте/команде с
// заданными Name/Description/Priority, после чего пересчитывает NextRunAt.
type ScheduledTask struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID   uuid.UUID `gorm:"type:uuid;not null" json:"project_id"`
	Project     *Project  `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	TeamID      *uuid.UUID `gorm:"type:uuid" json:"team_id"`
	Team        *Team     `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	// CreatedBy — пользователь-владелец расписания. От его имени (с его ролью)
	// раннер создаёт порождённые задачи, поэтому ABAC-проверки в TaskService
	// остаются валидными и в cron-пути.
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`

	// Name — короткое имя расписания; оно же становится Title порождаемых задач.
	Name string `gorm:"type:varchar(500);not null" json:"name"`
	// Description — описание задачи, которое получает порождаемая task'а.
	Description string `gorm:"type:text;not null;default:''" json:"description"`
	// CronExpression — стандартное 5-польное cron-выражение (robfig/cron ParseStandard).
	CronExpression string       `gorm:"type:varchar(255);not null" json:"cron_expression"`
	Priority       TaskPriority `gorm:"type:varchar(50);not null;default:'medium'" json:"priority"`
	IsActive       bool         `gorm:"type:boolean;not null;default:true" json:"is_active"`

	LastRunAt *time.Time `gorm:"type:timestamp with time zone" json:"last_run_at,omitempty"`
	NextRunAt *time.Time `gorm:"type:timestamp with time zone" json:"next_run_at,omitempty"`

	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (ScheduledTask) TableName() string {
	return "scheduled_tasks"
}

// BeforeCreate генерирует UUID если не задан.
func (s *ScheduledTask) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
