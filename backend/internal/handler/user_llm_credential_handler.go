package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
)

const maxLlmCredentialPatchBody = 8 * 1024 // 8 KiB

// UserLlmCredentialHandler GET/PATCH /me/llm-credentials.
type UserLlmCredentialHandler struct {
	svc service.UserLlmCredentialService
}

// NewUserLlmCredentialHandler создаёт хендлер.
func NewUserLlmCredentialHandler(svc service.UserLlmCredentialService) *UserLlmCredentialHandler {
	return &UserLlmCredentialHandler{svc: svc}
}

// Get возвращает маски ключей для текущего пользователя.
// @Summary Список LLM credentials (маски)
// @Description Возвращает masked_preview по каждому провайдеру (без has_key, без полного ключа)
// @Tags me
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.LlmCredentialsResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /me/llm-credentials [get]
func (h *UserLlmCredentialHandler) Get(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}
	out, err := h.svc.GetMasked(c.Request.Context(), uid)
	if err != nil {
		mapLlmCredErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// Patch частично обновляет ключи (batch в одной транзакции).
// @Summary Обновление LLM credentials
// @Description Частичное обновление API-ключей; тело см. dto.PatchLlmCredentialsRequest
// @Tags me
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param request body dto.PatchLlmCredentialsRequest true "Поля set/clear"
// @Success 200 {object} dto.LlmCredentialsResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse
// @Failure 413 {object} apierror.ErrorResponse
// @Failure 415 {object} apierror.ErrorResponse
// @Failure 429 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /me/llm-credentials [patch]
func (h *UserLlmCredentialHandler) Patch(c *gin.Context) {
	uid, ok := getUserID(c)
	if !ok {
		apierror.JSON(c, http.StatusUnauthorized, apierror.ErrAccessDenied, "Unauthorized")
		return
	}

	if c.ContentType() != "application/json" {
		apierror.JSON(c, http.StatusUnsupportedMediaType, apierror.ErrUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxLlmCredentialPatchBody)
	defer c.Request.Body.Close()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			apierror.JSON(c, http.StatusRequestEntityTooLarge, apierror.ErrRequestEntityTooLarge, "Request entity too large")
			return
		}
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid body")
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Empty body")
		return
	}

	req, err := dto.DecodePatchLlmCredentialsJSON(body)
	if err != nil {
		mapLlmCredDecodeErr(c, err)
		return
	}

	out, err := h.svc.Patch(c.Request.Context(), uid, req, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		mapLlmCredErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func mapLlmCredDecodeErr(c *gin.Context, err error) {
	var syn *json.SyntaxError
	if errors.As(err, &syn) {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	var ue *json.UnmarshalTypeError
	if errors.As(err, &ue) {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	if errors.Is(err, dto.ErrTrailingJSONInPatchBody) {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}
	apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
}

func mapLlmCredErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrLlmCredentialsConflictClearAndSet):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
	case errors.Is(err, service.ErrLlmCredentialsKeyTooShort),
		errors.Is(err, service.ErrLlmCredentialsKeyTooLong):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid api key length")
	case errors.Is(err, service.ErrDecryptionFailed):
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to process credentials")
	case errors.Is(err, service.ErrLlmCredentialsConcurrentModify):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, "Credential was modified concurrently, retry the request")
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}
