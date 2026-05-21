package mcp

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/devteam/backend/internal/service"
)

// ─────────────────────────────────────────────────────────────────────────────
// Params
// ─────────────────────────────────────────────────────────────────────────────

type ProjectSecretSetParams struct {
	ProjectID string `json:"project_id" jsonschema:"required,description=UUID проекта"`
	KeyName   string `json:"key_name" jsonschema:"required,description=Имя переменной (UPPERCASE_WITH_UNDERSCORES)"`
	Value     string `json:"value" jsonschema:"required,description=Plaintext-значение (шифруется AES-256-GCM; back-read невозможен)"`
}

type ProjectSecretDeleteParams struct {
	ProjectID string `json:"project_id" jsonschema:"required,description=UUID проекта"`
	SecretID  string `json:"secret_id" jsonschema:"required,description=UUID записи project_secrets"`
}

type UserSecretSetParams struct {
	KeyName string `json:"key_name" jsonschema:"required,description=Имя переменной (UPPERCASE_WITH_UNDERSCORES)"`
	Value   string `json:"value" jsonschema:"required,description=Plaintext-значение (шифруется AES-256-GCM; back-read невозможен)"`
}

type UserSecretDeleteParams struct {
	SecretID string `json:"secret_id" jsonschema:"required,description=UUID записи user_secrets"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

func RegisterSecretTools(
	server *mcp.Server,
	projectSecretSvc *service.ProjectSecretService,
	userSecretSvc *service.UserSecretService,
) {
	if projectSecretSvc != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "project_secret_set",
			Description: "Добавить/обновить секрет проекта. Значение шифруется AES-256-GCM, back-read невозможен.",
		}, makeProjectSecretSetHandler(projectSecretSvc))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "project_secret_delete",
			Description: "Удалить секрет проекта по UUID записи.",
		}, makeProjectSecretDeleteHandler(projectSecretSvc))
	}

	if userSecretSvc != nil {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "user_secret_set",
			Description: "Добавить/обновить персональный секрет текущего пользователя. Значение шифруется AES-256-GCM.",
		}, makeUserSecretSetHandler(userSecretSvc))

		mcp.AddTool(server, &mcp.Tool{
			Name:        "user_secret_delete",
			Description: "Удалить персональный секрет текущего пользователя по UUID записи.",
		}, makeUserSecretDeleteHandler(userSecretSvc))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func makeProjectSecretSetHandler(svc *service.ProjectSecretService) func(context.Context, *mcp.CallToolRequest, ProjectSecretSetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p ProjectSecretSetParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		projectID, err := uuid.Parse(p.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %v", err))
		}
		out, err := svc.Set(ctx, service.SetProjectSecretInput{
			ProjectID: projectID,
			KeyName:   p.KeyName,
			Value:     p.Value,
		})
		if err != nil {
			return Err("project secret set failed", err)
		}
		return OK("project secret saved", out)
	}
}

func makeProjectSecretDeleteHandler(svc *service.ProjectSecretService) func(context.Context, *mcp.CallToolRequest, ProjectSecretDeleteParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p ProjectSecretDeleteParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		secretID, err := uuid.Parse(p.SecretID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid secret_id: %v", err))
		}
		if err := svc.Delete(ctx, secretID); err != nil {
			return Err("project secret delete failed", err)
		}
		return OK("project secret deleted", nil)
	}
}

func makeUserSecretSetHandler(svc *service.UserSecretService) func(context.Context, *mcp.CallToolRequest, UserSecretSetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p UserSecretSetParams) (*mcp.CallToolResult, any, error) {
		uid, ok := UserIDFromContext(ctx)
		if !ok {
			return ValidationErr("authentication required")
		}
		out, err := svc.Set(ctx, service.SetUserSecretInput{
			UserID:  uid,
			KeyName: p.KeyName,
			Value:   p.Value,
		})
		if err != nil {
			return Err("user secret set failed", err)
		}
		return OK("user secret saved", out)
	}
}

func makeUserSecretDeleteHandler(svc *service.UserSecretService) func(context.Context, *mcp.CallToolRequest, UserSecretDeleteParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p UserSecretDeleteParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		secretID, err := uuid.Parse(p.SecretID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid secret_id: %v", err))
		}
		if err := svc.Delete(ctx, secretID); err != nil {
			return Err("user secret delete failed", err)
		}
		return OK("user secret deleted", nil)
	}
}
