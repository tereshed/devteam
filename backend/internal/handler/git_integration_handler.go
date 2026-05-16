package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
)

// GitIntegrationHandler — HTTP handler для OAuth-интеграций GitHub / GitLab / BYO GitLab.
//
// UI Refactoring §4a.5 — error-cases:
//  1. user_cancelled (access_denied)   -> 410, error_code=user_cancelled
//  2. invalid_state / state_expired    -> 410, error_code=invalid_state
//  3. provider_unreachable (network)   -> 502, error_code=provider_unreachable
//  4. invalid_host (BYO validation)    -> 400, error_code=invalid_host
//  5. oauth_not_configured             -> 503, error_code=oauth_not_configured
//
// Все логи — через logging.Handler. Тело ошибки никогда не содержит err.Error()
// от провайдера (может включать access_token=... в случае «болтливых» ответов).
type GitIntegrationHandler struct {
	svc service.GitIntegrationService
	log *slog.Logger
}

// NewGitIntegrationHandler — конструктор. По умолчанию использует NopLogger (redact-обёрнутый).
func NewGitIntegrationHandler(svc service.GitIntegrationService) *GitIntegrationHandler {
	return &GitIntegrationHandler{svc: svc, log: logging.NopLogger()}
}

// WithGitIntegrationLogger подменяет logger (DI в main / тестах).
func WithGitIntegrationLogger(h *GitIntegrationHandler, log *slog.Logger) *GitIntegrationHandler {
	if log != nil {
		h.log = log
	}
	return h
}

// InitGitHub — POST /integrations/github/auth/init.
//
// @Summary Старт GitHub OAuth
// @Description Возвращает URL, куда фронт открывает попап/таб для авторизации.
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.GitIntegrationInitRequest true "redirect_uri"
// @Success 200 {object} dto.GitIntegrationInitResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 503 {object} apierror.ErrorResponse
// @Router /integrations/github/auth/init [post]
func (h *GitIntegrationHandler) InitGitHub(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	var req dto.GitIntegrationInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	out, err := h.svc.InitGitHub(c.Request.Context(), uid, req.RedirectURI)
	if err != nil {
		h.mapErr(c, "init", err)
		return
	}
	c.JSON(http.StatusOK, dto.GitIntegrationInitResponse{AuthorizeURL: out.AuthorizeURL, State: out.State})
}

// InitGitLab — POST /integrations/gitlab/auth/init.
//
// @Summary Старт GitLab OAuth (gitlab.com shared или self-hosted BYO)
// @Description Если в теле передан host+byo_client_id+byo_client_secret — self-hosted; иначе gitlab.com.
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.GitIntegrationInitRequest true "redirect_uri (+ host/byo_* для self-hosted)"
// @Success 200 {object} dto.GitIntegrationInitResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 503 {object} apierror.ErrorResponse
// @Router /integrations/gitlab/auth/init [post]
func (h *GitIntegrationHandler) InitGitLab(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	var req dto.GitIntegrationInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	var (
		out *service.GitIntegrationInitResult
		err error
	)
	if req.Host != "" || req.ByoClientID != "" || req.ByoClientSecret != "" {
		out, err = h.svc.InitGitLabBYO(c.Request.Context(), uid, req.RedirectURI, service.BYOGitLabInit{
			Host: req.Host, ClientID: req.ByoClientID, ClientSecret: req.ByoClientSecret,
		})
	} else {
		out, err = h.svc.InitGitLabShared(c.Request.Context(), uid, req.RedirectURI)
	}
	if err != nil {
		h.mapErr(c, "init", err)
		return
	}
	c.JSON(http.StatusOK, dto.GitIntegrationInitResponse{AuthorizeURL: out.AuthorizeURL, State: out.State})
}

// CallbackGitHub — POST /integrations/github/auth/callback.
//
// @Summary Завершение GitHub OAuth (code + state от провайдера)
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.GitIntegrationCallbackRequest true "code + state"
// @Success 200 {object} dto.GitIntegrationCallbackResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 410 {object} apierror.ErrorResponse "user_cancelled / invalid_state"
// @Failure 502 {object} apierror.ErrorResponse "provider_unreachable"
// @Router /integrations/github/auth/callback [post]
func (h *GitIntegrationHandler) CallbackGitHub(c *gin.Context) {
	h.handleCallback(c, models.GitIntegrationProviderGitHub)
}

// CallbackGitLab — POST /integrations/gitlab/auth/callback.
//
// @Summary Завершение GitLab OAuth (code + state от провайдера)
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.GitIntegrationCallbackRequest true "code + state"
// @Success 200 {object} dto.GitIntegrationCallbackResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 410 {object} apierror.ErrorResponse "user_cancelled / invalid_state"
// @Failure 502 {object} apierror.ErrorResponse "provider_unreachable"
// @Router /integrations/gitlab/auth/callback [post]
func (h *GitIntegrationHandler) CallbackGitLab(c *gin.Context) {
	h.handleCallback(c, models.GitIntegrationProviderGitLab)
}

func (h *GitIntegrationHandler) handleCallback(c *gin.Context, expected models.GitIntegrationProvider) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	var req dto.GitIntegrationCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	res, err := h.svc.HandleCallback(c.Request.Context(), req.Code, req.State, req.Error)
	if err != nil {
		h.log.Warn("git_integration callback failed",
			"user_id", uid.String(),
			"provider", string(expected),
			"reason", classifyGitErr(err),
			"error_summary", logging.SafeRawAttr([]byte(err.Error())),
		)
		h.mapErr(c, "callback", err)
		return
	}
	if res.Provider != expected {
		// state принадлежал другому провайдеру.
		apierror.JSON(c, http.StatusBadRequest, "invalid_state", "State does not match provider")
		return
	}
	c.JSON(http.StatusOK, dto.GitIntegrationCallbackResponse{
		Provider: string(res.Provider),
		Status:   gitStatusToDTO(res.Status),
	})
}

