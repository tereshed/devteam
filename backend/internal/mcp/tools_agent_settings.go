package mcp

import (
	"context"
	"encoding/json"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AgentSettingsGetParams — параметры agent_settings_get.
type AgentSettingsGetParams struct {
	AgentID string `json:"agent_id" jsonschema:"required,description=UUID агента"`
}

// AgentSettingsUpdateParams — параметры agent_settings_update.
type AgentSettingsUpdateParams struct {
	AgentID             string          `json:"agent_id" jsonschema:"required,description=UUID агента"`
	LLMProviderID       *string         `json:"llm_provider_id,omitempty" jsonschema:"description=UUID LLM-провайдера (или null/опустить для сохранения текущего)"`
	ClearLLMProvider    bool            `json:"clear_llm_provider,omitempty" jsonschema:"description=Если true — сбрасывает llm_provider_id в null"`
	CodeBackend         *string         `json:"code_backend,omitempty" jsonschema:"description=claude-code | claude-code-via-proxy | aider | custom"`
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
func RegisterAgentSettingsTools(
	server *mcp.Server,
	teamSvc service.TeamService,
	mcpRegistry repository.MCPServerRegistryRepository,
	skills repository.AgentSkillRepository,
) {
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
			Name:        "mcp_server_list",
			Description: "Список MCP-серверов из реестра mcp_servers_registry.",
		}, makeMCPServerListHandler(mcpRegistry))
	}

	if skills != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "skill_list",
			Description: "Список Claude Code skills, доступных агентам (опционально — для конкретного agent_id).",
		}, makeSkillListHandler(skills))
	}
}

func makeAgentSettingsGetHandler(svc service.TeamService) func(ctx context.Context, req *mcp.CallToolRequest, params *AgentSettingsGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AgentSettingsGetParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		id, err := uuid.Parse(params.AgentID)
		if err != nil {
			return ValidationErr("invalid agent_id")
		}
		a, err := svc.GetAgentSettings(ctx, id)
		if err != nil {
			return Err("failed to get agent settings", err)
		}
		resp := dto.AgentSettingsResponse{
			AgentID:             a.ID,
			LLMProviderID:       a.LLMProviderID,
			CodeBackendSettings: rawOrEmptyObject(a.CodeBackendSettings),
			SandboxPermissions:  rawOrEmptyObject(a.SandboxPermissions),
		}
		if a.CodeBackend != nil {
			s := string(*a.CodeBackend)
			resp.CodeBackend = &s
		}
		return OK("agent settings", resp)
	}
}

func makeAgentSettingsUpdateHandler(svc service.TeamService) func(ctx context.Context, req *mcp.CallToolRequest, params *AgentSettingsUpdateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *AgentSettingsUpdateParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
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
		a, err := svc.UpdateAgentSettings(ctx, id, req)
		if err != nil {
			return Err("failed to update agent settings", err)
		}
		resp := dto.AgentSettingsResponse{
			AgentID:             a.ID,
			LLMProviderID:       a.LLMProviderID,
			CodeBackendSettings: rawOrEmptyObject(a.CodeBackendSettings),
			SandboxPermissions:  rawOrEmptyObject(a.SandboxPermissions),
		}
		if a.CodeBackend != nil {
			s := string(*a.CodeBackend)
			resp.CodeBackend = &s
		}
		return OK("agent settings updated", resp)
	}
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
		return OK("mcp servers", items)
	}
}

func makeSkillListHandler(repo repository.AgentSkillRepository) func(ctx context.Context, req *mcp.CallToolRequest, params *SkillListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, params *SkillListParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		if params.AgentID != nil {
			id, err := uuid.Parse(*params.AgentID)
			if err != nil {
				return ValidationErr("invalid agent_id")
			}
			items, err := repo.ListByAgent(ctx, id, params.OnlyActive)
			if err != nil {
				return Err("failed to list skills", err)
			}
			return OK("skills for agent", items)
		}
		items, err := repo.ListAll(ctx, params.OnlyActive)
		if err != nil {
			return Err("failed to list skills", err)
		}
		return OK("skills", items)
	}
}

func rawOrEmptyObject(b []byte) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}
