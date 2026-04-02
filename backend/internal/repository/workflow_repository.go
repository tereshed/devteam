package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"gorm.io/gorm"
)

type WorkflowRepository interface {
	// Workflow
	CreateWorkflow(ctx context.Context, wf *models.Workflow) error
	GetWorkflowByID(ctx context.Context, id uuid.UUID) (*models.Workflow, error)
	GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error)
	ListWorkflows(ctx context.Context) ([]models.Workflow, error)

	// Agent
	CreateAgent(ctx context.Context, agent *models.Agent) error
	GetAgentByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	GetAgentByName(ctx context.Context, name string) (*models.Agent, error)

	// Execution
	CreateExecution(ctx context.Context, exec *models.Execution) error
	GetExecutionByID(ctx context.Context, id uuid.UUID) (*models.Execution, error)
	UpdateExecution(ctx context.Context, exec *models.Execution) error
	ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error)
	AddExecutionStep(ctx context.Context, step *models.ExecutionStep) error
	GetExecutionSteps(ctx context.Context, executionID uuid.UUID) ([]models.ExecutionStep, error)
	GetNextPendingExecution(ctx context.Context) (*models.Execution, error)

	// Schedule
	CreateScheduledWorkflow(ctx context.Context, schedule *models.ScheduledWorkflow) error
	ListActiveSchedules(ctx context.Context) ([]models.ScheduledWorkflow, error)
	UpdateSchedule(ctx context.Context, schedule *models.ScheduledWorkflow) error
}

type workflowRepository struct {
	db *gorm.DB
}

func NewWorkflowRepository(db *gorm.DB) WorkflowRepository {
	return &workflowRepository{db: db}
}

// --- Workflow ---

func (r *workflowRepository) CreateWorkflow(ctx context.Context, wf *models.Workflow) error {
	return r.db.WithContext(ctx).Create(wf).Error
}

func (r *workflowRepository) GetWorkflowByID(ctx context.Context, id uuid.UUID) (*models.Workflow, error) {
	var wf models.Workflow
	if err := r.db.WithContext(ctx).First(&wf, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &wf, nil
}

func (r *workflowRepository) GetWorkflowByName(ctx context.Context, name string) (*models.Workflow, error) {
	var wf models.Workflow
	if err := r.db.WithContext(ctx).First(&wf, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &wf, nil
}

func (r *workflowRepository) ListWorkflows(ctx context.Context) ([]models.Workflow, error) {
	var wfs []models.Workflow
	if err := r.db.WithContext(ctx).Find(&wfs).Error; err != nil {
		return nil, err
	}
	return wfs, nil
}

// --- Agent ---

func (r *workflowRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	return r.db.WithContext(ctx).Create(agent).Error
}

func (r *workflowRepository) GetAgentByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	var agent models.Agent
	if err := r.db.WithContext(ctx).Preload("Prompt").First(&agent, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func (r *workflowRepository) GetAgentByName(ctx context.Context, name string) (*models.Agent, error) {
	var agent models.Agent
	if err := r.db.WithContext(ctx).Preload("Prompt").First(&agent, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

// --- Execution ---

func (r *workflowRepository) CreateExecution(ctx context.Context, exec *models.Execution) error {
	return r.db.WithContext(ctx).Create(exec).Error
}

func (r *workflowRepository) GetExecutionByID(ctx context.Context, id uuid.UUID) (*models.Execution, error) {
	var exec models.Execution
	if err := r.db.WithContext(ctx).Preload("Workflow").First(&exec, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &exec, nil
}

func (r *workflowRepository) UpdateExecution(ctx context.Context, exec *models.Execution) error {
	return r.db.WithContext(ctx).Save(exec).Error
}

func (r *workflowRepository) ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, int64, error) {
	var execs []models.Execution
	var count int64

	db := r.db.WithContext(ctx).Model(&models.Execution{})
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Preload("Workflow").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&execs).Error; err != nil {
		return nil, 0, err
	}
	return execs, count, nil
}

func (r *workflowRepository) AddExecutionStep(ctx context.Context, step *models.ExecutionStep) error {
	return r.db.WithContext(ctx).Create(step).Error
}

func (r *workflowRepository) GetExecutionSteps(ctx context.Context, executionID uuid.UUID) ([]models.ExecutionStep, error) {
	var steps []models.ExecutionStep
	if err := r.db.WithContext(ctx).
		Where("execution_id = ?", executionID).
		Preload("Agent").
		Order("created_at ASC").
		Find(&steps).Error; err != nil {
		return nil, err
	}
	return steps, nil
}

func (r *workflowRepository) GetNextPendingExecution(ctx context.Context) (*models.Execution, error) {
	var exec models.Execution
	if err := r.db.WithContext(ctx).
		Preload("Workflow").
		Where("status = ? OR status = ?", models.ExecutionPending, models.ExecutionRunning).
		Order("updated_at ASC").
		First(&exec).Error; err != nil {
		return nil, err
	}
	return &exec, nil
}

// --- Schedule ---

func (r *workflowRepository) CreateScheduledWorkflow(ctx context.Context, schedule *models.ScheduledWorkflow) error {
	return r.db.WithContext(ctx).Create(schedule).Error
}

func (r *workflowRepository) ListActiveSchedules(ctx context.Context) ([]models.ScheduledWorkflow, error) {
	var schedules []models.ScheduledWorkflow
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

func (r *workflowRepository) UpdateSchedule(ctx context.Context, schedule *models.ScheduledWorkflow) error {
	return r.db.WithContext(ctx).Save(schedule).Error
}
