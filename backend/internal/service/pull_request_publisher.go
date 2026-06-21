package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/google/uuid"
)

// PullRequestPublisher вызывается оркестратором после того, как задача
// дошла до status=completed: открывает Pull Request с веткой задачи в base
// проекта. Реализация может быть nil (тогда оркестратор просто пропустит шаг).
type PullRequestPublisher interface {
	Publish(ctx context.Context, task *models.Task, project *models.Project) (*gitprovider.PullRequest, error)
	// PublishForRepo открывает PR в конкретном репозитории проекта (мульти-репо):
	// head=task branch, base=repo.GitDefaultBranch, репозиторий repo.GitURL, креды —
	// repo-level git_credential c фоллбеком на project-level и OAuth-интеграцию владельца.
	PublishForRepo(ctx context.Context, task *models.Task, project *models.Project, repo *models.ProjectRepository) (*gitprovider.PullRequest, error)
	// LatestPipelineStatus — статус последнего CI-пайплайна ветки ref (CI-gate, Sprint 22).
	// repo=nil → одно-репо (поля project). Провайдер без поддержки CI или нерезолвимые
	// креды → PipelineStatusNone (гейт не блокирует), без ошибки.
	LatestPipelineStatus(ctx context.Context, project *models.Project, repo *models.ProjectRepository, ref string) (*gitprovider.PipelineResult, error)
}

// gitPRPublisher — продакшен-реализация: использует gitprovider.Factory + Encryptor
// для расшифровки токена из project.GitCredential, и зовёт provider.CreatePullRequest.
//
// Не падает фатально, если у проекта нет credentials или provider не поддерживает PR
// (например, local) — в этом случае Publish возвращает ErrPullRequestSkipped, и оркестратор
// логирует skip вместо ошибки.
type gitPRPublisher struct {
	factory   gitprovider.Factory
	encryptor Encryptor
	// gitIntegrations — fallback-резолв токена из OAuth-интеграции владельца проекта
	// (git_integration_credentials), если project-level git_credential не привязан.
	// Тот же путь, что у индексатора (project_service) и sandbox (orchestrator_context_builder).
	gitIntegrations repository.GitIntegrationCredentialRepository
	log             *slog.Logger
	// tokenRefresher (опц.) рефрешит истёкший OAuth-токен перед открытием PR/MR.
	// Без него истёкший токен (self-hosted GitLab TTL ~2ч) валит done-гейт на 401.
	tokenRefresher GitTokenRefresher
}

// NewGitPRPublisher создаёт PR-publisher. Любой из аргументов может быть nil — тогда
// Publish ничего не делает (удобно для тестов и dev-режима без AES-ключа).
// gitIntegrations опционален: при nil остаётся только project-level git_credential.
func NewGitPRPublisher(factory gitprovider.Factory, encryptor Encryptor, gitIntegrations repository.GitIntegrationCredentialRepository, log *slog.Logger) PullRequestPublisher {
	if log == nil {
		log = slog.Default()
	}
	return &gitPRPublisher{factory: factory, encryptor: encryptor, gitIntegrations: gitIntegrations, log: log}
}

// WithGitTokenRefresher внедряет рефрешер OAuth-токенов (post-construction).
func WithPRPublisherTokenRefresher(pub PullRequestPublisher, r GitTokenRefresher) {
	if p, ok := pub.(*gitPRPublisher); ok {
		p.tokenRefresher = r
	}
}

// integrationToken отдаёт живой access-token интеграционного аккаунта (рефреш истёкшего + персист,
// иначе расшифровка сохранённого).
func (p *gitPRPublisher) integrationToken(ctx context.Context, cred *models.GitIntegrationCredential) (string, error) {
	if p.tokenRefresher != nil {
		token, err := p.tokenRefresher.FreshAccessToken(ctx, cred)
		if err != nil {
			return "", fmt.Errorf("refresh git integration token: %w", err)
		}
		return token, nil
	}
	dec, derr := p.encryptor.Decrypt(cred.AccessTokenEnc, repository.GitIntegrationCredentialAAD(cred.ID))
	if derr != nil {
		return "", fmt.Errorf("decrypt git integration credential: %w", derr)
	}
	return string(dec), nil
}

