package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"gorm.io/gorm"
)

type WebhookRepository interface {
	// CRUD для триггеров
	Create(ctx context.Context, webhook *models.WebhookTrigger) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.WebhookTrigger, error)
	GetByName(ctx context.Context, name string) (*models.WebhookTrigger, error)
	List(ctx context.Context) ([]models.WebhookTrigger, error)
	Update(ctx context.Context, webhook *models.WebhookTrigger) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Обновление статистики
	IncrementTriggerCount(ctx context.Context, id uuid.UUID) error

	// Логи
	CreateLog(ctx context.Context, log *models.WebhookLog) error
	ListLogs(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]models.WebhookLog, int64, error)
}

type webhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &webhookRepository{db: db}
}

func (r *webhookRepository) Create(ctx context.Context, webhook *models.WebhookTrigger) error {
	return r.db.WithContext(ctx).Create(webhook).Error
}

func (r *webhookRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.WebhookTrigger, error) {
	var webhook models.WebhookTrigger
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&webhook).Error; err != nil {
		return nil, err
	}
	return &webhook, nil
}

func (r *webhookRepository) GetByName(ctx context.Context, name string) (*models.WebhookTrigger, error) {
	var webhook models.WebhookTrigger
	if err := r.db.WithContext(ctx).Where("name = ? AND is_active = ?", name, true).First(&webhook).Error; err != nil {
		return nil, err
	}
	return &webhook, nil
}

func (r *webhookRepository) List(ctx context.Context) ([]models.WebhookTrigger, error) {
	var webhooks []models.WebhookTrigger
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&webhooks).Error; err != nil {
		return nil, err
	}
	return webhooks, nil
}

func (r *webhookRepository) Update(ctx context.Context, webhook *models.WebhookTrigger) error {
	return r.db.WithContext(ctx).Save(webhook).Error
}

func (r *webhookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.WebhookTrigger{}, "id = ?", id).Error
}

func (r *webhookRepository) IncrementTriggerCount(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WebhookTrigger{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"trigger_count":  gorm.Expr("trigger_count + 1"),
			"last_triggered": now,
		}).Error
}

func (r *webhookRepository) CreateLog(ctx context.Context, log *models.WebhookLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *webhookRepository) ListLogs(ctx context.Context, webhookID uuid.UUID, limit, offset int) ([]models.WebhookLog, int64, error) {
	var logs []models.WebhookLog
	var total int64

	query := r.db.WithContext(ctx).Model(&models.WebhookLog{}).Where("webhook_id = ?", webhookID)
	query.Count(&total)

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

