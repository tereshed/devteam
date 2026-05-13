package mcp

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// actorFromMCPContext — Sprint 15.B (B4): builds AgentSettingsActor from MCP request ctx
// (populated by auth middleware). Если user не аутентифицирован — возвращает ok=false.
func actorFromMCPContext(ctx context.Context) (service.AgentSettingsActor, bool) {
	uid, ok := UserIDFromContext(ctx)
	if !ok {
		return service.AgentSettingsActor{}, false
	}
	role, _ := UserRoleFromContext(ctx)
	return service.AgentSettingsActor{UserID: uid, IsAdmin: role == models.RoleAdmin}, true
}

// AgentSettingsGetParams — параметры agent_settings_get.
type AgentSettingsGetParams struct {
	AgentID string `json:"agent_id" jsonschema:"required,description=UUID агента"`
}

// AgentSettingsUpdateParams — параметры agent_settings_update.
type AgentSettingsUpdateParams struct {
	AgentID             string          `json:"agent_id" jsonschema:"required,description=UUID агента"`
	LLMProviderID       *string         `json:"llm_provider_id,omitempty" jsonschema:"description=UUID LLM-провайдера (или null/опустить для сохранения текущего)"`
	ClearLLMProvider    bool            `json:"clear_llm_provider,omitempty" jsonschema:"description=Если true — сбрасывает llm_provider_id в null"`
	CodeBackend         *string         `json:"code_backend,omitempty" jsonschema:"description=claude-code | aider | custom"`
	CodeBackendSettings json.RawMessage `json:"code_backend_settings,omitempty" jsonschema:"description=JSON-объект code_backend_settings"`
	SandboxPermissions  json.RawMessage `json:"sandbox_permissions,omitempty" jsonschema:"description=JSON-объект permissions (allow/deny/defaultMode)"`
}

// MCPServerListParams — параметры mcp_server_list.
type MCPServerListParams struct {
	OnlyActive bool `json:"only_active,omitempty" jsonschema:"description=Только активные записи (is_active=true)"`
}

// SkillListParams — параметры skill_list.
type SkillListParams struct {
	AgentID    *string `json:"agent_id,omitempty" jsonschema:"description=Если задан — только Skills этого агента"`
	OnlyActive bool    `json:"only_active,omitempty" jsonschema:"description=Только активные записи"`
}

// RegisterAgentSettingsTools — Sprint 15.24.
// Sprint 15.minor — логируем, какие инструменты отключены из-за nil deps:
// при отладке полезно видеть «agent_settings_get skipped: teamSvc=nil», а не молчание.
func RegisterAgentSettingsTools(
	server *mcp.Server,
	teamSvc service.TeamService,
	mcpRegistry repository.MCPServerRegistryRepository,
	skills repository.AgentSkillRepository,
) {
	if teamSvc == nil {
		slog.Warn("mcp: agent_settings_get/update tools skipped (TeamService is nil)")
	}
	if mcpRegistry == nil {
		slog.Warn("mcp: mcp_server_list tool skipped (MCPServerRegistryRepo is nil)")
	}
	if skills == nil {
		slog.Warn("mcp: skill_list tool skipped (AgentSkillRepo is nil)")
	}
	if teamSvc != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "agent_settings_get",
			Description: "Получить per-agent настройки (llm_provider_id, code_backend, code_backend_settings, sandbox_permissions).",
		}, makeAgentSettingsGetHandler(teamSvc))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "agent_settings_update",
			Description: "Обновить per-agent настройки. Эквивалент PUT /agents/{id}/settings.",
		}, makeAgentSettingsUpdateHandler(teamSvc))
	}

	if mcpRegistry != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name: "mcp_server_list",
			Description: "Список MCP-серверов из ГЛОБАЛЬНОГО реестра mcp_servers_registry. " +
				"Доступен любому аутентифицированному пользователю — это каталог совместимых серверов, " +
				"а не per-user/per-project данные. env_template возвращается без values (только ключи) " +
				"для предотвращения утечки секретов (Sprint 15.M9).",
		}, makeMCPServerListHandler(mcpRegistry))
	}

	if skills != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "skill_list",
			Description: "Список Claude Code skills, доступных агентам (опционально — для конкретного agent_id).",
		}, makeSkillListHandler(skills, teamSvc))
	}
}

func makeAgentSettingsGetHandler(svc service.TeamService) func(ctx context.Context, req *mcp.CallToolRequest, params *AgentSettingsGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AgentSettingsGetParams) (*mcp.CallToolResult, any, error) {
		actor, ok := actorFromMCPContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		id, err := uuid.Parse(params.AgentID)
		if err != nil {
			return ValidationErr("invalid agent_id")
		}
		a, err := svc.GetAgentSettings(ctx, actor, id)
		if err != nil {
			return Err("failed to get agent settings", err)
		}
		// Sprint 15.Major DRY: единый маппер вместо локальной копии.
		return OK("agent settings", dto.AgentSettingsResponseFromModel(a))
	}
}