// StatusGitHub — GET /integrations/github/auth/status.
//
// @Summary Статус подключения GitHub
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.GitIntegrationStatusResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /integrations/github/auth/status [get]
func (h *GitIntegrationHandler) StatusGitHub(c *gin.Context) {
	h.handleStatus(c, models.GitIntegrationProviderGitHub)
}

// StatusGitLab — GET /integrations/gitlab/auth/status.
//
// @Summary Статус подключения GitLab
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.GitIntegrationStatusResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /integrations/gitlab/auth/status [get]
func (h *GitIntegrationHandler) StatusGitLab(c *gin.Context) {
	h.handleStatus(c, models.GitIntegrationProviderGitLab)
}

func (h *GitIntegrationHandler) handleStatus(c *gin.Context, provider models.GitIntegrationProvider) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	st, err := h.svc.Status(c.Request.Context(), uid, provider)
	if err != nil {
		h.mapErr(c, "status", err)
		return
	}
	c.JSON(http.StatusOK, gitStatusToDTO(*st))
}

// RevokeGitHub — DELETE /integrations/github/auth/revoke.
//
// @Summary Отозвать подключение GitHub
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.GitIntegrationRevokeResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /integrations/github/auth/revoke [delete]
func (h *GitIntegrationHandler) RevokeGitHub(c *gin.Context) {
	h.handleRevoke(c, models.GitIntegrationProviderGitHub)
}

// RevokeGitLab — DELETE /integrations/gitlab/auth/revoke.
//
// @Summary Отозвать подключение GitLab
// @Tags git-integrations
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.GitIntegrationRevokeResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /integrations/gitlab/auth/revoke [delete]
func (h *GitIntegrationHandler) RevokeGitLab(c *gin.Context) {
	h.handleRevoke(c, models.GitIntegrationProviderGitLab)
}

func (h *GitIntegrationHandler) handleRevoke(c *gin.Context, provider models.GitIntegrationProvider) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	remoteFailed, err := h.svc.Revoke(c.Request.Context(), uid, provider)
	if err != nil {
		h.mapErr(c, "revoke", err)
		return
	}
	c.JSON(http.StatusOK, dto.GitIntegrationRevokeResponse{
		Provider:           string(provider),
		RemoteRevokeFailed: remoteFailed,
	})
}

// mapErr — общий маппер service-ошибок в HTTP-коды (§4a.5).
func (h *GitIntegrationHandler) mapErr(c *gin.Context, phase string, err error) {
	switch {
	case errors.Is(err, service.ErrGitOAuthUserCancelled):
		apierror.JSON(c, http.StatusGone, "user_cancelled", "User cancelled authorization")
	case errors.Is(err, service.ErrGitOAuthInvalidGrant):
		apierror.JSON(c, http.StatusBadRequest, "invalid_state", "Invalid OAuth grant")
	case errors.Is(err, service.ErrGitOAuthStateNotFound):
		apierror.JSON(c, http.StatusGone, "invalid_state", "OAuth state expired or already used")
	case errors.Is(err, service.ErrGitOAuthProviderUnreachable):
		apierror.JSON(c, http.StatusBadGateway, "provider_unreachable", "Git provider returned an unexpected error")
	case errors.Is(err, service.ErrGitOAuthNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, "oauth_not_configured", "OAuth provider is not configured")
	case errors.Is(err, service.ErrInvalidGitProviderHost),
		errors.Is(err, service.ErrPrivateGitProviderHost),
		errors.Is(err, service.ErrGitProviderResolveFailed):
		apierror.JSON(c, http.StatusBadRequest, "invalid_host", "Provided git host is not allowed")
	default:
		h.log.Error("git_integration handler internal error",
			"phase", phase,
			"error_summary", logging.SafeRawAttr([]byte(err.Error())),
		)
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Internal error")
	}
}

func classifyGitErr(err error) string {
	switch {
	case errors.Is(err, service.ErrGitOAuthUserCancelled):
		return "user_cancelled"
	case errors.Is(err, service.ErrGitOAuthInvalidGrant):
		return "invalid_grant"
	case errors.Is(err, service.ErrGitOAuthStateNotFound):
		return "state_expired"
	case errors.Is(err, service.ErrGitOAuthProviderUnreachable):
		return "provider_unreachable"
	case errors.Is(err, service.ErrGitOAuthNotConfigured):
		return "oauth_not_configured"
	case errors.Is(err, service.ErrInvalidGitProviderHost),
		errors.Is(err, service.ErrPrivateGitProviderHost),
		errors.Is(err, service.ErrGitProviderResolveFailed):
		return "invalid_host"
	default:
		return "internal_error"
	}
}

func gitStatusToDTO(s service.GitIntegrationStatus) dto.GitIntegrationStatusResponse {
	return dto.GitIntegrationStatusResponse{
		Provider:     string(s.Provider),
		Connected:    s.Connected,
		Host:         s.Host,
		AccountLogin: s.AccountLogin,
		Scopes:       s.Scopes,
		ExpiresAt:    s.ExpiresAt,
		ConnectedAt:  s.ConnectedAt,
	}
}
