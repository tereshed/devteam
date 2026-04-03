package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/devteam/backend/pkg/jwt"
)

const (
	// AuthorizationHeader ключ заголовка Authorization
	AuthorizationHeader = "Authorization"
	// BearerPrefix префикс Bearer токена
	BearerPrefix = "Bearer "
	// ApiKeyHeader ключ заголовка X-API-Key
	ApiKeyHeader = "X-API-Key"
)

// AuthMiddleware создает middleware для аутентификации JWT + API Key
// Поддерживает два способа аутентификации:
// 1. JWT: Authorization: Bearer <token>
// 2. API Key: X-API-Key: wibe_<key>
func AuthMiddleware(jwtManager *jwt.Manager, apiKeyService service.ApiKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Пробуем API Key первым (X-API-Key header)
		apiKey := c.GetHeader(ApiKeyHeader)
		if apiKey != "" {
			authenticateWithApiKey(c, apiKeyService, apiKey)
			return
		}

		// Fallback на JWT (Authorization: Bearer <token>)
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader == "" {
			apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrTokenRequired, "Authorization header or X-API-Key required")
			return
		}

		authenticateWithJWT(c, jwtManager, authHeader)
	}
}

// authenticateWithJWT аутентификация через JWT токен
func authenticateWithJWT(c *gin.Context, jwtManager *jwt.Manager, authHeader string) {
	// Извлекаем токен из заголовка
	if !strings.HasPrefix(authHeader, BearerPrefix) {
		apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrInvalidAuthHeader, "Invalid authorization header format")
		return
	}

	tokenString := strings.TrimPrefix(authHeader, BearerPrefix)
	if tokenString == "" {
		apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrTokenRequired, "Token required")
		return
	}

	// Валидируем токен
	claims, err := jwtManager.ValidateToken(tokenString)
	if err != nil {
		if err == jwt.ErrExpiredToken {
			apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrTokenExpired, "Token expired")
		} else {
			apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrInvalidToken, "Invalid token")
		}
		return
	}

	// Сохраняем данные пользователя в контексте
	c.Set("userID", claims.UserID)
	c.Set("userRole", claims.Role)
	c.Set("authMethod", "jwt")

	c.Next()
}

// authenticateWithApiKey аутентификация через API-ключ
func authenticateWithApiKey(c *gin.Context, apiKeyService service.ApiKeyService, rawKey string) {
	if apiKeyService == nil {
		apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrInvalidToken, "API key authentication not available")
		return
	}

	_, user, err := apiKeyService.ValidateKey(c.Request.Context(), rawKey)
	if err != nil {
		switch err {
		case service.ErrApiKeyNotFound:
			apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrInvalidToken, "Invalid API key")
		case service.ErrApiKeyRevoked:
			apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrInvalidToken, "API key has been revoked")
		case service.ErrApiKeyExpired:
			apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrTokenExpired, "API key has expired")
		default:
			apierror.AbortJSON(c, http.StatusUnauthorized, apierror.ErrInvalidToken, "API key validation failed")
		}
		return
	}

	// Сохраняем данные пользователя в контексте (идентично JWT)
	c.Set("userID", user.ID)
	c.Set("userRole", string(user.Role))
	c.Set("authMethod", "api_key")

	c.Next()
}

// GetUserID извлекает userID из контекста Gin
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return uuid.Nil, false
	}
	id, ok := userID.(uuid.UUID)
	return id, ok
}

// GetUserRole извлекает роль пользователя из контекста Gin
func GetUserRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("userRole")
	if !exists {
		return "", false
	}
	r, ok := role.(string)
	return r, ok
}

// GetAuthMethod извлекает метод аутентификации из контекста Gin
func GetAuthMethod(c *gin.Context) string {
	method, exists := c.Get("authMethod")
	if !exists {
		return ""
	}
	m, _ := method.(string)
	return m
}

// AdminOnlyMiddleware создает middleware, разрешающий доступ только администраторам
func AdminOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := GetUserRole(c)
		if !exists || role != "admin" {
			apierror.AbortJSON(c, http.StatusForbidden, apierror.ErrForbidden, "Admin access required")
			return
		}
		c.Next()
	}
}
