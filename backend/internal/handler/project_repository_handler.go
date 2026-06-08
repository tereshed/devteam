package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// writeRepositoryServiceError маппит ошибки управления репозиториями в HTTP-ответы.
func writeRepositoryServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound), errors.Is(err, service.ErrRepoNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden), errors.Is(err, service.ErrGitCredentialForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrRepoSlugExists):
		apierror.JSON(c, http.StatusConflict, apierror.ErrAlreadyExists, err.Error())
	case errors.Is(err, service.ErrGitValidationFailed):
		apierror.JSON(c, http.StatusBadGateway, apierror.ErrExternalService, err.Error())
	case errors.Is(err, service.ErrRepoSlugRequired),
		errors.Is(err, service.ErrRepoURLRequired),
		errors.Is(err, service.ErrProjectInvalidProvider),
		errors.Is(err, service.ErrCannotRemovePrimaryRepo),
		errors.Is(err, service.ErrGitCredentialNotFound):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// ListRepositories godoc
// @Summary Список репозиториев проекта
// @Description Возвращает git-репозитории проекта (мульти-репо).
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.ProjectRepositoryListResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /projects/{id}/repositories [get]
func (h *ProjectHandler) ListRepositories(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}
	repos, err := h.service.ListRepositories(c.Request.Context(), uid, role, projectID)
	if err != nil {
		writeRepositoryServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToProjectRepositoryListResponse(repos))
}

// AddRepository godoc
// @Summary Добавление репозитория в проект
// @Description Добавляет git-репозиторий в проект (мульти-репо). Первый репозиторий становится primary.
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.AddRepositoryRequest true "Данные репозитория"
// @Success 201 {object} dto.ProjectRepositoryResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Router /projects/{id}/repositories [post]
func (h *ProjectHandler) AddRepository(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}
	var req dto.AddRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	repo, err := h.service.AddRepository(c.Request.Context(), uid, role, projectID, req)
	if err != nil {
		writeRepositoryServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.ToProjectRepositoryResponse(repo))
}

// UpdateRepository godoc
// @Summary Обновление репозитория проекта
// @Description Частичное обновление репозитория проекта (мульти-репо).
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param repoId path string true "Repository ID"
// @Param request body dto.UpdateRepositoryRequest true "Поля для обновления"
// @Success 200 {object} dto.ProjectRepositoryResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Router /projects/{id}/repositories/{repoId} [put]
func (h *ProjectHandler) UpdateRepository(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}
	repoID, err := uuid.Parse(c.Param("repoId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid repository ID format")
		return
	}
	var req dto.UpdateRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	repo, err := h.service.UpdateRepository(c.Request.Context(), uid, role, projectID, repoID, req)
	if err != nil {
		writeRepositoryServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToProjectRepositoryResponse(repo))
}

// RemoveRepository godoc
// @Summary Удаление репозитория проекта
// @Description Удаляет git-репозиторий из проекта. Primary нельзя удалить, пока есть другие репозитории.
// @Tags projects
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Produce json
// @Param id path string true "Project ID"
// @Param repoId path string true "Repository ID"
// @Success 204 "No Content"
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Router /projects/{id}/repositories/{repoId} [delete]
func (h *ProjectHandler) RemoveRepository(c *gin.Context) {
	uid, role, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid ID format")
		return
	}
	repoID, err := uuid.Parse(c.Param("repoId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid repository ID format")
		return
	}
	if err := h.service.RemoveRepository(c.Request.Context(), uid, role, projectID, repoID); err != nil {
		writeRepositoryServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
