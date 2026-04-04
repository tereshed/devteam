package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
)

// AuthHandler обрабатывает HTTP запросы для авторизации
type AuthHandler struct {
	authService service.AuthService
	jwtManager  JWTManager
}

// JWTManager интерфейс для работы с JWT (для получения TTL)
type JWTManager interface {
	GetAccessTokenTTL() time.Duration
}

// NewAuthHandler создает новый handler для авторизации
func NewAuthHandler(authService service.AuthService, jwtManager JWTManager) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		jwtManager:  jwtManager,
	}
}

// Register обрабатывает запрос на регистрацию
// @Summary Регистрация нового пользователя
// @Description Создает нового пользователя и возвращает токены
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Данные для регистрации"
// @Success 201 {object} dto.AuthResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	_, err := h.authService.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if err == service.ErrUserAlreadyExists {
			apierror.JSON(c, http.StatusConflict, apierror.ErrUserAlreadyExists, "User already exists")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to register user")
		return
	}

	// После регистрации автоматически логиним пользователя
	_, accessToken, refreshToken, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to login after registration")
		return
	}

	expiresIn := int64(h.jwtManager.GetAccessTokenTTL().Seconds())
	c.JSON(http.StatusCreated, dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	})
}

// Login обрабатывает запрос на вход
// @Summary Вход пользователя
// @Description Аутентифицирует пользователя и возвращает токены
// @Tags auth
// @Accept json,x-www-form-urlencoded
// @Produce json
// @Param request body dto.LoginRequest true "Данные для входа"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	// Swagger UI OAuth2 отправляет 'username', API клиенты отправляют 'email'
	email := req.Email
	if email == "" {
		email = req.Username
	}

	if email == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Email is required")
		return
	}

	_, accessToken, refreshToken, err := h.authService.Login(c.Request.Context(), email, req.Password)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			apierror.JSON(c, http.StatusUnauthorized, apierror.ErrInvalidCredentials, "Invalid credentials")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to login")
		return
	}

	expiresIn := int64(h.jwtManager.GetAccessTokenTTL().Seconds())
	// OAuth2 headers to prevent caching
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	})
}

// Refresh обрабатывает запрос на обновление токена
// @Summary Обновление токена
// @Description Обновляет access token используя refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	accessToken, refreshToken, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if err == service.ErrInvalidRefreshToken {
			apierror.JSON(c, http.StatusUnauthorized, apierror.ErrInvalidToken, "Invalid refresh token")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to refresh token")
		return
	}

	expiresIn := int64(h.jwtManager.GetAccessTokenTTL().Seconds())
	c.JSON(http.StatusOK, dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
	})
}

// Logout обрабатывает запрос на выход
// @Summary Выход пользователя
// @Description Отзывает все refresh токены пользователя
// @Tags auth
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	if err := h.authService.Logout(c.Request.Context(), userID.(uuid.UUID)); err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to logout")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// Me обрабатывает запрос на получение данных текущего пользователя
// @Summary Получение данных текущего пользователя
// @Description Возвращает данные аутентифицированного пользователя
// @Tags auth
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Success 200 {object} dto.UserResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), userID.(uuid.UUID))
	if err != nil {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrUserNotFound, "User not found")
		return
	}

	c.JSON(http.StatusOK, dto.UserResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		Role:          string(user.Role),
		EmailVerified: user.EmailVerified,
	})
}