// ErrPullRequestSkipped — publisher распознал, что PR создавать не нужно/нечем
// (нет credentials, провайдер local, ветка пуста). Не фатально для pipeline.
var ErrPullRequestSkipped = errors.New("pull request: skipped")

// projectMRTemplate — per-project шаблон тайтла MR ("" → дефолт).
func projectMRTemplate(project *models.Project) string {
	if project != nil && project.MRTitleTemplate != nil {
		return *project.MRTitleTemplate
	}
	return ""
}

// mrTitleVars собирает значения для RenderMRTitle из задачи (repoSlug пуст для одно-репо).
func mrTitleVars(task *models.Task, repoSlug string) MRTitleVars {
	v := MRTitleVars{TaskID: task.ID, Title: task.Title, RepoSlug: repoSlug, Now: time.Now()}
	if task.ExternalKey != nil {
		v.ExternalKey = *task.ExternalKey
	}
	if task.BranchName != nil {
		v.Branch = *task.BranchName
	}
	return v
}

func (p *gitPRPublisher) Publish(ctx context.Context, task *models.Task, project *models.Project) (*gitprovider.PullRequest, error) {
	if p == nil || p.factory == nil {
		return nil, ErrPullRequestSkipped
	}
	if task == nil || project == nil {
		return nil, ErrPullRequestSkipped
	}
	if task.BranchName == nil || *task.BranchName == "" {
		return nil, fmt.Errorf("%w: empty branch", ErrPullRequestSkipped)
	}
	if project.GitURL == "" || string(project.GitProvider) == "local" {
		return nil, fmt.Errorf("%w: non-PR provider (%s)", ErrPullRequestSkipped, project.GitProvider)
	}
	creds, err := p.resolveCredentials(ctx, project)
	if err != nil {
		return nil, err
	}

	provider, err := p.factory.Create(string(project.GitProvider), creds)
	if err != nil {
		return nil, fmt.Errorf("git factory: %w", err)
	}

	base := project.GitDefaultBranch
	if base == "" {
		base = "main"
	}
	body := fmt.Sprintf("Auto-generated by PolyMaths pipeline.\n\nTask: %s\n\n%s", task.Title, task.Description)
	pr, err := provider.CreatePullRequest(ctx, project.GitURL, gitprovider.PRCreateOptions{
		Title:      RenderMRTitle(projectMRTemplate(project), mrTitleVars(task, "")),
		Body:       body,
		HeadBranch: *task.BranchName,
		BaseBranch: base,
		Labels:     []string{"polymaths", "auto-generated"},
	})
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}
	p.log.Info("PR opened", "project_id", project.ID, "task_id", task.ID, "branch", *task.BranchName, "pr_number", pr.Number, "pr_url", pr.HTMLURL)
	return pr, nil
}

