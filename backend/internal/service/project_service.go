package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/gitprovider"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"log/slog"
)

const devTeamDefaultName = "Development Team"

const cloneTimeout = 10 * time.Minute
const indexingTimeout = 15 * time.Minute

var (
	ErrProjectNotFound        = errors.New("project not found")
	ErrProjectNameExists      = errors.New("project with this name already exists")
	ErrProjectForbidden       = errors.New("access to project denied")
	ErrGitCredentialNotFound  = errors.New("git credential not found")
	ErrGitCredentialForbidden = errors.New("git credential belongs to another user")
	ErrProjectInvalidName     = errors.New("project name is required")
	ErrProjectInvalidProvider = errors.New("invalid git provider")
	ErrProjectInvalidStatus   = errors.New("invalid project status")

	ErrUpdateProjectGitCredentialConflict = errors.New("cannot use git_credential_id together with remove_git_credential")
	ErrUpdateProjectTechStackConflict     = errors.New("cannot use tech_stack together with clear_tech_stack")
	ErrUpdateProjectSettingsConflict      = errors.New("cannot use settings together with clear_settings")

	ErrGitValidationFailed                 = errors.New("git access validation failed")
	ErrGitCloneFailed                      = errors.New("git clone failed")
	ErrDecryptionFailed                    = errors.New("failed to decrypt git credentials")
	ErrGitURLRequired                      = errors.New("git_url is required for remote git provider")
	ErrGitCredentialRequired               = errors.New("git_credential_id is required for remote git provider")
	ErrGitCredentialNotSupportedForLocal   = errors.New("git_credential_id is not supported for local provider")
)

// jsonbEmptyObject значение по умолчанию для очищенных jsonb-полей проекта (как в миграции default '{}').
var jsonbEmptyObject = datatypes.JSON([]byte("{}"))

// ProjectService бизнес-логика проектов.
// userRole — роль из JWT (admin обходит ABAC на чтение/список/изменение чужих проектов).
type ProjectService interface {
	Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error)
	GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error)
	List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error)
	Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error)
	Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error
	// HasAccess проверяет доступ пользователя к проекту. Возвращает nil при успехе,
	// ErrProjectNotFound если проект не существует (или это скрыто намеренно),
	// ErrProjectForbidden если доступ запрещен.
	HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error
}

type projectService struct {
	projectRepo  repository.ProjectRepository
	teamRepo     repository.TeamRepository
	gitCredRepo  repository.GitCredentialRepository
	transactions repository.TransactionManager
	gitFactory   gitprovider.Factory
	encryptor    Encryptor
	eventBus     events.EventBus
	indexer      indexer.CodeIndexer
	importDir    string
}

// NewProjectService создаёт сервис проектов.
func NewProjectService(
	projectRepo repository.ProjectRepository,
	teamRepo repository.TeamRepository,
	gitCredRepo repository.GitCredentialRepository,
	transactions repository.TransactionManager,
	gitFactory gitprovider.Factory,
	encryptor Encryptor,
	eventBus events.EventBus,
	indexer indexer.CodeIndexer,
	importDir string,
) ProjectService {
	return &projectService{
		projectRepo:  projectRepo,
		teamRepo:     teamRepo,
		gitCredRepo:  gitCredRepo,
		transactions: transactions,
		gitFactory:   gitFactory,
		encryptor:    encryptor,
		eventBus:     eventBus,
		indexer:      indexer,
		importDir:    importDir,
	}
}

func (s *projectService) checkProjectAccess(project *models.Project, userID uuid.UUID, userRole models.UserRole) error {
	if userRole == models.RoleAdmin {
		return nil
	}
	if project.UserID != userID {
		return ErrProjectForbidden
	}
	return nil
}

func mapProjectRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrProjectNotFound) {
		return ErrProjectNotFound
	}
	if errors.Is(err, repository.ErrProjectNameExists) {
		return ErrProjectNameExists
	}
	return err
}

func mapGitCredRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrGitCredentialNotFound) {
		return ErrGitCredentialNotFound
	}
	return err
}

