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

type RepoEnvFileListParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	RepoID    string `json:"repo_id" jsonschema:"UUID репозитория проекта (project_repositories.id)"`
}

type RepoEnvFileCreateParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	RepoID    string `json:"repo_id" jsonschema:"UUID репозитория проекта"`
	FileName  string `json:"file_name" jsonschema:"Имя создаваемого файла, например .env (без слешей и '..')"`
	TargetDir string `json:"target_dir" jsonschema:"Относительная папка внутри репо (пусто = корень)"`
	Content   string `json:"content" jsonschema:"Содержимое файла (шифруется AES-256-GCM; пишется в репо, исключается из git)"`
}

type RepoEnvFileUpdateParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	RepoID    string `json:"repo_id" jsonschema:"UUID репозитория проекта"`
	FileID    string `json:"file_id" jsonschema:"UUID записи repository_env_files"`
	FileName  string `json:"file_name" jsonschema:"Имя файла (без слешей и '..')"`
	TargetDir string `json:"target_dir" jsonschema:"Относительная папка внутри репо (пусто = корень)"`
	Content   string `json:"content" jsonschema:"Новое содержимое файла (полная перезапись)"`
}

type RepoEnvFileDeleteParams struct {
	ProjectID string `json:"project_id" jsonschema:"UUID проекта"`
	RepoID    string `json:"repo_id" jsonschema:"UUID репозитория проекта"`
	FileID    string `json:"file_id" jsonschema:"UUID записи repository_env_files"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Registration
// ─────────────────────────────────────────────────────────────────────────────

// RegisterRepoEnvFileTools регистрирует MCP-инструменты «инъекции env-файлов» уровня
// репозитория (несколько файлов на репо). Опционально: nil-сервис → не регистрируются.
func RegisterRepoEnvFileTools(server *mcp.Server, svc *service.RepositoryEnvFileService) {
	if svc == nil {
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "repo_env_file_list",
		Description: "Список инъектируемых env-файлов репозитория (метаданные: имя/папка/тайминги, без содержимого).",
	}, makeRepoEnvFileListHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "repo_env_file_create",
		Description: "Добавить инъектируемый env-файл репозитория. Содержимое шифруется AES-256-GCM; перед запуском агента файл пишется в рабочую копию репо и исключается из git (не коммитится).",
	}, makeRepoEnvFileCreateHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "repo_env_file_update",
		Description: "Перезаписать инъектируемый env-файл репозитория по UUID записи (имя/папка/содержимое целиком).",
	}, makeRepoEnvFileUpdateHandler(svc))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "repo_env_file_delete",
		Description: "Удалить инъектируемый env-файл репозитория по UUID записи.",
	}, makeRepoEnvFileDeleteHandler(svc))
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers
// ─────────────────────────────────────────────────────────────────────────────

func makeRepoEnvFileListHandler(svc *service.RepositoryEnvFileService) func(context.Context, *mcp.CallToolRequest, RepoEnvFileListParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p RepoEnvFileListParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		projectID, repoID, verr := parseRepoEnvIDs(p.ProjectID, p.RepoID)
		if verr != nil {
			return ValidationErr(verr.Error())
		}
		views, err := svc.List(ctx, projectID, repoID)
		if err != nil {
			return Err("repo env file list failed", err)
		}
		return OK("repo env files", views)
	}
}

func makeRepoEnvFileCreateHandler(svc *service.RepositoryEnvFileService) func(context.Context, *mcp.CallToolRequest, RepoEnvFileCreateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p RepoEnvFileCreateParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		projectID, repoID, verr := parseRepoEnvIDs(p.ProjectID, p.RepoID)
		if verr != nil {
			return ValidationErr(verr.Error())
		}
		out, err := svc.Create(ctx, service.CreateRepoEnvFileInput{
			ProjectID: projectID,
			RepoID:    repoID,
			FileName:  p.FileName,
			TargetDir: p.TargetDir,
			Content:   p.Content,
		})
		if err != nil {
			return Err("repo env file create failed", err)
		}
		return OK("repo env file created", out)
	}
}

func makeRepoEnvFileUpdateHandler(svc *service.RepositoryEnvFileService) func(context.Context, *mcp.CallToolRequest, RepoEnvFileUpdateParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p RepoEnvFileUpdateParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		projectID, repoID, verr := parseRepoEnvIDs(p.ProjectID, p.RepoID)
		if verr != nil {
			return ValidationErr(verr.Error())
		}
		fileID, err := uuid.Parse(p.FileID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid file_id: %v", err))
		}
		out, err := svc.Update(ctx, service.UpdateRepoEnvFileInput{
			ProjectID: projectID,
			RepoID:    repoID,
			FileID:    fileID,
			FileName:  p.FileName,
			TargetDir: p.TargetDir,
			Content:   p.Content,
		})
		if err != nil {
			return Err("repo env file update failed", err)
		}
		return OK("repo env file updated", out)
	}
}

func makeRepoEnvFileDeleteHandler(svc *service.RepositoryEnvFileService) func(context.Context, *mcp.CallToolRequest, RepoEnvFileDeleteParams) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, p RepoEnvFileDeleteParams) (*mcp.CallToolResult, any, error) {
		if _, ok := UserIDFromContext(ctx); !ok {
			return ValidationErr("authentication required")
		}
		projectID, repoID, verr := parseRepoEnvIDs(p.ProjectID, p.RepoID)
		if verr != nil {
			return ValidationErr(verr.Error())
		}
		fileID, err := uuid.Parse(p.FileID)
		if err != nil {
			return ValidationErr(fmt.Sprintf("invalid file_id: %v", err))
		}
		if err := svc.Delete(ctx, projectID, repoID, fileID); err != nil {
			return Err("repo env file delete failed", err)
		}
		return OK("repo env file deleted", nil)
	}
}

func parseRepoEnvIDs(projectIDRaw, repoIDRaw string) (uuid.UUID, uuid.UUID, error) {
	projectID, err := uuid.Parse(projectIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid project_id: %v", err)
	}
	repoID, err := uuid.Parse(repoIDRaw)
	if err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("invalid repo_id: %v", err)
	}
	return projectID, repoID, nil
}
