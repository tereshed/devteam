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

// SandboxServiceHandler — HTTP-слой деклараций сервис-сайдкаров проекта (Sprint 22).
type SandboxServiceHandler struct {
	service service.SandboxServiceConfigService
}

// NewSandboxServiceHandler создаёт обработчик деклараций сервис-сайдкаров.
func NewSandboxServiceHandler(svc service.SandboxServiceConfigService) *SandboxServiceHandler {
	return &SandboxServiceHandler{service: svc}
}

func writeSandboxServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrProjectNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrProjectForbidden):
		apierror.JSON(c, http.StatusForbidden, apierror.ErrForbidden, err.Error())
	case errors.Is(err, service.ErrSandboxServiceNotFound):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrSandboxServiceInvalidAlias),
		errors.Is(err, service.ErrSandboxServiceInvalidKind),
		errors.Is(err, service.ErrSandboxServiceInvalidSeedKind),
		errors.Is(err, service.ErrSandboxServiceInvalidImage),
		errors.Is(err, service.ErrSandboxServiceInvalidPort),
		errors.Is(err, service.ErrSandboxServiceInvalidTimeout),
		errors.Is(err, service.ErrSandboxServiceInvalidField),
		errors.Is(err, service.ErrSandboxServiceInvalidSeedValue):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Request failed")
	}
}

// List возвращает декларации сервис-сайдкаров проекта.
// @Summary Сервис-сайдкары проекта
// @Description Список эфемерных сервис-сайдкаров (например postgres для интеграционных тестов с БД), которые поднимаются рядом с sandbox-агентом.
// @Tags sandbox-services
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} dto.SandboxServiceListResponse
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/sandbox-services [get]
func (h *SandboxServiceHandler) List(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	items, err := h.service.List(c.Request.Context(), userID, userRole, projectID)
	if err != nil {
		writeSandboxServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToSandboxServiceListResponse(items))
}

// Upsert создаёт/обновляет декларацию сервис-сайдкара (по alias).
// @Summary Создание/обновление сервис-сайдкара
// @Description Upsert декларации сервиса по alias (тип, образ, db_name/db_user, порт, сид, таймаут готовности). Пароль БД не хранится — генерится на каждый прогон.
// @Tags sandbox-services
// @Security BearerAuth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body dto.UpsertSandboxServiceRequest true "Декларация сервиса"
// @Success 200 {object} dto.SandboxServiceConfigResponse
// @Failure 400 {object} apierror.ErrorResponse "Невалидные поля"
// @Failure 401 {object} apierror.ErrorResponse "Не авторизован"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Проект не найден"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/sandbox-services [put]
func (h *SandboxServiceHandler) Upsert(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	var req dto.UpsertSandboxServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	cfg, err := h.service.Upsert(c.Request.Context(), userID, userRole, projectID, req)
	if err != nil {
		writeSandboxServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.ToSandboxServiceConfigResponse(cfg))
}

// Delete удаляет декларацию сервис-сайдкара.
// @Summary Удаление сервис-сайдкара
// @Tags sandbox-services
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param id path string true "Project ID"
// @Param serviceId path string true "Sandbox Service Config ID"
// @Success 204 "Удалено"
// @Failure 403 {object} apierror.ErrorResponse "Нет доступа к проекту"
// @Failure 404 {object} apierror.ErrorResponse "Не найдено"
// @Failure 500 {object} apierror.ErrorResponse "Внутренняя ошибка"
// @Router /projects/{id}/sandbox-services/{serviceId} [delete]
func (h *SandboxServiceHandler) Delete(c *gin.Context) {
	userID, userRole, ok := requireAuth(c)
	if !ok {
		return
	}
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid project ID format")
		return
	}
	serviceID, err := uuid.Parse(c.Param("serviceId"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid service ID format")
		return
	}
	if err := h.service.Delete(c.Request.Context(), userID, userRole, projectID, serviceID); err != nil {
		writeSandboxServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