// mapGitProviderErr маппит ошибки gitprovider в ошибки сервиса.
func mapGitProviderErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, gitprovider.ErrAuthFailed):
		return fmt.Errorf("%w: authentication failed for repository", ErrGitValidationFailed)
	case errors.Is(err, gitprovider.ErrRepoNotFound):
		return fmt.Errorf("%w: repository not found", ErrGitValidationFailed)
	case errors.Is(err, gitprovider.ErrPermissionDenied):
		return fmt.Errorf("%w: insufficient permissions", ErrGitValidationFailed)
	case errors.Is(err, gitprovider.ErrRateLimited):
		return fmt.Errorf("%w: rate limit exceeded, try later", ErrGitValidationFailed)
	case errors.Is(err, gitprovider.ErrCloneFailed):
		return ErrGitCloneFailed
	case errors.Is(err, gitprovider.ErrUnknownProvider):
		return ErrProjectInvalidProvider
	default:
		return fmt.Errorf("%w: %v", ErrGitValidationFailed, err)
	}
}

func normalizeListPagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func parseGitProvider(s string) (models.GitProvider, error) {
	gp := models.GitProvider(s)
	if gp == "" {
		return models.GitProviderLocal, nil
	}
	if !gp.IsValid() {
		return "", ErrProjectInvalidProvider
	}
	return gp, nil
}

func parseProjectStatus(s string) (models.ProjectStatus, error) {
	ps := models.ProjectStatus(s)
	if ps == "" {
		return models.ProjectStatusActive, nil
	}
	if !ps.IsValid() {
		return "", ErrProjectInvalidStatus
	}
	return ps, nil
}

func listFilterFromDTO(req dto.ListProjectsRequest) (repository.ProjectFilter, error) {
	limit, offset := normalizeListPagination(req.Limit, req.Offset)
	f := repository.ProjectFilter{
		Limit:    limit,
		Offset:   offset,
		OrderBy:  req.OrderBy,
		OrderDir: req.OrderDir,
	}
	if req.Search != nil {
		f.Search = req.Search
	}
	if req.Status != nil && *req.Status != "" {
		st := models.ProjectStatus(*req.Status)
		if !st.IsValid() {
			return repository.ProjectFilter{}, ErrProjectInvalidStatus
		}
		f.Status = &st
	}
	if req.GitProvider != nil && *req.GitProvider != "" {
		gp := models.GitProvider(*req.GitProvider)
		if !gp.IsValid() {
			return repository.ProjectFilter{}, ErrProjectInvalidProvider
		}
		f.GitProvider = &gp
	}
	return f, nil
}

// buildGitProvider расшифровывает credentials и создаёт экземпляр GitProvider.
func (s *projectService) buildGitProvider(
	ctx context.Context,
	providerType models.GitProvider,
	credentialID *uuid.UUID,
	userID uuid.UUID,
) (gitprovider.GitProvider, error) {
	creds := gitprovider.Credentials{}

	if credentialID != nil {
		gitCred, err := s.gitCredRepo.GetByID(ctx, *credentialID)
		if err != nil {
			return nil, mapGitCredRepoErr(err)
		}
		if gitCred.UserID != userID {
			return nil, ErrGitCredentialForbidden
		}
		// AAD для git_credentials: []byte(credential.ID.String()). При Create/Update в сервисе
		// сохранения кредов шифровать с тем же AAD после того, как ID строки известен (BeforeCreate/UUID).
		aad := []byte(gitCred.ID.String())
		decrypted, err := s.encryptor.Decrypt(gitCred.EncryptedValue, aad)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
		}
		switch gitCred.AuthType {
		case models.GitCredentialAuthToken, models.GitCredentialAuthOAuth:
			creds.Token = string(decrypted)
		case models.GitCredentialAuthSSHKey:
			creds.SSHKey = string(decrypted)
		}
	}

	provider, err := s.gitFactory.Create(string(providerType), creds)
	if err != nil {
		if errors.Is(err, gitprovider.ErrUnknownProvider) {
			return nil, ErrProjectInvalidProvider
		}
		return nil, fmt.Errorf("create git provider: %w", err)
	}

	return provider, nil
}

