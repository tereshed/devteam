package dto

import "time"

// CreateWebhookRequest запрос на создание webhook
type CreateWebhookRequest struct {
	Name          string  `json:"name" binding:"required"`
	ProjectID     *string `json:"project_id"`
	TeamID        *string `json:"team_id"`
	Description   string  `json:"description"`
	Instructions  string  `json:"instructions"`
	AllowedIPs              string  `json:"allowed_ips"`
	RequireSecret           bool    `json:"require_secret"`
	TaskTitleTemplate       string  `json:"task_title_template"`
	TaskDescriptionTemplate string  `json:"task_description_template"`
	TaskPriorityTemplate    string  `json:"task_priority_template"`
}

// UpdateWebhookRequest запрос на обновление webhook
type UpdateWebhookRequest struct {
	ProjectID        *string `json:"project_id"`
	TeamID           *string `json:"team_id"`
	Description      *string `json:"description"`
	Instructions     *string `json:"instructions"`
	AllowedIPs       *string `json:"allowed_ips"`
	RequireSecret    *bool   `json:"require_secret"`
	IsActive                *bool   `json:"is_active"`
	RegenerateSecret        bool    `json:"regenerate_secret"`
	TaskTitleTemplate       *string `json:"task_title_template"`
	TaskDescriptionTemplate *string `json:"task_description_template"`
	TaskPriorityTemplate    *string `json:"task_priority_template"`
}

// WebhookResponse ответ с данными webhook
type WebhookResponse struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	ProjectID     *string    `json:"project_id"`
	TeamID        *string    `json:"team_id"`
	Description   string     `json:"description"`
	Instructions  string     `json:"instructions"`
	WebhookURL    string     `json:"webhook_url"`
	Secret        string     `json:"secret,omitempty"` // Показывается только при создании
	AllowedIPs    string     `json:"allowed_ips"`
	RequireSecret bool       `json:"require_secret"`
	TriggerCount  int64      `json:"trigger_count"`
	LastTriggered *time.Time `json:"last_triggered"`
	IsActive                bool       `json:"is_active"`
	CreatedAt               time.Time  `json:"created_at"`
	TaskTitleTemplate       string     `json:"task_title_template"`
	TaskDescriptionTemplate string     `json:"task_description_template"`
	TaskPriorityTemplate    string     `json:"task_priority_template"`
}

// WebhookLogResponse ответ с данными лога
type WebhookLogResponse struct {
	ID             string     `json:"id"`
	WebhookID      string     `json:"webhook_id"`
	ExecutionID    *string    `json:"execution_id"`
	ConversationID *string    `json:"conversation_id"`
	SourceIP       string     `json:"source_ip"`
	Method         string     `json:"method"`
	Success        bool       `json:"success"`
	ErrorMessage   string     `json:"error_message"`
	ResponseCode   int        `json:"response_code"`
	CreatedAt      time.Time  `json:"created_at"`
}

// WebhookTriggerResponse ответ на срабатывание webhook
type WebhookTriggerResponse struct {
	Success        bool   `json:"success"`
	ExecutionID    string `json:"execution_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Message        string `json:"message"`
}

