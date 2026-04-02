package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/repository"
)

// MockApiKeyRepository мок для ApiKeyRepository
type MockApiKeyRepository struct {
	mock.Mock
}

func (m *MockApiKeyRepository) Create(ctx context.Context, apiKey *models.ApiKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockApiKeyRepository) GetByKeyHash(ctx context.Context, keyHash string) (*models.ApiKey, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApiKey), args.Error(1)
}

func (m *MockApiKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ApiKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ApiKey), args.Error(1)
}

func (m *MockApiKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.ApiKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ApiKey), args.Error(1)
}

func (m *MockApiKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockApiKeyRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockApiKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockApiKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestApiKeyService_CreateKey(t *testing.T) {
	tests := []struct {
		name          string
		userID        uuid.UUID
		keyName       string
		scopes        string
		expiresAt     *time.Time
		mockSetup     func(*MockApiKeyRepository, *MockUserRepository)
		expectedError bool
		validate      func(*testing.T, *models.ApiKey, string)
	}{
		{
			name:    "successful creation without expiry",
			userID:  uuid.New(),
			keyName: "My API Key",
			scopes:  "",
			mockSetup: func(apiKeyRepo *MockApiKeyRepository, userRepo *MockUserRepository) {
				apiKeyRepo.On("Create", mock.Anything, mock.MatchedBy(func(key *models.ApiKey) bool {
					return key.Name == "My API Key" && key.Scopes == "*" && key.ExpiresAt == nil
				})).Return(nil)
			},
			validate: func(t *testing.T, key *models.ApiKey, rawKey string) {
				assert.NotNil(t, key)
				assert.Equal(t, "My API Key", key.Name)
				assert.Equal(t, "*", key.Scopes)
				assert.NotEmpty(t, rawKey)
				assert.True(t, len(rawKey) > 10)
				assert.Contains(t, rawKey, "wibe_")
				assert.Nil(t, key.ExpiresAt)
			},
		},
		{
			name:    "successful creation with expiry",
			userID:  uuid.New(),
			keyName: "Temp Key",
			scopes:  "read,write",
			expiresAt: func() *time.Time {
				t := time.Now().Add(24 * time.Hour)
				return &t
			}(),
			mockSetup: func(apiKeyRepo *MockApiKeyRepository, userRepo *MockUserRepository) {
				apiKeyRepo.On("Create", mock.Anything, mock.MatchedBy(func(key *models.ApiKey) bool {
					return key.Name == "Temp Key" && key.Scopes == "read,write" && key.ExpiresAt != nil
				})).Return(nil)
			},
			validate: func(t *testing.T, key *models.ApiKey, rawKey string) {
				assert.NotNil(t, key)
				assert.Equal(t, "read,write", key.Scopes)
				assert.NotNil(t, key.ExpiresAt)
			},
		},
		{
			name:    "creation with default scopes",
			userID:  uuid.New(),
			keyName: "Default Scopes Key",
			scopes:  "",
			mockSetup: func(apiKeyRepo *MockApiKeyRepository, userRepo *MockUserRepository) {
				apiKeyRepo.On("Create", mock.Anything, mock.MatchedBy(func(key *models.ApiKey) bool {
					return key.Scopes == "*"
				})).Return(nil)
			},
			validate: func(t *testing.T, key *models.ApiKey, rawKey string) {
				assert.Equal(t, "*", key.Scopes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApiKeyRepo := new(MockApiKeyRepository)
			mockUserRepo := new(MockUserRepository)
			tt.mockSetup(mockApiKeyRepo, mockUserRepo)

			svc := NewApiKeyService(mockApiKeyRepo, mockUserRepo)
			key, rawKey, err := svc.CreateKey(context.Background(), tt.userID, tt.keyName, tt.scopes, tt.expiresAt)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, key, rawKey)
				}
			}

			mockApiKeyRepo.AssertExpectations(t)
		})
	}
}

