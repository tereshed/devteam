package repository

import (
	"context"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentSkillRepository — read-only по agent_skills (Sprint 15.24, MCP-инструмент skill_list).
type AgentSkillRepository interface {
	ListByAgent(ctx context.Context, agentID uuid.UUID, onlyActive bool) ([]models.AgentSkill, error)
	ListAll(ctx context.Context, onlyActive bool) ([]models.AgentSkill, error)
}

type agentSkillRepository struct{ db *gorm.DB }

func NewAgentSkillRepository(db *gorm.DB) AgentSkillRepository {
	return &agentSkillRepository{db: db}
}

func (r *agentSkillRepository) ListByAgent(ctx context.Context, agentID uuid.UUID, onlyActive bool) ([]models.AgentSkill, error) {
	q := r.db.WithContext(ctx).Where("agent_id = ?", agentID)
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}
	var items []models.AgentSkill
	if err := q.Order("skill_name ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *agentSkillRepository) ListAll(ctx context.Context, onlyActive bool) ([]models.AgentSkill, error) {
	q := r.db.WithContext(ctx)
	if onlyActive {
		q = q.Where("is_active = ?", true)
	}
	var items []models.AgentSkill
	if err := q.Order("agent_id, skill_name").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
