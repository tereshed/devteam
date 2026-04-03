package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
)

// --- Mocks ---

type mockApiKeyService struct {
	mock.Mock
}

func (m *mockApiKeyService) CreateKey(ctx context.Context, userID uuid.UUID, name string, scopes string, expiresAt *time.Time) (*models.ApiKey, string, error) {
	args := m.Called(ctx, userID, name, scopes, expiresAt)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).(*models.ApiKey), args.String(1), args.Error(2)
}

func (m *mockApiKeyService) ValidateKey(ctx context.Context, rawKey string) (*models.ApiKey, *models.User, error) {
	args := m.Called(ctx, rawKey)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*models.ApiKey), args.Get(1).(*models.User), args.Error(2)
}

func (m *mockApiKeyService) ListKeys(ctx context.Context, userID uuid.UUID) ([]models.ApiKey, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.ApiKey), args.Error(1)
}

func (m *mockApiKeyService) RevokeKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error {
	args := m.Called(ctx, keyID, requestingUserID, isAdmin)
	return args.Error(0)
}

func (m *mockApiKeyService) DeleteKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error {
	args := m.Called(ctx, keyID, requestingUserID, isAdmin)
	return args.Error(0)
}

// testValidKey — реалистичный API-ключ для тестов: "wibe_" + 64 hex = 69 символов
const testValidKey = "wibe_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

// dummyHandler — возвращает 200, используется как next handler в middleware
var dummyHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

// --- extractApiKey ---

func TestExtractApiKey(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{
			name:     "X-API-Key header",
			headers:  map[string]string{"X-API-Key": "wibe_abc123"},
			expected: "wibe_abc123",
		},
		{
			name:     "Authorization Bearer",
			headers:  map[string]string{"Authorization": "Bearer wibe_abc123"},
			expected: "wibe_abc123",
		},
		{
			name:     "Authorization bearer (lowercase)",
			headers:  map[string]string{"Authorization": "bearer wibe_abc123"},
			expected: "wibe_abc123",
		},
		{
			name:     "X-API-Key takes priority over Bearer",
			headers:  map[string]string{"X-API-Key": "wibe_fromheader", "Authorization": "Bearer wibe_frombearer"},
			expected: "wibe_fromheader",
		},
		{
			name:     "empty headers",
			headers:  map[string]string{},
			expected: "",
		},
		{
			name:     "Authorization without Bearer prefix",
			headers:  map[string]string{"Authorization": "Basic dXNlcjpwYXNz"},
			expected: "",
		},
		{
			name:     "Bearer with spaces trimmed",
			headers:  map[string]string{"Authorization": "Bearer   wibe_abc123  "},
			expected: "wibe_abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			assert.Equal(t, tt.expected, extractApiKey(req))
		})
	}
}

// --- isValidKeyFormat ---

func TestIsValidKeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"valid key", testValidKey, true},
		{"too short", "wibe_abc", false},
		{"minimum length (21)", "wibe_1234567890123456", true},
		{"no prefix", "abcdefghijklmnopqrstuvwxyz", false},
		{"empty", "", false},
		{"too long (129)", "wibe_" + strings.Repeat("a", 124), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isValidKeyFormat(tt.key))
		})
	}
}

// --- NewAuthMiddleware ---

func TestNewAuthMiddleware_NilService(t *testing.T) {
	handler := NewAuthMiddleware(dummyHandler, nil)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assertAuthError(t, w, "INTERNAL_ERROR")
}

func TestNewAuthMiddleware_NoKey(t *testing.T) {
	svc := new(mockApiKeyService)
	handler := NewAuthMiddleware(dummyHandler, svc)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assertAuthError(t, w, "TOKEN_REQUIRED")
	assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Bearer")
}

func TestNewAuthMiddleware_InvalidFormat(t *testing.T) {
	svc := new(mockApiKeyService)
	handler := NewAuthMiddleware(dummyHandler, svc)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-API-Key", "invalid_short")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assertAuthError(t, w, "INVALID_TOKEN")
}

func TestNewAuthMiddleware_KeyNotFound(t *testing.T) {
	svc := new(mockApiKeyService)
	svc.On("ValidateKey", mock.Anything, testValidKey).Return(nil, nil, service.ErrApiKeyNotFound)

	handler := NewAuthMiddleware(dummyHandler, svc)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-API-Key", testValidKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assertAuthError(t, w, "INVALID_TOKEN")
}

func TestNewAuthMiddleware_KeyRevoked(t *testing.T) {
	svc := new(mockApiKeyService)
	svc.On("ValidateKey", mock.Anything, testValidKey).Return(nil, nil, service.ErrApiKeyRevoked)

	handler := NewAuthMiddleware(dummyHandler, svc)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-API-Key", testValidKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assertAuthError(t, w, "INVALID_TOKEN")
}

func TestNewAuthMiddleware_KeyExpired(t *testing.T) {
	svc := new(mockApiKeyService)
	svc.On("ValidateKey", mock.Anything, testValidKey).Return(nil, nil, service.ErrApiKeyExpired)

	handler := NewAuthMiddleware(dummyHandler, svc)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-API-Key", testValidKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assertAuthError(t, w, "TOKEN_EXPIRED")
}

func TestNewAuthMiddleware_InternalError(t *testing.T) {
	svc := new(mockApiKeyService)
	svc.On("ValidateKey", mock.Anything, testValidKey).Return(nil, nil, assert.AnError)

	handler := NewAuthMiddleware(dummyHandler, svc)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-API-Key", testValidKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assertAuthError(t, w, "INTERNAL_ERROR")
}

func TestNewAuthMiddleware_Success(t *testing.T) {
	svc := new(mockApiKeyService)

	userID := uuid.New()
	keyID := uuid.New()
	apiKey := &models.ApiKey{ID: keyID, UserID: userID}
	user := &models.User{ID: userID, Role: models.RoleUser}

	svc.On("ValidateKey", mock.Anything, testValidKey).Return(apiKey, user, nil)

	// Проверяем, что context обогащён правильными данными
	var capturedUserID uuid.UUID
	var capturedRole models.UserRole
	var capturedKeyID uuid.UUID

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID, _ = UserIDFromContext(r.Context())
		capturedRole, _ = UserRoleFromContext(r.Context())
		capturedKeyID, _ = ApiKeyIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := NewAuthMiddleware(nextHandler, svc)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testValidKey)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, userID, capturedUserID)
	assert.Equal(t, models.RoleUser, capturedRole)
	assert.Equal(t, keyID, capturedKeyID)
	svc.AssertExpectations(t)
}

// --- Security headers ---

func TestNewAuthMiddleware_SecurityHeaders(t *testing.T) {
	svc := new(mockApiKeyService)
	handler := NewAuthMiddleware(dummyHandler, svc)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Contains(t, w.Header().Get("WWW-Authenticate"), "Bearer")
}

// --- Context helpers ---

func TestContextHelpers_EmptyContext(t *testing.T) {
	ctx := context.Background()

	_, ok := UserIDFromContext(ctx)
	assert.False(t, ok)

	_, ok = UserRoleFromContext(ctx)
	assert.False(t, ok)

	_, ok = ApiKeyIDFromContext(ctx)
	assert.False(t, ok)
}

// --- Helpers ---

func assertAuthError(t *testing.T, w *httptest.ResponseRecorder, expectedCode string) {
	t.Helper()
	var resp authErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, expectedCode, resp.Error)
	assert.NotEmpty(t, resp.Details)
}
