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

// MockApiKeyService мок для ApiKeyService
type MockApiKeyService struct {
	mock.Mock
}

func (m *MockApiKeyService) CreateKey(ctx context.Context, userID uuid.UUID, name string, scopes string, expiresAt *time.Time) (*models.ApiKey, string, error) {
	args := m.Called(ctx, userID, name, scopes, expiresAt)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).(*models.ApiKey), args.String(1), args.Error(2)
}

func (m *MockApiKeyService) ValidateKey(ctx context.Context, rawKey string) (*models.ApiKey, *models.User, error) {
	args := m.Called(ctx, rawKey)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*models.ApiKey), args.Get(1).(*models.User), args.Error(2)
}

func (m *MockApiKeyService) ListKeys(ctx context.Context, userID uuid.UUID) ([]models.ApiKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ApiKey), args.Error(1)
}

func (m *MockApiKeyService) RevokeKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error {
	args := m.Called(ctx, keyID, requestingUserID, isAdmin)
	return args.Error(0)
}

func (m *MockApiKeyService) DeleteKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error {
	args := m.Called(ctx, keyID, requestingUserID, isAdmin)
	return args.Error(0)
}

// setupAuthenticatedRouter создает роутер с имитацией авторизованного пользователя
func setupAuthenticatedRouter(userID uuid.UUID, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", role)
		c.Next()
	})
	return r
}

func TestApiKeyHandler_Create(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	tests := []struct {
		name             string
		requestBody      dto.CreateApiKeyRequest
		mockSetup        func(*MockApiKeyService)
		expectedStatus   int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "successful creation",
			requestBody: dto.CreateApiKeyRequest{
				Name: "My Key",
			},
			mockSetup: func(svc *MockApiKeyService) {
				key := &models.ApiKey{
					ID:        uuid.New(),
					UserID:    userID,
					Name:      "My Key",
					KeyPrefix: "wibe_abc1234",
					Scopes:    "*",
					CreatedAt: time.Now(),
				}
				svc.On("CreateKey", mock.Anything, userID, "My Key", "", (*time.Time)(nil)).Return(key, "wibe_full_raw_key_here", nil)
			},
			expectedStatus: http.StatusCreated,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response dto.ApiKeyCreatedResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "My Key", response.Name)
				assert.Equal(t, "wibe_full_raw_key_here", response.RawKey)
				assert.Equal(t, "*", response.Scopes)
			},
		},
		{
			name: "missing name returns 400",
			requestBody: dto.CreateApiKeyRequest{
				Name: "",
			},
			mockSetup:      func(svc *MockApiKeyService) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockApiKeyService)
			tt.mockSetup(mockSvc)

			handler := NewApiKeyHandler(mockSvc, nil)
			router := setupAuthenticatedRouter(userID, "user")
			router.POST("/api-keys", handler.Create)

			body, _ := json.Marshal(tt.requestBody)
			req, _ := http.NewRequest("POST", "/api-keys", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestApiKeyHandler_List(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	tests := []struct {
		name             string
		mockSetup        func(*MockApiKeyService)
		expectedStatus   int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "returns list of keys",
			mockSetup: func(svc *MockApiKeyService) {
				keys := []models.ApiKey{
					{ID: uuid.New(), UserID: userID, Name: "Key 1", KeyPrefix: "wibe_abc", Scopes: "*", CreatedAt: time.Now()},
					{ID: uuid.New(), UserID: userID, Name: "Key 2", KeyPrefix: "wibe_def", Scopes: "read", CreatedAt: time.Now()},
				}
				svc.On("ListKeys", mock.Anything, userID).Return(keys, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response []dto.ApiKeyResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Len(t, response, 2)
				assert.Equal(t, "Key 1", response[0].Name)
				assert.Equal(t, "Key 2", response[1].Name)
			},
		},
		{
			name: "returns empty array when no keys",
			mockSetup: func(svc *MockApiKeyService) {
				svc.On("ListKeys", mock.Anything, userID).Return([]models.ApiKey{}, nil)
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response []dto.ApiKeyResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Len(t, response, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockApiKeyService)
			tt.mockSetup(mockSvc)

			handler := NewApiKeyHandler(mockSvc, nil)
			router := setupAuthenticatedRouter(userID, "user")
			router.GET("/api-keys", handler.List)

			req, _ := http.NewRequest("GET", "/api-keys", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestApiKeyHandler_Revoke(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	keyID := uuid.New()

	tests := []struct {
		name           string
		keyIDParam     string
		role           string
		mockSetup      func(*MockApiKeyService)
		expectedStatus int
	}{
		{
			name:       "successful revoke",
			keyIDParam: keyID.String(),
			role:       "user",
			mockSetup: func(svc *MockApiKeyService) {
				svc.On("RevokeKey", mock.Anything, keyID, userID, false).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "key not found",
			keyIDParam: keyID.String(),
			role:       "user",
			mockSetup: func(svc *MockApiKeyService) {
				svc.On("RevokeKey", mock.Anything, keyID, userID, false).Return(service.ErrApiKeyNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "access denied",
			keyIDParam: keyID.String(),
			role:       "user",
			mockSetup: func(svc *MockApiKeyService) {
				svc.On("RevokeKey", mock.Anything, keyID, userID, false).Return(service.ErrApiKeyAccessDenied)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "invalid UUID",
			keyIDParam:     "not-a-uuid",
			role:           "user",
			mockSetup:      func(svc *MockApiKeyService) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockApiKeyService)
			tt.mockSetup(mockSvc)

			handler := NewApiKeyHandler(mockSvc, nil)
			router := setupAuthenticatedRouter(userID, tt.role)
			router.POST("/api-keys/:id/revoke", handler.Revoke)

			req, _ := http.NewRequest("POST", "/api-keys/"+tt.keyIDParam+"/revoke", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockSvc.AssertExpectations(t)
		})
	}
}

func TestApiKeyHandler_Delete(t *testing.T) {
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	keyID := uuid.New()

	tests := []struct {
		name           string
		keyIDParam     string
		mockSetup      func(*MockApiKeyService)
		expectedStatus int
	}{
		{
			name:       "successful delete",
			keyIDParam: keyID.String(),
			mockSetup: func(svc *MockApiKeyService) {
				svc.On("DeleteKey", mock.Anything, keyID, userID, false).Return(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:       "not found",
			keyIDParam: keyID.String(),
			mockSetup: func(svc *MockApiKeyService) {
				svc.On("DeleteKey", mock.Anything, keyID, userID, false).Return(service.ErrApiKeyNotFound)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:       "access denied",
			keyIDParam: keyID.String(),
			mockSetup: func(svc *MockApiKeyService) {
				svc.On("DeleteKey", mock.Anything, keyID, userID, false).Return(service.ErrApiKeyAccessDenied)
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(MockApiKeyService)
			tt.mockSetup(mockSvc)

			handler := NewApiKeyHandler(mockSvc, nil)
			router := setupAuthenticatedRouter(userID, "user")
			router.DELETE("/api-keys/:id", handler.Delete)

			req, _ := http.NewRequest("DELETE", "/api-keys/"+tt.keyIDParam, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			mockSvc.AssertExpectations(t)
		})
	}
}
