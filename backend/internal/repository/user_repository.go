package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/devteam/backend/internal/models"
	"gorm.io/gorm"
)

// Удаляем определения ошибок, так как они теперь в errors.go

// UserRepository определяет интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
}

// userRepository реализация UserRepository
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository создает новый репозиторий пользователей
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create создает нового пользователя
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		// Проверяем ошибку дубликата ключа
		// GORM может вернуть gorm.ErrDuplicatedKey или ошибку от PostgreSQL
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrUserExists
		}

		// Проверяем ошибку PostgreSQL (код 23505 - unique_violation)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrUserExists
		}

		// Проверяем по тексту ошибки (fallback)
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return ErrUserExists
		}

		return err
	}
	return nil
}

// GetByID получает пользователя по ID
func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail получает пользователя по email
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// Update обновляет пользователя
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}