// PublishForRepo открывает PR в конкретном репозитории проекта (мульти-репо).
func (p *gitPRPublisher) PublishForRepo(ctx context.Context, task *models.Task, project *models.Project, repo *models.ProjectRepository) (*gitprovider.PullRequest, error) {
	if p == nil || p.factory == nil {
		return nil, ErrPullRequestSkipped
	}
	if task == nil || project == nil || repo == nil {
		return nil, ErrPullRequestSkipped
	}
	if task.BranchName == nil || *task.BranchName == "" {
		return nil, fmt.Errorf("%w: empty branch", ErrPullRequestSkipped)
	}
	if repo.GitURL == "" || string(repo.GitProvider) == "local" {
		return nil, fmt.Errorf("%w: non-PR provider (%s) for repo %q", ErrPullRequestSkipped, repo.GitProvider, repo.Slug)
	}
	creds, err := p.resolveRepoCredentials(ctx, project, repo)
	if err != nil {
		return nil, err
	}

	provider, err := p.factory.Create(string(repo.GitProvider), creds)
	if err != nil {
		return nil, fmt.Errorf("git factory: %w", err)
	}

	base := repo.GitDefaultBranch
	if base == "" {
		base = "main"
	}
	body := fmt.Sprintf("Auto-generated by PolyMaths pipeline.\n\nTask: %s\nRepository: %s\n\n%s", task.Title, repo.Slug, task.Description)
	pr, err := provider.CreatePullRequest(ctx, repo.GitURL, gitprovider.PRCreateOptions{
		Title:      RenderMRTitle(projectMRTemplate(project), mrTitleVars(task, repo.Slug)),
		Body:       body,
		HeadBranch: *task.BranchName,
		BaseBranch: base,
		Labels:     []string{"polymaths", "auto-generated"},
	})
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}
	p.log.Info("PR opened (repo-scoped)", "project_id", project.ID, "task_id", task.ID, "repo_slug", repo.Slug, "branch", *task.BranchName, "pr_number", pr.Number, "pr_url", pr.HTMLURL)
	return pr, nil
}

// LatestPipelineStatus резолвит креды и провайдер (как для открытия PR), затем
// читает статус CI через опциональный gitprovider.PipelineStatusReader. Провайдер
// без поддержки CI / нерезолвимые креды → PipelineStatusNone (не блокируем гейт).
func (p *gitPRPublisher) LatestPipelineStatus(ctx context.Context, project *models.Project, repo *models.ProjectRepository, ref string) (*gitprovider.PipelineResult, error) {
	none := &gitprovider.PipelineResult{Status: gitprovider.PipelineStatusNone}
	if p == nil || p.factory == nil || project == nil || ref == "" {
		return none, nil
	}
	var (
		creds        gitprovider.Credentials
		providerType string
		repoURL      string
		err          error
	)
	if repo != nil {
		creds, err = p.resolveRepoCredentials(ctx, project, repo)
		providerType = string(repo.GitProvider)
		repoURL = repo.GitURL
	} else {
		creds, err = p.resolveCredentials(ctx, project)
		providerType = string(project.GitProvider)
		repoURL = project.GitURL
	}
	if err != nil {
		// Нет кред / non-PR провайдер — проверить CI нечем, не блокируем гейт.
		return none, nil
	}
	provider, err := p.factory.Create(providerType, creds)
	if err != nil {
		return none, nil
	}
	reader, ok := provider.(gitprovider.PipelineStatusReader)
	if !ok {
		return none, nil
	}
	return reader.GetLatestPipelineStatus(ctx, repoURL, ref)
}

