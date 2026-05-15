package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// TaskEventKind — тип события в durable очереди.
type TaskEventKind string

const (
	// TaskEventKindStepReq — пнуть Orchestrator.Step для задачи; Router решит что дальше.
	TaskEventKindStepReq TaskEventKind = "step_req"
	// TaskEventKindAgentJob — запустить конкретного агента с заданным input.
	TaskEventKindAgentJob TaskEventKind = "agent_job"
)

// IsValid проверяет валидность kind.
func (k TaskEventKind) IsValid() bool {
	switch k {
	case TaskEventKindStepReq, TaskEventKindAgentJob:
		return true
	default:
		return false
	}
}

// TaskEvent — единица работы в durable очереди.
//
// Yugabyte НЕ поддерживает LISTEN/NOTIFY — wakeup через Redis Pub/Sub
// (internal/service/redis_notifier.go), забор работы — polling + SELECT FOR UPDATE SKIP LOCKED.
//
// ВАЖНО: ID — BIGSERIAL (int64), не UUID. Это сериализуется быстро и упорядочивает
// очередь по вставке без коллизий, что важно для FIFO-семантики.
//
// Retry-политика: при ошибке воркер делает UPDATE attempts=attempts+1, last_error=...,
// scheduled_at = now() + backoff. После Attempts >= MaxAttempts — событие "умирает":
// остаётся в таблице для аудита, но idx_task_events_pollable его не вернёт
// (см. WHERE attempts < max_attempts).
type TaskEvent struct {
	ID           int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID       uuid.UUID      `gorm:"type:uuid;not null" json:"task_id"`
	Kind         TaskEventKind  `gorm:"type:varchar(32);not null" json:"kind"`
	Payload      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"payload" swaggertype:"object"`
	ScheduledAt  time.Time      `gorm:"type:timestamp with time zone;not null;default:now()" json:"scheduled_at"`
	LockedBy     *string        `gorm:"type:varchar(255)" json:"locked_by,omitempty"`
	LockedAt     *time.Time     `gorm:"type:timestamp with time zone" json:"locked_at,omitempty"`
	Attempts     int            `gorm:"type:integer;not null;default:0" json:"attempts"`
	MaxAttempts  int            `gorm:"type:integer;not null;default:3" json:"max_attempts"`
	LastError    *string        `gorm:"type:text" json:"last_error,omitempty"`
	CompletedAt  *time.Time     `gorm:"type:timestamp with time zone" json:"completed_at,omitempty"`
	CreatedAt    time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

// TableName возвращает имя таблицы.
func (TaskEvent) TableName() string {
	return "task_events"
}

// IsDead — событие исчерпало попытки и больше не будет обработано.
func (e *TaskEvent) IsDead() bool {
	return e.Attempts >= e.MaxAttempts && e.CompletedAt == nil
}

// IsCompleted — событие успешно обработано.
func (e *TaskEvent) IsCompleted() bool {
	return e.CompletedAt != nil
}

// AgentJobPayload — типобезопасное представление payload для kind=agent_job.
// Сериализуется в TaskEvent.Payload (datatypes.JSON).
type AgentJobPayload struct {
	AgentName  string         `json:"agent"`              // имя агента из реестра (agents.name)
	Input      map[string]any `json:"input"`              // произвольный input для агента
	WorktreeID *uuid.UUID     `json:"worktree_id,omitempty"` // только для sandbox-агентов
}