func (s *projectService) Create(ctx context.Context, userID uuid.UUID, req dto.CreateProjectRequest) (*models.Project, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrProjectInvalidName
	}
	gp, err := parseGitProvider(req.GitProvider)
	if err != nil {
		return nil, err
	}
	status, err := parseProjectStatus(req.Status)
	if err != nil {
		return nil, err
	}

	isRemote := gp != models.GitProviderLocal
	if !isRemote && req.GitCredentialID != nil {
		return nil, ErrGitCredentialNotSupportedForLocal
	}

	branch := strings.TrimSpace(req.GitDefaultBranch)
	if branch == "" {
		branch = "main"
	}

	gitURL := strings.TrimSpace(req.GitURL)

	var provider gitprovider.GitProvider

	if isRemote && gitURL != "" {
		provider, err = s.buildGitProvider(ctx, gp, req.GitCredentialID, userID)
		if err != nil {
			return nil, err
		}
		if err := provider.ValidateAccess(ctx, gitURL); err != nil {
			return nil, mapGitProviderErr(err)
		}
		repoInfo, infoErr := provider.GetRepoInfo(ctx, gitURL)
		if infoErr == nil && repoInfo != nil {
			if branch == "main" && repoInfo.DefaultBranch != "" {
				branch = repoInfo.DefaultBranch
			}
		}
	} else if isRemote && req.GitCredentialID != nil {
		if _, err := s.buildGitProvider(ctx, gp, req.GitCredentialID, userID); err != nil {
			return nil, err
		}
	}

	project := &models.Project{
		Name:             name,
		Description:      req.Description,
		GitProvider:      gp,
		GitURL:           gitURL,
		GitDefaultBranch: branch,
		GitCredentialsID: req.GitCredentialID,
		VectorCollection: req.VectorCollection,
		TechStack:        req.TechStack,
		Status:           status,
		Settings:         req.Settings,
		UserID:           userID,
	}

	if provider != nil && s.importDir != "" {
		project.Status = models.ProjectStatusIndexing
	}

	err = s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.projectRepo.Create(txCtx, project); err != nil {
			return mapProjectRepoErr(err)
		}
		team := &models.Team{
			Name:      devTeamDefaultName,
			ProjectID: project.ID,
			Type:      models.TeamTypeDevelopment,
		}
		return s.teamRepo.Create(txCtx, team)
	})
	if err != nil {
		return nil, err
	}

	if provider != nil && s.importDir != "" {
		go s.runIndexingPipeline(provider, project.ID, gitURL, branch)
	}

	return project, nil
}

