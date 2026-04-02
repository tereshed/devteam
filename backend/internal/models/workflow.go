package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Agent представляет AI-агента
type Agent struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name        string         `gorm:"not null"`
	Role        string         `gorm:"not null"` // 'worker', 'supervisor'
	PromptID    *uuid.UUID     `gorm:"type:uuid"`
	Prompt      *Prompt        `gorm:"foreignKey:PromptID"`
	ModelConfig datatypes.JSON `gorm:"type:jsonb"` // { "temperature": 0.7, "model": "gpt-4" }
	IsActive    bool           `gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Workflow представляет шаблон процесса
type Workflow struct {
	ID            uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name          string         `gorm:"uniqueIndex;not null"`
	Description   string         `gorm:"type:text"`
	Configuration datatypes.JSON `gorm:"type:jsonb;not null"` // Описание графа
	IsActive      bool           `gorm:"default:true"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ScheduledWorkflow представляет запланированный запуск воркфлоу
type ScheduledWorkflow struct {
	ID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name           string    `gorm:"uniqueIndex;not null"`
	WorkflowName   string    `gorm:"not null"`
	CronExpression string    `gorm:"not null"` // cron spec
	InputTemplate  string    `gorm:"type:text"`
	IsActive       bool      `gorm:"default:true"`
	LastRunAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ExecutionStatus статус выполнения
type ExecutionStatus string

const (
	ExecutionPending   ExecutionStatus = "pending"
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionCompleted ExecutionStatus = "completed"
	ExecutionFailed    ExecutionStatus = "failed"
	ExecutionCancelled ExecutionStatus = "cancelled"
)

// Execution представляет запуск процесса
type Execution struct {
	ID            uuid.UUID       `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	WorkflowID    uuid.UUID       `gorm:"type:uuid"`
	Workflow      Workflow        `gorm:"foreignKey:WorkflowID"`
	Status        ExecutionStatus `gorm:"not null;default:'pending'"`
	CurrentStepID string
	InputData     string         `gorm:"type:text"`
	OutputData    string         `gorm:"type:text"` // New field for final result
	Context       datatypes.JSON `gorm:"type:jsonb;default:'{}'"` // Shared memory
	StepCount     int            `gorm:"default:0"`
	MaxSteps      int            `gorm:"default:20"`
	ErrorMessage  string         `gorm:"type:text"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FinishedAt    *time.Time
}

// ExecutionStep представляет один шаг в истории выполнения
type ExecutionStep struct {
	ID            uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ExecutionID   uuid.UUID  `gorm:"type:uuid"`
	StepID        string     `gorm:"not null"`
	AgentID       *uuid.UUID `gorm:"type:uuid"`
	Agent         *Agent     `gorm:"foreignKey:AgentID"`
	PromptSnapshot string     `gorm:"type:text"`
	InputContext  string     `gorm:"type:text"`
	OutputContent string     `gorm:"type:text"`
	TokensUsed    int        `gorm:"default:0"`
	DurationMs    int        `gorm:"default:0"`
	CreatedAt     time.Time
}

// --- Вспомогательные структуры для парсинга Configuration JSON ---

// StepType определяет тип шага воркфлоу
type StepType string

const (
	StepTypeLLM       StepType = "llm"       // Вызов LLM через агента
	StepTypeCondition StepType = "condition" // Условное ветвление
	StepTypeLoop      StepType = "loop"      // Цикл с условием выхода
	StepTypeAPICall   StepType = "api_call"  // Вызов внешнего API
)

// WorkflowConfig структура конфигурации воркфлоу
type WorkflowConfig struct {
	StartStep string                `json:"start_step"`
	MaxSteps  int                   `json:"max_steps"` // Опционально переопределяет дефолт
	Steps     map[string]StepConfig `json:"steps"`
}

// StepConfig конфигурация одного шага
type StepConfig struct {
	Type            StepType          `json:"type"`                       // 'llm', 'condition', 'loop', 'api_call'
	AgentID         string            `json:"agent_id,omitempty"`         // Для type=llm
	Next            *string           `json:"next,omitempty"`             // ID следующего шага (если линейно)
	ConditionPrompt string            `json:"condition_prompt,omitempty"` // Для type=condition
	Routes          map[string]string `json:"routes,omitempty"`           // map[Response]NextStepID

	// --- Loop Config ---
	Loop *LoopConfig `json:"loop,omitempty"` // Для type=loop

	// --- API Call Config ---
	APICall *APICallConfig `json:"api_call,omitempty"` // Для type=api_call
}

// LoopConfig конфигурация цикла
type LoopConfig struct {
	BodyStepID     string `json:"body_step_id"`               // Шаг, который выполняется в цикле
	MaxIterations  int    `json:"max_iterations"`             // Максимум итераций (защита от бесконечного цикла)
	ExitCondition  string `json:"exit_condition"`             // Промпт для LLM: "Should we exit? Answer YES or NO"
	ExitAgentID    string `json:"exit_agent_id,omitempty"`    // Агент для проверки условия (опционально)
	ExitOnResponse string `json:"exit_on_response,omitempty"` // При каком ответе выходить (default: "YES")
}

// APICallConfig конфигурация вызова внешнего API
type APICallConfig struct {
	Method       string            `json:"method"`                  // GET, POST, PUT, DELETE
	URL          string            `json:"url"`                     // URL (можно с шаблонами {{.Input}})
	Headers      map[string]string `json:"headers,omitempty"`       // Заголовки
	BodyTemplate string            `json:"body_template,omitempty"` // Шаблон тела запроса (для POST/PUT)
	TimeoutSec   int               `json:"timeout_sec,omitempty"`   // Таймаут в секундах (default: 30)
	ExtractPath  string            `json:"extract_path,omitempty"`  // JSONPath для извлечения результата
}

// LoopState хранит состояние цикла в контексте выполнения
type LoopState struct {
	StepID         string `json:"step_id"`
	CurrentIteration int    `json:"current_iteration"`
	MaxIterations  int    `json:"max_iterations"`
	ReturnToStepID string `json:"return_to_step_id"` // Куда вернуться после тела цикла
}
