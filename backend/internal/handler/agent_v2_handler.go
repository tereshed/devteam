package handler

import (
	"errors"
	"net/http"

	"github.com/devteam/backend/pkg/apierror"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// agent_v2_handler.go — Sprint 17 / Sprint 5F.3 — HTTP API для v2 admin operations
// (реестр агентов + секреты). Параллель MCP-инструментам из internal/mcp/tools_agents_v2.go,
// но через REST для Frontend (Flutter) и других HTTP-клиентов.
//
// Все handlers ТОНКИЕ: парсят JSON → вызывают service.AgentService → маппят
// service-sentinel'ы в HTTP-статусы.
//
// Маршруты:
//   GET    /api/v1/agents              — список агентов (с фильтрами через query params)
//   GET    /api/v1/agents/:id          — полная запись агента
//   POST   /api/v1/agents              — создать
//   PUT    /api/v1/agents/:id          — обновить (partial)
//   POST   /api/v1/agents/:id/secrets  — установить/обновить секрет
//   DELETE /api/v1/agents/secrets/:secret_id — удалить секрет

// AgentV2Handler — HTTP-обёртка над AgentService.
type AgentV2Handler struct {
	svc *service.AgentService
}

// NewAgentV2Handler — конструктор.
func NewAgentV2Handler(svc *service.AgentService) *AgentV2Handler {
	return &AgentV2Handler{svc: svc}
}

// ─────────────────────────────────────────────────────────────────────────────
// Request DTOs (handler-level — отдельно от service.CreateAgentInput чтобы JSON tag'и
// и optional pointer-семантика были явными в API-контракте)
// ─────────────────────────────────────────────────────────────────────────────

type listAgentsQuery struct {
	OnlyActive    *bool   `form:"only_active"`
	ExecutionKind *string `form:"execution_kind"`
	Role          *string `form:"role"`
	NameLike      *string `form:"name_like"`
	Limit         *int    `form:"limit"`
	Offset        *int    `form:"offset"`
}

type createAgentRequest struct {
	Name            string   `json:"name" binding:"required"`
	Role            string   `json:"role" binding:"required"`
	ExecutionKind   string   `json:"execution_kind" binding:"required"`
	RoleDescription *string  `json:"role_description,omitempty"`
	SystemPrompt    *string  `json:"system_prompt,omitempty"`
	Model           *string  `json:"model,omitempty"`
	CodeBackend     *string  `json:"code_backend,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
	IsActive        *bool    `json:"is_active,omitempty"`
}

type updateAgentRequest struct {
	RoleDescription *string  `json:"role_description,omitempty"`
	SystemPrompt    *string  `json:"system_prompt,omitempty"`
	Model           *string  `json:"model,omitempty"`
	CodeBackend     *string  `json:"code_backend,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
	IsActive        *bool    `json:"is_active,omitempty"`
}

type setSecretRequest struct {
	KeyName string `json:"key_name" binding:"required"`
	Value   string `json:"value" binding:"required"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

// List возвращает список агентов с фильтрами и пагинацией.
// @Summary List v2 agents
// @Description Реестр LLM/sandbox-агентов с фильтрами (only_active, execution_kind, role, name_like) и пагинацией. system_prompt НЕ включается; полную запись — через GET /agents/:id.
// @Tags agents-v2
// @Security BearerAuth
// @Produce json
// @Param only_active query bool false "только is_active=true"
// @Param execution_kind query string false "llm | sandbox"
// @Param role query string false "router/planner/decomposer/reviewer/developer/merger/tester/..."
// @Param name_like query string false "частичный поиск по name"
// @Param limit query int false "1-200; default 50"
// @Param offset query int false "default 0"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Router /agents [get]
func (h *AgentV2Handler) List(c *gin.Context) {
	var q listAgentsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	f := repository.AgentFilter{}
	if q.OnlyActive != nil {
		f.OnlyActive = *q.OnlyActive
	}
	if q.ExecutionKind != nil {
		k := models.AgentExecutionKind(*q.ExecutionKind)
		if !k.IsValid() {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid execution_kind")
			return
		}
		f.ExecutionKind = &k
	}
	if q.Role != nil {
		r := models.AgentRole(*q.Role)
		if !r.IsValid() {
			apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid role")
			return
		}
		f.Role = &r
	}
	if q.NameLike != nil {
		f.NameLike = *q.NameLike
	}
	if q.Limit != nil {
		f.Limit = *q.Limit
	}
	if q.Offset != nil {
		f.Offset = *q.Offset
	}

	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":  total,
		"items":  items,
		"limit":  f.Limit,
		"offset": f.Offset,
	})
}

// Get возвращает полную запись агента (включая system_prompt).
// @Summary Get v2 agent by ID
// @Tags agents-v2
// @Security BearerAuth
// @Produce json
// @Param id path string true "Agent UUID"
// @Success 200 {object} models.Agent
// @Failure 404 {object} apierror.ErrorResponse
// @Router /agents/{id} [get]
func (h *AgentV2Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}
	a, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		writeAgentServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

// Create создаёт нового агента.
// @Summary Create v2 agent
// @Tags agents-v2
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body createAgentRequest true "agent payload"
// @Success 201 {object} models.Agent
// @Failure 400 {object} apierror.ErrorResponse "validation"
// @Failure 409 {object} apierror.ErrorResponse "name already taken"
// @Router /agents [post]
func (h *AgentV2Handler) Create(c *gin.Context) {
	var req createAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	in := service.CreateAgentInput{
		Name:            req.Name,
		Role:            models.AgentRole(req.Role),
		ExecutionKind:   models.AgentExecutionKind(req.ExecutionKind),
		RoleDescription: req.RoleDescription,
		SystemPrompt:    req.SystemPrompt,
		Temperature:     req.Temperature,
		MaxTokens:       req.MaxTokens,
		IsActive:        req.IsActive,
	}
	if req.Model != nil && *req.Model != "" {
		in.Model = req.Model
	}
	if req.CodeBackend != nil && *req.CodeBackend != "" {
		cb := models.CodeBackend(*req.CodeBackend)
		in.CodeBackend = &cb
	}
	a, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		writeAgentServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, a)
}

// Update обновляет агента (partial).
// @Summary Update v2 agent
// @Tags agents-v2
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Agent UUID"
// @Param body body updateAgentRequest true "patch fields"
// @Success 200 {object} models.Agent
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 404 {object} apierror.ErrorResponse
// @Failure 409 {object} apierror.ErrorResponse "concurrent update"
// @Router /agents/{id} [put]
func (h *AgentV2Handler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}
	var req updateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	in := service.UpdateAgentInput{
		RoleDescription: req.RoleDescription,
		SystemPrompt:    req.SystemPrompt,
		Model:           req.Model,
		Temperature:     req.Temperature,
		MaxTokens:       req.MaxTokens,
		IsActive:        req.IsActive,
	}
	if req.CodeBackend != nil && *req.CodeBackend != "" {
		cb := models.CodeBackend(*req.CodeBackend)
		in.CodeBackend = &cb
	}
	a, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		writeAgentServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, a)
}

// SetSecret устанавливает/обновляет секрет агента.
// @Summary Set v2 agent secret (encrypted)
// @Description Шифрует value через AES-256-GCM (AAD = secret.id), сохраняет в agent_secrets. Back-read невозможен через API.
// @Tags agents-v2
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Agent UUID"
// @Param body body setSecretRequest true "key_name + plaintext value"
// @Success 201 {object} service.SetSecretOutput
// @Failure 400 {object} apierror.ErrorResponse
// @Router /agents/{id}/secrets [post]
func (h *AgentV2Handler) SetSecret(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid agent id")
		return
	}
	var req setSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
		return
	}
	out, err := h.svc.SetSecret(c.Request.Context(), service.SetSecretInput{
		AgentID: agentID,
		KeyName: req.KeyName,
		Value:   req.Value,
	})
	if err != nil {
		writeAgentServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// DeleteSecret удаляет секрет по UUID записи.
// @Summary Delete v2 agent secret
// @Tags agents-v2
// @Security BearerAuth
// @Produce json
// @Param secret_id path string true "Secret UUID"
// @Success 204
// @Failure 404 {object} apierror.ErrorResponse
// @Router /agents/secrets/{secret_id} [delete]
func (h *AgentV2Handler) DeleteSecret(c *gin.Context) {
	id, err := uuid.Parse(c.Param("secret_id"))
	if err != nil {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "invalid secret id")
		return
	}
	if err := h.svc.DeleteSecret(c.Request.Context(), id); err != nil {
		writeAgentServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// writeAgentServiceError — маппинг service-sentinel'ов в HTTP-статусы.
func writeAgentServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrAgentValidation),
		errors.Is(err, service.ErrAgentSecretInvalidKey):
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, err.Error())
	case errors.Is(err, service.ErrAgentNameAlreadyTaken):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, err.Error())
	case errors.Is(err, service.ErrAgentNotInRegistry):
		apierror.JSON(c, http.StatusNotFound, apierror.ErrNotFound, err.Error())
	case errors.Is(err, service.ErrAgentConcurrentUpdate):
		apierror.JSON(c, http.StatusConflict, apierror.ErrConflict, "agent was modified concurrently — please reload and retry")
	case errors.Is(err, service.ErrEncryptorNotConfigured):
		apierror.JSON(c, http.StatusServiceUnavailable, apierror.ErrInternalServerError, "encryption is not configured on server")
	default:
		apierror.JSON(c, http.StatusInternalServerError, apierror.ErrInternalServerError, err.Error())
	}
}
