package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/devteam/backend/pkg/llm"
)

// LLMHandler handles LLM related requests
type LLMHandler struct {
	llmService service.LLMService
}

// NewLLMHandler creates a new instance of LLMHandler
func NewLLMHandler(llmService service.LLMService) *LLMHandler {
	return &LLMHandler{
		llmService: llmService,
	}
}

// Chat handles the chat generation request
// @Summary Chat with LLM
// @Description Generates a response from the LLM provider
// @Tags llm
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param request body llm.Request true "Chat Request"
// @Success 200 {object} llm.Response
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Failure 403 {object} apierror.ErrorResponse
// @Failure 500 {object} apierror.ErrorResponse
// @Router /llm/chat [post]
func (h *LLMHandler) Chat(c *gin.Context) {
	var req llm.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "Invalid request body")
		return
	}

	resp, err := h.llmService.Generate(c.Request.Context(), req)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListLogs возвращает список логов LLM
// @Summary Список логов LLM
// @Description Возвращает историю запросов к LLM с пагинацией
// @Tags llm
// @Security BearerAuth
// @Security OAuth2Password
// @Accept json
// @Produce json
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Success 200 {object} dto.LLMLogListResponse
// @Router /llm/logs [get]
func (h *LLMHandler) ListLogs(c *gin.Context) {
	limit := 50
	offset := 0
	// TODO: Parse query params

	logs, total, err := h.llmService.ListLogs(c.Request.Context(), limit, offset)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, "Failed to list logs")
		return
	}

	var list []dto.LLMLogResponse
	for _, l := range logs {
		wfID := ""
		if l.WorkflowExecutionID != nil {
			wfID = l.WorkflowExecutionID.String()
		}
		agentID := ""
		if l.AgentID != nil {
			agentID = l.AgentID.String()
		}

		list = append(list, dto.LLMLogResponse{
			ID:                  l.ID.String(),
			Provider:            l.Provider,
			Model:               l.Model,
			InputTokens:         l.InputTokens,
			OutputTokens:        l.OutputTokens,
			TotalTokens:         l.TotalTokens,
			Cost:                l.Cost,
			DurationMs:          l.DurationMs,
			WorkflowExecutionID: wfID,
			StepID:              l.StepID,
			AgentID:             agentID,
			ErrorMessage:        l.ErrorMessage,
			CreatedAt:           l.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, dto.LLMLogListResponse{
		Logs:  list,
		Total: total,
	})
}
