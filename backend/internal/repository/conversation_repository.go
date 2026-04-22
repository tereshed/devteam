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
	"gorm.io/plugin/dbresolver"
)

const (
	conversationListDefaultLimit = 20
	conversationListMaxLimit     = 100
)

const (
	OrderByCreatedAt = "created_at"
	OrderByUpdatedAt = "updated_at"
	OrderByTitle     = "title"
)

var allowedConversationOrderColumns = map[string]bool{
	OrderByCreatedAt: true,
	OrderByUpdatedAt: true,
	OrderByTitle:     true,
}

// ConversationFilter фильтры и пагинация для списка разговоров
type ConversationFilter struct {
	UserID   *uuid.UUID
	Status   *models.ConversationStatus
	Search   *string
	Limit    int
	Offset   int
	OrderBy  string
	OrderDir string
	Master   bool // Force read from master
}

// ConversationRepository интерфейс для CRUD-операций с таблицей conversations
type ConversationRepository interface {
	// WithTx возвращает копию репозитория, использующую переданную транзакцию
	WithTx(tx *gorm.DB) ConversationRepository

	Create(ctx context.Context, conv *models.Conversation) error

	// GetByID требует projectID для защиты от IDOR (Tenant Isolation)
	GetByID(ctx context.Context, projectID, id uuid.UUID, master bool) (*models.Conversation, error)

	// GetOnlyByID возвращает чат по ID без projectID (для сервиса)
	GetOnlyByID(ctx context.Context, id uuid.UUID, master bool) (*models.Conversation, error)

	ListByProjectID(ctx context.Context, projectID uuid.UUID, filter ConversationFilter) ([]*models.Conversation, int64, error)

	// Update использует Patch-семантику (только измененные поля) для предотвращения Race Conditions
	Update(ctx context.Context, projectID, id uuid.UUID, updates map[string]interface{}) error

	Delete(ctx context.Context, projectID, id uuid.UUID) error
}

type conversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository создает новый репозиторий разговоров
func NewConversationRepository(db *gorm.DB) ConversationRepository {
	return &conversationRepository{db: db}
}

func (r *conversationRepository) WithTx(tx *gorm.DB) ConversationRepository {
	return &conversationRepository{db: tx}
}

func (r *conversationRepository) Create(ctx context.Context, conv *models.Conversation) error {
	if conv == nil {
		return ErrInvalidInput
	}
	if conv.ProjectID == uuid.Nil || conv.UserID == uuid.Nil {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(conv).Error; err != nil {
		return r.mapDBError(err)
	}
	return nil
}

func (r *conversationRepository) GetByID(ctx context.Context, projectID, id uuid.UUID, master bool) (*models.Conversation, error) {
	if projectID == uuid.Nil || id == uuid.Nil {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	if master {
		db = db.Clauses(dbresolver.Write)
	}
	var conv models.Conversation
	err := db.WithContext(ctx).
		Where("id = ? AND project_id = ?", id, projectID).
		First(&conv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}
	return &conv, nil
}

func (r *conversationRepository) GetOnlyByID(ctx context.Context, id uuid.UUID, master bool) (*models.Conversation, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	if master {
		db = db.Clauses(dbresolver.Write)
	}
	var conv models.Conversation
	err := db.WithContext(ctx).
		Where("id = ?", id).
		First(&conv).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, fmt.Errorf("failed to get conversation by id: %w", err)
	}
	return &conv, nil
}

func (r *conversationRepository) ListByProjectID(ctx context.Context, projectID uuid.UUID, filter ConversationFilter) ([]*models.Conversation, int64, error) {
	if projectID == uuid.Nil {
		return nil, 0, ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	if filter.Master {
		db = db.Clauses(dbresolver.Write)
	}
	
	// TODO: migrate to cursor pagination
	var count int64
	countQuery := db.WithContext(ctx).Model(&models.Conversation{}).
		Where("project_id = ?", projectID).
		Scopes(
			FilterByUserID(filter.UserID),
			FilterByConversationStatus(filter.Status),
			FilterByConversationSearch(filter.Search),
		)
	
	if err := countQuery.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count conversations: %w", err)
	}

	if count == 0 {
		return []*models.Conversation{}, 0, nil
	}

	var conversations []*models.Conversation
	order := r.sanitizeOrder(filter.OrderBy, filter.OrderDir)
	limit := normalizeLimit(filter.Limit, conversationListDefaultLimit, conversationListMaxLimit)
	offset := normalizeOffset(filter.Offset)

	q := db.WithContext(ctx).Model(&models.Conversation{}).
		Where("project_id = ?", projectID).
		Scopes(
			FilterByUserID(filter.UserID),
			FilterByConversationStatus(filter.Status),
			FilterByConversationSearch(filter.Search),
		).
		Order(order).
		Limit(limit).
		Offset(offset)

	if err := q.Find(&conversations).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list conversations: %w", err)
	}

	return conversations, count, nil
}

func (r *conversationRepository) Update(ctx context.Context, projectID, id uuid.UUID, updates map[string]interface{}) error {
	if projectID == uuid.Nil || id == uuid.Nil || len(updates) == 0 {
		return ErrInvalidInput
	}

	// Защита от изменения системных полей
	delete(updates, "id")
	delete(updates, "project_id")

	db := gormDB(ctx, r.db)
	result := db.WithContext(ctx).Model(&models.Conversation{}).
		Where("id = ? AND project_id = ?", id, projectID).
		Updates(updates)

	if result.Error != nil {
		return r.mapDBError(result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrConversationNotFound
	}

	return nil
}

func (r *conversationRepository) Delete(ctx context.Context, projectID, id uuid.UUID) error {
	if projectID == uuid.Nil || id == uuid.Nil {
		return ErrInvalidInput
	}

	db := gormDB(ctx, r.db)
	result := db.WithContext(ctx).
		Where("id = ? AND project_id = ?", id, projectID).
		Delete(&models.Conversation{})

	if result.Error != nil {
		return fmt.Errorf("failed to delete conversation: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrConversationNotFound
	}

	return nil
}

// Scopes

func FilterByUserID(userID *uuid.UUID) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if userID != nil && *userID != uuid.Nil {
			return db.Where("user_id = ?", *userID)
		}
		return db
	}
}

func FilterByConversationStatus(status *models.ConversationStatus) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if status != nil && *status != "" {
			return db.Where("status = ?", *status)
		}
		return db
	}
}

func FilterByConversationSearch(search *string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if search != nil && *search != "" {
			pattern := "%" + escapeILIKEWildcards(*search) + "%"
			return db.Where("title ILIKE ? ESCAPE '\\'", pattern)
		}
		return db
	}
}

// Helpers

func (r *conversationRepository) sanitizeOrder(orderBy, orderDir string) string {
	if !allowedConversationOrderColumns[orderBy] {
		orderBy = OrderByCreatedAt
	}
	return orderBy + " " + sanitizeOrderDir(orderDir) + ", id ASC"
}

func (r *conversationRepository) mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503": // foreign_key_violation
			if strings.Contains(pgErr.ConstraintName, "project_id") {
				return ErrProjectNotFound
			}
			if strings.Contains(pgErr.ConstraintName, "user_id") {
				return ErrUserNotFound
			}
		case "23514": // check_violation
			if strings.Contains(pgErr.ConstraintName, "status") {
				return ErrInvalidConversationStatus
			}
		}
	}
	return fmt.Errorf("database error: %w", err)
}
