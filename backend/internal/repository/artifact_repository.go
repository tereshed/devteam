package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrArtifactNotFound — sentinel для caller'а.
var ErrArtifactNotFound = errors.New("artifact not found")

// ArtifactRepository — CRUD + специализированные методы для оркестратора.
//
// Семантика SupersedePrevious: когда новая итерация артефакта (например, исправленный
// план после review='changes_requested') производится, старый артефакт того же
// kind/parent_id помечается status='superseded'. Router учитывает только status='ready'.
type ArtifactRepository interface {
	Create(ctx context.Context, art *models.Artifact) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Artifact, error)

	// ListByTaskID — все артефакты задачи в порядке создания.
	// Параметр onlyReady=true фильтрует status='ready' (Router-режим);
	// false — все, включая superseded (для UI/аудита).
	ListByTaskID(ctx context.Context, taskID uuid.UUID, onlyReady bool) ([]models.Artifact, error)

	// ListMetadataByTaskID — то же что ListByTaskID, но БЕЗ content (для Router'а).
	// Защита от переполнения контекста LLM: content не подгружается, только summary и поля.
	ListMetadataByTaskID(ctx context.Context, taskID uuid.UUID, onlyReady bool) ([]models.Artifact, error)

	// SupersedePrevious — помечает все предыдущие 'ready' артефакты с тем же
	// (task_id, parent_id, kind) как 'superseded'. Вызывается ПЕРЕД Create новой итерации.
	// Возвращает количество затронутых записей.
	SupersedePrevious(ctx context.Context, taskID uuid.UUID, parentID *uuid.UUID, kind models.ArtifactKind) (int64, error)
}

type artifactRepository struct {
	db *gorm.DB
}

// NewArtifactRepository — конструктор.
func NewArtifactRepository(db *gorm.DB) ArtifactRepository {
	return &artifactRepository{db: db}
}

// artifactMetadataColumns — всё кроме content (защита от переполнения контекста Router'а).
const artifactMetadataColumns = "id, task_id, parent_id, producer_agent, kind, summary, status, iteration, created_at"

func (r *artifactRepository) Create(ctx context.Context, art *models.Artifact) error {
	if !models.ValidateArtifactSummary(art.Summary) {
		return fmt.Errorf("invalid artifact summary: must be non-empty and ≤ 500 chars, got %d chars", len(art.Summary))
	}
	if err := r.db.WithContext(ctx).Create(art).Error; err != nil {
		return fmt.Errorf("failed to create artifact: %w", err)
	}
	return nil
}

func (r *artifactRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Artifact, error) {
	var art models.Artifact
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&art).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrArtifactNotFound
		}
		return nil, fmt.Errorf("failed to get artifact %s: %w", id, err)
	}
	return &art, nil
}

func (r *artifactRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID, onlyReady bool) ([]models.Artifact, error) {
	q := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC")
	if onlyReady {
		q = q.Where("status = ?", models.ArtifactStatusReady)
	}
	var arts []models.Artifact
	if err := q.Find(&arts).Error; err != nil {
		return nil, fmt.Errorf("failed to list artifacts for task %s: %w", taskID, err)
	}
	return arts, nil
}

func (r *artifactRepository) ListMetadataByTaskID(ctx context.Context, taskID uuid.UUID, onlyReady bool) ([]models.Artifact, error) {
	q := r.db.WithContext(ctx).
		Select(artifactMetadataColumns).
		Where("task_id = ?", taskID).
		Order("created_at ASC")
	if onlyReady {
		q = q.Where("status = ?", models.ArtifactStatusReady)
	}
	var arts []models.Artifact
	if err := q.Find(&arts).Error; err != nil {
		return nil, fmt.Errorf("failed to list artifact metadata for task %s: %w", taskID, err)
	}
	return arts, nil
}

func (r *artifactRepository) SupersedePrevious(ctx context.Context, taskID uuid.UUID, parentID *uuid.UUID, kind models.ArtifactKind) (int64, error) {
	q := r.db.WithContext(ctx).Model(&models.Artifact{}).
		Where("task_id = ? AND kind = ? AND status = ?", taskID, kind, models.ArtifactStatusReady)
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	result := q.Update("status", models.ArtifactStatusSuperseded)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to supersede artifacts for task %s kind=%s: %w", taskID, kind, result.Error)
	}
	return result.RowsAffected, nil
}
