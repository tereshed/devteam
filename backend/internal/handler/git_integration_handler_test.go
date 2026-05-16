package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubGitIntegrationSvc — конфигурируемая реализация GitIntegrationService для handler-тестов.
type stubGitIntegrationSvc struct {
	initRes        *service.GitIntegrationInitResult
	initErr        error
	callbackRes    *service.GitIntegrationCallbackResult
	callbackErr    error
	revokeFailed   bool
	revokeErr      error
	statusRes      *service.GitIntegrationStatus
	statusErr      error
	listRes        []service.GitIntegrationStatus
	listErr        error
}

func (s *stubGitIntegrationSvc) InitGitHub(context.Context, uuid.UUID, string) (*service.GitIntegrationInitResult, error) {
	return s.initRes, s.initErr
}
func (s *stubGitIntegrationSvc) InitGitLabShared(context.Context, uuid.UUID, string) (*service.GitIntegrationInitResult, error) {
	return s.initRes, s.initErr
}
func (s *stubGitIntegrationSvc) InitGitLabBYO(context.Context, uuid.UUID, string, service.BYOGitLabInit) (*service.GitIntegrationInitResult, error) {
	return s.initRes, s.initErr
}
func (s *stubGitIntegrationSvc) HandleCallback(context.Context, string, string, string) (*service.GitIntegrationCallbackResult, error) {
	return s.callbackRes, s.callbackErr
}
func (s *stubGitIntegrationSvc) Revoke(context.Context, uuid.UUID, models.GitIntegrationProvider) (bool, error) {
	return s.revokeFailed, s.revokeErr
}
func (s *stubGitIntegrationSvc) Status(context.Context, uuid.UUID, models.GitIntegrationProvider) (*service.GitIntegrationStatus, error) {
	return s.statusRes, s.statusErr
}
func (s *stubGitIntegrationSvc) ListStatuses(context.Context, uuid.UUID) ([]service.GitIntegrationStatus, error) {
	return s.listRes, s.listErr
}

func setupGitRouter(t *testing.T, svc service.GitIntegrationService) (*gin.Engine, *bytes.Buffer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uuid.New())
		c.Set("userRole", "user")
		c.Next()
	})
	logBuf := &bytes.Buffer{}
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	h := WithGitIntegrationLogger(NewGitIntegrationHandler(svc), logger)
	r.POST("/integrations/github/auth/init", h.InitGitHub)
	r.POST("/integrations/github/auth/callback", h.CallbackGitHub)
	r.GET("/integrations/github/auth/status", h.StatusGitHub)
	r.DELETE("/integrations/github/auth/revoke", h.RevokeGitHub)
	r.POST("/integrations/gitlab/auth/init", h.InitGitLab)
	return r, logBuf
}

func TestGitIntegrationHandler_InitGitHub_OK(t *testing.T) {
	svc := &stubGitIntegrationSvc{initRes: &service.GitIntegrationInitResult{AuthorizeURL: "https://gh/auth", State: "s123"}}
	r, _ := setupGitRouter(t, svc)

	body, _ := json.Marshal(dto.GitIntegrationInitRequest{RedirectURI: "https://app/cb"})
	req := httptest.NewRequest(http.MethodPost, "/integrations/github/auth/init", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	var out dto.GitIntegrationInitResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.Equal(t, "https://gh/auth", out.AuthorizeURL)
}

func TestGitIntegrationHandler_InitGitHub_BadRequest(t *testing.T) {
	r, _ := setupGitRouter(t, &stubGitIntegrationSvc{})
	req := httptest.NewRequest(http.MethodPost, "/integrations/github/auth/init", bytes.NewReader([]byte("{")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGitIntegrationHandler_Callback_UserCancelled(t *testing.T) {
	svc := &stubGitIntegrationSvc{callbackErr: service.ErrGitOAuthUserCancelled}
	r, _ := setupGitRouter(t, svc)
	body, _ := json.Marshal(dto.GitIntegrationCallbackRequest{State: "s", Error: "access_denied"})
	req := httptest.NewRequest(http.MethodPost, "/integrations/github/auth/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusGone, rr.Code)
	assert.Contains(t, rr.Body.String(), "user_cancelled")
}

func TestGitIntegrationHandler_Callback_StateExpired(t *testing.T) {
	svc := &stubGitIntegrationSvc{callbackErr: service.ErrGitOAuthStateNotFound}
	r, _ := setupGitRouter(t, svc)
	body, _ := json.Marshal(dto.GitIntegrationCallbackRequest{State: "s", Code: "c"})
	req := httptest.NewRequest(http.MethodPost, "/integrations/github/auth/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusGone, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid_state")
}

func TestGitIntegrationHandler_Callback_ProviderUnreachable(t *testing.T) {
	svc := &stubGitIntegrationSvc{callbackErr: service.ErrGitOAuthProviderUnreachable}
	r, _ := setupGitRouter(t, svc)
	body, _ := json.Marshal(dto.GitIntegrationCallbackRequest{State: "s", Code: "c"})
	req := httptest.NewRequest(http.MethodPost, "/integrations/github/auth/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadGateway, rr.Code)
	assert.Contains(t, rr.Body.String(), "provider_unreachable")
}

func TestGitIntegrationHandler_InitGitLab_BYO_InvalidHost(t *testing.T) {
	svc := &stubGitIntegrationSvc{initErr: service.ErrPrivateGitProviderHost}
	r, _ := setupGitRouter(t, svc)
	body, _ := json.Marshal(dto.GitIntegrationInitRequest{
		RedirectURI:     "https://app/cb",
		Host:            "https://192.168.1.1",
		ByoClientID:     "cid",
		ByoClientSecret: "csec",
	})
	req := httptest.NewRequest(http.MethodPost, "/integrations/gitlab/auth/init", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid_host")
}

func TestGitIntegrationHandler_Revoke_OK_WithRemoteFailed(t *testing.T) {
	svc := &stubGitIntegrationSvc{revokeFailed: true}
	r, _ := setupGitRouter(t, svc)
	req := httptest.NewRequest(http.MethodDelete, "/integrations/github/auth/revoke", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var out dto.GitIntegrationRevokeResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.True(t, out.RemoteRevokeFailed)
	assert.Equal(t, "github", out.Provider)
}

func TestGitIntegrationHandler_Status_NotConnected(t *testing.T) {
	svc := &stubGitIntegrationSvc{statusRes: &service.GitIntegrationStatus{Provider: models.GitIntegrationProviderGitHub, Connected: false}}
	r, _ := setupGitRouter(t, svc)
	req := httptest.NewRequest(http.MethodGet, "/integrations/github/auth/status", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var out dto.GitIntegrationStatusResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	assert.False(t, out.Connected)
}

func TestGitIntegrationHandler_InitGitHub_InternalError(t *testing.T) {
	svc := &stubGitIntegrationSvc{initErr: errors.New("boom")}
	r, _ := setupGitRouter(t, svc)
	body, _ := json.Marshal(dto.GitIntegrationInitRequest{RedirectURI: "https://app/cb"})
	req := httptest.NewRequest(http.MethodPost, "/integrations/github/auth/init", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}