// resolveCredentials выбирает git-credential для открытия PR.
//
// Приоритет: явный project-level git_credential (project.GitCredential). Если он не привязан —
// FALLBACK на OAuth-интеграцию владельца проекта (git_integration_credentials по project.UserID).
// Это тот же токен, которым индексатор клонирует репозиторий, а sandbox клонирует и пушит ветки
// (см. project_service.go и orchestrator_context_builder.go). Без этого fallback PR не открывался,
// пока к проекту не привязан явный credential, хотя доступ к репозиторию уже есть.
// resolveRepoCredentials выбирает git-credential для PR в конкретном репо (мульти-репо).
// Приоритет: repo-level git_credential (repo.GitCredential) → project-level → OAuth-интеграция
// владельца проекта (через resolveCredentials).
func (p *gitPRPublisher) resolveRepoCredentials(ctx context.Context, project *models.Project, repo *models.ProjectRepository) (gitprovider.Credentials, error) {
	var creds gitprovider.Credentials
	if p.encryptor == nil {
		return creds, fmt.Errorf("%w: no encryptor", ErrPullRequestSkipped)
	}
	// Мульти-аккаунт: явно выбранный OAuth-аккаунт (repo → project) имеет высший приоритет.
	var integID *uuid.UUID
	if repo != nil && repo.GitIntegrationCredentialID != nil {
		integID = repo.GitIntegrationCredentialID
	} else if project != nil && project.GitIntegrationCredentialID != nil {
		integID = project.GitIntegrationCredentialID
	}
	if integID != nil && p.gitIntegrations != nil {
		cred, err := p.gitIntegrations.GetByID(ctx, *integID)
		if err == nil && cred != nil && len(cred.AccessTokenEnc) > 0 {
			token, terr := p.integrationToken(ctx, cred)
			if terr != nil {
				return creds, terr
			}
			creds.Token = token
			return creds, nil
		}
	}
	if repo != nil && repo.GitCredential != nil {
		decrypted, err := p.encryptor.Decrypt(repo.GitCredential.EncryptedValue, []byte(repo.GitCredential.ID.String()))
		if err != nil {
			return creds, fmt.Errorf("decrypt repo git credential: %w", err)
		}
		switch repo.GitCredential.AuthType {
		case models.GitCredentialAuthToken, models.GitCredentialAuthOAuth:
			creds.Token = string(decrypted)
		case models.GitCredentialAuthSSHKey:
			creds.SSHKey = string(decrypted)
		}
		return creds, nil
	}
	// Фоллбек на project-level credential / OAuth-интеграцию владельца.
	return p.resolveCredentials(ctx, project)
}

func (p *gitPRPublisher) resolveCredentials(ctx context.Context, project *models.Project) (gitprovider.Credentials, error) {
	var creds gitprovider.Credentials
	if p.encryptor == nil {
		return creds, fmt.Errorf("%w: no encryptor", ErrPullRequestSkipped)
	}

	// 0. Мульти-аккаунт: явно выбранный OAuth-аккаунт проекта.
	if project.GitIntegrationCredentialID != nil && p.gitIntegrations != nil {
		cred, err := p.gitIntegrations.GetByID(ctx, *project.GitIntegrationCredentialID)
		if err == nil && cred != nil && len(cred.AccessTokenEnc) > 0 {
			token, terr := p.integrationToken(ctx, cred)
			if terr != nil {
				return creds, terr
			}
			creds.Token = token
			return creds, nil
		}
	}

	// 1. Явный project-level credential.
	if project.GitCredential != nil {
		decrypted, err := p.encryptor.Decrypt(project.GitCredential.EncryptedValue, []byte(project.GitCredential.ID.String()))
		if err != nil {
			return creds, fmt.Errorf("decrypt git credential: %w", err)
		}
		switch project.GitCredential.AuthType {
		case models.GitCredentialAuthToken, models.GitCredentialAuthOAuth:
			creds.Token = string(decrypted)
		case models.GitCredentialAuthSSHKey:
			creds.SSHKey = string(decrypted)
		}
		return creds, nil
	}

	// 2. Fallback: OAuth-интеграция владельца проекта.
	if p.gitIntegrations == nil {
		return creds, fmt.Errorf("%w: no git_credential on project", ErrPullRequestSkipped)
	}
	integProvider, ok := mapGitProviderToIntegration(project.GitProvider)
	if !ok {
		return creds, fmt.Errorf("%w: provider %q is not OAuth-integratable", ErrPullRequestSkipped, project.GitProvider)
	}
	cred, err := p.gitIntegrations.GetByUserAndProvider(ctx, project.UserID, integProvider)
	if err != nil || cred == nil || len(cred.AccessTokenEnc) == 0 {
		return creds, fmt.Errorf("%w: no project git_credential and no %s OAuth integration for project owner", ErrPullRequestSkipped, integProvider)
	}
	token, terr := p.integrationToken(ctx, cred)
	if terr != nil {
		return creds, terr
	}
	creds.Token = token
	return creds, nil
}