func (s *projectService) runIndexingPipeline(
	provider gitprovider.GitProvider,
	projectID uuid.UUID,
	gitURL string,
	branch string,
) {
	maskedURL := maskGitURL(gitURL)

	// 1. Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			slog.Error("PANIC in runIndexingPipeline",
				slog.String("project_id", projectID.String()),
				slog.Any("recover", r),
			)
			// Пытаемся перевести статус в ошибку при панике
			if err := s.projectRepo.UpdateStatus(context.Background(), projectID, models.ProjectStatusIndexing, models.ProjectStatusIndexingFailed); err != nil {
				slog.Error("failed to update project status after panic",
					slog.String("project_id", projectID.String()),
					slog.String("error", err.Error()),
				)
			}
		}
	}()

	// 2. Context with timeout (detached from request)
	ctx, cancel := context.WithTimeout(context.Background(), indexingTimeout)
	defer cancel()

	// 3. Secure WorkDir
	workDir, err := os.MkdirTemp(s.importDir, fmt.Sprintf("project-%s-*", projectID))
	if err != nil {
		slog.Error("failed to create temp workdir",
			slog.String("project_id", projectID.String()),
			slog.String("error", err.Error()),
		)
		if err := s.projectRepo.UpdateStatus(ctx, projectID, models.ProjectStatusIndexing, models.ProjectStatusIndexingFailed); err != nil {
			slog.Error("failed to update project status after workdir error",
				slog.String("project_id", projectID.String()),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	// 4. Cleanup Logic (guaranteed removal)
	defer func() {
		if rmErr := os.RemoveAll(workDir); rmErr != nil {
			slog.Error("failed to remove workdir",
				slog.String("project_id", projectID.String()),
				slog.String("work_dir", workDir),
				slog.String("error", rmErr.Error()),
			)
		}
	}()

	// 5. Clone
	slog.Info("starting clone for indexing",
		slog.String("project_id", projectID.String()),
		slog.String("url", maskedURL),
	)

	if err := provider.Clone(ctx, gitURL, gitprovider.CloneOptions{
		Branch:   branch,
		DestPath: workDir,
		Depth:    0,
	}); err != nil {
		safeErr := strings.ReplaceAll(err.Error(), gitURL, maskedURL)
		slog.Error("clone failed",
			slog.String("project_id", projectID.String()),
			slog.String("error", safeErr),
		)
		if err := s.projectRepo.UpdateStatus(ctx, projectID, models.ProjectStatusIndexing, models.ProjectStatusIndexingFailed); err != nil {
			slog.Error("failed to update project status after clone error",
				slog.String("project_id", projectID.String()),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	// 6. Indexing
	slog.Info("starting code indexing",
		slog.String("project_id", projectID.String()),
		slog.String("work_dir", workDir),
	)
	err = s.indexer.IndexProject(ctx, indexer.IndexingRequest{
		ProjectID: projectID,
		RepoPath:  workDir,
	})

	if err != nil {
		slog.Error("indexing failed",
			slog.String("project_id", projectID.String()),
			slog.String("error", err.Error()),
		)
		if err := s.projectRepo.UpdateStatus(ctx, projectID, models.ProjectStatusIndexing, models.ProjectStatusIndexingFailed); err != nil {
			slog.Error("failed to update project status after indexing error",
				slog.String("project_id", projectID.String()),
				slog.String("error", err.Error()),
			)
		}
		return
	}

	// 7. Success Status Update
	if err := s.projectRepo.UpdateStatus(ctx, projectID, models.ProjectStatusIndexing, models.ProjectStatusReady); err != nil {
		slog.Error("failed to update status to Ready",
			slog.String("project_id", projectID.String()),
			slog.String("error", err.Error()),
		)
	}

	slog.Info("indexing pipeline completed successfully",
		slog.String("project_id", projectID.String()),
	)
}

func (s *projectService) GetByID(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (*models.Project, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return nil, mapProjectRepoErr(err)
	}
	if err := s.checkProjectAccess(project, userID, userRole); err != nil {
		return nil, err
	}
	return project, nil
}

func (s *projectService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, req dto.ListProjectsRequest) ([]models.Project, int64, error) {
	filter, err := listFilterFromDTO(req)
	if err != nil {
		return nil, 0, err
	}
	if userRole == models.RoleAdmin {
		return s.projectRepo.List(ctx, filter)
	}
	return s.projectRepo.ListByUserID(ctx, userID, filter)
}

func (s *projectService) Update(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpdateProjectRequest) (*models.Project, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return nil, mapProjectRepoErr(err)
	}
	if err := s.checkProjectAccess(project, userID, userRole); err != nil {
		return nil, err
	}

	if req.RemoveGitCredential && req.GitCredentialID != nil {
		return nil, ErrUpdateProjectGitCredentialConflict
	}
	if req.ClearTechStack && req.TechStack != nil {
		return nil, ErrUpdateProjectTechStackConflict
	}
	if req.ClearSettings && req.Settings != nil {
		return nil, ErrUpdateProjectSettingsConflict
	}

	oldGitURL := project.GitURL
	oldGitProvider := project.GitProvider

	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return nil, ErrProjectInvalidName
		}
		project.Name = n
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.GitProvider != nil {
		gp := models.GitProvider(*req.GitProvider)
		if !gp.IsValid() {
			return nil, ErrProjectInvalidProvider
		}
		project.GitProvider = gp
	}
	if req.GitURL != nil {
		project.GitURL = strings.TrimSpace(*req.GitURL)
	}
	if req.GitDefaultBranch != nil {
		b := strings.TrimSpace(*req.GitDefaultBranch)
		if b != "" {
			project.GitDefaultBranch = b
		}
	}
	switch {
	case req.RemoveGitCredential:
		project.GitCredentialsID = nil
	case req.GitCredentialID != nil:
		project.GitCredentialsID = req.GitCredentialID
	}
	if req.VectorCollection != nil {
		project.VectorCollection = *req.VectorCollection
	}
	if req.ClearTechStack {
		project.TechStack = jsonbEmptyObject
	} else if req.TechStack != nil {
		project.TechStack = *req.TechStack
	}
	if req.Status != nil {
		st := models.ProjectStatus(*req.Status)
		if !st.IsValid() {
			return nil, ErrProjectInvalidStatus
		}
		project.Status = st
	}
	if req.ClearSettings {
		project.Settings = jsonbEmptyObject
	} else if req.Settings != nil {
		project.Settings = *req.Settings
	}

	if project.GitProvider == models.GitProviderLocal && project.GitCredentialsID != nil {
		return nil, ErrGitCredentialNotSupportedForLocal
	}

	needsRevalidation := req.GitURL != nil || req.GitProvider != nil || req.GitCredentialID != nil || req.RemoveGitCredential
	isRemote := project.GitProvider != models.GitProviderLocal
	var provider gitprovider.GitProvider

	if needsRevalidation && isRemote {
		if project.GitURL != "" {
			provider, err = s.buildGitProvider(ctx, project.GitProvider, project.GitCredentialsID, userID)
			if err != nil {
				return nil, err
			}
			if err := provider.ValidateAccess(ctx, project.GitURL); err != nil {
				return nil, mapGitProviderErr(err)
			}
		} else if project.GitCredentialsID != nil {
			if _, err := s.buildGitProvider(ctx, project.GitProvider, project.GitCredentialsID, userID); err != nil {
				return nil, err
			}
		}
	}

	if err := s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.projectRepo.Update(txCtx, project); err != nil {
			return mapProjectRepoErr(err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	gitURLChanged := req.GitURL != nil && project.GitURL != oldGitURL
	providerChanged := oldGitProvider != project.GitProvider
	// Любая смена провайдера или URL: чистим importDir (в т.ч. remote→local), затем клон только если remote и есть URL.
	needsCloneRefresh := (gitURLChanged || providerChanged) && s.importDir != ""

	if needsCloneRefresh {
		oldWorkDir := filepath.Join(s.importDir, project.ID.String())
		if rmErr := os.RemoveAll(oldWorkDir); rmErr != nil {
			slog.Error("failed to remove import workdir",
				slog.String("project_id", project.ID.String()),
				slog.String("error", rmErr.Error()),
			)
		}

		if provider != nil && project.GitURL != "" {
			if err := s.projectRepo.UpdateStatus(ctx, project.ID, models.ProjectStatusActive, models.ProjectStatusIndexing); err != nil {
				slog.Error("failed to update status to Indexing",
					slog.String("project_id", project.ID.String()),
					slog.String("error", err.Error()),
				)
			} else {
				project.Status = models.ProjectStatusIndexing
				go s.runIndexingPipeline(provider, project.ID, project.GitURL, project.GitDefaultBranch)
			}
		}
	}

	return project, nil
}

func (s *projectService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return mapProjectRepoErr(err)
	}
	if err := s.checkProjectAccess(project, userID, userRole); err != nil {
		return err
	}

	err = s.projectRepo.Delete(ctx, projectID)
	if err != nil {
		return mapProjectRepoErr(err)
	}

	// Публикуем событие удаления проекта для очистки Weaviate и других ресурсов
	s.eventBus.Publish(ctx, events.ProjectDeleted{
		ProjectID:  projectID,
		OccurredAt: time.Now(),
	})

	return nil
}

// HasAccess проверяет доступ пользователя к проекту.
// Возвращает nil при успехе, ErrProjectNotFound если проект не существует,
// ErrProjectForbidden если доступ запрещен.
// Используется для WebSocket-авторизации (7.2).
func (s *projectService) HasAccess(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return mapProjectRepoErr(err)
	}
	return s.checkProjectAccess(project, userID, userRole)
}

// maskGitURL маскирует токены/пароли в URL.
func maskGitURL(url string) string {
	if !strings.Contains(url, "://") {
		return url
	}
	// Простейшая маскировка: https://user:token@github.com -> https://user:***@github.com
	parts := strings.SplitN(url, "://", 2)
	scheme := parts[0]
	rest := parts[1]

	atIndex := strings.LastIndex(rest, "@")
	if atIndex == -1 {
		return url
	}

	credentials := rest[:atIndex]
	hostPath := rest[atIndex:]

	if strings.Contains(credentials, ":") {
		credParts := strings.SplitN(credentials, ":", 2)
		return fmt.Sprintf("%s://%s:***%s", scheme, credParts[0], hostPath)
	}

	return fmt.Sprintf("%s://***%s", scheme, hostPath)
}
