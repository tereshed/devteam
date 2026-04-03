package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
)

// ApiKeyHandler обрабатывает HTTP-запросы для управления API-ключами
type ApiKeyHandler struct {
	apiKeyService service.ApiKeyService
	mcpConfig     *config.MCPConfig
}

// NewApiKeyHandler создает новый handler для API-ключей
func NewApiKeyHandler(apiKeyService service.ApiKeyService, mcpConfig *config.MCPConfig) *ApiKeyHandler {
	return &ApiKeyHandler{
		apiKeyService: apiKeyService,
		mcpConfig:     mcpConfig,
	}
}

// Create создает новый API-ключ
// @Summary Создание API-ключа
// @Description Создает новый долгосрочный API-ключ для текущего пользователя. Сырой ключ показывается только один раз!
// @Tags api-keys
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.CreateApiKeyRequest true "Данные для создания ключа"
// @Success 201 {object} dto.ApiKeyCreatedResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/api-keys [post]
func (h *ApiKeyHandler) Create(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	var req dto.CreateApiKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	// Вычисляем время истечения
	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	apiKey, rawKey, err := h.apiKeyService.CreateKey(
		c.Request.Context(),
		userID,
		req.Name,
		req.Scopes,
		expiresAt,
	)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to create API key")
		return
	}

	c.JSON(http.StatusCreated, dto.ApiKeyCreatedResponse{
		ApiKeyResponse: toApiKeyResponse(apiKey),
		RawKey:         rawKey,
	})
}

// List возвращает все API-ключи текущего пользователя
// @Summary Список API-ключей
// @Description Возвращает все активные API-ключи текущего пользователя
// @Tags api-keys
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {array} dto.ApiKeyResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/api-keys [get]
func (h *ApiKeyHandler) List(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	keys, err := h.apiKeyService.ListKeys(c.Request.Context(), userID)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to list API keys")
		return
	}

	var response []dto.ApiKeyResponse
	for _, key := range keys {
		response = append(response, toApiKeyResponse(&key))
	}

	// Возвращаем пустой массив вместо null
	if response == nil {
		response = []dto.ApiKeyResponse{}
	}

	c.JSON(http.StatusOK, response)
}

// Revoke отзывает API-ключ
// @Summary Отзыв API-ключа
// @Description Отзывает (деактивирует) API-ключ. Ключ перестает работать, но запись остается.
// @Tags api-keys
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "ID API-ключа"
// @Success 200 {object} map[string]string
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /auth/api-keys/{id}/revoke [post]
func (h *ApiKeyHandler) Revoke(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid API key ID")
		return
	}

	role, _ := middleware.GetUserRole(c)
	isAdmin := role == "admin"

	if err := h.apiKeyService.RevokeKey(c.Request.Context(), keyID, userID, isAdmin); err != nil {
		switch err {
		case service.ErrApiKeyNotFound:
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "API key not found")
		case service.ErrApiKeyAccessDenied:
			apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, "You can only revoke your own API keys")
		default:
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to revoke API key")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

// Delete удаляет API-ключ
// @Summary Удаление API-ключа
// @Description Полностью удаляет API-ключ из системы
// @Tags api-keys
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "ID API-ключа"
// @Success 204
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /auth/api-keys/{id} [delete]
func (h *ApiKeyHandler) Delete(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid API key ID")
		return
	}

	role, _ := middleware.GetUserRole(c)
	isAdmin := role == "admin"

	if err := h.apiKeyService.DeleteKey(c.Request.Context(), keyID, userID, isAdmin); err != nil {
		switch err {
		case service.ErrApiKeyNotFound:
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "API key not found")
		case service.ErrApiKeyAccessDenied:
			apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, "You can only delete your own API keys")
		default:
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to delete API key")
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GetMCPConfig возвращает готовый конфиг для подключения к MCP-серверу
// @Summary Получить MCP-конфигурацию
// @Description Возвращает готовую конфигурацию для подключения к MCP-серверу (для Cursor, Claude Desktop, VS Code Copilot)
// @Tags api-keys
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param apiKey query string false "Конкретный API-ключ (по умолчанию используется первый активный)"
// @Success 200 {object} dto.MCPConfigResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /auth/api-keys/mcp-config [get]
func (h *ApiKeyHandler) GetMCPConfig(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	// Если MCP выключен, вернём ошибку
	if !h.mcpConfig.Enabled {
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "MCP server is disabled")
		return
	}

	// Если URL не задан, вернём ошибку
	if h.mcpConfig.PublicURL == "" {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "MCP_PUBLIC_URL is not configured")
		return
	}

	// Получаем конкретный ключ или первый активный
	apiKeyParam := c.Query("apiKey")
	var rawKey string

	if apiKeyParam != "" {
		// Используем переданный ключ
		rawKey = apiKeyParam
	} else {
		// Берём первый активный ключ пользователя
		keys, err := h.apiKeyService.ListKeys(c.Request.Context(), userID)
		if err != nil {
			apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to fetch API keys")
			return
		}

		if len(keys) == 0 {
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "No API keys found. Please create one first.")
			return
		}

		// Используем key_prefix первого ключа как демо (пользователь подставит реальный)
		rawKey = keys[0].KeyPrefix + "***"
	}

	// Формируем URL MCP-сервера
	serverURL := h.mcpConfig.PublicURL + "/mcp"

	// Генерируем конфиг
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"wibe": map[string]interface{}{
				"url": serverURL,
				"headers": map[string]string{
					"X-API-Key": rawKey,
				},
			},
		},
	}

	instructions := "Copy this config to your LLM client settings (Cursor: .cursor/config.json, Claude Desktop: ~/Library/Application Support/Claude/claude_desktop_config.json)"

	c.JSON(http.StatusOK, dto.MCPConfigResponse{
		Config:       config,
		Instructions: instructions,
		ServerURL:    serverURL,
	})
}

// toApiKeyResponse конвертирует модель в DTO ответа
func toApiKeyResponse(key *models.ApiKey) dto.ApiKeyResponse {
	return dto.ApiKeyResponse{
		ID:         key.ID.String(),
		Name:       key.Name,
		KeyPrefix:  key.KeyPrefix,
		Scopes:     key.Scopes,
		ExpiresAt:  key.ExpiresAt,
		LastUsedAt: key.LastUsedAt,
		CreatedAt:  key.CreatedAt,
	}
}
