package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"gorm.io/gorm"
)

var ErrAgentRolePromptNotFound = errors.New("agent role prompt not found")

type AgentRolePromptRepository interface {
	GetByRole(ctx context.Context, role string) (*models.AgentRolePrompt, error)
	List(ctx context.Context) ([]models.AgentRolePrompt, error)
	Upsert(ctx context.Context, prompt *models.AgentRolePrompt) error
}

type agentRolePromptRepository struct {
	db *gorm.DB
}

func NewAgentRolePromptRepository(db *gorm.DB) AgentRolePromptRepository {
	return &agentRolePromptRepository{db: db}
}

func (r *agentRolePromptRepository) GetByRole(ctx context.Context, role string) (*models.AgentRolePrompt, error) {
	db := gormDB(ctx, r.db)
	var p models.AgentRolePrompt
	err := db.WithContext(ctx).Where("role = ?", role).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentRolePromptNotFound
		}
		return nil, fmt.Errorf("get agent role prompt %q: %w", role, err)
	}
	return &p, nil
}

func (r *agentRolePromptRepository) List(ctx context.Context) ([]models.AgentRolePrompt, error) {
	db := gormDB(ctx, r.db)
	var prompts []models.AgentRolePrompt
	err := db.WithContext(ctx).Order("role ASC").Find(&prompts).Error
	if err != nil {
		return nil, fmt.Errorf("list agent role prompts: %w", err)
	}
	return prompts, nil
}

func (r *agentRolePromptRepository) Upsert(ctx context.Context, prompt *models.AgentRolePrompt) error {
	db := gormDB(ctx, r.db)
	err := db.WithContext(ctx).
		Where("role = ?", prompt.Role).
		Assign(models.AgentRolePrompt{
			Content:     prompt.Content,
			Description: prompt.Description,
			UpdatedBy:   prompt.UpdatedBy,
		}).
		FirstOrCreate(prompt).Error
	if err != nil {
		return fmt.Errorf("upsert agent role prompt %q: %w", prompt.Role, err)
	}
	return nil
}
