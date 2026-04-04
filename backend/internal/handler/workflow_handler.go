package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
)

type WorkflowHandler struct {
	engine service.WorkflowEngine
}

func NewWorkflowHandler(engine service.WorkflowEngine) *WorkflowHandler {
	return &WorkflowHandler{engine: engine}
}

// Start запускает воркфлоу по имени
// @Summary Запуск воркфлоу
// @Description Запускает новый процесс выполнения воркфлоу
// @Tags workflows
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param name path string true "Имя воркфлоу"
// @Param request body dto.StartWorkflowRequest true "Входные данные"
// @Success 200 {object} dto.ExecutionResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /workflows/{name}/start [post]
func (h *WorkflowHandler) Start(c *gin.Context) {
	name := c.Param("name")
	var req dto.StartWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}

	execution, err := h.engine.StartWorkflow(c.Request.Context(), name, req.Input)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, h.mapExecutionToDTO(execution))
}

// List возвращает список всех воркфлоу
// @Summary Список воркфлоу
// @Description Возвращает все доступные воркфлоу
// @Tags workflows
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Success 200 {array} dto.WorkflowResponse
// @Router /workflows [get]
func (h *WorkflowHandler) List(c *gin.Context) {
	wfs, err := h.engine.ListWorkflows(c.Request.Context())
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to list workflows")
		return
	}

	var response []dto.WorkflowResponse
	for _, wf := range wfs {
		response = append(response, dto.WorkflowResponse{
			ID:          wf.ID.String(),
			Name:        wf.Name,
			Description: wf.Description,
			IsActive:    wf.IsActive,
			CreatedAt:   wf.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, response)
}

// ListExecutions возвращает список выполнений
// @Summary Список выполнений
// @Description Возвращает историю запусков с пагинацией
// @Tags workflows
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} dto.ExecutionListResponse
// @Router /executions [get]
func (h *WorkflowHandler) ListExecutions(c *gin.Context) {
	// Simple pagination params (can be improved)
	limit := 20
	offset := 0

	execs, total, err := h.engine.ListExecutions(c.Request.Context(), limit, offset)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to list executions")
		return
	}

	var list []dto.ExecutionResponse
	for _, e := range execs {
		list = append(list, h.mapExecutionToDTO(&e))
	}

	c.JSON(http.StatusOK, dto.ExecutionListResponse{
		Executions: list,
		Total:      total,
	})
}

// GetExecutionSteps возвращает шаги выполнения
// @Summary Шаги выполнения
// @Description Возвращает детализацию шагов конкретного запуска
// @Tags workflows
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param id path string true "Execution ID"
// @Success 200 {array} dto.ExecutionStepResponse
// @Router /executions/{id}/steps [get]
func (h *WorkflowHandler) GetExecutionSteps(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid UUID")
		return
	}

	steps, err := h.engine.GetExecutionSteps(c.Request.Context(), id)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to get steps")
		return
	}

	var response []dto.ExecutionStepResponse
	for _, s := range steps {
		agentName := ""
		if s.Agent != nil {
			agentName = s.Agent.Name
		}
		response = append(response, dto.ExecutionStepResponse{
			ID:            s.ID.String(),
			StepID:        s.StepID,
			AgentName:     agentName,
			InputContext:  s.InputContext,
			OutputContent: s.OutputContent,
			DurationMs:    s.DurationMs,
			TokensUsed:    s.TokensUsed,
			CreatedAt:     s.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, response)
}

// GetExecution получает статус выполнения
// @Summary Получение статуса выполнения
// @Description Возвращает текущее состояние процесса
// @Tags workflows
// @Accept json
// @Produce json
// @Security BearerAuth
// @Security ApiKeyAuth
// @Security OAuth2Password
// @Param id path string true "ID выполнения (UUID)"
// @Success 200 {object} dto.ExecutionResponse
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /executions/{id} [get]
func (h *WorkflowHandler) GetExecution(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid UUID")
		return
	}

	execution, err := h.engine.GetExecution(c.Request.Context(), id)
	if err != nil {
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, "Execution not found")
		return
	}

	c.JSON(http.StatusOK, h.mapExecutionToDTO(execution))
}

func (h *WorkflowHandler) mapExecutionToDTO(e *models.Execution) dto.ExecutionResponse {
	return dto.ExecutionResponse{
		ID:            e.ID.String(),
		WorkflowID:    e.WorkflowID.String(),
		Status:        string(e.Status),
		CurrentStepID: e.CurrentStepID,
		InputData:     e.InputData,
		OutputData:    e.OutputData,
		StepCount:     e.StepCount,
		CreatedAt:     e.CreatedAt,
		FinishedAt:    e.FinishedAt,
		ErrorMessage:  e.ErrorMessage,
	}
}

