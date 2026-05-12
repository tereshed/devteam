package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// AgentRole роль AI-агента в команде
type AgentRole string

const (
	AgentRoleWorker       AgentRole = "worker"
	AgentRoleSupervisor   AgentRole = "supervisor"
	AgentRoleOrchestrator AgentRole = "orchestrator"
	AgentRolePlanner      AgentRole = "planner"
	AgentRoleDeveloper    AgentRole = "developer"
	AgentRoleReviewer     AgentRole = "reviewer"
	AgentRoleTester       AgentRole = "tester"
	AgentRoleDevOps       AgentRole = "devops"
)

// IsValid проверяет валидность роли агента
func (r AgentRole) IsValid() bool {
	switch r {
	case AgentRoleWorker, AgentRoleSupervisor, AgentRoleOrchestrator,
		AgentRolePlanner, AgentRoleDeveloper, AgentRoleReviewer,
		AgentRoleTester, AgentRoleDevOps:
		return true
	default:
		return false
	}
}

// CodeBackend тип бэкенда для написания кода
type CodeBackend string

const (
	CodeBackendClaudeCode         CodeBackend = "claude-code"
	CodeBackendClaudeCodeViaProxy CodeBackend = "claude-code-via-proxy"
	CodeBackendAider              CodeBackend = "aider"
	CodeBackendCustom             CodeBackend = "custom"
)

// IsValid проверяет валидность code backend
func (cb CodeBackend) IsValid() bool {
	switch cb {
	case CodeBackendClaudeCode, CodeBackendClaudeCodeViaProxy, CodeBackendAider, CodeBackendCustom:
		return true
	default:
		return false
	}
}

// Agent представляет AI-агента
type Agent struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Role        AgentRole      `gorm:"type:varchar(50);not null" json:"role"`
	TeamID      *uuid.UUID     `gorm:"type:uuid" json:"team_id"`
	Team        *Team          `gorm:"foreignKey:TeamID" json:"team,omitempty"`
	Model       *string        `gorm:"type:varchar(255)" json:"model"`
	PromptID    *uuid.UUID     `gorm:"type:uuid" json:"prompt_id"`
	Prompt      *Prompt        `gorm:"foreignKey:PromptID" json:"prompt,omitempty"`
	Skills      datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"skills"`
	CodeBackend *CodeBackend   `gorm:"type:varchar(50)" json:"code_backend"`
	Settings    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"settings"`
	ModelConfig datatypes.JSON `gorm:"type:jsonb" json:"model_config"`
	// Sprint 15.3 — связь с llm_providers и per-agent code-backend настройки.
	LLMProviderID       *uuid.UUID     `gorm:"type:uuid" json:"llm_provider_id"`
	LLMProvider         *LLMProvider   `gorm:"foreignKey:LLMProviderID" json:"llm_provider,omitempty"`
	CodeBackendSettings datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"code_backend_settings"`
	SandboxPermissions  datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"sandbox_permissions"`
	IsActive            bool           `gorm:"default:true" json:"is_active"`
	RequiresCodeContext bool           `gorm:"default:false" json:"requires_code_context"`
	CreatedAt           time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`

	ToolBindings []AgentToolBinding `gorm:"foreignKey:AgentID" json:"tool_bindings,omitempty"`
	MCPBindings  []AgentMCPBinding  `gorm:"foreignKey:AgentID" json:"mcp_bindings,omitempty"`
	AgentSkills  []AgentSkill       `gorm:"foreignKey:AgentID" json:"agent_skills,omitempty"`
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
