package dto

import "time"

// LLMLogResponse ответ с логом LLM
type LLMLogResponse struct {
	ID                  string    `json:"id"`
	Provider            string    `json:"provider"`
	Model               string    `json:"model"`
	InputTokens         int       `json:"input_tokens"`
	OutputTokens        int       `json:"output_tokens"`
	TotalTokens         int       `json:"total_tokens"`
	Cost                float64   `json:"cost"`
	DurationMs          int       `json:"duration_ms"`
	WorkflowExecutionID string    `json:"workflow_execution_id,omitempty"`
	StepID              string    `json:"step_id,omitempty"`
	AgentID             string    `json:"agent_id,omitempty"`
	ErrorMessage        string    `json:"error_message,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

// LLMLogListResponse список логов с пагинацией
type LLMLogListResponse struct {
	Logs  []LLMLogResponse `json:"logs"`
	Total int64            `json:"total"`
}