func TestApiKeyService_ValidateKey(t *testing.T) {
	userID := uuid.New()
	keyID := uuid.New()
	rawKey := "wibe_abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	keyHash := func() string {
		h := sha256.Sum256([]byte(rawKey))
		return hex.EncodeToString(h[:])
	}()

	tests := []struct {
		name          string
		rawKey        string
		mockSetup     func(*MockApiKeyRepository, *MockUserRepository)
		expectedError error
	}{
		{
			name:   "valid key",
			rawKey: rawKey,
			mockSetup: func(apiKeyRepo *MockApiKeyRepository, userRepo *MockUserRepository) {
				apiKey := &models.ApiKey{
					ID:     keyID,
					UserID: userID,
					Name:   "Test Key",
					Scopes: "*",
				}
				apiKeyRepo.On("GetByKeyHash", mock.Anything, keyHash).Return(apiKey, nil)
				userRepo.On("GetByID", mock.Anything, userID).Return(&models.User{
					ID:    userID,
					Email: "user@example.com",
					Role:  models.RoleUser,
				}, nil)
				apiKeyRepo.On("UpdateLastUsed", mock.Anything, keyID).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "key not found",
			rawKey: "wibe_nonexistent",
			mockSetup: func(apiKeyRepo *MockApiKeyRepository, userRepo *MockUserRepository) {
				apiKeyRepo.On("GetByKeyHash", mock.Anything, mock.Anything).Return(nil, repository.ErrApiKeyNotFound)
			},
			expectedError: ErrApiKeyNotFound,
		},
		{
			name:   "revoked key",
			rawKey: rawKey,
			mockSetup: func(apiKeyRepo *MockApiKeyRepository, userRepo *MockUserRepository) {
				now := time.Now()
				apiKey := &models.ApiKey{
					ID:        keyID,
					UserID:    userID,
					RevokedAt: &now,
				}
				apiKeyRepo.On("GetByKeyHash", mock.Anything, keyHash).Return(apiKey, nil)
			},
			expectedError: ErrApiKeyRevoked,
		},
		{
			name:   "expired key",
			rawKey: rawKey,
			mockSetup: func(apiKeyRepo *MockApiKeyRepository, userRepo *MockUserRepository) {
				expired := time.Now().Add(-1 * time.Hour)
				apiKey := &models.ApiKey{
					ID:        keyID,
					UserID:    userID,
					ExpiresAt: &expired,
				}
				apiKeyRepo.On("GetByKeyHash", mock.Anything, keyHash).Return(apiKey, nil)
			},
			expectedError: ErrApiKeyExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApiKeyRepo := new(MockApiKeyRepository)
			mockUserRepo := new(MockUserRepository)
			tt.mockSetup(mockApiKeyRepo, mockUserRepo)

			svc := NewApiKeyService(mockApiKeyRepo, mockUserRepo)
			_, user, err := svc.ValidateKey(context.Background(), tt.rawKey)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
			}

			// Даём время goroutine UpdateLastUsed завершиться
			time.Sleep(10 * time.Millisecond)
			mockApiKeyRepo.AssertExpectations(t)
			mockUserRepo.AssertExpectations(t)
		})
	}
}

