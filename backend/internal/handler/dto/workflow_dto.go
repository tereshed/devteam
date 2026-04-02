package dto

import "time"

// StartWorkflowRequest запрос на запуск воркфлоу
type StartWorkflowRequest struct {
	Input string `json:"input" binding:"required"`
}

// ExecutionResponse ответ с состоянием выполнения
type ExecutionResponse struct {
	ID            string     `json:"id"`
	WorkflowID    string     `json:"workflow_id"`
	Status        string     `json:"status"`
	CurrentStepID string     `json:"current_step_id"`
	InputData     string     `json:"input_data"`
	OutputData    string     `json:"output_data,omitempty"` // Финальный результат
	StepCount     int        `json:"step_count"`
	CreatedAt     time.Time  `json:"created_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
}

// WorkflowResponse ответ с информацией о воркфлоу
type WorkflowResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// ExecutionListResponse список выполнений с пагинацией
type ExecutionListResponse struct {
	Executions []ExecutionResponse `json:"executions"`
	Total      int64               `json:"total"`
}

// ExecutionStepResponse шаг выполнения
type ExecutionStepResponse struct {
	ID            string    `json:"id"`
	StepID        string    `json:"step_id"`
	AgentName     string    `json:"agent_name,omitempty"`
	InputContext  string    `json:"input_context"`
	OutputContent string    `json:"output_content"`
	DurationMs    int       `json:"duration_ms"`
	TokensUsed    int       `json:"tokens_used"`
	CreatedAt     time.Time `json:"created_at"`
}

