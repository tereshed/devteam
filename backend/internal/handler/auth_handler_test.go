package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
)

// MockAuthService мок для AuthService
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Register(ctx context.Context, email, password string) (*models.User, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (*models.User, string, string, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, "", "", args.Error(3)
	}
	return args.Get(0).(*models.User), args.String(1), args.String(2), args.Error(3)
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	args := m.Called(ctx, refreshToken)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockAuthService) Logout(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockAuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// MockJWTManager мок для JWTManager
type MockJWTManager struct {
	mock.Mock
}

func (m *MockJWTManager) GetAccessTokenTTL() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestAuthHandler_Register(t *testing.T) {
	tests := []struct {
		name             string
		requestBody      dto.RegisterRequest
		mockSetup        func(*MockAuthService, *MockJWTManager)
		expectedStatus   int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful registration",
			requestBody: dto.RegisterRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockSetup: func(authSvc *MockAuthService, jwtMgr *MockJWTManager) {
				user := &models.User{
					ID:    uuid.New(),
					Email: "test@example.com",
					Role:  models.RoleUser,
				}
				authSvc.On("Register", mock.Anything, "test@example.com", "password123").Return(user, nil)
				authSvc.On("Login", mock.Anything, "test@example.com", "password123").Return(user, "access_token", "refresh_token", nil)
				jwtMgr.On("GetAccessTokenTTL").Return(15 * time.Minute)
			},
			expectedStatus: http.StatusCreated,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response dto.AuthResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "access_token", response.AccessToken)
				assert.Equal(t, "refresh_token", response.RefreshToken)
				assert.Equal(t, "Bearer", response.TokenType)
			},
		},
		{
			name: "user already exists",
			requestBody: dto.RegisterRequest{
				Email:    "existing@example.com",
				Password: "password123",
			},
			mockSetup: func(authSvc *MockAuthService, jwtMgr *MockJWTManager) {
				authSvc.On("Register", mock.Anything, "existing@example.com", "password123").Return(nil, service.ErrUserAlreadyExists)
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name: "invalid request body",
			requestBody: dto.RegisterRequest{
				Email:    "invalid-email",
				Password: "123",
			},
			mockSetup:      func(authSvc *MockAuthService, jwtMgr *MockJWTManager) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockAuthService := new(MockAuthService)
			mockJWTManager := new(MockJWTManager)
			tt.mockSetup(mockAuthService, mockJWTManager)

			handler := &AuthHandler{
				authService: mockAuthService,
				jwtManager:  mockJWTManager,
			}

			router := setupRouter()
			router.POST("/register", handler.Register)

			// Request
			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
			mockAuthService.AssertExpectations(t)
			mockJWTManager.AssertExpectations(t)
		})
	}
}

func TestAuthHandler_Login(t *testing.T) {
	tests := []struct {
		name             string
		requestBody      dto.LoginRequest
		mockSetup        func(*MockAuthService, *MockJWTManager)
		expectedStatus   int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful login",
			requestBody: dto.LoginRequest{
				Email:    "test@example.com",
				Password: "password123",
			},
			mockSetup: func(authSvc *MockAuthService, jwtMgr *MockJWTManager) {
				user := &models.User{
					ID:    uuid.New(),
					Email: "test@example.com",
					Role:  models.RoleUser,
				}
				authSvc.On("Login", mock.Anything, "test@example.com", "password123").Return(user, "access_token", "refresh_token", nil)
				jwtMgr.On("GetAccessTokenTTL").Return(15 * time.Minute)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response dto.AuthResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "access_token", response.AccessToken)
				assert.Equal(t, "refresh_token", response.RefreshToken)
			},
		},
		{
			name: "invalid credentials",
			requestBody: dto.LoginRequest{
				Email:    "test@example.com",
				Password: "wrongpassword",
			},
			mockSetup: func(authSvc *MockAuthService, jwtMgr *MockJWTManager) {
				authSvc.On("Login", mock.Anything, "test@example.com", "wrongpassword").Return(nil, "", "", service.ErrInvalidCredentials)
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuthService := new(MockAuthService)
			mockJWTManager := new(MockJWTManager)
			tt.mockSetup(mockAuthService, mockJWTManager)

			handler := &AuthHandler{
				authService: mockAuthService,
				jwtManager:  mockJWTManager,
			}

			router := setupRouter()
			router.POST("/login", handler.Login)

			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
			mockAuthService.AssertExpectations(t)
			mockJWTManager.AssertExpectations(t)
		})
	}
}

func TestAuthHandler_Refresh(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    dto.RefreshTokenRequest
		mockSetup      func(*MockAuthService, *MockJWTManager)
		expectedStatus int
	}{
		{
			name: "successful refresh",
			requestBody: dto.RefreshTokenRequest{
				RefreshToken: "valid_refresh_token",
			},
			mockSetup: func(authSvc *MockAuthService, jwtMgr *MockJWTManager) {
				authSvc.On("RefreshToken", mock.Anything, "valid_refresh_token").Return("new_access_token", "new_refresh_token", nil)
				jwtMgr.On("GetAccessTokenTTL").Return(15 * time.Minute)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "invalid refresh token",
			requestBody: dto.RefreshTokenRequest{
				RefreshToken: "invalid_token",
			},
			mockSetup: func(authSvc *MockAuthService, jwtMgr *MockJWTManager) {
				authSvc.On("RefreshToken", mock.Anything, "invalid_token").Return("", "", service.ErrInvalidRefreshToken)
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuthService := new(MockAuthService)
			mockJWTManager := new(MockJWTManager)
			tt.mockSetup(mockAuthService, mockJWTManager)

			handler := &AuthHandler{
				authService: mockAuthService,
				jwtManager:  mockJWTManager,
			}

			router := setupRouter()
			router.POST("/refresh", handler.Refresh)

			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/refresh", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockAuthService.AssertExpectations(t)
			mockJWTManager.AssertExpectations(t)
		})
	}
}

func TestAuthHandler_Me(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setupRouter := func() *gin.Engine {
		router := gin.New()
		router.Use(func(c *gin.Context) {
			// Симулируем middleware, который устанавливает userID
			c.Set("userID", uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"))
			c.Next()
		})
		return router
	}

	tests := []struct {
		name           string
		mockSetup      func(*MockAuthService)
		expectedStatus int
		expectedBody   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful get current user",
			mockSetup: func(authSvc *MockAuthService) {
				userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
				user := &models.User{
					ID:            userID,
					Email:         "test@example.com",
					Role:          models.RoleUser,
					EmailVerified: false,
				}
				authSvc.On("GetCurrentUser", mock.Anything, userID).Return(user, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response dto.UserResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "test@example.com", response.Email)
				assert.Equal(t, "user", response.Role)
				assert.False(t, response.EmailVerified)
			},
		},
		{
			name: "user not found",
			mockSetup: func(authSvc *MockAuthService) {
				userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
				authSvc.On("GetCurrentUser", mock.Anything, userID).Return(nil, service.ErrInvalidCredentials)
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAuthService := new(MockAuthService)
			mockJWTManager := new(MockJWTManager)
			tt.mockSetup(mockAuthService)

			handler := &AuthHandler{
				authService: mockAuthService,
				jwtManager:  mockJWTManager,
			}

			router := setupRouter()
			router.GET("/me", handler.Me)

			req, _ := http.NewRequest("GET", "/me", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != nil {
				tt.expectedBody(t, w)
			}
			mockAuthService.AssertExpectations(t)
		})
	}
}
