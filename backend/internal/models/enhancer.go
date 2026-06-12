package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// EnhancerAutonomy — режим применения предложений энхансера.
type EnhancerAutonomy string

const (
	// EnhancerAutonomyPropose — предложения копятся в enhancer_changes и ждут
	// решения человека (дефолт; единственный режим фазы 1).
	EnhancerAutonomyPropose EnhancerAutonomy = "propose"
	// EnhancerAutonomyAutoApply — зарезервировано под фазу 3 (автоприменение
	// с замером эффекта). API фазы 1 этот режим не принимает.
	EnhancerAutonomyAutoApply EnhancerAutonomy = "auto_apply"
)

// IsValid проверяет валидность режима автономии.
func (a EnhancerAutonomy) IsValid() bool {
	return a == EnhancerAutonomyPropose || a == EnhancerAutonomyAutoApply
}

// EnhancerRunTrigger — что запустило прогон.
type EnhancerRunTrigger string

const (
	EnhancerRunTriggerManual EnhancerRunTrigger = "manual"
	EnhancerRunTriggerCron   EnhancerRunTrigger = "cron"
)

// EnhancerRunStatus — состояние прогона.
type EnhancerRunStatus string

const (
	EnhancerRunStatusRunning EnhancerRunStatus = "running"
	EnhancerRunStatusDone    EnhancerRunStatus = "done"
	EnhancerRunStatusFailed  EnhancerRunStatus = "failed"
)

// EnhancerChangeKind — тип цели предложения.
type EnhancerChangeKind string

const (
	// EnhancerChangeKindAgentOverride — проектный оверрайд промпта/настроек
	// агента (применение — фаза 2, project_agent_overrides). Глобальные промпты
	// агентов энхансер не трогает никогда: blast radius ограничен проектом.
	EnhancerChangeKindAgentOverride EnhancerChangeKind = "agent_override"
	// EnhancerChangeKindProjectDescription — правка описания проекта.
	EnhancerChangeKindProjectDescription EnhancerChangeKind = "project_description"
	// EnhancerChangeKindProjectSettings — правка projects.settings.
	EnhancerChangeKindProjectSettings EnhancerChangeKind = "project_settings"
)

// IsValid проверяет валидность типа цели.
func (k EnhancerChangeKind) IsValid() bool {
	switch k {
	case EnhancerChangeKindAgentOverride, EnhancerChangeKindProjectDescription, EnhancerChangeKindProjectSettings:
		return true
	default:
		return false
	}
}

// EnhancerChangeStatus — жизненный цикл предложения.
type EnhancerChangeStatus string

const (
	EnhancerChangeStatusProposed   EnhancerChangeStatus = "proposed"
	EnhancerChangeStatusApproved   EnhancerChangeStatus = "approved"
	EnhancerChangeStatusApplied    EnhancerChangeStatus = "applied"
	EnhancerChangeStatusRejected   EnhancerChangeStatus = "rejected"
	EnhancerChangeStatusRolledBack EnhancerChangeStatus = "rolled_back"
)

// EnhancerConfig — per-project конфиг энхансера. Одна строка на проект.
// Раннер (leader-gated) выбирает созревшие строки (is_active && next_run_at <=
// now) и запускает прогон; ручной запуск идёт мимо next_run_at.
type EnhancerConfig struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"project_id"`
	// CreatedBy — владелец конфига: от его имени резолвятся enhancer-агент и
	// LLM-ключи (user_llm_credentials), а ABAC-проверки доступа к данным
	// проекта в tool-каталоге остаются валидными в cron-пути.
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`

	IsActive bool             `gorm:"type:boolean;not null;default:false" json:"is_active"`
	Autonomy EnhancerAutonomy `gorm:"type:varchar(16);not null;default:'propose'" json:"autonomy"`
	// CronExpression — расписание автозапуска (5-польный cron, robfig/cron
	// ParseStandard); nil/пусто — только ручной запуск.
	CronExpression *string `gorm:"type:varchar(255)" json:"cron_expression,omitempty"`
	// AnalysisWindowDays — окно истории задач для анализа.
	AnalysisWindowDays int `gorm:"type:integer;not null;default:7" json:"analysis_window_days"`
	// MaxChangesPerRun — гардрейл: лимит предложений за прогон (enforced в Go).
	MaxChangesPerRun int `gorm:"type:integer;not null;default:5" json:"max_changes_per_run"`

	LastRunAt *time.Time `gorm:"type:timestamp with time zone" json:"last_run_at,omitempty"`
	NextRunAt *time.Time `gorm:"type:timestamp with time zone" json:"next_run_at,omitempty"`

	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (EnhancerConfig) TableName() string {
	return "enhancer_configs"
}

