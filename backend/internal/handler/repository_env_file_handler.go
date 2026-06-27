package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RepositoryEnvFileHandler — REST для «инъекции env-файлов» уровня репозитория проекта.
// Коллекция: на один репозиторий может быть несколько файлов.
type RepositoryEnvFileHandler struct {
	svc *service.RepositoryEnvFileService
}

func NewRepositoryEnvFileHandler(svc *service.RepositoryEnvFileService) *RepositoryEnvFileHandler {
	return &RepositoryEnvFileHandler{svc: svc}
}

type repoEnvFileRequest struct {
	FileName  string `json:"file_name" binding:"required"`
	TargetDir string `json:"target_dir"`
	Content   string `json:"content" binding:"required"`
}

// List returns all env files of a repository (metadata only, no content).
// @Summary List repository env files
// @Tags repository-env-files
// @Security BearerAuth
// @Produce json
// @Param id path string true "Project UUID"
// @Param repoId path string true "Repository UUID"
// @Success 200 {array} service.RepoEnvFileView
// @Router /projects/{id}/repositories/{repoId}/env-files [get]
func (h *RepositoryEnvFileHandler) List(c *gin.Context) {
	projectID, repoID, ok := parseProjectAndRepoID(c)
	if !ok {
		return
	}
	views, err := h.svc.List(c.Request.Context(), projectID, repoID)
	if err != nil {
		writeRepoEnvFileError(c, err)
		return
	}
	c.JSON(http.StatusOK, views)
}

// Create adds a new env file to a repository.
// @Summary Create repository env file
// @Tags repository-env-files
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Project UUID"
// @Param repoId path string true "Repository UUID"
// @Param body body repoEnvFileRequest true "file_name + target_dir + content"
// @Success 201 {object} service.RepoEnvFileView
// @Router /projects/{id}/repositories/{repoId}/env-files [post]
func (h *RepositoryEnvFileHandler) Create(c *gin.Context) {
	projectID, repoID, ok := parseProjectAndRepoID(c)
	if !ok {
		return
	}
	var req repoEnvFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	view, err := h.svc.Create(c.Request.Context(), service.CreateRepoEnvFileInput{
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
	c.JSON(http.StatusCreated, view)
}

// Update replaces an existing env file (full overwrite).
// @Summary Update repository env file
// @Tags repository-env-files
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Project UUID"
// @Param repoId path string true "Repository UUID"
// @Param fileId path string true "Env file UUID"
// @Param body body repoEnvFileRequest true "file_name + target_dir + content"
// @Success 200 {object} service.RepoEnvFileView
// @Router /projects/{id}/repositories/{repoId}/env-files/{fileId} [put]
func (h *RepositoryEnvFileHandler) Update(c *gin.Context) {
	projectID, repoID, ok := parseProjectAndRepoID(c)
	if !ok {
		return
	}
	fileID, err := uuid.Parse(c.Param("fileId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid file id")
		return
	}
	var req repoEnvFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	view, err := h.svc.Update(c.Request.Context(), service.UpdateRepoEnvFileInput{
		ProjectID: projectID,
		RepoID:    repoID,
		FileID:    fileID,
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

// Delete removes an env file.
// @Summary Delete repository env file
// @Tags repository-env-files
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param repoId path string true "Repository UUID"
// @Param fileId path string true "Env file UUID"
// @Success 204
// @Router /projects/{id}/repositories/{repoId}/env-files/{fileId} [delete]
func (h *RepositoryEnvFileHandler) Delete(c *gin.Context) {
	projectID, repoID, ok := parseProjectAndRepoID(c)
	if !ok {
		return
	}
	fileID, err := uuid.Parse(c.Param("fileId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid file id")
		return
	}
	if err := h.svc.Delete(c.Request.Context(), projectID, repoID, fileID); err != nil {
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
	case errors.Is(err, service.ErrRepoEnvFileValidation),
		errors.Is(err, service.ErrRepoEnvFileDuplicate):
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
