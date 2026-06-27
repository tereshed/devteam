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

type RepoEnvFileSetParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	RepoID    string `json:"repo_id" jsonschema:"UUID репозитория проекта (project_repositories.id)"`
	FileName  string `json:"file_name" jsonschema:"Имя создаваемого файла, например .env (без слешей и '..')"`
	TargetDir string `json:"target_dir" jsonschema:"Относительная папка внутри репо (пусто = корень)"`
	Content   string `json:"content" jsonschema:"Содержимое файла (шифруется AES-256-GCM; пишется в репо, исключается из git)"`
}

type RepoEnvFileDeleteParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	RepoID    string `json:"repo_id" jsonschema:"UUID репозитория проекта"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

// RegisterRepoEnvFileTools регистрирует MCP-инструменты «инъекции env-файла» уровня
// репозитория (один файл на репо). Опционально: nil-сервис → инструменты не регистрируются.
func RegisterRepoEnvFileTools(server *mcp.Server, svc *service.RepositoryEnvFileService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "repo_env_file_set",
		Description: "Задать/обновить инъектируемый env-файл репозитория (содержимое, имя, папка). Содержимое шифруется AES-256-GCM; перед запуском агента файл пишется в рабочую копию репо и исключается из git (не коммитится).",
	}, makeRepoEnvFileSetHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "repo_env_file_delete",
		Description: "Удалить инъектируемый env-файл репозитория.",
	}, makeRepoEnvFileDeleteHandler(svc))
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func makeRepoEnvFileSetHandler(svc *service.RepositoryEnvFileService) func(context.Context, *mcp.CallToolRequest, RepoEnvFileSetParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p RepoEnvFileSetParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		projectID, err := uuid.Parse(p.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %v", err))
		}
		repoID, err := uuid.Parse(p.RepoID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid repo_id: %v", err))
		}
		out, err := svc.Set(ctx, service.SetRepoEnvFileInput{
			ProjectID: projectID,
			RepoID:    repoID,
			FileName:  p.FileName,
			TargetDir: p.TargetDir,
			Content:   p.Content,
		})
		if err != nil {
			return Err("repo env file set failed", err)
		}
		// Не возвращаем содержимое обратно в ответе инструмента — только метаданные.
		return OK("repo env file saved", map[string]any{
			"id":                    out.ID,
			"project_repository_id": out.ProjectRepositoryID,
			"file_name":             out.FileName,
			"target_dir":            out.TargetDir,
		})
	}
}

func makeRepoEnvFileDeleteHandler(svc *service.RepositoryEnvFileService) func(context.Context, *mcp.CallToolRequest, RepoEnvFileDeleteParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p RepoEnvFileDeleteParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		projectID, err := uuid.Parse(p.ProjectID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid project_id: %v", err))
		}
		repoID, err := uuid.Parse(p.RepoID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid repo_id: %v", err))
		}
		if err := svc.Delete(ctx, projectID, repoID); err != nil {
			return Err("repo env file delete failed", err)
		}
		return OK("repo env file deleted", nil)
	}
}
