package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// claudeCodeAuthSvcStub — реализация service.ClaudeCodeAuthService для handler-тестов §4a.5.
// Каждое поле задаёт ответ соответствующего метода; необязательные оставлены nil.
type claudeCodeAuthSvcStub struct {
	completeErr error
}

func (s *claudeCodeAuthSvcStub) InitDeviceCode(_ context.Context, _ uuid.UUID) (*service.ClaudeCodeDeviceInit, error) {
	return &service.ClaudeCodeDeviceInit{
		DeviceCode: "dc", UserCode: "X", VerificationURI: "https://example",
		Interval: time.Second, ExpiresIn: time.Hour,
	}, nil
}

func (s *claudeCodeAuthSvcStub) CompleteDeviceCode(_ context.Context, _ uuid.UUID, _ string) (*service.ClaudeCodeAuthStatus, error) {
	if s.completeErr != nil {
		return nil, s.completeErr
	}
	return &service.ClaudeCodeAuthStatus{Connected: true, TokenType: "Bearer"}, nil
}

func (s *claudeCodeAuthSvcStub) Status(_ context.Context, _ uuid.UUID) (*service.ClaudeCodeAuthStatus, error) {
	return &service.ClaudeCodeAuthStatus{Connected: false}, nil
}

func (s *claudeCodeAuthSvcStub) Revoke(_ context.Context, _ uuid.UUID) error { return nil }

func (s *claudeCodeAuthSvcStub) AccessTokenForSandbox(_ context.Context, _ uuid.UUID) (string, error) {
	return "", nil
}

func (s *claudeCodeAuthSvcStub) RefreshOne(_ context.Context, _ *models.ClaudeCodeSubscription) error {
	return nil
}

func callbackRequestBody(t *testing.T, deviceCode string) []byte {
	t.Helper()
	body, err := json.Marshal(dto.ClaudeCodeAuthCallbackRequest{DeviceCode: deviceCode})
	require.NoError(t, err)
	return body
}

// setupCallbackRouter возвращает gin-router + buffer с захваченными логами через redact-handler.
func setupCallbackRouter(t *testing.T, svc service.ClaudeCodeAuthService) (*gin.Engine, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	uid := uuid.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uid)
		c.Set("userRole", "user")
		c.Next()
	})
	logBuf := &bytes.Buffer{}
	logger := slog.New(logging.NewHandler(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	h := WithClaudeCodeAuthLogger(NewClaudeCodeAuthHandler(svc), logger)
	r.POST("/callback", h.Callback)
	return r, logBuf
}

func performCallback(t *testing.T, r *gin.Engine) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/callback",
		bytes.NewReader(callbackRequestBody(t, "dc")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

// §4a.5 case 1: cancel (?error=access_denied) → 410 + error_code=access_denied.
func TestClaudeCodeAuthHandler_Callback_AccessDenied(t *testing.T) {
	r, _ := setupCallbackRouter(t, &claudeCodeAuthSvcStub{completeErr: service.ErrAccessDenied})
	rr := performCallback(t, r)

	assert.Equal(t, http.StatusGone, rr.Code)
	assert.Contains(t, rr.Body.String(), `"access_denied"`)
}

// §4a.5 case 3a: invalid_state (expired device_code).
func TestClaudeCodeAuthHandler_Callback_InvalidState_Expired(t *testing.T) {
	r, _ := setupCallbackRouter(t, &claudeCodeAuthSvcStub{completeErr: service.ErrExpiredToken})
	rr := performCallback(t, r)

	assert.Equal(t, http.StatusGone, rr.Code)
	assert.Contains(t, rr.Body.String(), `"invalid_state"`)
}

// §4a.5 case 3b: invalid_grant также маппится в invalid_state.
func TestClaudeCodeAuthHandler_Callback_InvalidState_InvalidGrant(t *testing.T) {
	r, _ := setupCallbackRouter(t, &claudeCodeAuthSvcStub{completeErr: service.ErrOAuthInvalidGrant})
	rr := performCallback(t, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), `"invalid_state"`)
}

// §4a.5 case 4: provider_unreachable (любая нестандартная ошибка от провайдера).
func TestClaudeCodeAuthHandler_Callback_ProviderUnreachable(t *testing.T) {
	r, _ := setupCallbackRouter(t, &claudeCodeAuthSvcStub{completeErr: errors.New("dial tcp: i/o timeout")})
	rr := performCallback(t, r)

	assert.Equal(t, http.StatusBadGateway, rr.Code)
	assert.Contains(t, rr.Body.String(), `"provider_unreachable"`)
}

// §4a.1 security: error.Error() от провайдера может содержать access_token.
// В логах ни в каком виде секрет не должен присутствовать.
func TestClaudeCodeAuthHandler_Callback_LogsDoNotLeakSecrets(t *testing.T) {
	const secret = "sk-ant-XXXX-LEAK-CANARY-XXXX"
	leakErr := errors.New("oauth error: server_error body=access_token=" +
		secret + "&token_type=Bearer&client_secret=AAA")
	r, logBuf := setupCallbackRouter(t, &claudeCodeAuthSvcStub{completeErr: leakErr})

	rr := performCallback(t, r)
	assert.Equal(t, http.StatusBadGateway, rr.Code)

	// Тело ответа клиенту: общая formula, без секрета и без "access_token=...".
	assert.NotContains(t, rr.Body.String(), secret)
	assert.NotContains(t, rr.Body.String(), "access_token")

	logs := logBuf.String()
	assert.NotEmpty(t, logs, "ожидался warning-лог callback'а")
	assert.False(t, strings.Contains(logs, secret),
		"raw secret утёк в лог: %s", logs)
	assert.False(t, strings.Contains(logs, "client_secret=AAA"),
		"client_secret утёк в лог: %s", logs)
	// Структурное поле error_kind=internal_error (default-ветка classifyClaudeCodeAuthErr).
	assert.Contains(t, logs, "error_kind")
	assert.Contains(t, logs, "internal_error")
}

// Pending/slow_down — промежуточные poll-состояния, warning-логов быть не должно.
func TestClaudeCodeAuthHandler_Callback_PendingDoesNotLog(t *testing.T) {
	r, logBuf := setupCallbackRouter(t, &claudeCodeAuthSvcStub{completeErr: service.ErrAuthorizationPending})
	rr := performCallback(t, r)

	assert.Equal(t, http.StatusAccepted, rr.Code)
	assert.NotContains(t, logBuf.String(), "handler returned error")
}
