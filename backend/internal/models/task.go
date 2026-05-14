package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TaskStatus статус задачи в pipeline агентов
type TaskStatus string

const (
	TaskStatusPending          TaskStatus = "pending"
	TaskStatusPlanning         TaskStatus = "planning"
	TaskStatusInProgress       TaskStatus = "in_progress"
	TaskStatusReview           TaskStatus = "review"
	TaskStatusChangesRequested TaskStatus = "changes_requested"
	TaskStatusTesting          TaskStatus = "testing"
	TaskStatusCompleted        TaskStatus = "completed"
	TaskStatusFailed           TaskStatus = "failed"
	TaskStatusCancelled        TaskStatus = "cancelled"
	TaskStatusPaused           TaskStatus = "paused"
)

// IsValid проверяет валидность статуса задачи
func (s TaskStatus) IsValid() bool {
	switch s {
	case TaskStatusPending, TaskStatusPlanning, TaskStatusInProgress,
		TaskStatusReview, TaskStatusChangesRequested, TaskStatusTesting,
		TaskStatusCompleted, TaskStatusFailed, TaskStatusCancelled, TaskStatusPaused:
		return true
	default:
		return false
	}
}

// TaskPriority приоритет задачи
type TaskPriority string

const (
	TaskPriorityCritical TaskPriority = "critical"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityLow      TaskPriority = "low"
)

// IsValid проверяет валидность приоритета
func (p TaskPriority) IsValid() bool {
	switch p {
	case TaskPriorityCritical, TaskPriorityHigh, TaskPriorityMedium, TaskPriorityLow:
		return true
	default:
		return false
	}
}

// CreatedByType кто создал задачу (полиморфная ссылка created_by_id)
type CreatedByType string

const (
	CreatedByUser  CreatedByType = "user"
	CreatedByAgent CreatedByType = "agent"
)

// IsValid проверяет валидность типа создателя
func (c CreatedByType) IsValid() bool {
	switch c {
	case CreatedByUser, CreatedByAgent:
		return true
	default:
		return false
	}
}

// TaskState — Sprint 17 / Orchestration v2 — упрощённое жизненное состояние задачи.
// Заменяет TaskStatus (10 значений) на 5 high-level состояний. Введено параллельно;
// TaskStatus останется до Sprint 3, когда удалится legacy-оркестратор.
type TaskState string

const (
	TaskStateActive      TaskState = "active"
	TaskStateDone        TaskState = "done"
	TaskStateFailed      TaskState = "failed"
	TaskStateCancelled   TaskState = "cancelled"
	TaskStateNeedsHuman  TaskState = "needs_human"
)

// IsValid проверяет валидность состояния.
func (s TaskState) IsValid() bool {
	switch s {
	case TaskStateActive, TaskStateDone, TaskStateFailed,
		TaskStateCancelled, TaskStateNeedsHuman:
		return true
	default:
		return false
	}
}

// Task единица работы между агентами в рамках проекта
type Task struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID       uuid.UUID      `gorm:"type:uuid;not null" json:"project_id"`
	Project         *Project       `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	ParentTaskID    *uuid.UUID     `gorm:"type:uuid" json:"parent_task_id"`
	ParentTask      *Task          `gorm:"foreignKey:ParentTaskID" json:"parent_task,omitempty"`
	SubTasks        []Task         `gorm:"foreignKey:ParentTaskID" json:"sub_tasks,omitempty"`
	Title           string         `gorm:"type:varchar(500);not null" json:"title"`
	Description     string         `gorm:"type:text;not null;default:''" json:"description"`
	Status          TaskStatus     `gorm:"type:varchar(50);not null;default:'pending'" json:"status"`
	Priority        TaskPriority   `gorm:"type:varchar(50);not null;default:'medium'" json:"priority"`
	AssignedAgentID *uuid.UUID     `gorm:"type:uuid" json:"assigned_agent_id"`
	AssignedAgent   *Agent         `gorm:"foreignKey:AssignedAgentID" json:"assigned_agent,omitempty"`
	CreatedByType   CreatedByType  `gorm:"type:varchar(50);not null" json:"created_by_type"`
	CreatedByID     uuid.UUID      `gorm:"type:uuid;not null" json:"created_by_id"`
	Context         datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"context"`
	Result          *string        `gorm:"type:text" json:"result"`
	Artifacts       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"artifacts"`
	BranchName      *string        `gorm:"type:varchar(255)" json:"branch_name"`
	ErrorMessage    *string        `gorm:"type:text" json:"error_message"`
	StartedAt       *time.Time     `gorm:"type:timestamp with time zone" json:"started_at"`
	CompletedAt     *time.Time     `gorm:"type:timestamp with time zone" json:"completed_at"`

	// Sprint 17 / Orchestration v2 — новые поля. Status выше остаётся до Sprint 3.
	// State — новый источник правды; маппится из Status через миграцию 037.
	// CancelRequested — кооперативная отмена; Orchestrator.Step и Agent Worker'ы её поллят.
	// CurrentStepNo — счётчик шагов оркестратора (для max_steps_per_task).
	// LockedBy/LockedAt — observability + детект "застрявших" Step-обработок.
	//
	// NB: колонка custom_timeout (INTERVAL) добавлена миграцией 037, но Go-поле
	// введём в Sprint 3 — GORM ↔ PostgreSQL INTERVAL требует кастомного scanner'а
	// (time.Duration сам по себе не маппится). Сейчас колонка просто NULL для всех задач.
	State           TaskState  `gorm:"type:varchar(32);not null;default:'active'" json:"state"`
	CancelRequested bool       `gorm:"type:boolean;not null;default:false" json:"cancel_requested"`
	CurrentStepNo   int        `gorm:"type:integer;not null;default:0" json:"current_step_no"`
	LockedBy        *string    `gorm:"type:varchar(255)" json:"locked_by,omitempty"`
	LockedAt        *time.Time `gorm:"type:timestamp with time zone" json:"locked_at,omitempty"`

	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы
func (Task) TableName() string {
	return "tasks"
}

// BeforeCreate генерирует UUID если не задан
func (t *Task) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// GetSearchQuery формирует поисковый запрос для векторного поиска.
// Возвращает пустую строку, если заголовок и описание пусты.
// Description обрезается до 2000 символов UTF-8 safe способом.
func (t *Task) GetSearchQuery() string {
	title := strings.TrimSpace(t.Title)
	desc := strings.TrimSpace(t.Description)

	if title == "" && desc == "" {
		return ""
	}

	const maxDescLen = 2000
	if len(desc) > maxDescLen {
		// UTF-8 safe truncation
		count := 0
		for i := range desc {
			if count == maxDescLen {
				desc = desc[:i]
				break
			}
			count++
		}
	}

	if title != "" && desc != "" {
		return title + " " + desc
	}
	if title != "" {
		return title
	}
	return desc
}
