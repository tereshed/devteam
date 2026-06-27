package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RepositoryEnvFileHandler — REST для «инъекции env-файла» уровня репозитория проекта.
// Один файл на репозиторий: GET (с содержимым для редактирования), PUT (upsert), DELETE.
type RepositoryEnvFileHandler struct {
	svc *service.RepositoryEnvFileService
}

func NewRepositoryEnvFileHandler(svc *service.RepositoryEnvFileService) *RepositoryEnvFileHandler {
	return &RepositoryEnvFileHandler{svc: svc}
}

type setRepoEnvFileRequest struct {
	FileName  string `json:"file_name" binding:"required"`
	TargetDir string `json:"target_dir"`
	Content   string `json:"content" binding:"required"`
}

// Get returns the repository env file (with decrypted content for editing).
// @Summary Get repository env file
// @Tags repository-env-files
// @Security BearerAuth
// @Produce json
// @Param id path string true "Project UUID"
// @Param repoId path string true "Repository UUID"
// @Success 200 {object} service.RepoEnvFileView
// @Success 204 "no env file configured"
// @Router /projects/{id}/repositories/{repoId}/env-file [get]
func (h *RepositoryEnvFileHandler) Get(c *gin.Context) {
	projectID, repoID, ok := parseProjectAndRepoID(c)
	if !ok {
		return
	}
	view, err := h.svc.Get(c.Request.Context(), projectID, repoID)
	if err != nil {
		writeRepoEnvFileError(c, err)
		return
	}
	if view == nil {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, view)
}

// Set creates or updates the repository env file.
// @Summary Set repository env file
// @Tags repository-env-files
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Project UUID"
// @Param repoId path string true "Repository UUID"
// @Param body body setRepoEnvFileRequest true "file_name + target_dir + content"
// @Success 200 {object} service.RepoEnvFileView
// @Router /projects/{id}/repositories/{repoId}/env-file [put]
func (h *RepositoryEnvFileHandler) Set(c *gin.Context) {
	projectID, repoID, ok := parseProjectAndRepoID(c)
	if !ok {
		return
	}
	var req setRepoEnvFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	view, err := h.svc.Set(c.Request.Context(), service.SetRepoEnvFileInput{
		ProjectID: projectID,
		RepoID:    repoID,
		FileName:  req.FileName,
		TargetDir: req.TargetDir,
		Content:   req.Content,
	})
	if err != nil {
		writeRepoEnvFileError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

// Delete removes the repository env file.
// @Summary Delete repository env file
// @Tags repository-env-files
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param repoId path string true "Repository UUID"
// @Success 204
// @Router /projects/{id}/repositories/{repoId}/env-file [delete]
func (h *RepositoryEnvFileHandler) Delete(c *gin.Context) {
	projectID, repoID, ok := parseProjectAndRepoID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), projectID, repoID); err != nil {
		writeRepoEnvFileError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func parseProjectAndRepoID(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid project id")
		return uuid.Nil, uuid.Nil, false
	}
	repoID, err := uuid.Parse(c.Param("repoId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid repository id")
		return uuid.Nil, uuid.Nil, false
	}
	return projectID, repoID, true
}

func writeRepoEnvFileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRepoEnvFileValidation):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrRepoEnvFileRepoMismatch):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "repository not found in project")
	case errors.Is(err, service.ErrRepoEnvFileNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrEncryptorNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, apierror.ErrInternalServerError, "encryption is not configured on server")
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	}
}
