package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
)

// ClaudeCodeAuthHandler — OAuth-подписка Claude Code (Sprint 15.12, 15.15).
type ClaudeCodeAuthHandler struct {
	svc service.ClaudeCodeAuthService
}

func NewClaudeCodeAuthHandler(svc service.ClaudeCodeAuthService) *ClaudeCodeAuthHandler {
	return &ClaudeCodeAuthHandler{svc: svc}
}

// Init инициирует device-flow Claude Code.
// @Summary Старт OAuth (device flow) для Claude Code
// @Description Возвращает device-code и URL, по которому пользователь должен подтвердить вход.
// @Tags claude-code
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.ClaudeCodeAuthInitResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Failure 503 {object} apierror.ErrorResponse
// @Router /claude-code/auth/init [post]
func (h *ClaudeCodeAuthHandler) Init(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	out, err := h.svc.InitDeviceCode(c.Request.Context(), uid)
	if err != nil {
		mapClaudeCodeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ClaudeCodeAuthInitResponse{
		DeviceCode:              out.DeviceCode,
		UserCode:                out.UserCode,
		VerificationURI:         out.VerificationURI,
		VerificationURIComplete: out.VerificationURIComplete,
		IntervalSeconds:         int(out.Interval.Seconds()),
		ExpiresInSeconds:        int(out.ExpiresIn.Seconds()),
	})
}

// Callback завершает device-flow: меняет device_code на access/refresh-токены и сохраняет подписку.
// @Summary Завершение OAuth (device flow) для Claude Code
// @Description Должен вызываться периодически (раз в interval_seconds), пока не вернёт 200 или 410.
// @Tags claude-code
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.ClaudeCodeAuthCallbackRequest true "device_code из /init"
// @Success 200 {object} dto.ClaudeCodeAuthStatusResponse
// @Failure 202 {object} apierror.ErrorResponse "Пользователь ещё не подтвердил доступ (authorization_pending)"
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 410 {object} apierror.ErrorResponse "expired_token / access_denied"
// @Failure 429 {object} apierror.ErrorResponse "slow_down"
// @Failure 500 {object} apierror.ErrorResponse
// @Router /claude-code/auth/callback [post]
func (h *ClaudeCodeAuthHandler) Callback(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	var req dto.ClaudeCodeAuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	status, err := h.svc.CompleteDeviceCode(c.Request.Context(), uid, req.DeviceCode)
	if err != nil {
		mapClaudeCodeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ClaudeCodeAuthStatusResponse(*status))
}

// Status возвращает текущий статус подписки.
// @Summary Статус OAuth-подписки Claude Code
// @Tags claude-code
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.ClaudeCodeAuthStatusResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /claude-code/auth/status [get]
func (h *ClaudeCodeAuthHandler) Status(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	status, err := h.svc.Status(c.Request.Context(), uid)
	if err != nil {
		mapClaudeCodeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ClaudeCodeAuthStatusResponse(*status))
}

// Revoke отзывает подписку у пользователя.
// @Summary Отзыв OAuth-подписки Claude Code
// @Tags claude-code
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 204 ""
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /claude-code/auth [delete]
func (h *ClaudeCodeAuthHandler) Revoke(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	if err := h.svc.Revoke(c.Request.Context(), uid); err != nil {
		mapClaudeCodeAuthErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func mapClaudeCodeAuthErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDeviceCodeOwnerMismatch):
		apierror.JSON(c, http.StatusForbidden, "device_code_owner_mismatch",
			"device_code was not initiated by this user")
	case errors.Is(err, service.ErrAuthorizationPending):
		apierror.JSON(c, http.StatusAccepted, "authorization_pending", "Waiting for user to authorize")
	case errors.Is(err, service.ErrSlowDown):
		apierror.JSON(c, http.StatusTooManyRequests, "slow_down", "Polling too fast")
	case errors.Is(err, service.ErrExpiredToken):
		apierror.JSON(c, http.StatusGone, "expired_token", "Device code has expired")
	case errors.Is(err, service.ErrAccessDenied):
		apierror.JSON(c, http.StatusGone, "access_denied", "User denied authorization")
	case errors.Is(err, service.ErrOAuthInvalidGrant):
		apierror.JSON(c, http.StatusBadRequest, "invalid_grant", "Invalid grant")
	case errors.Is(err, service.ErrOAuthNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, "oauth_not_configured", "Claude Code OAuth is not configured on this server")
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	}
}
