package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/jwt"
	passwordpkg "github.com/devteam/backend/pkg/password"
)

// MockUserRepository мок для UserRepository
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) Update(ctx context.Context, user *models.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockRefreshTokenRepository мок для RefreshTokenRepository
type MockRefreshTokenRepository struct {
	mock.Mock
}

func (m *MockRefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RefreshToken), args.Error(1)
}

func (m *MockRefreshTokenRepository) Revoke(ctx context.Context, tokenID uuid.UUID) error {
	args := m.Called(ctx, tokenID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRefreshTokenRepository) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockEventBus мок для EventBus
type MockEventBus struct {
	mock.Mock
}

func (m *MockEventBus) Publish(ctx context.Context, ev events.DomainEvent) {
	m.Called(ctx, ev)
}

func (m *MockEventBus) Subscribe(name string, buffer int) (<-chan events.DomainEvent, func()) {
	args := m.Called(name, buffer)
	return args.Get(0).(<-chan events.DomainEvent), args.Get(1).(func())
}

func (m *MockEventBus) Close() {
	m.Called()
}

// Используем реальный JWT Manager для тестов, но можем мокать его методы через интерфейс
// Для упрощения используем реальный объект с тестовым секретом
func createTestJWTManager() *jwt.Manager {
	return jwt.NewManager("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour)
}

func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		password       string
		mockSetup      func(*MockUserRepository, *MockRefreshTokenRepository, interface{})
		expectedError  error
		validateResult func(*testing.T, *models.User)
	}{
		{
			name:     "successful registration",
			email:    "newuser@example.com",
			password: "password123",
			mockSetup: func(userRepo *MockUserRepository, tokenRepo *MockRefreshTokenRepository, jwtMgr interface{}) {
				userRepo.On("GetByEmail", mock.Anything, "newuser@example.com").Return(nil, repository.ErrUserNotFound)
				userRepo.On("Create", mock.Anything, mock.MatchedBy(func(user *models.User) bool {
					return user.Email == "newuser@example.com" && user.PasswordHash != ""
				})).Return(nil)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, user *models.User) {
				assert.NotNil(t, user)
				assert.Equal(t, "newuser@example.com", user.Email)
				assert.Equal(t, models.RoleUser, user.Role)
			},
		},
		{
			name:     "user already exists",
			email:    "existing@example.com",
			password: "password123",
			mockSetup: func(userRepo *MockUserRepository, tokenRepo *MockRefreshTokenRepository, jwtMgr interface{}) {
				existingUser := &models.User{Email: "existing@example.com"}
				userRepo.On("GetByEmail", mock.Anything, "existing@example.com").Return(existingUser, nil)
			},
			expectedError: ErrUserAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(MockUserRepository)
			mockTokenRepo := new(MockRefreshTokenRepository)
			mockBus := new(MockEventBus)
			jwtMgr := createTestJWTManager()
			tt.mockSetup(mockUserRepo, mockTokenRepo, jwtMgr)

			service := NewAuthService(mockUserRepo, mockTokenRepo, jwtMgr, mockBus)
			user, err := service.Register(context.Background(), tt.email, tt.password)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				if tt.validateResult != nil {
					tt.validateResult(t, user)
				}
			}

			mockUserRepo.AssertExpectations(t)
			mockTokenRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		password       string
		mockSetup      func(*MockUserRepository, *MockRefreshTokenRepository, interface{})
		expectedError  error
		validateTokens func(*testing.T, string, string)
	}{
		{
			name:     "successful login",
			email:    "user@example.com",
			password: "correctpassword",
			mockSetup: func(userRepo *MockUserRepository, tokenRepo *MockRefreshTokenRepository, jwtMgr interface{}) {
				// Генерируем правильный хеш для пароля
				passwordHash, _ := passwordpkg.Hash("correctpassword")
				user := &models.User{
					ID:           uuid.New(),
					Email:        "user@example.com",
					PasswordHash: passwordHash,
					Role:         models.RoleUser,
				}
				userRepo.On("GetByEmail", mock.Anything, "user@example.com").Return(user, nil)
				tokenRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: nil,
			validateTokens: func(t *testing.T, accessToken, refreshToken string) {
				assert.NotEmpty(t, accessToken)
				assert.NotEmpty(t, refreshToken)
			},
		},
		{
			name:     "user not found",
			email:    "notfound@example.com",
			password: "password",
			mockSetup: func(userRepo *MockUserRepository, tokenRepo *MockRefreshTokenRepository, jwtMgr interface{}) {
				userRepo.On("GetByEmail", mock.Anything, "notfound@example.com").Return(nil, repository.ErrUserNotFound)
			},
			expectedError: ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserRepo := new(MockUserRepository)
			mockTokenRepo := new(MockRefreshTokenRepository)
			mockBus := new(MockEventBus)
			jwtMgr := createTestJWTManager()
			tt.mockSetup(mockUserRepo, mockTokenRepo, jwtMgr)

			service := NewAuthService(mockUserRepo, mockTokenRepo, jwtMgr, mockBus)
			user, accessToken, refreshToken, err := service.Login(context.Background(), tt.email, tt.password)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
				assert.Nil(t, user)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, user)
				if tt.validateTokens != nil {
					tt.validateTokens(t, accessToken, refreshToken)
				}
			}

			mockUserRepo.AssertExpectations(t)
			mockTokenRepo.AssertExpectations(t)
		})
	}
}

