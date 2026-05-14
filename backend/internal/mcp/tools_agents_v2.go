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

// tools_agents_v2.go — Sprint 17 / Sprint 5 — MCP-инструменты для реестра агентов v2.
//
// Sprint 5 review fix #1 (layer violation): handlers ТОНКИЕ, всё бизнес — в service.AgentService.
// Никакой шифрации, валидации или идемпотентности здесь — только парсинг входа,
// вызов сервиса, маппинг service-sentinel'ов в MCP-ответы.

// ─────────────────────────────────────────────────────────────────────────────
// Params
// ─────────────────────────────────────────────────────────────────────────────

type AgentListParams struct {
	OnlyActive    *bool   `json:"only_active,omitempty" jsonschema:"description=Только is_active=true агенты"`
	ExecutionKind *string `json:"execution_kind,omitempty" jsonschema:"description=Фильтр: llm | sandbox"`
	Role          *string `json:"role,omitempty" jsonschema:"description=Фильтр по роли"`
	NameLike      *string `json:"name_like,omitempty" jsonschema:"description=Частичный поиск по name"`
	Limit         *int    `json:"limit,omitempty" jsonschema:"description=Лимит 1-200 (default 50)"`
	Offset        *int    `json:"offset,omitempty" jsonschema:"description=Смещение (default 0)"`
}

type AgentGetParams struct {
	AgentID string `json:"agent_id" jsonschema:"required,description=UUID агента"`
}

type AgentCreateParams struct {
	Name            string   `json:"name" jsonschema:"required,description=Уникальное имя агента"`
	Role            string   `json:"role" jsonschema:"required,description=Роль (router/planner/decomposer/reviewer/developer/merger/tester/...)"`
	ExecutionKind   string   `json:"execution_kind" jsonschema:"required,description=llm или sandbox"`
	RoleDescription *string  `json:"role_description,omitempty"`
	SystemPrompt    *string  `json:"system_prompt,omitempty"`
	Model           *string  `json:"model,omitempty" jsonschema:"description=Обязателен для llm; запрещён для sandbox"`
	CodeBackend     *string  `json:"code_backend,omitempty" jsonschema:"description=Обязателен для sandbox (claude-code/aider/hermes/custom); запрещён для llm"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
	IsActive        *bool    `json:"is_active,omitempty"`
}

type AgentUpdateParams struct {
	AgentID         string   `json:"agent_id" jsonschema:"required,description=UUID агента"`
	RoleDescription *string  `json:"role_description,omitempty"`
	SystemPrompt    *string  `json:"system_prompt,omitempty"`
	Model           *string  `json:"model,omitempty" jsonschema:"description=Только для llm-агентов"`
	// Sprint 5 review fix #4: CodeBackend для sandbox-агентов (например, перейти с claude-code на aider).
	CodeBackend     *string  `json:"code_backend,omitempty" jsonschema:"description=Только для sandbox-агентов (claude-code/aider/hermes/custom)"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
	IsActive        *bool    `json:"is_active,omitempty"`
}

type AgentSetSecretParams struct {
	AgentID string `json:"agent_id" jsonschema:"required,description=UUID агента"`
	KeyName string `json:"key_name" jsonschema:"required,description=Имя env-переменной (UPPERCASE_WITH_UNDERSCORES)"`
	Value   string `json:"value" jsonschema:"required,description=Plaintext-значение секрета (зашифруется; back-read невозможен)"`
}

