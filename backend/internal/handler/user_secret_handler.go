package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserSecretHandler struct {
	svc *service.UserSecretService
}

func NewUserSecretHandler(svc *service.UserSecretService) *UserSecretHandler {
	return &UserSecretHandler{svc: svc}
}

type userSecretResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	KeyName   string    `json:"key_name"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

type setUserSecretRequest struct {
	KeyName string `json:"key_name" binding:"required"`
	Value   string `json:"value" binding:"required"`
}

// List returns all user secret keys (without values).
// @Summary List user secrets
// @Tags user-secrets
// @Security BearerAuth
// @Produce json
// @Success 200 {array} userSecretResponse
// @Router /me/secrets [get]
func (h *UserSecretHandler) List(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "unauthorized")
		return
	}

	secrets, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	result := make([]userSecretResponse, 0, len(secrets))
	for _, s := range secrets {
		result = append(result, userSecretResponse{
			ID:        s.ID,
			UserID:    s.UserID,
			KeyName:   s.KeyName,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	c.JSON(http.StatusOK, result)
}

// Set creates or updates a user secret.
// @Summary Set user secret
// @Tags user-secrets
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body setUserSecretRequest true "key_name + plaintext value"
// @Success 201 {object} service.SetUserSecretOutput
// @Router /me/secrets [post]
func (h *UserSecretHandler) Set(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "unauthorized")
		return
	}

	var req setUserSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	out, err := h.svc.Set(c.Request.Context(), service.SetUserSecretInput{
		UserID:  userID,
		KeyName: req.KeyName,
		Value:   req.Value,
	})
	if err != nil {
		writeUserSecretError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// Delete removes a user secret.
// @Summary Delete user secret
// @Tags user-secrets
// @Security BearerAuth
// @Param secret_id path string true "Secret UUID"
// @Success 204
// @Router /me/secrets/{secret_id} [delete]
func (h *UserSecretHandler) Delete(c *gin.Context) {
	secretID, err := uuid.Parse(c.Param("secret_id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid secret id")
		return
	}

	if err := h.svc.Delete(c.Request.Context(), secretID); err != nil {
		writeUserSecretError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writeUserSecretError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserSecretValidation),
		errors.Is(err, service.ErrUserSecretInvalidKey):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrUserSecretNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrEncryptorNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, apierror.ErrInternalServerError, "encryption is not configured on server")
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	}
}
