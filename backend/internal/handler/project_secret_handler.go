package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ProjectSecretHandler struct {
	svc *service.ProjectSecretService
}

func NewProjectSecretHandler(svc *service.ProjectSecretService) *ProjectSecretHandler {
	return &ProjectSecretHandler{svc: svc}
}

type projectSecretResponse struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	KeyName   string    `json:"key_name"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

type setProjectSecretRequest struct {
	KeyName string `json:"key_name" binding:"required"`
	Value   string `json:"value" binding:"required"`
}

// List returns all project secret keys (without values).
// @Summary List project secrets
// @Tags project-secrets
// @Security BearerAuth
// @Produce json
// @Param id path string true "Project UUID"
// @Success 200 {array} projectSecretResponse
// @Router /projects/{id}/secrets [get]
func (h *ProjectSecretHandler) List(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid project id")
		return
	}

	secrets, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	result := make([]projectSecretResponse, 0, len(secrets))
	for _, s := range secrets {
		result = append(result, projectSecretResponse{
			ID:        s.ID,
			ProjectID: s.ProjectID,
			KeyName:   s.KeyName,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	c.JSON(http.StatusOK, result)
}

// Set creates or updates a project secret.
// @Summary Set project secret
// @Tags project-secrets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Project UUID"
// @Param body body setProjectSecretRequest true "key_name + plaintext value"
// @Success 201 {object} service.SetProjectSecretOutput
// @Router /projects/{id}/secrets [post]
func (h *ProjectSecretHandler) Set(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid project id")
		return
	}

	var req setProjectSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	out, err := h.svc.Set(c.Request.Context(), service.SetProjectSecretInput{
		ProjectID: projectID,
		KeyName:   req.KeyName,
		Value:     req.Value,
	})
	if err != nil {
		writeProjectSecretError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// Delete removes a project secret.
// @Summary Delete project secret
// @Tags project-secrets
// @Security BearerAuth
// @Param id path string true "Project UUID"
// @Param secret_id path string true "Secret UUID"
// @Success 204
// @Router /projects/{id}/secrets/{secret_id} [delete]
func (h *ProjectSecretHandler) Delete(c *gin.Context) {
	secretID, err := uuid.Parse(c.Param("secret_id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid secret id")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), secretID); err != nil {
		writeProjectSecretError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeProjectSecretError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectSecretValidation),
		errors.Is(err, service.ErrProjectSecretInvalidKey):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrProjectSecretNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrEncryptorNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, apierror.ErrInternalServerError, "encryption is not configured on server")
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	}
}
