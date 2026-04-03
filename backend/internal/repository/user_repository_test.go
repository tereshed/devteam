//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/password"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Используем существующую БД из переменных окружения или дефолтные значения
	// Для шаблона используем основную БД, так как миграции уже применены
	dsn := "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "Failed to connect to test database")

	// Проверяем подключение
	sqlDB, err := db.DB()
	require.NoError(t, err, "Failed to get sql.DB")
	err = sqlDB.Ping()
	require.NoError(t, err, "Failed to ping database")

	return db
}

func cleanupTestDB(t *testing.T, db *gorm.DB) {
	// Очищаем таблицы в правильном порядке (сначала зависимые)
	db.Exec("TRUNCATE TABLE llm_logs, scheduled_workflows, execution_steps, executions, workflows, agents, users, prompts, refresh_tokens, api_keys CASCADE")
}

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewUserRepository(db)

	tests := []struct {
		name        string
		user        *models.User
		expectError bool
	}{
		{
			name: "create valid user",
			user: &models.User{
				Email:        "test@example.com",
				PasswordHash: "hashed_password",
				Role:         models.RoleUser,
			},
			expectError: false,
		},
		{
			name: "create user with duplicate email",
			user: &models.User{
				Email:        "duplicate@example.com",
				PasswordHash: "hashed_password",
				Role:         models.RoleUser,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), tt.user)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEqual(t, uuid.Nil, tt.user.ID)
			}
		})
	}

	// Проверяем дубликат
	duplicateUser := &models.User{
		Email:        "duplicate@example.com",
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	err := repo.Create(context.Background(), duplicateUser)
	assert.Error(t, err)
	assert.Equal(t, ErrUserExists, err)
}

func TestUserRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewUserRepository(db)

	// Создаем тестового пользователя
	user := &models.User{
		Email:        "getbyid@example.com",
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	tests := []struct {
		name        string
		userID      uuid.UUID
		expectError bool
		expectFound bool
	}{
		{
			name:        "get existing user",
			userID:      user.ID,
			expectError: false,
			expectFound: true,
		},
		{
			name:        "get non-existing user",
			userID:      uuid.New(),
			expectError: true,
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundUser, err := repo.GetByID(context.Background(), tt.userID)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrUserNotFound, err)
				assert.Nil(t, foundUser)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, foundUser)
				assert.Equal(t, tt.userID, foundUser.ID)
			}
		})
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewUserRepository(db)

	// Создаем тестового пользователя
	user := &models.User{
		Email:        "getbyemail@example.com",
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	tests := []struct {
		name        string
		email       string
		expectError bool
		expectFound bool
	}{
		{
			name:        "get existing user by email",
			email:       "getbyemail@example.com",
			expectError: false,
			expectFound: true,
		},
		{
			name:        "get non-existing user by email",
			email:       "notfound@example.com",
			expectError: true,
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundUser, err := repo.GetByEmail(context.Background(), tt.email)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrUserNotFound, err)
				assert.Nil(t, foundUser)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, foundUser)
				assert.Equal(t, tt.email, foundUser.Email)
			}
		})
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewUserRepository(db)

	// Создаем тестового пользователя
	user := &models.User{
		Email:        "update@example.com",
		PasswordHash: "old_hash",
		Role:         models.RoleUser,
	}
	err := repo.Create(context.Background(), user)
	require.NoError(t, err)

	// Обновляем пользователя
	user.PasswordHash = "new_hash"
	user.Role = models.RoleAdmin
	err = repo.Update(context.Background(), user)
	assert.NoError(t, err)

	// Проверяем обновление
	updatedUser, err := repo.GetByID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Equal(t, "new_hash", updatedUser.PasswordHash)
	assert.Equal(t, models.RoleAdmin, updatedUser.Role)
}

func TestUserRepository_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	repo := NewUserRepository(db)

	// Полный цикл: создание, чтение, обновление
	passwordHash, err := password.Hash("testpassword")
	require.NoError(t, err)

	user := &models.User{
		Email:        "integration@example.com",
		PasswordHash: passwordHash,
		Role:         models.RoleUser,
	}

	// Create
	err = repo.Create(context.Background(), user)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, user.ID)

	// GetByID
	foundUser, err := repo.GetByID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Equal(t, user.Email, foundUser.Email)

	// GetByEmail
	foundByEmail, err := repo.GetByEmail(context.Background(), user.Email)
	assert.NoError(t, err)
	assert.Equal(t, user.ID, foundByEmail.ID)

	// Update
	foundUser.Role = models.RoleAdmin
	err = repo.Update(context.Background(), foundUser)
	assert.NoError(t, err)

	// Verify update
	updatedUser, err := repo.GetByID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, updatedUser.Role)
}