func TestAuthService_RefreshToken(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockTokenRepo := new(MockRefreshTokenRepository)
	mockBus := new(MockEventBus)
	jwtMgr := createTestJWTManager()
	service := NewAuthService(mockUserRepo, mockTokenRepo, jwtMgr, mockBus)
	ctx := context.Background()

	t.Run("successful refresh", func(t *testing.T) {
		refreshToken, _ := jwtMgr.GenerateRefreshToken()
		tokenHash := hashToken(refreshToken)
		userID := uuid.New()
		tokenModel := &models.RefreshToken{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		user := &models.User{ID: userID, Role: models.RoleUser}

		mockTokenRepo.On("GetByTokenHash", ctx, tokenHash).Return(tokenModel, nil)
		mockUserRepo.On("GetByID", ctx, userID).Return(user, nil)
		mockTokenRepo.On("Revoke", ctx, tokenModel.ID).Return(nil)
		mockTokenRepo.On("Create", ctx, mock.Anything).Return(nil)

		accessToken, newRefreshToken, err := service.RefreshToken(ctx, refreshToken)

		assert.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, newRefreshToken)
		mockTokenRepo.AssertExpectations(t)
		mockUserRepo.AssertExpectations(t)
	})
}

func TestAuthService_Logout(t *testing.T) {
	mockTokenRepo := new(MockRefreshTokenRepository)
	mockBus := new(MockEventBus)
	service := NewAuthService(nil, mockTokenRepo, nil, mockBus)
	ctx := context.Background()
	userID := uuid.New()

	mockTokenRepo.On("RevokeAllForUser", ctx, userID).Return(nil)

	err := service.Logout(ctx, userID)

	assert.NoError(t, err)
	mockTokenRepo.AssertExpectations(t)
}

func TestAuthService_DeleteUser(t *testing.T) {
	mockUserRepo := new(MockUserRepository)
	mockBus := new(MockEventBus)
	service := NewAuthService(mockUserRepo, nil, nil, mockBus)
	ctx := context.Background()
	userID := uuid.New()

	user := &models.User{ID: userID}
	mockUserRepo.On("GetByID", ctx, userID).Return(user, nil)
	mockUserRepo.On("Delete", ctx, userID).Return(nil)
	mockBus.On("Publish", mock.Anything, mock.MatchedBy(func(ev events.UserDeleted) bool {
		return ev.UserID == userID
	})).Return()

	err := service.DeleteUser(ctx, userID)

	assert.NoError(t, err)
	mockUserRepo.AssertExpectations(t)
	mockBus.AssertExpectations(t)
}
