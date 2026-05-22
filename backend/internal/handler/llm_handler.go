package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/devteam/backend/pkg/llm"
	"github.com/gin-gonic/gin"
)

// LLMHandler handles LLM related requests
type LLMHandler struct {
	llmService      service.LLMService
	userCreds       service.UserLlmCredentialService
	claudeCodeAuth  service.ClaudeCodeAuthService
	antigravityAuth service.AntigravityAuthService
	cfg             *config.Config
}

// NewLLMHandler creates a new instance of LLMHandler
func NewLLMHandler(
	llmService service.LLMService,
	userCreds service.UserLlmCredentialService,
	claudeCodeAuth service.ClaudeCodeAuthService,
	antigravityAuth service.AntigravityAuthService,
	cfg *config.Config,
) *LLMHandler {
	return &LLMHandler{
		llmService:      llmService,
		userCreds:       userCreds,
		claudeCodeAuth:  claudeCodeAuth,
		antigravityAuth: antigravityAuth,
		cfg:             cfg,
	}
}

// Chat handles the chat generation request
// @Summary Chat with LLM
// @Description Generates a response from the LLM provider
// @Tags llm
// @Security BearerAuth
// @Security ApiKeyAuth
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
// @Security ApiKeyAuth
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

var fallbackModels = map[string][]string{
	"anthropic": {
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-latest",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	},
	"anthropic_oauth": {
		"claude-3-5-sonnet-20241022",
		"claude-3-5-sonnet-latest",
		"claude-3-opus-20240229",
		"claude-3-sonnet-20240229",
		"claude-3-haiku-20240307",
	},
	"deepseek": {
		"deepseek-chat",
		"deepseek-reasoner",
	},
	"zhipu": {
		"glm-4-plus",
		"glm-4-flash",
		"glm-4-air",
		"glm-4-zero",
	},
	"openrouter": {
		"anthropic/claude-3.5-sonnet",
		"deepseek/deepseek-chat",
		"google/gemini-2.5-pro",
		"google/gemini-2.5-flash",
		"openai/gpt-4o",
		"meta-llama/llama-3.3-70b-instruct",
	},
	"antigravity": {
		"antigravity-default",
	},
	"antigravity_oauth": {
		"antigravity-default",
	},
}

// ListModels returns a list of available models for a given provider
// @Summary List available LLM models for a provider
// @Description Fetches available models from the provider or returns fallback models if unavailable
// @Tags llm
// @Security BearerAuth
// @Security ApiKeyAuth
// @Produce json
// @Param provider query string true "Provider Kind (e.g. anthropic, deepseek, openrouter, antigravity)"
// @Success 200 {array} string
// @Failure 400 {object} apierror.ErrorResponse
// @Failure 401 {object} apierror.ErrorResponse
// @Router /llm/models [get]
func (h *LLMHandler) ListModels(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "provider parameter is required")
		return
	}

	fallback, exists := fallbackModels[provider]
	if !exists {
		apierror.JSON(c, http.StatusBadRequest, apierror.ErrBadRequest, "unknown provider: "+provider)
		return
	}

	uid, authOk := getUserID(c)
	if !authOk {
		c.JSON(http.StatusOK, fallback)
		return
	}

	var token string
	var err error
	ctx := c.Request.Context()

	switch provider {
	case "anthropic":
		token, err = h.userCreds.GetPlaintext(ctx, uid, models.UserLLMProviderAnthropic)
	case "anthropic_oauth":
		token, err = h.claudeCodeAuth.AccessTokenForSandbox(ctx, uid)
	case "deepseek":
		token, err = h.userCreds.GetPlaintext(ctx, uid, models.UserLLMProviderDeepSeek)
	case "zhipu":
		token, err = h.userCreds.GetPlaintext(ctx, uid, models.UserLLMProviderZhipu)
	case "openrouter":
		token, err = h.userCreds.GetPlaintext(ctx, uid, models.UserLLMProviderOpenRouter)
	case "antigravity":
		token, err = h.userCreds.GetPlaintext(ctx, uid, models.UserLLMProviderAntigravity)
	case "antigravity_oauth":
		token, err = h.antigravityAuth.AccessTokenForSandbox(ctx, uid)
	}

	if (err != nil || token == "") && h.cfg != nil {
		switch provider {
		case "anthropic", "anthropic_oauth":
			token = h.cfg.LLM.Anthropic.APIKey
		case "deepseek":
			token = h.cfg.LLM.Deepseek.APIKey
		case "zhipu":
			token = h.cfg.LLM.Zhipu.APIKey
		case "openrouter":
			token = h.cfg.LLM.OpenRouter.APIKey
		case "antigravity", "antigravity_oauth":
			token = h.cfg.LLM.Antigravity.APIKey
		}
	}

	if token == "" && provider != "openrouter" {
		log.Printf("Warning: credentials missing for provider %q (user: %s)", provider, uid)
		c.JSON(http.StatusOK, fallback)
		return
	}

	var reqURL string
	var headers = make(map[string]string)

	switch provider {
	case "anthropic", "anthropic_oauth":
		reqURL = "https://api.anthropic.com/v1/models"
		headers["anthropic-version"] = "2023-06-01"
		if provider == "anthropic" || (h.cfg != nil && token == h.cfg.LLM.Anthropic.APIKey && token != "") {
			headers["x-api-key"] = token
		} else {
			headers["Authorization"] = "Bearer " + token
		}
	case "deepseek":
		reqURL = "https://api.deepseek.com/models"
		headers["Authorization"] = "Bearer " + token
	case "zhipu":
		reqURL = "https://open.bigmodel.cn/api/paas/v4/models"
		headers["Authorization"] = "Bearer " + token
	case "openrouter":
		reqURL = "https://openrouter.ai/api/v1/models"
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	case "antigravity", "antigravity_oauth":
		reqURL = "https://api.antigravity.ai/v1/models"
		headers["Authorization"] = "Bearer " + token
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		log.Printf("Warning: failed to create HTTP request for provider %q: %v", provider, err)
		c.JSON(http.StatusOK, fallback)
		return
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("Warning: HTTP request failed for provider %q: %v", provider, err)
		c.JSON(http.StatusOK, fallback)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Warning: provider %q API returned status %d", provider, resp.StatusCode)
		c.JSON(http.StatusOK, fallback)
		return
	}

	var responseData struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		log.Printf("Warning: failed to decode JSON response for provider %q: %v", provider, err)
		c.JSON(http.StatusOK, fallback)
		return
	}

	var modelIDs []string
	for _, item := range responseData.Data {
		if item.ID != "" {
			modelIDs = append(modelIDs, item.ID)
		}
	}

	if len(modelIDs) == 0 {
		log.Printf("Warning: provider %q returned empty model list", provider)
		c.JSON(http.StatusOK, fallback)
		return
	}

	c.JSON(http.StatusOK, modelIDs)
}
