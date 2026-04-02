package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/service"
)

// --- Context keys ---

// contextKey — приватный тип для ключей контекста (предотвращает конфликты).
type contextKey string

const (
	// CtxKeyUserID — ключ для uuid.UUID пользователя в context.Context.
	CtxKeyUserID contextKey = "mcp_user_id"
	// CtxKeyUserRole — ключ для models.UserRole пользователя в context.Context.
	CtxKeyUserRole contextKey = "mcp_user_role"
	// CtxKeyApiKeyID — ключ для uuid.UUID API-ключа в context.Context.
	CtxKeyApiKeyID contextKey = "mcp_api_key_id"
)

// --- Валидация формата ключа ---

const (
	// apiKeyPrefix — обязательный префикс API-ключей.
	apiKeyPrefix = "wibe_"
	// apiKeyMinLength — минимальная длина ключа: "wibe_" (5) + минимум 16 hex символов.
	apiKeyMinLength = 21
	// apiKeyMaxLength — максимальная длина ключа: "wibe_" (5) + 64 hex символов + запас.
	apiKeyMaxLength = 128
)

// --- Context helpers ---

// UserIDFromContext извлекает userID из context.Context (установленного middleware).
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxKeyUserID).(uuid.UUID)
	return id, ok
}

// UserRoleFromContext извлекает роль пользователя из context.Context.
func UserRoleFromContext(ctx context.Context) (models.UserRole, bool) {
	role, ok := ctx.Value(CtxKeyUserRole).(models.UserRole)
	return role, ok
}

// ApiKeyIDFromContext извлекает ID API-ключа из context.Context.
func ApiKeyIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CtxKeyApiKeyID).(uuid.UUID)
	return id, ok
}

// --- Auth error response ---

// authErrorResponse — JSON-ответ при ошибке аутентификации.
// Формат согласован с pkg/apierror, но без зависимости от Gin.
type authErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details"`
}

// writeAuthError отправляет JSON-ответ с ошибкой аутентификации.
func writeAuthError(w http.ResponseWriter, statusCode int, errCode string, details string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")

	// RFC 7235: 401 ответы ДОЛЖНЫ содержать WWW-Authenticate
	if statusCode == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="mcp"`)
	}

	w.WriteHeader(statusCode)

	resp := authErrorResponse{
		Error:   errCode,
		Details: details,
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("[mcp/auth] failed to write error response: %v", err)
	}
}

// --- Коды ошибок (согласованы с pkg/apierror) ---

const (
	errCodeTokenRequired  = "TOKEN_REQUIRED"
	errCodeInvalidToken   = "INVALID_TOKEN"
	errCodeTokenExpired   = "TOKEN_EXPIRED"
	errCodeInternalError  = "INTERNAL_ERROR"
)

// --- Middleware ---

// NewAuthMiddleware создает HTTP middleware для аутентификации MCP-запросов по API Key.
//
// MCP-клиенты (Cursor, Claude Desktop, VS Code Copilot) передают ключ через:
//   - Authorization: Bearer wibe_<key>  (стандарт для MCP)
//   - X-API-Key: wibe_<key>             (альтернатива)
//
// При успешной аутентификации middleware помещает в context.Context:
//   - CtxKeyUserID   → uuid.UUID
//   - CtxKeyUserRole → models.UserRole
//   - CtxKeyApiKeyID → uuid.UUID
//
// При ошибке возвращает HTTP 401 (auth) или 500 (internal) с JSON-телом.
func NewAuthMiddleware(next http.Handler, apiKeyService service.ApiKeyService) http.Handler {
	// Fail-fast: если сервис не предоставлен, каждый запрос получит 500
	if apiKeyService == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[mcp/auth] CRITICAL: apiKeyService is nil, cannot authenticate requests")
			writeAuthError(w, http.StatusInternalServerError, errCodeInternalError,
				"Authentication service unavailable")
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Извлекаем API-ключ из заголовков
		rawKey := extractApiKey(r)
		if rawKey == "" {
			writeAuthError(w, http.StatusUnauthorized, errCodeTokenRequired,
				"API key required. Use 'Authorization: Bearer wibe_<key>' or 'X-API-Key: wibe_<key>'")
			return
		}

		// Быстрая проверка формата — не идём в БД с заведомо невалидным ключом
		if !isValidKeyFormat(rawKey) {
			writeAuthError(w, http.StatusUnauthorized, errCodeInvalidToken, "Invalid API key format")
			return
		}

		// Валидируем ключ через сервис
		apiKey, user, err := apiKeyService.ValidateKey(r.Context(), rawKey)
		if err != nil {
			handleValidationError(w, err)
			return
		}

		// TODO: проверка scopes — убедиться что ключ имеет scope "*" или "mcp".
		// Сейчас все валидные ключи получают полный доступ ко всем MCP-инструментам.
		// При добавлении granular scopes — добавить проверку здесь.

		// Помещаем данные пользователя в context
		ctx := r.Context()
		ctx = context.WithValue(ctx, CtxKeyUserID, user.ID)
		ctx = context.WithValue(ctx, CtxKeyUserRole, user.Role)
		ctx = context.WithValue(ctx, CtxKeyApiKeyID, apiKey.ID)

		// Передаём запрос дальше с обогащённым контекстом
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- Internal helpers ---

// extractApiKey извлекает API-ключ из HTTP-запроса.
// Приоритет: X-API-Key header → Authorization: Bearer <key>.
func extractApiKey(r *http.Request) string {
	// 1. X-API-Key header (прямой способ)
	if key := strings.TrimSpace(r.Header.Get("X-API-Key")); key != "" {
		return key
	}

	// 2. Authorization: Bearer <key> (стандарт MCP)
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}

	// Поддерживаем "Bearer <key>" (case-insensitive для "Bearer")
	const bearerPrefix = "Bearer "
	if len(authHeader) > len(bearerPrefix) && strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
		return strings.TrimSpace(authHeader[len(bearerPrefix):])
	}

	return ""
}

// isValidKeyFormat выполняет быструю проверку формата API-ключа
// без обращения к базе данных. Отсекает заведомо невалидные ключи.
func isValidKeyFormat(key string) bool {
	if len(key) < apiKeyMinLength || len(key) > apiKeyMaxLength {
		return false
	}
	if !strings.HasPrefix(key, apiKeyPrefix) {
		return false
	}
	return true
}

// handleValidationError маппит ошибки ApiKeyService в HTTP-ответы.
// Известные auth-ошибки → 401; неизвестные (БД, сеть) → 500.
func handleValidationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrApiKeyNotFound):
		writeAuthError(w, http.StatusUnauthorized, errCodeInvalidToken, "Invalid API key")
	case errors.Is(err, service.ErrApiKeyRevoked):
		writeAuthError(w, http.StatusUnauthorized, errCodeInvalidToken, "API key has been revoked")
	case errors.Is(err, service.ErrApiKeyExpired):
		writeAuthError(w, http.StatusUnauthorized, errCodeTokenExpired, "API key has expired")
	default:
		// Внутренняя ошибка (БД, сеть и т.д.) — 500, не 401
		log.Printf("[mcp/auth] internal validation error: %v", err)
		writeAuthError(w, http.StatusInternalServerError, errCodeInternalError,
			"Internal error during authentication")
	}
}
