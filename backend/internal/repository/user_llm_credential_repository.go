package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrUserLlmCredentialNotUpdated — UPDATE не затронул строку (параллельный DELETE и т.п.).
var ErrUserLlmCredentialNotUpdated = errors.New("user llm credential row not updated")

// UserLlmCredentialRepository CRUD по user_llm_credentials и аудиту (bytea — уже зашифровано в service).
type UserLlmCredentialRepository interface {
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.UserLlmCredential, error)
	GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (*models.UserLlmCredential, error)
	Create(ctx context.Context, row *models.UserLlmCredential) error
	Update(ctx context.Context, row *models.UserLlmCredential) error
	DeleteByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (int64, error)
	CreateAudit(ctx context.Context, row *models.UserLlmCredentialAudit) error
}

type userLlmCredentialRepository struct {
	db *gorm.DB
}

// NewUserLlmCredentialRepository создаёт репозиторий.
func NewUserLlmCredentialRepository(db *gorm.DB) UserLlmCredentialRepository {
	return &userLlmCredentialRepository{db: db}
}

func (r *userLlmCredentialRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.UserLlmCredential, error) {
	var rows []models.UserLlmCredential
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("user_id = ?", userID).
		Order("provider ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list user llm credentials: %w", err)
	}
	return rows, nil
}

func (r *userLlmCredentialRepository) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (*models.UserLlmCredential, error) {
	var row models.UserLlmCredential
	err := gormDB(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, string(provider)).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get user llm credential: %w", err)
	}
	return &row, nil
}

func (r *userLlmCredentialRepository) Create(ctx context.Context, row *models.UserLlmCredential) error {
	if err := gormDB(ctx, r.db).WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create user llm credential: %w", err)
	}
	return nil
}

func (r *userLlmCredentialRepository) Update(ctx context.Context, row *models.UserLlmCredential) error {
	res := gormDB(ctx, r.db).WithContext(ctx).Model(&models.UserLlmCredential{}).
		Where("id = ?", row.ID).
		Updates(map[string]interface{}{
			"encrypted_key": row.EncryptedKey,
			"updated_at":    gorm.Expr("CURRENT_TIMESTAMP"),
		})
	if res.Error != nil {
		return fmt.Errorf("update user llm credential: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("%w", ErrUserLlmCredentialNotUpdated)
	}
	return nil
}

func (r *userLlmCredentialRepository) DeleteByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.UserLLMProvider) (int64, error) {
	res := gormDB(ctx, r.db).WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, string(provider)).
		Delete(&models.UserLlmCredential{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete user llm credential: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (r *userLlmCredentialRepository) CreateAudit(ctx context.Context, row *models.UserLlmCredentialAudit) error {
	if err := gormDB(ctx, r.db).WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create user llm credential audit: %w", err)
	}
	return nil
}