type AgentDeleteSecretParams struct {
	SecretID string `json:"secret_id" jsonschema:"required,description=UUID записи agent_secrets"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

// RegisterAgentV2Tools — handlers зависят от *service.AgentService (не от repos).
func RegisterAgentV2Tools(server *mcp.Server, agentSvc *service.AgentService) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_list",
		Description: "Список агентов реестра v2 с фильтрами. БЕЗ system_prompt (читай через agent_get).",
	}, makeAgentListHandler(agentSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_get",
		Description: "Полная запись агента по UUID (включая system_prompt).",
	}, makeAgentGetHandler(agentSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_create",
		Description: "Создать нового агента. llm/sandbox требуют разные поля.",
	}, makeAgentCreateHandler(agentSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_update",
		Description: "Обновить prompt/model/code_backend/temperature/is_active.",
	}, makeAgentUpdateHandler(agentSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_set_secret",
		Description: "Добавить/обновить секрет агента (AES-256-GCM, back-read невозможен).",
	}, makeAgentSetSecretHandler(agentSvc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "agent_delete_secret",
		Description: "Удалить секрет агента по UUID записи agent_secrets.",
	}, makeAgentDeleteSecretHandler(agentSvc))
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers (тонкие: parse input → service-call → map result)
// ─────────────────────────────────────────────────────────────────────────────

func makeAgentListHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentListParams) (*mcp.CallToolResult, any, error) {
		f := repository.AgentFilter{}
		if p.OnlyActive != nil {
			f.OnlyActive = *p.OnlyActive
		}
		if p.ExecutionKind != nil {
			k := models.AgentExecutionKind(*p.ExecutionKind)
			if !k.IsValid() {
				return ValidationErr(fmt.Sprintf("invalid execution_kind %q", *p.ExecutionKind))
			}
			f.ExecutionKind = &k
		}
		if p.Role != nil {
			r := models.AgentRole(*p.Role)
			if !r.IsValid() {
				return ValidationErr(fmt.Sprintf("invalid role %q", *p.Role))
			}
			f.Role = &r
		}
		if p.NameLike != nil {
			f.NameLike = *p.NameLike
		}
		if p.Limit != nil {
			f.Limit = *p.Limit
		}
		if p.Offset != nil {
			f.Offset = *p.Offset
		}
		agents, total, err := svc.List(ctx, f)
		if err != nil {
			return Err("failed to list agents", err)
		}
		return OK(fmt.Sprintf("Found %d/%d agents", len(agents), total), map[string]any{
			"total":  total,
			"items":  agents,
			"limit":  f.Limit,
			"offset": f.Offset,
		})
	}
}

func makeAgentGetHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentGetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentGetParams) (*mcp.CallToolResult, any, error) {
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
		return OK(fmt.Sprintf("Agent %q (%s)", a.Name, a.ExecutionKind), a)
	}
}

func makeAgentCreateHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentCreateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentCreateParams) (*mcp.CallToolResult, any, error) {
		in := service.CreateAgentInput{
			Name:            p.Name,
			Role:            models.AgentRole(p.Role),
			ExecutionKind:   models.AgentExecutionKind(p.ExecutionKind),
			RoleDescription: p.RoleDescription,
			SystemPrompt:    p.SystemPrompt,
			Temperature:     p.Temperature,
			MaxTokens:       p.MaxTokens,
			IsActive:        p.IsActive,
		}
		if p.Model != nil && *p.Model != "" {
			in.Model = p.Model
		}
		if p.CodeBackend != nil && *p.CodeBackend != "" {
			cb := models.CodeBackend(*p.CodeBackend)
			in.CodeBackend = &cb
		}

		a, err := svc.Create(ctx, in)
		if err != nil {
			return mapAgentServiceErr(err)
		}
		return OK(fmt.Sprintf("Agent %q created (id=%s)", a.Name, a.ID), a)
	}
}

func makeAgentUpdateHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentUpdateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentUpdateParams) (*mcp.CallToolResult, any, error) {
		id, err := uuid.Parse(p.AgentID)
		if err != nil {
			return ValidationErr("invalid agent_id (must be UUID)")
		}
		in := service.UpdateAgentInput{
			RoleDescription: p.RoleDescription,
			SystemPrompt:    p.SystemPrompt,
			Model:           p.Model,
			Temperature:     p.Temperature,
			MaxTokens:       p.MaxTokens,
			IsActive:        p.IsActive,
		}
		if p.CodeBackend != nil && *p.CodeBackend != "" {
			cb := models.CodeBackend(*p.CodeBackend)
			in.CodeBackend = &cb
		}
		a, err := svc.Update(ctx, id, in)
		if err != nil {
			return mapAgentServiceErr(err)
		}
		return OK(fmt.Sprintf("Agent %q updated", a.Name), a)
	}
}

func makeAgentSetSecretHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentSetSecretParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentSetSecretParams) (*mcp.CallToolResult, any, error) {
		agentID, err := uuid.Parse(p.AgentID)
		if err != nil {
			return ValidationErr("invalid agent_id (must be UUID)")
		}
		out, err := svc.SetSecret(ctx, service.SetSecretInput{
			AgentID: agentID,
			KeyName: p.KeyName,
			Value:   p.Value,
		})
		if err != nil {
			return mapAgentServiceErr(err)
		}
		return OK(fmt.Sprintf("Secret %q stored for agent %s", out.KeyName, out.AgentID), out)
	}
}

func makeAgentDeleteSecretHandler(svc *service.AgentService) func(context.Context, *mcp.CallToolRequest, AgentDeleteSecretParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p AgentDeleteSecretParams) (*mcp.CallToolResult, any, error) {
		id, err := uuid.Parse(p.SecretID)
		if err != nil {
			return ValidationErr("invalid secret_id (must be UUID)")
		}
		if err := svc.DeleteSecret(ctx, id); err != nil {
			if errors.Is(err, service.ErrAgentNotInRegistry) {
				return Err("secret not found", err)
			}
			return Err("failed to delete secret", err)
		}
		return OK("Secret deleted", map[string]any{"secret_id": id})
	}
}

// mapAgentServiceErr — единая точка маппинга service-уровневых sentinel'ов в MCP-ответ.
func mapAgentServiceErr(err error) (*mcp.CallToolResult, any, error) {
	switch {
	case errors.Is(err, service.ErrAgentValidation),
		errors.Is(err, service.ErrAgentSecretInvalidKey):
		return ValidationErr(err.Error())
	case errors.Is(err, service.ErrAgentNameAlreadyTaken):
		return Err(err.Error(), err)
	case errors.Is(err, service.ErrAgentNotInRegistry):
		return Err("agent not found", err)
	case errors.Is(err, service.ErrEncryptorNotConfigured):
		return Err("encryptor not configured (set ENCRYPTION_KEY)", err)
	default:
		return Err("agent operation failed", err)
	}
}
