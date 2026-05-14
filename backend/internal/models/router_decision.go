package models

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// RouterDecisionOutcome — финальный исход задачи, выставляется когда Router возвращает Done.
type RouterDecisionOutcome string

const (
	RouterDecisionOutcomeDone        RouterDecisionOutcome = "done"
	RouterDecisionOutcomeFailed      RouterDecisionOutcome = "failed"
	RouterDecisionOutcomeNeedsHuman  RouterDecisionOutcome = "needs_human"
	RouterDecisionOutcomeCancelled   RouterDecisionOutcome = "cancelled"
)

// AllRouterDecisionOutcomes — единый источник всех допустимых outcome.
// Используется и в IsValid(), и в промпт-билдере (через String()) — чтобы при
// добавлении нового значения не нужно было править несколько мест (DRY).
//
// Порядок стабилен; меняется только при намеренном изменении контракта.
func AllRouterDecisionOutcomes() []RouterDecisionOutcome {
	return []RouterDecisionOutcome{
		RouterDecisionOutcomeDone,
		RouterDecisionOutcomeFailed,
		RouterDecisionOutcomeNeedsHuman,
		RouterDecisionOutcomeCancelled,
	}
}

// IsValid проверяет валидность outcome. Сверяется с AllRouterDecisionOutcomes,
// чтобы поведение оставалось синхронизированным при добавлении новых значений.
func (o RouterDecisionOutcome) IsValid() bool {
	for _, valid := range AllRouterDecisionOutcomes() {
		if o == valid {
			return true
		}
	}
	return false
}

// RouterDecision — лог одного решения Router-агента (по одному на каждый Orchestrator.Step).
//
// Безопасность:
//   - EncryptedRawResponse — blob pkg/crypto (≥ 29 байт), AAD = id записи. Никогда не
//     попадает в логи в открытом виде (см. internal/logging/redact.go).
//   - Reason — короткое объяснение (1-2 предложения), не-sensitive, plain text.
//   - Retention: 30 дней через cron job, см. internal/service/router_decisions_retention.go.
type RouterDecision struct {
	ID                   uuid.UUID             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TaskID               uuid.UUID             `gorm:"type:uuid;not null" json:"task_id"`
	StepNo               int                   `gorm:"type:integer;not null" json:"step_no"`
	ChosenAgents         pq.StringArray        `gorm:"type:text[];not null;default:'{}'" json:"chosen_agents"`
	Outcome              *RouterDecisionOutcome `gorm:"type:varchar(32)" json:"outcome,omitempty"`
	Reason               string                `gorm:"type:text;not null" json:"reason"`
	EncryptedRawResponse []byte                `gorm:"type:bytea" json:"-"` // никогда не сериализуем
	CreatedAt            time.Time             `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

// TableName возвращает имя таблицы.
func (RouterDecision) TableName() string {
	return "router_decisions"
}

// BeforeCreate генерирует UUID если не задан (AAD строится из ID, см. RouterDecisionRepository).
func (d *RouterDecision) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
