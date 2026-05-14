package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrRouterDecisionNotFound — sentinel.
var ErrRouterDecisionNotFound = errors.New("router decision not found")

// routerDecisionListColumns — НЕ включает encrypted_raw_response.
// Open-text reason + chosen_agents достаточно для UI / аналитики.
const routerDecisionListColumns = "id, task_id, step_no, chosen_agents, outcome, reason, created_at"

// RouterDecisionRepository — лог решений Router'а с шифрованным raw_response.
//
// Контракт: encrypted_raw_response записывается УЖЕ зашифрованным
// (pkg/crypto.AESEncryptor, AAD = id.String()).
// Retention: 30 дней через DeleteOlderThan, вызывается cron-job'ом.
type RouterDecisionRepository interface {
	Create(ctx context.Context, d *models.RouterDecision) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.RouterDecision, error)

	// ListByTaskID — все решения задачи в порядке step_no (для timeline в UI).
	// withRawResponse=false — encrypted_raw_response НЕ загружается (UI/таймлайн).
	// withRawResponse=true — загружает шифрованный blob (для расшифровки в дебаг-режиме).
	ListByTaskID(ctx context.Context, taskID uuid.UUID, withRawResponse bool) ([]models.RouterDecision, error)

	// DeleteOlderThan — удаляет записи старше cutoff. Возвращает количество удалённых.
	// Используется cron'ом для retention 30 дней.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type routerDecisionRepository struct {
	db *gorm.DB
}

// NewRouterDecisionRepository — конструктор.
func NewRouterDecisionRepository(db *gorm.DB) RouterDecisionRepository {
	return &routerDecisionRepository{db: db}
}

func (r *routerDecisionRepository) Create(ctx context.Context, d *models.RouterDecision) error {
	if d.Reason == "" {
		return fmt.Errorf("router decision reason is required (non-sensitive plain text)")
	}
	if d.Outcome != nil && !d.Outcome.IsValid() {
		return fmt.Errorf("invalid router decision outcome: %q", *d.Outcome)
	}
	// Если raw_response сохраняем — он обязан выглядеть как валидный blob pkg/crypto.
	if len(d.EncryptedRawResponse) > 0 && len(d.EncryptedRawResponse) < 29 {
		return fmt.Errorf("encrypted_raw_response too short (%d bytes), refusing to write — looks unencrypted", len(d.EncryptedRawResponse))
	}
	if err := r.db.WithContext(ctx).Create(d).Error; err != nil {
		return fmt.Errorf("failed to create router decision: %w", err)
	}
	return nil
}

func (r *routerDecisionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.RouterDecision, error) {
	var d models.RouterDecision
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&d).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRouterDecisionNotFound
		}
		return nil, fmt.Errorf("failed to get router decision %s: %w", id, err)
	}
	return &d, nil
}

func (r *routerDecisionRepository) ListByTaskID(ctx context.Context, taskID uuid.UUID, withRawResponse bool) ([]models.RouterDecision, error) {
	q := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("step_no ASC")
	if !withRawResponse {
		q = q.Select(routerDecisionListColumns)
	}
	var ds []models.RouterDecision
	if err := q.Find(&ds).Error; err != nil {
		return nil, fmt.Errorf("failed to list router decisions for task %s: %w", taskID, err)
	}
	return ds, nil
}

func (r *routerDecisionRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&models.RouterDecision{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete router decisions older than %s: %w", cutoff, result.Error)
	}
	return result.RowsAffected, nil
}
