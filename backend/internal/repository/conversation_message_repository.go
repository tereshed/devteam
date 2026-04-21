package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	messageListDefaultLimit = 20
	messageListMaxLimit     = 100
)

const (
	OrderByMessageCreatedAt = "created_at"
)

var allowedMessageOrderColumns = map[string]bool{
	OrderByMessageCreatedAt: true,
}

// MessageFilter фильтры и пагинация для списка сообщений
type MessageFilter struct {
	Role     *models.ConversationRole
	Limit    int
	Offset   int
	OrderBy  string
	OrderDir string
}

// ConversationMessageRepository интерфейс для CRUD-операций с таблицей conversation_messages
type ConversationMessageRepository interface {
	// WithTx возвращает копию репозитория, использующую переданную транзакцию
	WithTx(tx *gorm.DB) ConversationMessageRepository

	Create(ctx context.Context, msg *models.ConversationMessage) error

	// GetByID требует conversationID для защиты от IDOR
	GetByID(ctx context.Context, conversationID, id uuid.UUID) (*models.ConversationMessage, error)

	// ListByConversationID возвращает сообщения конкретного чата с пагинацией
	ListByConversationID(ctx context.Context, conversationID uuid.UUID, filter MessageFilter) ([]*models.ConversationMessage, int64, error)

	// Update используется для обновления статуса сообщения или его контента (редко)
	Update(ctx context.Context, conversationID, id uuid.UUID, updates map[string]interface{}) error

	Delete(ctx context.Context, conversationID, id uuid.UUID) error
}

type conversationMessageRepository struct {
	db *gorm.DB
}

// NewConversationMessageRepository создает новый репозиторий сообщений чата
func NewConversationMessageRepository(db *gorm.DB) ConversationMessageRepository {
	return &conversationMessageRepository{db: db}
}

func (r *conversationMessageRepository) WithTx(tx *gorm.DB) ConversationMessageRepository {
	return &conversationMessageRepository{db: tx}
}

func (r *conversationMessageRepository) Create(ctx context.Context, msg *models.ConversationMessage) error {
	if msg == nil {
		return ErrInvalidInput
	}
	if msg.ConversationID == uuid.Nil || msg.Role == "" || msg.Content == "" {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(msg).Error; err != nil {
		return r.mapDBError(err)
	}
	return nil
}

func (r *conversationMessageRepository) GetByID(ctx context.Context, conversationID, id uuid.UUID) (*models.ConversationMessage, error) {
	if conversationID == uuid.Nil || id == uuid.Nil {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	var msg models.ConversationMessage
	err := db.WithContext(ctx).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		First(&msg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	return &msg, nil
}

func (r *conversationMessageRepository) ListByConversationID(ctx context.Context, conversationID uuid.UUID, filter MessageFilter) ([]*models.ConversationMessage, int64, error) {
	if conversationID == uuid.Nil {
		return nil, 0, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)

	baseQuery := db.WithContext(ctx).Model(&models.ConversationMessage{}).
		Where("conversation_id = ?", conversationID).
		Scopes(
			FilterByMessageRole(filter.Role),
		)

	var count int64
	if err := baseQuery.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count messages: %w", err)
	}

	if count == 0 {
		return []*models.ConversationMessage{}, 0, nil
	}

	var messages []*models.ConversationMessage
	order := r.sanitizeOrder(filter.OrderBy, filter.OrderDir)
	limit := normalizeLimit(filter.Limit, messageListDefaultLimit, messageListMaxLimit)
	offset := normalizeOffset(filter.Offset)

	if err := baseQuery.Order(order).Limit(limit).Offset(offset).Find(&messages).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list messages: %w", err)
	}

	return messages, count, nil
}

func (r *conversationMessageRepository) Update(ctx context.Context, conversationID, id uuid.UUID, updates map[string]interface{}) error {
	if conversationID == uuid.Nil || id == uuid.Nil {
		return ErrInvalidInput
	}
	if len(updates) == 0 {
		return nil // Early return as per requirements
	}

	// Защита от изменения системных полей без мутации исходной мапы
	safeUpdates := make(map[string]interface{}, len(updates))
	for k, v := range updates {
		if k == "id" || k == "conversation_id" {
			continue
		}
		safeUpdates[k] = v
	}

	if len(safeUpdates) == 0 {
		return nil
	}

	db := gormDB(ctx, r.db)
	result := db.WithContext(ctx).Model(&models.ConversationMessage{}).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		Updates(safeUpdates)

	if result.Error != nil {
		return r.mapDBError(result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrMessageNotFound
	}

	return nil
}

func (r *conversationMessageRepository) Delete(ctx context.Context, conversationID, id uuid.UUID) error {
	if conversationID == uuid.Nil || id == uuid.Nil {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	result := db.WithContext(ctx).
		Where("id = ? AND conversation_id = ?", id, conversationID).
		Delete(&models.ConversationMessage{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete message: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrMessageNotFound
	}

	return nil
}

// Scopes

func FilterByMessageRole(role *models.ConversationRole) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if role != nil && *role != "" {
			return db.Where("role = ?", *role)
		}
		return db
	}
}

// Helpers

func (r *conversationMessageRepository) sanitizeOrder(orderBy, orderDir string) string {
	if !allowedMessageOrderColumns[orderBy] {
		orderBy = OrderByMessageCreatedAt
	}
	return orderBy + " " + sanitizeOrderDir(orderDir) + ", id ASC"
}

func (r *conversationMessageRepository) mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503": // foreign_key_violation
			if strings.Contains(pgErr.ConstraintName, "conversation_id") {
				return ErrConversationNotFound
			}
		case "23514": // check_violation
			if strings.Contains(pgErr.ConstraintName, "role") {
				return ErrInvalidMessageRole
			}
		}
	}
	return fmt.Errorf("database error: %w", err)
}
