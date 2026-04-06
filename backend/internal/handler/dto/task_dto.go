package dto

import (
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// CreateTaskRequest создание задачи (тело POST).
type CreateTaskRequest struct {
	Title           string         `json:"title" binding:"required,min=1,max=500"`
	Description     string         `json:"description"`
	Priority        string         `json:"priority"`
	ParentTaskID    *uuid.UUID     `json:"parent_task_id"`
	AssignedAgentID *uuid.UUID     `json:"assigned_agent_id"`
	Context         datatypes.JSON `json:"context" swaggertype:"string"`
}

// ListTasksRequest фильтры и пагинация списка задач проекта.
type ListTasksRequest struct {
	Status          *string    `form:"status"`
	Statuses        []string   `form:"statuses"`
	Priority        *string    `form:"priority"`
	AssignedAgentID *uuid.UUID `form:"assigned_agent_id"`
	CreatedByType   *string    `form:"created_by_type"`
	CreatedByID     *uuid.UUID `form:"created_by_id"`
	ParentTaskID    *uuid.UUID `form:"parent_task_id"`
	RootOnly        bool       `form:"root_only"`
	BranchName      *string    `form:"branch_name"`
	Search          *string    `form:"search"`
	Limit           int        `form:"limit"`
	Offset          int        `form:"offset"`
	OrderBy         string     `form:"order_by"`
	OrderDir        string     `form:"order_dir"`
}

// UpdateTaskRequest частичное обновление задачи.
type UpdateTaskRequest struct {
	Title              *string    `json:"title"`
	Description        *string    `json:"description"`
	Priority           *string    `json:"priority"`
	Status             *string    `json:"status"`
	AssignedAgentID    *uuid.UUID `json:"assigned_agent_id"`
	ClearAssignedAgent bool       `json:"clear_assigned_agent"`
	BranchName         *string    `json:"branch_name"`
}

// CreateTaskMessageRequest сообщение в контексте задачи.
type CreateTaskMessageRequest struct {
	Content     string         `json:"content" binding:"required,min=1"`
	MessageType string         `json:"message_type" binding:"required"`
	Metadata    datatypes.JSON `json:"metadata" swaggertype:"string"`
}

// ListTaskMessagesRequest пагинация и фильтры сообщений задачи.
type ListTaskMessagesRequest struct {
	MessageType *string `form:"message_type"`
	SenderType  *string `form:"sender_type"`
	Limit       int     `form:"limit"`
	Offset      int     `form:"offset"`
}

// AgentSummary краткая информация об агенте (вложена в TaskResponse / TaskListItem).
type AgentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// TaskSummary краткая информация о подзадаче.
type TaskSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

// TaskResponse полная карточка задачи (GET /tasks/:id).
type TaskResponse struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	ParentTaskID  *string        `json:"parent_task_id,omitempty"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Status        string         `json:"status"`
	Priority      string         `json:"priority"`
	AssignedAgent *AgentSummary  `json:"assigned_agent,omitempty"`
	CreatedByType string         `json:"created_by_type"`
	CreatedByID   string         `json:"created_by_id"`
	Context       datatypes.JSON `json:"context" swaggertype:"string"`
	Result        *string        `json:"result,omitempty"`
	Artifacts     datatypes.JSON `json:"artifacts" swaggertype:"string"`
	BranchName    *string        `json:"branch_name,omitempty"`
	ErrorMessage  *string        `json:"error_message,omitempty"`
	SubTasks      []TaskSummary  `json:"sub_tasks,omitempty"`
	MessageCount  *int64         `json:"message_count,omitempty"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

// TaskListItem элемент списка задач (без тяжёлых JSONB и подзадач).
type TaskListItem struct {
	ID            string        `json:"id"`
	ProjectID     string        `json:"project_id"`
	ParentTaskID  *string       `json:"parent_task_id,omitempty"`
	Title         string        `json:"title"`
	Status        string        `json:"status"`
	Priority      string        `json:"priority"`
	AssignedAgent *AgentSummary `json:"assigned_agent,omitempty"`
	CreatedByType string        `json:"created_by_type"`
	CreatedByID   string        `json:"created_by_id"`
	BranchName    *string       `json:"branch_name,omitempty"`
	StartedAt     *time.Time    `json:"started_at,omitempty"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// TaskListResponse пагинированный список задач.
type TaskListResponse struct {
	Tasks  []TaskListItem `json:"tasks"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// TaskMessageResponse одно сообщение задачи.
type TaskMessageResponse struct {
	ID          string         `json:"id"`
	TaskID      string         `json:"task_id"`
	SenderType  string         `json:"sender_type"`
	SenderID    string         `json:"sender_id"`
	Content     string         `json:"content"`
	MessageType string         `json:"message_type"`
	Metadata    datatypes.JSON `json:"metadata" swaggertype:"string"`
	CreatedAt   time.Time      `json:"created_at"`
}

// TaskMessageListResponse пагинированный список сообщений.
type TaskMessageListResponse struct {
	Messages []TaskMessageResponse `json:"messages"`
	Total    int64                 `json:"total"`
	Limit    int                   `json:"limit"`
	Offset   int                   `json:"offset"`
}

// uuidPtrToStringPtr маппинг *uuid.UUID → *string для JSON omitempty.
func uuidPtrToStringPtr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// ToAgentSummary маппинг models.Agent → *AgentSummary (nil-safe).
func ToAgentSummary(a *models.Agent) *AgentSummary {
	if a == nil {
		return nil
	}
	return &AgentSummary{
		ID:   a.ID.String(),
		Name: a.Name,
		Role: string(a.Role),
	}
}

// ToTaskSummary маппинг models.Task → TaskSummary.
func ToTaskSummary(t *models.Task) TaskSummary {
	if t == nil {
		return TaskSummary{}
	}
	return TaskSummary{
		ID:       t.ID.String(),
		Title:    t.Title,
		Status:   string(t.Status),
		Priority: string(t.Priority),
	}
}

// ToTaskResponse маппинг models.Task → TaskResponse.
// MessageCount не заполняется — задаётся в handler при необходимости.
func ToTaskResponse(t *models.Task) TaskResponse {
	if t == nil {
		return TaskResponse{}
	}
	sub := make([]TaskSummary, 0, len(t.SubTasks))
	for i := range t.SubTasks {
		sub = append(sub, ToTaskSummary(&t.SubTasks[i]))
	}
	return TaskResponse{
		ID:            t.ID.String(),
		ProjectID:     t.ProjectID.String(),
		ParentTaskID:  uuidPtrToStringPtr(t.ParentTaskID),
		Title:         t.Title,
		Description:   t.Description,
		Status:        string(t.Status),
		Priority:      string(t.Priority),
		AssignedAgent: ToAgentSummary(t.AssignedAgent),
		CreatedByType: string(t.CreatedByType),
		CreatedByID:   t.CreatedByID.String(),
		Context:       t.Context,
		Result:        t.Result,
		Artifacts:     t.Artifacts,
		BranchName:    t.BranchName,
		ErrorMessage:  t.ErrorMessage,
		SubTasks:      sub,
		StartedAt:     t.StartedAt,
		CompletedAt:   t.CompletedAt,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

// ToTaskListItem маппинг models.Task → TaskListItem (облегчённый вид для списка).
func ToTaskListItem(t *models.Task) TaskListItem {
	if t == nil {
		return TaskListItem{}
	}
	return TaskListItem{
		ID:            t.ID.String(),
		ProjectID:     t.ProjectID.String(),
		ParentTaskID:  uuidPtrToStringPtr(t.ParentTaskID),
		Title:         t.Title,
		Status:        string(t.Status),
		Priority:      string(t.Priority),
		AssignedAgent: ToAgentSummary(t.AssignedAgent),
		CreatedByType: string(t.CreatedByType),
		CreatedByID:   t.CreatedByID.String(),
		BranchName:    t.BranchName,
		StartedAt:     t.StartedAt,
		CompletedAt:   t.CompletedAt,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	}
}

// ToTaskListResponse обёртка списка задач с пагинацией.
func ToTaskListResponse(tasks []models.Task, total int64, limit, offset int) TaskListResponse {
	out := make([]TaskListItem, 0, len(tasks))
	for i := range tasks {
		out = append(out, ToTaskListItem(&tasks[i]))
	}
	return TaskListResponse{
		Tasks:  out,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}
}

// ToTaskMessageResponse маппинг models.TaskMessage → TaskMessageResponse.
func ToTaskMessageResponse(m *models.TaskMessage) TaskMessageResponse {
	if m == nil {
		return TaskMessageResponse{}
	}
	return TaskMessageResponse{
		ID:          m.ID.String(),
		TaskID:      m.TaskID.String(),
		SenderType:  string(m.SenderType),
		SenderID:    m.SenderID.String(),
		Content:     m.Content,
		MessageType: string(m.MessageType),
		Metadata:    m.Metadata,
		CreatedAt:   m.CreatedAt,
	}
}

// ToTaskMessageListResponse обёртка списка сообщений с пагинацией.
func ToTaskMessageListResponse(msgs []models.TaskMessage, total int64, limit, offset int) TaskMessageListResponse {
	out := make([]TaskMessageResponse, 0, len(msgs))
	for i := range msgs {
		out = append(out, ToTaskMessageResponse(&msgs[i]))
	}
	return TaskMessageListResponse{
		Messages: out,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}
}
