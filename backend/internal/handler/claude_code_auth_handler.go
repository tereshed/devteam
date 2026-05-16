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

// ClaudeCodeAuthHandler — OAuth-подписка Claude Code (Sprint 15.12, 15.15).
// UI Refactoring §4a.5 — 4 явные ветки ошибок (cancel / access_denied / network / invalid_state).
// UI Refactoring §4a.1 — все логи проходят через redact-handler.
//
// MCP-инструмент для /claude-code/auth/* эндпоинтов НЕ создаётся (исключение
// из `docs/rules/backend.md` §7.1): device-flow требует физического перехода
// пользователя по URL в браузере и ручного подтверждения в UI Anthropic. У
// MCP-клиента (агента) нет ни UI-контекста, ни возможности нажать «Authorize»,
// поэтому инструмент был бы бесполезной обёрткой над URL. Завершение flow
// фронт получает через WS-событие `IntegrationConnectionChanged` (см. §4a.4) —
// у этого канала тоже нет MCP-аналога. Пересмотреть, если появится non-device
// OAuth с поддержкой Anthropic-side approval API.
type ClaudeCodeAuthHandler struct {
	svc service.ClaudeCodeAuthService
	log *slog.Logger
}

// NewClaudeCodeAuthHandler — конструктор, использующий redact-обёрнутый logger по умолчанию.
func NewClaudeCodeAuthHandler(svc service.ClaudeCodeAuthService) *ClaudeCodeAuthHandler {
	return &ClaudeCodeAuthHandler{svc: svc, log: logging.NopLogger()}
}

// WithClaudeCodeAuthLogger подменяет logger (используется при инициализации в main / тестах).
// Logger ОБЯЗАТЕЛЬНО должен быть обёрнут в logging.Handler — все callback-логи проходят через redact.
func WithClaudeCodeAuthLogger(h *ClaudeCodeAuthHandler, log *slog.Logger) *ClaudeCodeAuthHandler {
	if log != nil {
		h.log = log
	}
	return h
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
		// Тело уже потенциально содержит секрет (device_code), поэтому не логируем raw.
		h.log.Warn("claude_code_oauth callback: bind failed",
			"user_id", uid.String(),
			"error_kind", "bad_request",
		)
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	status, err := h.svc.CompleteDeviceCode(c.Request.Context(), uid, req.DeviceCode)
	if err != nil {
		h.logCallbackError(uid.String(), err)
		mapClaudeCodeAuthErr(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ClaudeCodeAuthStatusResponse(*status))
}

// logCallbackError — структурированный warning без сырого error-text провайдера.
// Маппинг см. §4a.5 (cancel / access_denied / network / invalid_state).
// Никаких access_token / refresh / code в логах: error.Error() обёрнут в SafeRawAttr.
func (h *ClaudeCodeAuthHandler) logCallbackError(userID string, err error) {
	kind := classifyClaudeCodeAuthErr(err)
	if kind == "authorization_pending" || kind == "slow_down" {
		// Промежуточные poll-состояния — слишком шумно для info; пропускаем.
		return
	}
	h.log.Warn("claude_code_oauth callback: handler returned error",
		"user_id", userID,
		"error_kind", kind,
		"error_summary", logging.SafeRawAttr([]byte(err.Error())),
	)
}

// classifyClaudeCodeAuthErr возвращает короткий код, пригодный для логов/метрик.
// Без error.Error() — текст может прийти от провайдера и содержать секреты.
func classifyClaudeCodeAuthErr(err error) string {
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
	case errors.Is(err, service.ErrOAuthNotConfigured):
		return "oauth_not_configured"
	default:
		return "internal_error"
	}
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

// mapClaudeCodeAuthErr — 4 явные ветки из §4a.5:
//
//  1. cancel (user_cancelled / access_denied)        -> 410, error_code=access_denied
//  2. invalid_state (expired_token / invalid_grant)  -> 410/400, error_code=invalid_state
//  3. provider_unreachable (network)                 -> 502, error_code=provider_unreachable
//  4. all other (server_error от провайдера и т.п.)  -> 500, error_code=internal_error
//
// Тело ответа никогда не включает err.Error() от провайдера —
// чтобы случайные access_token/refresh_token в сообщении не утекали клиенту.
func mapClaudeCodeAuthErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDeviceCodeOwnerMismatch):
		apierror.JSON(c, http.StatusForbidden, "device_code_owner_mismatch",
			"device_code was not initiated by this user")
	case errors.Is(err, service.ErrAuthorizationPending):
		apierror.JSON(c, http.StatusAccepted, "authorization_pending", "Waiting for user to authorize")
	case errors.Is(err, service.ErrSlowDown):
		apierror.JSON(c, http.StatusTooManyRequests, "slow_down", "Polling too fast")
	// §4a.5 case 1: пользователь нажал Cancel.
	case errors.Is(err, service.ErrAccessDenied):
		apierror.JSON(c, http.StatusGone, "access_denied", "User denied authorization")
	// §4a.5 case 3: устаревший device_code (CSRF / state) — поднимаем единым кодом invalid_state.
	case errors.Is(err, service.ErrExpiredToken):
		apierror.JSON(c, http.StatusGone, "invalid_state", "Device code has expired")
	case errors.Is(err, service.ErrOAuthInvalidGrant):
		apierror.JSON(c, http.StatusBadRequest, "invalid_state", "Invalid grant")
	case errors.Is(err, service.ErrOAuthNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, "oauth_not_configured",
			"Claude Code OAuth is not configured on this server")
	default:
		// §4a.5 case 4: server_error / network — провайдер недоступен.
		apierror.JSON(c, http.StatusBadGateway, "provider_unreachable",
			"Claude Code OAuth provider returned an unexpected error")
	}
}
