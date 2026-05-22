package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
)

// AntigravityAuthHandler — OAuth-подписка Antigravity.
type AntigravityAuthHandler struct {
	svc service.AntigravityAuthService
	log *slog.Logger
}

// NewAntigravityAuthHandler — конструктор, использующий redact-обёрнутый logger по умолчанию.
func NewAntigravityAuthHandler(svc service.AntigravityAuthService) *AntigravityAuthHandler {
	return &AntigravityAuthHandler{svc: svc, log: logging.NopLogger()}
}

// WithAntigravityAuthLogger подменяет logger (используется при инициализации в main / тестах).
func WithAntigravityAuthLogger(h *AntigravityAuthHandler, log *slog.Logger) *AntigravityAuthHandler {
	if log != nil {
		h.log = log
	}
	return h
}

// Init инициирует device-flow Antigravity.
// @Summary Старт OAuth (device flow) для Antigravity
// @Description Возвращает device-code и URL, по которому пользователь должен подтвердить вход.
// @Tags antigravity
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.AntigravityAuthInitResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Failure 503 {object} apierror.ErrorResponse
// @Router /antigravity/auth/init [post]
func (h *AntigravityAuthHandler) Init(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	out, err := h.svc.InitDeviceCode(c.Request.Context(), uid)
	if err != nil {
		mapAntigravityAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.AntigravityAuthInitResponse{
		DeviceCode:              out.DeviceCode,
		UserCode:                out.UserCode,
		VerificationURI:         out.VerificationURI,
		VerificationURIComplete: out.VerificationURIComplete,
		IntervalSeconds:         int(out.Interval.Seconds()),
		ExpiresInSeconds:        int(out.ExpiresIn.Seconds()),
	})
}

// Callback завершает device-flow: меняет device_code на access/refresh-токены и сохраняет подписку.
// @Summary Завершение OAuth (device flow) для Antigravity
// @Description Должен вызываться периодически (раз в interval_seconds), пока не вернёт 200 или 410.
// @Tags antigravity
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.AntigravityAuthCallbackRequest true "device_code из /init"
// @Success 200 {object} dto.AntigravityAuthStatusResponse
// @Failure 202 {object} apierror.ErrorResponse "Пользователь ещё не подтвердил доступ (authorization_pending)"
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 410 {object} apierror.ErrorResponse "expired_token / access_denied"
// @Failure 429 {object} apierror.ErrorResponse "slow_down"
// @Failure 500 {object} apierror.ErrorResponse
// @Router /antigravity/auth/callback [post]
func (h *AntigravityAuthHandler) Callback(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	var req dto.AntigravityAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.Warn("antigravity_oauth callback: bind failed",
			"user_id", uid.String(),
			"error_kind", "bad_request",
		)
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	status, err := h.svc.CompleteDeviceCode(c.Request.Context(), uid, req.DeviceCode)
	if err != nil {
		h.logCallbackError(uid.String(), err)
		mapAntigravityAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.AntigravityAuthStatusResponse(*status))
}

func (h *AntigravityAuthHandler) logCallbackError(userID string, err error) {
	kind := classifyAntigravityAuthErr(err)
	if kind == "authorization_pending" || kind == "slow_down" {
		return
	}
	h.log.Warn("antigravity_oauth callback: handler returned error",
		"user_id", userID,
		"error_kind", kind,
		"error_summary", logging.SafeRawAttr([]byte(err.Error())),
	)
}

func classifyAntigravityAuthErr(err error) string {
	switch {
	case errors.Is(err, service.ErrDeviceCodeOwnerMismatch):
		return "device_code_owner_mismatch"
	case errors.Is(err, service.ErrAuthorizationPending):
		return "authorization_pending"
	case errors.Is(err, service.ErrSlowDown):
		return "slow_down"
	case errors.Is(err, service.ErrExpiredToken):
		return "expired_token"
	case errors.Is(err, service.ErrAccessDenied):
		return "access_denied"
	case errors.Is(err, service.ErrOAuthInvalidGrant):
		return "invalid_grant"
	case errors.Is(err, service.ErrAntigravityOAuthNotConfigured):
		return "oauth_not_configured"
	default:
		return "internal_error"
	}
}

// ManualToken — PUT /antigravity/auth/manual-token.
// @Summary Сохранить заранее полученный токен подписки Antigravity
// @Tags antigravity
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.AntigravityAuthManualTokenRequest true "Токен"
// @Success 200 {object} dto.AntigravityAuthStatusResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /antigravity/auth/manual-token [put]
func (h *AntigravityAuthHandler) ManualToken(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	var req dto.AntigravityAuthManualTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	tok := &service.AntigravityOAuthToken{
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		TokenType:    req.TokenType,
		Scopes:       req.Scopes,
		ExpiresAt:    req.ExpiresAt,
	}
	status, err := h.svc.SaveManualToken(c.Request.Context(), uid, tok)
	if err != nil {
		mapAntigravityAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.AntigravityAuthStatusResponse(*status))
}

// Status возвращает текущий статус подписки.
// @Summary Статус OAuth-подписки Antigravity
// @Tags antigravity
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.AntigravityAuthStatusResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /antigravity/auth/status [get]
func (h *AntigravityAuthHandler) Status(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	status, err := h.svc.Status(c.Request.Context(), uid)
	if err != nil {
		mapAntigravityAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.AntigravityAuthStatusResponse(*status))
}

// Revoke отзывает подписку у пользователя.
// @Summary Отзыв OAuth-подписки Antigravity
// @Tags antigravity
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 204 ""
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /antigravity/auth [delete]
func (h *AntigravityAuthHandler) Revoke(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), uid); err != nil {
		mapAntigravityAuthErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func mapAntigravityAuthErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDeviceCodeOwnerMismatch):
		apierror.JSON(c, http.StatusForbidden, "device_code_owner_mismatch",
			"device_code was not initiated by this user")
	case errors.Is(err, service.ErrAuthorizationPending):
		apierror.JSON(c, http.StatusAccepted, "authorization_pending", "Waiting for user to authorize")
	case errors.Is(err, service.ErrSlowDown):
		apierror.JSON(c, http.StatusTooManyRequests, "slow_down", "Polling too fast")
	case errors.Is(err, service.ErrAccessDenied):
		apierror.JSON(c, http.StatusGone, "access_denied", "User denied authorization")
	case errors.Is(err, service.ErrExpiredToken):
		apierror.JSON(c, http.StatusGone, "invalid_state", "Device code has expired")
	case errors.Is(err, service.ErrOAuthInvalidGrant):
		apierror.JSON(c, http.StatusBadRequest, "invalid_state", "Invalid grant")
	case errors.Is(err, service.ErrAntigravityOAuthNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, "oauth_not_configured",
			"Antigravity OAuth is not configured on this server")
	default:
		apierror.JSON(c, http.StatusBadGateway, "provider_unreachable",
			"Antigravity OAuth provider returned an unexpected error")
	}
}
