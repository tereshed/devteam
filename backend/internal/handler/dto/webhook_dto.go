package dto

import "time"

// CreateWebhookRequest запрос на создание webhook
type CreateWebhookRequest struct {
	Name          string `json:"name" binding:"required"`
	WorkflowName  string `json:"workflow_name" binding:"required"`
	Description   string `json:"description"`
	InputJSONPath string `json:"input_json_path"`
	InputTemplate string `json:"input_template"`
	AllowedIPs    string `json:"allowed_ips"`
	RequireSecret bool   `json:"require_secret"`
}

// UpdateWebhookRequest запрос на обновление webhook
type UpdateWebhookRequest struct {
	WorkflowName  *string `json:"workflow_name"`
	Description   *string `json:"description"`
	InputJSONPath *string `json:"input_json_path"`
	InputTemplate *string `json:"input_template"`
	AllowedIPs    *string `json:"allowed_ips"`
	RequireSecret *bool   `json:"require_secret"`
	IsActive      *bool   `json:"is_active"`
	RegenerateSecret bool `json:"regenerate_secret"`
}

// WebhookResponse ответ с данными webhook
type WebhookResponse struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	WorkflowName  string     `json:"workflow_name"`
	Description   string     `json:"description"`
	WebhookURL    string     `json:"webhook_url"`
	Secret        string     `json:"secret,omitempty"` // Показывается только при создании
	InputJSONPath string     `json:"input_json_path"`
	InputTemplate string     `json:"input_template"`
	AllowedIPs    string     `json:"allowed_ips"`
	RequireSecret bool       `json:"require_secret"`
	TriggerCount  int64      `json:"trigger_count"`
	LastTriggered *time.Time `json:"last_triggered"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
}

// WebhookLogResponse ответ с данными лога
type WebhookLogResponse struct {
	ID           string     `json:"id"`
	WebhookID    string     `json:"webhook_id"`
	ExecutionID  *string    `json:"execution_id"`
	SourceIP     string     `json:"source_ip"`
	Method       string     `json:"method"`
	Success      bool       `json:"success"`
	ErrorMessage string     `json:"error_message"`
	ResponseCode int        `json:"response_code"`
	CreatedAt    time.Time  `json:"created_at"`
}

// WebhookTriggerResponse ответ на срабатывание webhook
type WebhookTriggerResponse struct {
	Success     bool   `json:"success"`
	ExecutionID string `json:"execution_id,omitempty"`
	Message     string `json:"message"`
}

