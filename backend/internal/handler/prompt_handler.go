package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/handler/dto"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/service"
	"github.com/wibe-flutter-gin-template/backend/pkg/apierror"
)

type PromptHandler struct {
	service service.PromptService
}

func NewPromptHandler(service service.PromptService) *PromptHandler {
	return &PromptHandler{service: service}
}

// Create создает новый промпт
// @Summary Создание промпта
// @Description Создает новый шаблон промпта (только админ)
// @Tags prompts
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param request body dto.CreatePromptRequest true "Данные промпта"
// @Success 201 {object} dto.PromptResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /prompts [post]
func (h *PromptHandler) Create(c *gin.Context) {
	var req dto.CreatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	prompt, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		if err == service.ErrPromptAlreadyExists {
			apierror.JSON(c, http.StatusConflict, apierror.ErrAlreadyExists, "Prompt with this name already exists")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to create prompt")
		return
	}

	c.JSON(http.StatusCreated, toPromptResponse(prompt))
}

// List возвращает список всех промптов
// @Summary Список промптов
// @Description Возвращает все промпты (только админ)
// @Tags prompts
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Success 200 {array} dto.PromptResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /prompts [get]
func (h *PromptHandler) List(c *gin.Context) {
	prompts, err := h.service.List(c.Request.Context())
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to list prompts")
		return
	}

	var response []dto.PromptResponse
	for _, p := range prompts {
		response = append(response, toPromptResponse(&p))
	}

	c.JSON(http.StatusOK, response)
}

// GetByID возвращает промпт по ID
// @Summary Получение промпта
// @Description Возвращает промпт по ID (только админ)
// @Tags prompts
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Prompt ID"
// @Success 200 {object} dto.PromptResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /prompts/{id} [get]
func (h *PromptHandler) GetByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	prompt, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == service.ErrPromptNotFound {
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "Prompt not found")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to get prompt")
		return
	}

	c.JSON(http.StatusOK, toPromptResponse(prompt))
}

// Update обновляет промпт
// @Summary Обновление промпта
// @Description Обновляет данные промпта (только админ)
// @Tags prompts
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Prompt ID"
// @Param request body dto.UpdatePromptRequest true "Данные для обновления"
// @Success 200 {object} dto.PromptResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /prompts/{id} [put]
func (h *PromptHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	var req dto.UpdatePromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	prompt, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		if err == service.ErrPromptNotFound {
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "Prompt not found")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to update prompt")
		return
	}

	c.JSON(http.StatusOK, toPromptResponse(prompt))
}

// Delete удаляет промпт
// @Summary Удаление промпта
// @Description Удаляет промпт по ID (только админ)
// @Tags prompts
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Prompt ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /prompts/{id} [delete]
func (h *PromptHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		if err == service.ErrPromptNotFound {
			apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "Prompt not found")
			return
		}
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to delete prompt")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "prompt deleted successfully"})
}

func toPromptResponse(p *models.Prompt) dto.PromptResponse {
	return dto.PromptResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		Template:    p.Template,
		JSONSchema:  p.JSONSchema,
		IsActive:    p.IsActive,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