func TestApiKeyService_RevokeKey(t *testing.T) {
	ownerID := uuid.New()
	otherUserID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name          string
		keyID         uuid.UUID
		userID        uuid.UUID
		isAdmin       bool
		mockSetup     func(*MockApiKeyRepository)
		expectedError error
	}{
		{
			name:    "owner revokes own key",
			keyID:   keyID,
			userID:  ownerID,
			isAdmin: false,
			mockSetup: func(repo *MockApiKeyRepository) {
				repo.On("GetByID", mock.Anything, keyID).Return(&models.ApiKey{
					ID:     keyID,
					UserID: ownerID,
				}, nil)
				repo.On("Revoke", mock.Anything, keyID).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:    "admin revokes any key",
			keyID:   keyID,
			userID:  otherUserID,
			isAdmin: true,
			mockSetup: func(repo *MockApiKeyRepository) {
				repo.On("GetByID", mock.Anything, keyID).Return(&models.ApiKey{
					ID:     keyID,
					UserID: ownerID,
				}, nil)
				repo.On("Revoke", mock.Anything, keyID).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:    "non-owner non-admin gets access denied",
			keyID:   keyID,
			userID:  otherUserID,
			isAdmin: false,
			mockSetup: func(repo *MockApiKeyRepository) {
				repo.On("GetByID", mock.Anything, keyID).Return(&models.ApiKey{
					ID:     keyID,
					UserID: ownerID,
				}, nil)
			},
			expectedError: ErrApiKeyAccessDenied,
		},
		{
			name:    "key not found",
			keyID:   uuid.New(),
			userID:  ownerID,
			isAdmin: false,
			mockSetup: func(repo *MockApiKeyRepository) {
				repo.On("GetByID", mock.Anything, mock.Anything).Return(nil, repository.ErrApiKeyNotFound)
			},
			expectedError: ErrApiKeyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApiKeyRepo := new(MockApiKeyRepository)
			mockUserRepo := new(MockUserRepository)
			tt.mockSetup(mockApiKeyRepo)

			svc := NewApiKeyService(mockApiKeyRepo, mockUserRepo)
			err := svc.RevokeKey(context.Background(), tt.keyID, tt.userID, tt.isAdmin)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			mockApiKeyRepo.AssertExpectations(t)
		})
	}
}

func TestApiKeyService_DeleteKey(t *testing.T) {
	ownerID := uuid.New()
	otherUserID := uuid.New()
	keyID := uuid.New()

	tests := []struct {
		name          string
		userID        uuid.UUID
		isAdmin       bool
		mockSetup     func(*MockApiKeyRepository)
		expectedError error
	}{
		{
			name:    "owner deletes own key",
			userID:  ownerID,
			isAdmin: false,
			mockSetup: func(repo *MockApiKeyRepository) {
				repo.On("GetByID", mock.Anything, keyID).Return(&models.ApiKey{ID: keyID, UserID: ownerID}, nil)
				repo.On("Delete", mock.Anything, keyID).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:    "non-owner gets access denied",
			userID:  otherUserID,
			isAdmin: false,
			mockSetup: func(repo *MockApiKeyRepository) {
				repo.On("GetByID", mock.Anything, keyID).Return(&models.ApiKey{ID: keyID, UserID: ownerID}, nil)
			},
			expectedError: ErrApiKeyAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockApiKeyRepo := new(MockApiKeyRepository)
			mockUserRepo := new(MockUserRepository)
			tt.mockSetup(mockApiKeyRepo)

			svc := NewApiKeyService(mockApiKeyRepo, mockUserRepo)
			err := svc.DeleteKey(context.Background(), keyID, tt.userID, tt.isAdmin)

			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			mockApiKeyRepo.AssertExpectations(t)
		})
	}
}

func TestApiKeyService_ListKeys(t *testing.T) {
	userID := uuid.New()

	mockApiKeyRepo := new(MockApiKeyRepository)
	mockUserRepo := new(MockUserRepository)

	keys := []models.ApiKey{
		{ID: uuid.New(), UserID: userID, Name: "Key 1"},
		{ID: uuid.New(), UserID: userID, Name: "Key 2"},
	}
	mockApiKeyRepo.On("ListByUserID", mock.Anything, userID).Return(keys, nil)

	svc := NewApiKeyService(mockApiKeyRepo, mockUserRepo)
	result, err := svc.ListKeys(context.Background(), userID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Key 1", result[0].Name)
	assert.Equal(t, "Key 2", result[1].Name)

	mockApiKeyRepo.AssertExpectations(t)
}
