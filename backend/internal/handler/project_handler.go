package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/devteam/backend/pkg/httputil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ProjectHandler HTTP-слой для проектов (тонкий: bind → service → DTO).
type ProjectHandler struct {
	service service.ProjectService
}

// NewProjectHandler создаёт обработчик проектов.
func NewProjectHandler(svc service.ProjectService) *ProjectHandler {
	return &ProjectHandler{service: svc}
}

func getUserID(c *gin.Context) (uuid.UUID, bool) {
	return middleware.GetUserID(c)
}

func getUserRole(c *gin.Context) (models.UserRole, bool) {
	r, ok := middleware.GetUserRole(c)
	if !ok {
		return "", false
	}
	return models.UserRole(r), true
}

func requireAuth(c *gin.Context) (uuid.UUID, models.UserRole, bool) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return uuid.Nil, "", false
	}
	role, ok := getUserRole(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return uuid.Nil, "", false
	}
	return uid, role, true
}

func normalizeListPagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func writeProjectServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound),
		errors.Is(err, service.ErrConversationNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden),
		errors.Is(err, service.ErrConversationForbidden),
		errors.Is(err, service.ErrGitCredentialForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrProjectNameExists):
		apierror.JSON(c, http.StatusConflict, apierror.ErrAlreadyExists, err.Error())
	case errors.Is(err, service.ErrGitValidationFailed):
		apierror.JSON(c, http.StatusBadGateway, apierror.ErrExternalService, err.Error())
	case errors.Is(err, service.ErrGitCloneFailed):
		apierror.JSON(c, http.StatusBadGateway, apierror.ErrExternalService, err.Error())
	case errors.Is(err, service.ErrDecryptionFailed):
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to process credentials")
	case errors.Is(err, service.ErrGitURLRequired),
		errors.Is(err, service.ErrGitCredentialRequired),
		errors.Is(err, service.ErrGitCredentialNotSupportedForLocal),
		errors.Is(err, service.ErrInvalidConversationTitle),
		errors.Is(err, service.ErrInvalidMessageContent):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrGitCredentialNotFound),
		errors.Is(err, service.ErrProjectInvalidName),
		errors.Is(err, service.ErrProjectInvalidProvider),
		errors.Is(err, service.ErrProjectInvalidStatus),
		errors.Is(err, service.ErrUpdateProjectGitCredentialConflict),
		errors.Is(err, service.ErrUpdateProjectTechStackConflict),
		errors.Is(err, service.ErrUpdateProjectSettingsConflict):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// Create создаёт новый проект
// @Summary Создание проекта
// @Description Создаёт проект с автоматическим созданием команды
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param request body dto.CreateProjectRequest true "Данные проекта"
// @Success 201 {object} dto.ProjectResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects [post]
func (h *ProjectHandler) Create(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	var req dto.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	project, err := h.service.Create(c.Request.Context(), uid, req)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	resp := dto.ToProjectResponse(project)
	c.JSON(http.StatusCreated, resp)
}

// List возвращает список проектов с пагинацией и фильтрами
// @Summary Список проектов
// @Description Возвращает проекты текущего пользователя (для admin — все)
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param status query string false "Фильтр по статусу"
// @Param git_provider query string false "Фильтр по git-провайдеру"
// @Param search query string false "Поиск по имени"
// @Param limit query int false "Лимит (1–100, по умолчанию 20)"
// @Param offset query int false "Смещение"
// @Param order_by query string false "Поле сортировки"
// @Param order_dir query string false "Направление сортировки"
// @Success 200 {object} dto.ProjectListResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects [get]
func (h *ProjectHandler) List(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	var req dto.ListProjectsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	projects, total, err := h.service.List(c.Request.Context(), uid, role, req)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	limit, offset := normalizeListPagination(req.Limit, req.Offset)
	c.JSON(http.StatusOK, dto.ToProjectListResponse(projects, total, limit, offset))
}

// GetByID возвращает проект по ID
// @Summary Получение проекта
// @Description Возвращает проект по UUID
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.ProjectResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects/{id} [get]
func (h *ProjectHandler) GetByID(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	project, err := h.service.GetByID(c.Request.Context(), uid, role, projectID)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToProjectResponse(project))
}

// Update обновляет проект
// @Summary Обновление проекта
// @Description Частичное обновление полей проекта
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.UpdateProjectRequest true "Поля для обновления"
// @Success 200 {object} dto.ProjectResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects/{id} [put]
func (h *ProjectHandler) Update(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	var req dto.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	project, err := h.service.Update(c.Request.Context(), uid, role, projectID, req)
	if err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.ToProjectResponse(project))
}

// Delete удаляет проект
// @Summary Удаление проекта
// @Description Удаляет проект по UUID
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /projects/{id} [delete]
func (h *ProjectHandler) Delete(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}

	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}

	if err := h.service.Delete(c.Request.Context(), uid, role, projectID); err != nil {
		httputil.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project deleted successfully"})
}