// BeforeCreate генерирует UUID если не задан.
func (c *EnhancerConfig) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// EnhancerRun — один прогон энхансера: LLM-агент-петля с read-инструментами по
// истории проекта и write-инструментом enhancer_propose_change.
type EnhancerRun struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID  `gorm:"type:uuid;not null" json:"project_id"`
	ConfigID  *uuid.UUID `gorm:"type:uuid" json:"config_id,omitempty"`

	TriggerKind EnhancerRunTrigger `gorm:"type:varchar(16);not null;default:'manual'" json:"trigger_kind"`
	Status      EnhancerRunStatus  `gorm:"type:varchar(16);not null;default:'running'" json:"status"`
	// Report — итоговый отчёт агента (markdown).
	Report string `gorm:"type:text;not null;default:''" json:"report"`
	Error  string `gorm:"type:text;not null;default:''" json:"error"`

	StartedAt  time.Time  `gorm:"type:timestamp with time zone;default:now()" json:"started_at"`
	FinishedAt *time.Time `gorm:"type:timestamp with time zone" json:"finished_at,omitempty"`
}

// TableName возвращает имя таблицы.
func (EnhancerRun) TableName() string {
	return "enhancer_runs"
}

// BeforeCreate генерирует UUID если не задан.
func (r *EnhancerRun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

// ProjectAgentOverride — проектный оверрайд промпта агента: материализованная
// свёртка всех применённых enhancer_changes вида agent_override для пары
// (проект, агент). Apply/rollback пересобирают prompt_addendum из
// applied-предложений (конкатенация по applied_at), ContextBuilder дописывает
// активный addendum к системному промпту агента при исполнении задач проекта.
type ProjectAgentOverride struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_project_agent_override" json:"project_id"`
	AgentID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_project_agent_override" json:"agent_id"`

	PromptAddendum string     `gorm:"type:text;not null;default:''" json:"prompt_addendum"`
	IsActive       bool       `gorm:"type:boolean;not null;default:true" json:"is_active"`
	UpdatedBy      *uuid.UUID `gorm:"type:uuid" json:"updated_by,omitempty"`

	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы.
func (ProjectAgentOverride) TableName() string {
	return "project_agent_overrides"
}

// BeforeCreate генерирует UUID если не задан.
func (o *ProjectAgentOverride) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// EnhancerChange — предложение изменения, рождённое прогоном. payload —
// самодостаточный дифф {old, new, ...}, формат зависит от target_kind.
type EnhancerChange struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RunID     uuid.UUID `gorm:"type:uuid;not null" json:"run_id"`
	ProjectID uuid.UUID `gorm:"type:uuid;not null" json:"project_id"`

	TargetKind    EnhancerChangeKind `gorm:"type:varchar(32);not null" json:"target_kind"`
	TargetAgentID *uuid.UUID         `gorm:"type:uuid" json:"target_agent_id,omitempty"`
	Payload       datatypes.JSON     `gorm:"type:jsonb;not null;default:'{}'" json:"payload"`
	// Reason — на каких наблюдениях основано предложение.
	Reason string `gorm:"type:text;not null;default:''" json:"reason"`
	// ExpectedEffect — ожидаемый измеримый эффект (для замера в фазе 3).
	ExpectedEffect string `gorm:"type:text;not null;default:''" json:"expected_effect"`

	Status    EnhancerChangeStatus `gorm:"type:varchar(16);not null;default:'proposed'" json:"status"`
	DecidedBy *uuid.UUID           `gorm:"type:uuid" json:"decided_by,omitempty"`
	DecidedAt *time.Time           `gorm:"type:timestamp with time zone" json:"decided_at,omitempty"`
	AppliedAt *time.Time           `gorm:"type:timestamp with time zone" json:"applied_at,omitempty"`

	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

// TableName возвращает имя таблицы.
func (EnhancerChange) TableName() string {
	return "enhancer_changes"
}

// BeforeCreate генерирует UUID если не задан.
func (c *EnhancerChange) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
