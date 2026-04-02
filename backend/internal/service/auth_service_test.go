package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/repository"
	"github.com/wibe-flutter-gin-template/backend/pkg/jwt"
	passwordpkg "github.com/wibe-flutter-gin-template/backend/pkg/password"
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
			jwtMgr := createTestJWTManager()
			tt.mockSetup(mockUserRepo, mockTokenRepo, jwtMgr)

			service := NewAuthService(mockUserRepo, mockTokenRepo, jwtMgr)
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
			jwtMgr := createTestJWTManager()
			tt.mockSetup(mockUserRepo, mockTokenRepo, jwtMgr)

			service := NewAuthService(mockUserRepo, mockTokenRepo, jwtMgr)
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
