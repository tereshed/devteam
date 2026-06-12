package dto

import (
	"time"

	"github.com/devteam/backend/internal/models"
	"gorm.io/datatypes"
)

// UpdateEnhancerConfigRequest — частичное обновление конфига энхансера (PUT).
// Конфиг создаётся лениво при первом апдейте; до этого GET отдаёт дефолт.
type UpdateEnhancerConfigRequest struct {
	IsActive *bool `json:"is_active"`
	// Autonomy — в фазе 1 принимается только 'propose'; 'auto_apply'
	// зарезервирован и отклоняется валидацией сервиса.
	Autonomy *string `json:"autonomy"`
	// CronExpression — 5-польный cron; "" = убрать расписание (только ручной запуск).
	CronExpression     *string `json:"cron_expression"`
	AnalysisWindowDays *int    `json:"analysis_window_days"`
	MaxChangesPerRun   *int    `json:"max_changes_per_run"`
}

// EnhancerConfigResponse — конфиг энхансера проекта.
type EnhancerConfigResponse struct {
	ProjectID          string     `json:"project_id"`
	IsActive           bool       `json:"is_active"`
	Autonomy           string     `json:"autonomy"`
	CronExpression     *string    `json:"cron_expression,omitempty"`
	AnalysisWindowDays int        `json:"analysis_window_days"`
	MaxChangesPerRun   int        `json:"max_changes_per_run"`
	LastRunAt          *time.Time `json:"last_run_at,omitempty"`
	NextRunAt          *time.Time `json:"next_run_at,omitempty"`
}

// EnhancerRunResponse — карточка прогона энхансера.
type EnhancerRunResponse struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"project_id"`
	TriggerKind string     `json:"trigger_kind"`
	Status      string     `json:"status"`
	Report      string     `json:"report"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// EnhancerRunListResponse — список прогонов проекта.
type EnhancerRunListResponse struct {
	Runs  []EnhancerRunResponse `json:"runs"`
	Total int                   `json:"total"`
}

// EnhancerChangeResponse — предложение изменения.
type EnhancerChangeResponse struct {
	ID             string         `json:"id"`
	RunID          string         `json:"run_id"`
	ProjectID      string         `json:"project_id"`
	TargetKind     string         `json:"target_kind"`
	TargetAgentID  *string        `json:"target_agent_id,omitempty"`
	Payload        datatypes.JSON `json:"payload" swaggertype:"object"`
	Reason         string         `json:"reason"`
	ExpectedEffect string         `json:"expected_effect"`
	Status         string         `json:"status"`
	CreatedAt      time.Time      `json:"created_at"`
}

// EnhancerChangeListResponse — предложения одного прогона.
type EnhancerChangeListResponse struct {
	Changes []EnhancerChangeResponse `json:"changes"`
	Total   int                      `json:"total"`
}

// ToEnhancerConfigResponse маппит модель в DTO.
func ToEnhancerConfigResponse(cfg *models.EnhancerConfig) EnhancerConfigResponse {
	if cfg == nil {
		return EnhancerConfigResponse{}
	}
	return EnhancerConfigResponse{
		ProjectID:          cfg.ProjectID.String(),
		IsActive:           cfg.IsActive,
		Autonomy:           string(cfg.Autonomy),
		CronExpression:     cfg.CronExpression,
		AnalysisWindowDays: cfg.AnalysisWindowDays,
		MaxChangesPerRun:   cfg.MaxChangesPerRun,
		LastRunAt:          cfg.LastRunAt,
		NextRunAt:          cfg.NextRunAt,
	}
}

// ToEnhancerRunResponse маппит модель в DTO.
func ToEnhancerRunResponse(run *models.EnhancerRun) EnhancerRunResponse {
	if run == nil {
		return EnhancerRunResponse{}
	}
	return EnhancerRunResponse{
		ID:          run.ID.String(),
		ProjectID:   run.ProjectID.String(),
		TriggerKind: string(run.TriggerKind),
		Status:      string(run.Status),
		Report:      run.Report,
		Error:       run.Error,
		StartedAt:   run.StartedAt,
		FinishedAt:  run.FinishedAt,
	}
}

// ToEnhancerRunListResponse оборачивает список прогонов.
func ToEnhancerRunListResponse(items []models.EnhancerRun) EnhancerRunListResponse {
	out := make([]EnhancerRunResponse, 0, len(items))
	for i := range items {
		out = append(out, ToEnhancerRunResponse(&items[i]))
	}
	return EnhancerRunListResponse{Runs: out, Total: len(out)}
}

// ToEnhancerChangeResponse маппит модель в DTO.
func ToEnhancerChangeResponse(ch *models.EnhancerChange) EnhancerChangeResponse {
	if ch == nil {
		return EnhancerChangeResponse{}
	}
	return EnhancerChangeResponse{
		ID:             ch.ID.String(),
		RunID:          ch.RunID.String(),
		ProjectID:      ch.ProjectID.String(),
		TargetKind:     string(ch.TargetKind),
		TargetAgentID:  uuidPtrToStringPtr(ch.TargetAgentID),
		Payload:        ch.Payload,
		Reason:         ch.Reason,
		ExpectedEffect: ch.ExpectedEffect,
		Status:         string(ch.Status),
		CreatedAt:      ch.CreatedAt,
	}
}

// ToEnhancerChangeListResponse оборачивает список предложений.
func ToEnhancerChangeListResponse(items []models.EnhancerChange) EnhancerChangeListResponse {
	out := make([]EnhancerChangeResponse, 0, len(items))
	for i := range items {
		out = append(out, ToEnhancerChangeResponse(&items[i]))
	}
	return EnhancerChangeListResponse{Changes: out, Total: len(out)}
}
