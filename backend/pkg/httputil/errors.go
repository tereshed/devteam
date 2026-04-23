package httputil

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
)

// RespondError централизованно маппит ошибки сервиса в HTTP-статусы и отправляет JSON-ответ
func RespondError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	switch {
	// Ошибки поиска (404)
	case errors.Is(err, service.ErrProjectNotFound),
		errors.Is(err, service.ErrConversationNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())

	// Ошибки доступа (403)
	case errors.Is(err, service.ErrProjectForbidden),
		errors.Is(err, service.ErrConversationForbidden),
		errors.Is(err, service.ErrGitCredentialForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())

	// Ошибки конфликта (409)
	case errors.Is(err, service.ErrProjectNameExists):
		apierror.JSON(c, http.StatusConflict, apierror.ErrAlreadyExists, err.Error())
	case errors.Is(err, service.ErrProjectIndexingConflict):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())

	// Ошибки внешних сервисов (502)
	case errors.Is(err, service.ErrGitValidationFailed),
		errors.Is(err, service.ErrGitCloneFailed):
		apierror.JSON(c, http.StatusBadGateway, apierror.ErrExternalService, err.Error())

	// Ошибки лимитов (429)
	case errors.Is(err, service.ErrMessageRateLimit):
		apierror.JSON(c, http.StatusTooManyRequests, apierror.ErrTooManyRequests, err.Error())

	// Ошибки валидации и некорректных запросов (400)
	case errors.Is(err, service.ErrGitURLRequired),
		errors.Is(err, service.ErrGitCredentialRequired),
		errors.Is(err, service.ErrGitCredentialNotSupportedForLocal),
		errors.Is(err, service.ErrInvalidConversationTitle),
		errors.Is(err, service.ErrInvalidMessageContent),
		errors.Is(err, service.ErrGitCredentialNotFound),
		errors.Is(err, service.ErrProjectInvalidName),
		errors.Is(err, service.ErrProjectInvalidProvider),
		errors.Is(err, service.ErrProjectInvalidStatus),
		errors.Is(err, service.ErrProjectLocalCannotReindex),
		errors.Is(err, service.ErrUpdateProjectGitCredentialConflict),
		errors.Is(err, service.ErrUpdateProjectTechStackConflict),
		errors.Is(err, service.ErrUpdateProjectSettingsConflict):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())

	// Внутренние ошибки (500)
	case errors.Is(err, service.ErrDecryptionFailed):
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to process credentials")

	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}
