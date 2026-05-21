package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// tools_agents_my.go — Phase 4 §4.2 — MCP-инструмент для управления user-level агентами.
// Аналог agent_update, но с ABAC-проверкой agent.UserID == currentUser.ID.

type AgentUpdateMyParams struct {
	AgentID            string   `json:"agent_id" jsonschema:"UUID агента"`
	RoleDescription    *string  `json:"role_description,omitempty"`
	SystemPrompt       *string  `json:"system_prompt,omitempty"`
	Model              *string  `json:"model,omitempty" jsonschema:"Только для llm-агентов"`
	ProviderKind       *string  `json:"provider_kind,omitempty" jsonschema:"anthropic/deepseek/zhipu/openrouter"`
	Temperature        *float64 `json:"temperature,omitempty"`
	MaxTokens          *int     `json:"max_tokens,omitempty"`
	IsActive           *bool    `json:"is_active,omitempty"`
	InternalMCPEnabled *bool    `json:"internal_mcp_enabled,omitempty" jsonschema:"Подключить внутренний MCP DevTeam"`
}

type AgentGetMyParams struct {
	AgentID string `json:"agent_id" jsonschema:"UUID агента"`
}

type AgentListMyParams struct {
}

func RegisterAgentMyTools(server *mcp.Server, agentSvc *service.AgentService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_list_my",
		Description: "Список user-level агентов текущего пользователя.",
	}, makeAgentListMyHandler(agentSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_get_my",
		Description: "Полная запись user-level агента (с ABAC: только свои).",
	}, makeAgentGetMyHandler(agentSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_update_my",
		Description: "Обновить user-level агента (с ABAC: только свои). Запрещено менять role/execution_kind.",
	}, makeAgentUpdateMyHandler(agentSvc))
}

func makeAgentListMyHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentListMyParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ AgentListMyParams) (*mcp.CallToolResult, any, error) {
		userID, ok := UserIDFromContext(ctx)
		if !ok {
			return Err("missing user context", fmt.Errorf("no userID in context"))
		}
		agents, total, err := svc.List(ctx, repository.AgentFilter{UserID: &userID})
		if err != nil {
			return Err("failed to list agents", err)
		}
		return OK(fmt.Sprintf("Found %d user agents", total), map[string]any{
			"total": total,
			"items": agents,
		})
	}
}

func makeAgentGetMyHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentGetMyParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentGetMyParams) (*mcp.CallToolResult, any, error) {
		userID, ok := UserIDFromContext(ctx)
		if !ok {
			return Err("missing user context", fmt.Errorf("no userID in context"))
		}
		id, err := uuid.Parse(p.AgentID)
		if err != nil {
			return ValidationErr("invalid agent_id (must be UUID)")
		}
		a, err := svc.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, service.ErrAgentNotInRegistry) {
				return Err("agent not found", err)
			}
			return Err("failed to get agent", err)
		}
		if a.UserID == nil || *a.UserID != userID {
			return Err("access denied", fmt.Errorf("agent %s does not belong to user %s", id, userID))
		}
		return OK(fmt.Sprintf("Agent %q (%s)", a.Name, a.ExecutionKind), a)
	}
}

func makeAgentUpdateMyHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentUpdateMyParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentUpdateMyParams) (*mcp.CallToolResult, any, error) {
		userID, ok := UserIDFromContext(ctx)
		if !ok {
			return Err("missing user context", fmt.Errorf("no userID in context"))
		}
		id, err := uuid.Parse(p.AgentID)
		if err != nil {
			return ValidationErr("invalid agent_id (must be UUID)")
		}

		// ABAC: проверяем принадлежность.
		a, err := svc.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, service.ErrAgentNotInRegistry) {
				return Err("agent not found", err)
			}
			return Err("failed to get agent", err)
		}
		if a.UserID == nil || *a.UserID != userID {
			return Err("access denied", fmt.Errorf("agent %s does not belong to user %s", id, userID))
		}

		in := service.UpdateAgentInput{
			RoleDescription:    p.RoleDescription,
			SystemPrompt:       p.SystemPrompt,
			Model:              p.Model,
			Temperature:        p.Temperature,
			MaxTokens:          p.MaxTokens,
			IsActive:           p.IsActive,
			InternalMCPEnabled: p.InternalMCPEnabled,
		}
		if p.ProviderKind != nil && *p.ProviderKind != "" {
			pk := models.AgentProviderKind(*p.ProviderKind)
			in.ProviderKind = &pk
		}

		// §4.3 — валидация провайдера.
		providerToCheck := in.ProviderKind
		if providerToCheck == nil {
			providerToCheck = a.ProviderKind
		}
		if in.Model != nil || in.ProviderKind != nil {
			if err := svc.ValidateProviderConnected(ctx, userID, providerToCheck); err != nil {
				if errors.Is(err, service.ErrAgentProviderNotConnected) {
					return ValidationErr(err.Error())
				}
				return Err("provider validation failed", err)
			}
		}

		updated, err := svc.Update(ctx, id, in)
		if err != nil {
			return mapAgentServiceErr(err)
		}
		return OK(fmt.Sprintf("Agent %q updated", updated.Name), updated)
	}
}