func makeAgentSettingsUpdateHandler(svc service.TeamService) func(ctx context.Context, req *mcp.CallToolRequest, params *AgentSettingsUpdateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AgentSettingsUpdateParams) (*mcp.CallToolResult, any, error) {
		actor, ok := actorFromMCPContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		id, err := uuid.Parse(params.AgentID)
		if err != nil {
			return ValidationErr("invalid agent_id")
		}
		req := dto.UpdateAgentSettingsRequest{
			ClearLLMProvider:    params.ClearLLMProvider,
			CodeBackend:         params.CodeBackend,
			CodeBackendSettings: params.CodeBackendSettings,
			SandboxPermissions:  params.SandboxPermissions,
		}
		if params.LLMProviderID != nil {
			pid, err := uuid.Parse(*params.LLMProviderID)
			if err != nil {
				return ValidationErr("invalid llm_provider_id")
			}
			req.LLMProviderID = &pid
		}
		a, err := svc.UpdateAgentSettings(ctx, actor, id, req)
		if err != nil {
			return Err("failed to update agent settings", err)
		}
		return OK("agent settings updated", dto.AgentSettingsResponseFromModel(a))
	}
}

// MCPServerListItem — Sprint 15.M9 фильтр: возвращаем имена ключей env_template,
// но НЕ их значения (там могут быть плейсхолдеры с угаданными именами секретов
// или placeholder-syntax типа "${OPENAI_API_KEY}" с реальным ключом).
type MCPServerListItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Transport   string   `json:"transport"`
	Command     string   `json:"command,omitempty"`
	URL         string   `json:"url,omitempty"`
	Scope       string   `json:"scope"`
	IsActive    bool     `json:"is_active"`
	EnvKeys     []string `json:"env_keys"`
}

func makeMCPServerListHandler(repo repository.MCPServerRegistryRepository) func(ctx context.Context, req *mcp.CallToolRequest, params *MCPServerListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *MCPServerListParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		items, err := repo.List(ctx, params.OnlyActive)
		if err != nil {
			return Err("failed to list mcp servers", err)
		}
		out := make([]MCPServerListItem, 0, len(items))
		for i := range items {
			it := items[i]
			envKeys := []string{}
			if len(it.EnvTemplate) > 0 {
				var tmpl map[string]string
				if err := json.Unmarshal(it.EnvTemplate, &tmpl); err != nil {
					// Sprint 15.minor: пишем warning вместо silent ignore, чтобы кривой
					// env_template в БД не оставался без диагностики.
					slog.Warn("mcp_server_list: failed to parse env_template; returning empty env_keys",
						"server", it.Name, "err", err)
				}
				for k := range tmpl {
					envKeys = append(envKeys, k)
				}
			}
			out = append(out, MCPServerListItem{
				ID:          it.ID.String(),
				Name:        it.Name,
				Description: it.Description,
				Transport:   string(it.Transport),
				Command:     it.Command,
				URL:         it.URL,
				Scope:       string(it.Scope),
				IsActive:    it.IsActive,
				EnvKeys:     envKeys,
			})
		}
		return OK("mcp servers", out)
	}
}

func makeSkillListHandler(
	repo repository.AgentSkillRepository,
	teamSvc service.TeamService,
) func(ctx context.Context, req *mcp.CallToolRequest, params *SkillListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *SkillListParams) (*mcp.CallToolResult, any, error) {
		actor, ok := actorFromMCPContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		if params.AgentID != nil {
			id, err := uuid.Parse(*params.AgentID)
			if err != nil {
				return ValidationErr("invalid agent_id")
			}
			// Sprint 15.Major (MCP ownership): per-agent skill_list требует ownership-check
			// агента — иначе user A может перечислять skills user B.
			if teamSvc != nil {
				if _, err := teamSvc.GetAgentSettings(ctx, actor, id); err != nil {
					return Err("agent not found or access denied", err)
				}
			}
			items, err := repo.ListByAgent(ctx, id, params.OnlyActive)
			if err != nil {
				return Err("failed to list skills", err)
			}
			return OK("skills for agent", items)
		}
		// Без agent_id — список глобальных skills (admin-only).
		if !actor.IsAdmin {
			return ValidationErr("agent_id is required for non-admin caller")
		}
		items, err := repo.ListAll(ctx, params.OnlyActive)
		if err != nil {
			return Err("failed to list skills", err)
		}
		return OK("skills", items)
	}
}

