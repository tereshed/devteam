//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/models"
)

// createTestUser создает тестового пользователя и возвращает его ID
func createTestUser(t *testing.T, userRepo UserRepository) *models.User {
	user := &models.User{
		Email:        "apikey-test-" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed_password",
		Role:         models.RoleUser,
	}
	err := userRepo.Create(context.Background(), user)
	require.NoError(t, err)
	return user
}

// hashKey хеширует ключ для теста
func hashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

func TestApiKeyRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	apiKey := &models.ApiKey{
		UserID:    user.ID,
		Name:      "Test Key",
		KeyHash:   hashKey("wibe_test_raw_key_12345"),
		KeyPrefix: "wibe_test_ra",
		Scopes:    "*",
	}

	err := repo.Create(context.Background(), apiKey)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, apiKey.ID)
	assert.False(t, apiKey.CreatedAt.IsZero())
}

func TestApiKeyRepository_GetByKeyHash(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)
	rawKey := "wibe_hash_test_key_123456"
	kh := hashKey(rawKey)

	apiKey := &models.ApiKey{
		UserID:    user.ID,
		Name:      "Hash Test Key",
		KeyHash:   kh,
		KeyPrefix: "wibe_hash_te",
		Scopes:    "read,write",
	}
	err := repo.Create(context.Background(), apiKey)
	require.NoError(t, err)

	tests := []struct {
		name        string
		keyHash     string
		expectError bool
	}{
		{
			name:        "find existing key",
			keyHash:     kh,
			expectError: false,
		},
		{
			name:        "key not found",
			keyHash:     hashKey("nonexistent_key"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, err := repo.GetByKeyHash(context.Background(), tt.keyHash)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, ErrApiKeyNotFound, err)
				assert.Nil(t, found)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, found)
				assert.Equal(t, "Hash Test Key", found.Name)
				assert.Equal(t, "read,write", found.Scopes)
			}
		})
	}
}

func TestApiKeyRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	apiKey := &models.ApiKey{
		UserID:    user.ID,
		Name:      "ID Test Key",
		KeyHash:   hashKey("wibe_id_test_key"),
		KeyPrefix: "wibe_id_tes",
		Scopes:    "*",
	}
	err := repo.Create(context.Background(), apiKey)
	require.NoError(t, err)

	// Найти существующий
	found, err := repo.GetByID(context.Background(), apiKey.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "ID Test Key", found.Name)

	// Не найти несуществующий
	notFound, err := repo.GetByID(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Equal(t, ErrApiKeyNotFound, err)
	assert.Nil(t, notFound)
}

func TestApiKeyRepository_ListByUserID(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	// Создаем 3 ключа
	for i := 0; i < 3; i++ {
		apiKey := &models.ApiKey{
			UserID:    user.ID,
			Name:      "Key " + string(rune('A'+i)),
			KeyHash:   hashKey("wibe_list_key_" + string(rune('A'+i))),
			KeyPrefix: "wibe_list_ke",
			Scopes:    "*",
		}
		err := repo.Create(context.Background(), apiKey)
		require.NoError(t, err)
	}

	keys, err := repo.ListByUserID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, keys, 3)

	// Ключи другого пользователя не видны
	otherUser := createTestUser(t, userRepo)
	otherKeys, err := repo.ListByUserID(context.Background(), otherUser.ID)
	assert.NoError(t, err)
	assert.Len(t, otherKeys, 0)
}

func TestApiKeyRepository_Revoke(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	apiKey := &models.ApiKey{
		UserID:    user.ID,
		Name:      "Revoke Test",
		KeyHash:   hashKey("wibe_revoke_test_key"),
		KeyPrefix: "wibe_revoke",
		Scopes:    "*",
	}
	err := repo.Create(context.Background(), apiKey)
	require.NoError(t, err)

	// Отзываем
	err = repo.Revoke(context.Background(), apiKey.ID)
	assert.NoError(t, err)

	// Проверяем, что ключ отозван
	found, err := repo.GetByID(context.Background(), apiKey.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found.RevokedAt)

	// Отозванный ключ не должен появляться в ListByUserID
	keys, err := repo.ListByUserID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, keys, 0)

	// Повторный отзыв — ошибка
	err = repo.Revoke(context.Background(), apiKey.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrApiKeyNotFound, err)
}

func TestApiKeyRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	apiKey := &models.ApiKey{
		UserID:    user.ID,
		Name:      "Delete Test",
		KeyHash:   hashKey("wibe_delete_test_key"),
		KeyPrefix: "wibe_delete",
		Scopes:    "*",
	}
	err := repo.Create(context.Background(), apiKey)
	require.NoError(t, err)

	// Удаляем
	err = repo.Delete(context.Background(), apiKey.ID)
	assert.NoError(t, err)

	// Проверяем, что удалён
	_, err = repo.GetByID(context.Background(), apiKey.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrApiKeyNotFound, err)

	// Повторное удаление — ошибка
	err = repo.Delete(context.Background(), apiKey.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrApiKeyNotFound, err)
}

func TestApiKeyRepository_UpdateLastUsed(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	apiKey := &models.ApiKey{
		UserID:    user.ID,
		Name:      "LastUsed Test",
		KeyHash:   hashKey("wibe_lastused_test_key"),
		KeyPrefix: "wibe_lastus",
		Scopes:    "*",
	}
	err := repo.Create(context.Background(), apiKey)
	require.NoError(t, err)

	// LastUsedAt изначально nil
	found, err := repo.GetByID(context.Background(), apiKey.ID)
	require.NoError(t, err)
	assert.Nil(t, found.LastUsedAt)

	// Обновляем
	err = repo.UpdateLastUsed(context.Background(), apiKey.ID)
	assert.NoError(t, err)

	// Проверяем
	found, err = repo.GetByID(context.Background(), apiKey.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found.LastUsedAt)
}

func TestApiKeyRepository_RevokeAllForUser(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	// Создаем 3 ключа
	for i := 0; i < 3; i++ {
		apiKey := &models.ApiKey{
			UserID:    user.ID,
			Name:      "Bulk Revoke " + string(rune('A'+i)),
			KeyHash:   hashKey("wibe_bulk_revoke_" + string(rune('A'+i))),
			KeyPrefix: "wibe_bulk_r",
			Scopes:    "*",
		}
		err := repo.Create(context.Background(), apiKey)
		require.NoError(t, err)
	}

	keys, err := repo.ListByUserID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Отзываем все
	err = repo.RevokeAllForUser(context.Background(), user.ID)
	assert.NoError(t, err)

	// Проверяем — список пуст (ListByUserID фильтрует отозванные)
	keys, err = repo.ListByUserID(context.Background(), user.ID)
	assert.NoError(t, err)
	assert.Len(t, keys, 0)
}

func TestApiKeyRepository_ExpiresAt(t *testing.T) {
	db := setupTestDB(t)
	defer cleanupTestDB(t, db)

	userRepo := NewUserRepository(db)
	repo := NewApiKeyRepository(db)

	user := createTestUser(t, userRepo)

	futureTime := time.Now().Add(24 * time.Hour)
	apiKey := &models.ApiKey{
		UserID:    user.ID,
		Name:      "Expiry Test",
		KeyHash:   hashKey("wibe_expiry_test_key"),
		KeyPrefix: "wibe_expiry",
		Scopes:    "*",
		ExpiresAt: &futureTime,
	}
	err := repo.Create(context.Background(), apiKey)
	require.NoError(t, err)

	found, err := repo.GetByID(context.Background(), apiKey.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found.ExpiresAt)
	assert.True(t, found.ExpiresAt.After(time.Now()))
}
