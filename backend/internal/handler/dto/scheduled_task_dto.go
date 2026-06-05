package dto

import (
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// CreateScheduledTaskRequest — создание регулярной задачи (тело POST).
type CreateScheduledTaskRequest struct {
	Name           string     `json:"name" binding:"required,min=1,max=500"`
	Description    string     `json:"description"`
	CronExpression string     `json:"cron_expression" binding:"required,min=1,max=255"`
	Priority       string     `json:"priority"`
	TeamID         *uuid.UUID `json:"team_id"`
	IsActive       *bool      `json:"is_active"`
}

// UpdateScheduledTaskRequest — частичное обновление регулярной задачи.
type UpdateScheduledTaskRequest struct {
	Name           *string    `json:"name"`
	Description    *string    `json:"description"`
	CronExpression *string    `json:"cron_expression"`
	Priority       *string    `json:"priority"`
	TeamID         *uuid.UUID `json:"team_id"`
	ClearTeam      bool       `json:"clear_team"`
	IsActive       *bool      `json:"is_active"`
}

// ScheduledTaskResponse — карточка регулярной задачи.
type ScheduledTaskResponse struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	TeamID         *string    `json:"team_id,omitempty"`
	CreatedBy      string     `json:"created_by"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	CronExpression string     `json:"cron_expression"`
	Priority       string     `json:"priority"`
	IsActive       bool       `json:"is_active"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	NextRunAt      *time.Time `json:"next_run_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ScheduledTaskListResponse — список регулярных задач проекта.
type ScheduledTaskListResponse struct {
	ScheduledTasks []ScheduledTaskResponse `json:"scheduled_tasks"`
	Total          int                     `json:"total"`
}

// ToScheduledTaskResponse маппит модель в DTO.
func ToScheduledTaskResponse(st *models.ScheduledTask) ScheduledTaskResponse {
	if st == nil {
		return ScheduledTaskResponse{}
	}
	return ScheduledTaskResponse{
		ID:             st.ID.String(),
		ProjectID:      st.ProjectID.String(),
		TeamID:         uuidPtrToStringPtr(st.TeamID),
		CreatedBy:      st.CreatedBy.String(),
		Name:           st.Name,
		Description:    st.Description,
		CronExpression: st.CronExpression,
		Priority:       string(st.Priority),
		IsActive:       st.IsActive,
		LastRunAt:      st.LastRunAt,
		NextRunAt:      st.NextRunAt,
		CreatedAt:      st.CreatedAt,
		UpdatedAt:      st.UpdatedAt,
	}
}

// ToScheduledTaskListResponse оборачивает список расписаний.
func ToScheduledTaskListResponse(items []models.ScheduledTask) ScheduledTaskListResponse {
	out := make([]ScheduledTaskResponse, 0, len(items))
	for i := range items {
		out = append(out, ToScheduledTaskResponse(&items[i]))
	}
	return ScheduledTaskListResponse{
		ScheduledTasks: out,
		Total:          len(out),
	}
}
