package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	ErrProjectNotFound           = errors.New("project not found")
	ErrProjectNameExists         = errors.New("project with this name already exists")
	ErrProjectForbidden          = errors.New("access to project denied")
	ErrGitCredentialNotFound     = errors.New("git credential not found")
	ErrGitCredentialForbidden    = errors.New("git credential belongs to another user")
	ErrProjectInvalidName        = errors.New("project name is required")
	ErrProjectInvalidProvider    = errors.New("invalid git provider")
	ErrProjectInvalidStatus      = errors.New("invalid project status")
	ErrProjectIndexingConflict   = errors.New("project is already being indexed")
	ErrProjectLocalCannotReindex = errors.New("local projects cannot be reindexed via this API")

	ErrUpdateProjectGitCredentialConflict = errors.New("cannot use git_credential_id together with remove_git_credential")
	ErrUpdateProjectTechStackConflict     = errors.New("cannot use tech_stack together with clear_tech_stack")
	ErrUpdateProjectSettingsConflict      = errors.New("cannot use settings together with clear_settings")

	ErrGitValidationFailed               = errors.New("git access validation failed")
	ErrGitCloneFailed                    = errors.New("git clone failed")
	ErrDecryptionFailed                  = errors.New("failed to decrypt git credentials")
	ErrGitURLRequired                    = errors.New("git_url is required for remote git provider")
	ErrGitCredentialRequired             = errors.New("git_credential_id is required for remote git provider")
	ErrGitCredentialNotSupportedForLocal = errors.New("git_credential_id is not supported for local provider")

	// Мульти-репо.
	ErrRepoNotFound            = errors.New("project repository not found")
	ErrRepoSlugExists          = errors.New("project repository with this slug already exists")
	ErrRepoSlugRequired        = errors.New("repository slug is required")
	ErrRepoURLRequired         = errors.New("repository git_url is required")
	ErrCannotRemovePrimaryRepo = errors.New("cannot remove the primary repository while other repositories exist; reassign primary first")
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
	// Reindex запускает переиндексацию проекта в фоновом режиме.
	Reindex(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error
	// GetOwnerID возвращает user_id владельца проекта (без ABAC-проверки)
	GetOwnerID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	// RunBackgroundReindexing запускает фоновую переиндексацию для измененных проектов.
	RunBackgroundReindexing(ctx context.Context) error
	// SearchCode выполняет контекстный поиск по проиндексированному коду проекта.
	SearchCode(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, query string, limit int) ([]indexer.Chunk, error)
	// GetProjectRepoPath возвращает локальный путь к репозиторию проекта.
	GetProjectRepoPath(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (string, error)

	// --- Мульти-репо: управление репозиториями проекта ---
	// ListRepositories возвращает репозитории проекта (с ABAC).
	ListRepositories(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.ProjectRepository, error)
	// AddRepository добавляет репозиторий в проект.
	AddRepository(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.AddRepositoryRequest) (*models.ProjectRepository, error)
	// UpdateRepository частично обновляет репозиторий проекта.
	UpdateRepository(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, repoID uuid.UUID, req dto.UpdateRepositoryRequest) (*models.ProjectRepository, error)
	// RemoveRepository удаляет репозиторий проекта.
	RemoveRepository(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, repoID uuid.UUID) error
}

type projectService struct {
	projectRepo     repository.ProjectRepository
	projectRepoRepo repository.ProjectRepoRepository
	teamRepo        repository.TeamRepository
	gitCredRepo     repository.GitCredentialRepository
	gitIntegrations repository.GitIntegrationCredentialRepository
	transactions    repository.TransactionManager
	gitFactory      gitprovider.Factory
	encryptor       Encryptor
	eventBus        events.EventBus
	indexer         indexer.CodeIndexer
	importDir       string
	agentService    *AgentService

	// Сериализация индексации per-project: клоны идут в общий importDir/<projectID>[/<slug>],
	// поэтому два одновременных прохода (ручной Reindex / AddRepository / фоновый sync)
	// дерутся за один каталог → "invalid index-pack output" / "no such file" / конфликт
	// финального CAS статуса. Гард пускает один проход на проект; повторный запрос,
	// пришедший во время прохода, коалесцируется и выполняется одним догоняющим проходом
	// (чтобы добавленный во время индексации репо тоже проиндексировался).
	indexingMu      sync.Mutex
	indexingActive  map[uuid.UUID]struct{}
	indexingPending map[uuid.UUID]func()

	// tokenRefresher (опц.) рефрешит истёкшие OAuth-токены интеграционных аккаунтов перед
	// клонированием. Без него используется сохранённый токен (упадёт на 401 после истечения,
	// напр. self-hosted GitLab TTL ~2ч). Внедряется через WithGitTokenRefresher.
	tokenRefresher GitTokenRefresher
}

// GitTokenRefresher отдаёт валидный (при необходимости — обновлённый и персистнутый) OAuth
// access-token интеграционного аккаунта. Реализуется gitIntegrationService.FreshAccessToken.
type GitTokenRefresher interface {
	FreshAccessToken(ctx context.Context, cred *models.GitIntegrationCredential) (string, error)
}

// WithGitTokenRefresher внедряет рефрешер OAuth-токенов (post-construction: gitIntegrationSvc
// создаётся позже projectService в app.go).
func WithGitTokenRefresher(svc ProjectService, r GitTokenRefresher) ProjectService {
	if ps, ok := svc.(*projectService); ok {
		ps.tokenRefresher = r
	}
	return svc
}

// startIndexing запускает индексацию проекта под per-project гардом. Если проход уже
// идёт — запоминает последний запрошенный run и выходит; догоняющий проход выполнится
// после завершения текущего. run сам отвечает за перевод статуса проекта indexing→ready/failed.
func (s *projectService) startIndexing(projectID uuid.UUID, run func()) {
	s.indexingMu.Lock()
	if s.indexingActive == nil {
		s.indexingActive = make(map[uuid.UUID]struct{})
	}
	if s.indexingPending == nil {
		s.indexingPending = make(map[uuid.UUID]func())
	}
	if _, active := s.indexingActive[projectID]; active {
		// Коалесцируем: достаточно одного догоняющего прохода (он переиндексирует все репо).
		s.indexingPending[projectID] = run
		s.indexingMu.Unlock()
		return
	}
	s.indexingActive[projectID] = struct{}{}
	s.indexingMu.Unlock()

	go func() {
		current := run
		for {
			current()

			s.indexingMu.Lock()
			pending, ok := s.indexingPending[projectID]
			if !ok {
				delete(s.indexingActive, projectID)
				s.indexingMu.Unlock()
				return
			}
			delete(s.indexingPending, projectID)
			s.indexingMu.Unlock()
			// Догоняющий проход: предыдущий проход уже выставил ready/failed, поэтому
			// перед повтором возвращаем status=indexing — иначе финальный CAS не сработает.
			s.ensureIndexingStatus(projectID)
			current = pending
		}
	}()
}

// ensureIndexingStatus переводит проект в status=indexing, если он сейчас в другом статусе
// (для догоняющего прохода гарда). Best-effort: ошибки не критичны для самой индексации.
func (s *projectService) ensureIndexingStatus(projectID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return
	}
	if p.Status == models.ProjectStatusIndexing {
		return
	}
	_ = s.projectRepo.UpdateStatus(ctx, projectID, p.Status, models.ProjectStatusIndexing)
}

// NewProjectService создаёт сервис проектов.
//
// gitIntegrations может быть nil (на момент написания не все вызовы main передают
// репозиторий, и старые тесты тоже) — в этом случае fallback на OAuth-токен из
// git_integration_credentials отключён, поведение совпадает со старым.
func NewProjectService(
	projectRepo repository.ProjectRepository,
	projectRepoRepo repository.ProjectRepoRepository,
	teamRepo repository.TeamRepository,
	gitCredRepo repository.GitCredentialRepository,
	gitIntegrations repository.GitIntegrationCredentialRepository,
	transactions repository.TransactionManager,
	gitFactory gitprovider.Factory,
	encryptor Encryptor,
	eventBus events.EventBus,
	indexer indexer.CodeIndexer,
	importDir string,
) ProjectService {
	return &projectService{
		projectRepo:     projectRepo,
		projectRepoRepo: projectRepoRepo,
		teamRepo:        teamRepo,
		gitCredRepo:     gitCredRepo,
		gitIntegrations: gitIntegrations,
		transactions:    transactions,
		gitFactory:      gitFactory,
		encryptor:       encryptor,
		eventBus:        eventBus,
		indexer:         indexer,
		importDir:       importDir,
	}
}

// WithAgentService sets the AgentService for auto-creating project agents.
func WithAgentService(svc ProjectService, agentSvc *AgentService) ProjectService {
	if ps, ok := svc.(*projectService); ok {
		ps.agentService = agentSvc
	}
	return svc
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

// mapGitProviderToIntegration переводит models.GitProvider (поле проекта) в
// models.GitIntegrationProvider (ключ OAuth-таблицы). Для local — false.
func mapGitProviderToIntegration(p models.GitProvider) (models.GitIntegrationProvider, bool) {
	switch p {
	case models.GitProviderGitHub:
		return models.GitIntegrationProviderGitHub, true
	case models.GitProviderGitLab:
		return models.GitIntegrationProviderGitLab, true
	default:
		return "", false
	}
}

// buildGitProvider расшифровывает credentials и создаёт экземпляр GitProvider.
func (s *projectService) buildGitProvider(
	ctx context.Context,
	providerType models.GitProvider,
	credentialID *uuid.UUID,
	userID uuid.UUID,
	integrationCredID *uuid.UUID,
) (gitprovider.GitProvider, error) {
	creds := gitprovider.Credentials{}

	switch {
	case credentialID != nil:
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
	case integrationCredID != nil && s.gitIntegrations != nil:
		// Мульти-аккаунт: явно выбранный OAuth-аккаунт.
		cred, err := s.gitIntegrations.GetByID(ctx, *integrationCredID)
		if err == nil && cred != nil && cred.UserID == userID && len(cred.AccessTokenEnc) > 0 {
			token, terr := s.integrationToken(ctx, cred)
			if terr != nil {
				return nil, terr
			}
			creds.Token = token
		}
	case s.gitIntegrations != nil:
		// Fallback: если creds к проекту не привязаны, но юзер подключал
		// провайдер через OAuth (страница «Git-провайдеры»), используем тот
		// токен (первый аккаунт провайдера). Без этого создание GitHub/GitLab-проекта
		// без явных кредов шло анонимно и валилось на приватных репах.
		if integProvider, ok := mapGitProviderToIntegration(providerType); ok {
			cred, err := s.gitIntegrations.GetByUserAndProvider(ctx, userID, integProvider)
			if err == nil && cred != nil && len(cred.AccessTokenEnc) > 0 {
				token, terr := s.integrationToken(ctx, cred)
				if terr != nil {
					return nil, terr
				}
				creds.Token = token
			}
			// На случай repository.ErrGitIntegrationNotFound и прочих ошибок —
			// тихо падаем в анонимный клиент: для публичных репо этого хватит,
			// для приватных бэк отдаст «repository not found» (диагностируемо).
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

// integrationToken отдаёт живой access-token интеграционного аккаунта: через tokenRefresher
// (рефреш истёкшего OAuth-токена + персист), иначе — расшифровкой сохранённого. Истёкший токен
// без рефрешера → 401 на клонировании (self-hosted GitLab TTL ~2ч).
func (s *projectService) integrationToken(ctx context.Context, cred *models.GitIntegrationCredential) (string, error) {
	if s.tokenRefresher != nil {
		token, err := s.tokenRefresher.FreshAccessToken(ctx, cred)
		if err != nil {
			return "", fmt.Errorf("%w: %v", ErrGitValidationFailed, err)
		}
		return token, nil
	}
	aad := repository.GitIntegrationCredentialAAD(cred.ID)
	decrypted, err := s.encryptor.Decrypt(cred.AccessTokenEnc, aad)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}
	return string(decrypted), nil
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
		provider, err = s.buildGitProvider(ctx, gp, req.GitCredentialID, userID, req.GitIntegrationCredentialID)
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
		if _, err := s.buildGitProvider(ctx, gp, req.GitCredentialID, userID, req.GitIntegrationCredentialID); err != nil {
			return nil, err
		}
	}

	project := &models.Project{
		Name:                       name,
		Description:                req.Description,
		GitProvider:                gp,
		GitURL:                     gitURL,
		GitDefaultBranch:           branch,
		GitCredentialsID:           req.GitCredentialID,
		GitIntegrationCredentialID: req.GitIntegrationCredentialID,
		VectorCollection:           req.VectorCollection,
		TechStack:                  req.TechStack,
		Status:                     status,
		Settings:                   req.Settings,
		UserID:                     userID,
	}

	if provider != nil && s.importDir != "" {
		project.Status = models.ProjectStatusIndexing
	}

	err = s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.projectRepo.Create(txCtx, project); err != nil {
			return mapProjectRepoErr(err)
		}
		// Мульти-репо: для проекта с git_url заводим primary-репозиторий slug='main',
		// зеркалящий git-поля проекта (бэк-компат с одно-репо моделью).
		if gitURL != "" && s.projectRepoRepo != nil {
			primaryRepo := &models.ProjectRepository{
				ProjectID:                  project.ID,
				Slug:                       "main",
				DisplayName:                name,
				GitProvider:                gp,
				GitURL:                     gitURL,
				GitDefaultBranch:           branch,
				GitCredentialsID:           req.GitCredentialID,
				GitIntegrationCredentialID: req.GitIntegrationCredentialID,
				VectorCollection:           req.VectorCollection,
				Status:                     models.ProjectStatusActive,
				IsPrimary:                  true,
				SortOrder:                  0,
			}
			if err := s.projectRepoRepo.Create(txCtx, primaryRepo); err != nil {
				return err
			}
		}
		team := &models.Team{
			Name:      devTeamDefaultName,
			ProjectID: project.ID,
			Type:      models.TeamTypeDevelopment,
		}
		if err := s.teamRepo.Create(txCtx, team); err != nil {
			return err
		}
		if s.agentService != nil {
			return s.agentService.CreateDefaultProjectAgents(txCtx, team.ID, string(team.Type))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if provider != nil && s.importDir != "" {
		// Create всегда даёт ровно один (primary) репо → одно-репо пайплайн (без префикса).
		// Мульти-репо возникает позже через AddRepository, который сам префиксует и
		// переиндексирует весь проект.
		pid, gp, gu, br := project.ID, provider, gitURL, branch
		s.startIndexing(pid, func() { s.runIndexingPipeline(gp, pid, gu, br, "") })
	}

	return project, nil
}

func (s *projectService) runIndexingPipeline(
	provider gitprovider.GitProvider,
	projectID uuid.UUID,
	gitURL string,
	branch string,
	commitSHA string,
) {
	var pipelineErr error
	maskedURL := maskGitURL(gitURL)

	// 1. Final Status Update & Panic Recovery
	defer func() {
		if r := recover(); r != nil {
			pipelineErr = fmt.Errorf("panic: %v", r)
			slog.Error("PANIC in runIndexingPipeline",
				slog.String("project_id", projectID.String()),
				slog.Any("recover", r),
			)
		}

		finalStatus := models.ProjectStatusReady
		if pipelineErr != nil {
			finalStatus = models.ProjectStatusIndexingFailed
		}

		var updateErr error
		if pipelineErr == nil {
			updateErr = s.projectRepo.UpdateStatusAndCommit(context.Background(), projectID, models.ProjectStatusIndexing, finalStatus, commitSHA)
		} else {
			updateErr = s.projectRepo.UpdateStatus(context.Background(), projectID, models.ProjectStatusIndexing, finalStatus)
		}

		if updateErr != nil {
			slog.Error("failed to update final project status",
				slog.String("project_id", projectID.String()),
				slog.String("from", string(models.ProjectStatusIndexing)),
				slog.String("to", string(finalStatus)),
				slog.String("error", updateErr.Error()),
			)
		}

		if pipelineErr == nil {
			slog.Info("indexing pipeline completed successfully",
				slog.String("project_id", projectID.String()),
			)
		}
	}()

	// 2. Context with timeout (detached from request)
	ctx, cancel := context.WithTimeout(context.Background(), indexingTimeout)
	defer cancel()

	if provider != nil {
		// Run ValidateAccess. If it succeeds, but GetLatestCommitSHA(ctx, gitURL, "") fails, it is an empty repository.
		if valErr := provider.ValidateAccess(ctx, gitURL); valErr == nil {
			if _, shaErr := provider.GetLatestCommitSHA(ctx, gitURL, ""); shaErr != nil {
				slog.Info("empty repository detected during indexing, attempting to initialize it",
					slog.String("project_id", projectID.String()),
					slog.String("url", maskedURL),
				)
				if err := os.MkdirAll(s.importDir, 0755); err != nil {
					pipelineErr = fmt.Errorf("failed to create import dir: %w", err)
					return
				}
				if err := s.initializeEmptyRepo(ctx, provider, gitURL, branch, projectID); err != nil {
					pipelineErr = fmt.Errorf("failed to initialize empty repository: %w", err)
					slog.Error("failed to initialize empty repository",
						slog.String("project_id", projectID.String()),
						slog.String("error", err.Error()),
					)
					return
				}
			}
		}
	}

	if commitSHA == "" && provider != nil {
		if sha, shaErr := provider.GetLatestCommitSHA(ctx, gitURL, branch); shaErr == nil {
			commitSHA = sha
		} else {
			slog.Warn("failed to fetch latest commit SHA during indexing pipeline",
				slog.String("project_id", projectID.String()),
				slog.String("error", shaErr.Error()),
			)
		}
	}

	// 3. Secure WorkDir
	if err := os.MkdirAll(s.importDir, 0755); err != nil {
		pipelineErr = fmt.Errorf("failed to create import dir: %w", err)
		slog.Error("failed to create import dir",
			slog.String("project_id", projectID.String()),
			slog.String("error", err.Error()),
		)
		return
	}
	workDir := filepath.Join(s.importDir, projectID.String())
	// Clean up existing folder to guarantee a clean clone
	if err := os.RemoveAll(workDir); err != nil {
		slog.Warn("failed to clean up existing workdir before cloning",
			slog.String("project_id", projectID.String()),
			slog.String("error", err.Error()),
		)
	}

	// 4. Cleanup Logic (removal ONLY on failure)
	defer func() {
		if pipelineErr != nil {
			if rmErr := os.RemoveAll(workDir); rmErr != nil {
				slog.Error("failed to remove workdir on failure",
					slog.String("project_id", projectID.String()),
					slog.String("work_dir", workDir),
					slog.String("error", rmErr.Error()),
				)
			}
		}
	}()

	// 5. Clone
	slog.Info("starting clone for indexing",
		slog.String("project_id", projectID.String()),
		slog.String("url", maskedURL),
	)

	if pipelineErr = provider.Clone(ctx, gitURL, gitprovider.CloneOptions{
		Branch:   branch,
		DestPath: workDir,
		Depth:    0,
	}); pipelineErr != nil {
		safeErr := strings.ReplaceAll(pipelineErr.Error(), gitURL, maskedURL)
		slog.Error("clone failed",
			slog.String("project_id", projectID.String()),
			slog.String("error", safeErr),
		)
		return
	}

	// 6. Indexing
	slog.Info("starting code indexing",
		slog.String("project_id", projectID.String()),
		slog.String("work_dir", workDir),
	)
	if pipelineErr = s.indexer.IndexProject(ctx, indexer.IndexingRequest{
		ProjectID: projectID,
		RepoPath:  workDir,
	}); pipelineErr != nil {
		slog.Error("indexing failed",
			slog.String("project_id", projectID.String()),
			slog.String("error", pipelineErr.Error()),
		)
		return
	}
}

// cloneAndIndexInto клонирует репозиторий в workDir и индексирует его в project-namespace
// с префиксом pathPrefix (мульти-репо; пустой префикс — индексация без префикса). Пустой
// remote инициализируется. Возвращает ошибку; статусы/cleanup — на ответственности вызывающего.
func (s *projectService) cloneAndIndexInto(ctx context.Context, provider gitprovider.GitProvider, projectID uuid.UUID, gitURL, branch, workDir, pathPrefix string) error {
	maskedURL := maskGitURL(gitURL)

	// Пустой remote → инициализируем (README + первый коммит), как в одно-репо пайплайне.
	if provider != nil {
		if valErr := provider.ValidateAccess(ctx, gitURL); valErr == nil {
			if _, shaErr := provider.GetLatestCommitSHA(ctx, gitURL, ""); shaErr != nil {
				if err := os.MkdirAll(s.importDir, 0755); err != nil {
					return fmt.Errorf("create import dir: %w", err)
				}
				if err := s.initializeEmptyRepo(ctx, provider, gitURL, branch, projectID); err != nil {
					return fmt.Errorf("initialize empty repository: %w", err)
				}
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(workDir), 0755); err != nil {
		return fmt.Errorf("create import dir: %w", err)
	}
	if err := os.RemoveAll(workDir); err != nil {
		slog.Warn("failed to clean workdir before clone",
			slog.String("project_id", projectID.String()), slog.String("error", err.Error()))
	}

	if err := provider.Clone(ctx, gitURL, gitprovider.CloneOptions{Branch: branch, DestPath: workDir, Depth: 0}); err != nil {
		safeErr := strings.ReplaceAll(err.Error(), gitURL, maskedURL)
		return fmt.Errorf("clone failed: %s", safeErr)
	}

	if err := s.indexer.IndexProject(ctx, indexer.IndexingRequest{
		ProjectID:  projectID,
		RepoPath:   workDir,
		PathPrefix: pathPrefix,
	}); err != nil {
		return fmt.Errorf("indexing failed: %w", err)
	}
	return nil
}

// runProjectIndexing индексирует все репозитории проекта (мульти-репо). Каждый репо
// индексируется в общий project-namespace; при >1 репо пути префиксуются slug'ом репо.
// Per-repo status/last_indexed_commit обновляются индивидуально, статус проекта — агрегат.
// ownerID — владелец проекта (для резолва git-credential'ов репозиториев).
//
// Контракт: вызывающий ДОЛЖЕН предварительно перевести проект в status=indexing (CAS),
// чтобы финальный CAS indexing→ready/failed сработал.
func (s *projectService) runProjectIndexing(projectID, ownerID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), indexingTimeout)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("PANIC in runProjectIndexing", slog.String("project_id", projectID.String()), slog.Any("recover", r))
			_ = s.projectRepo.UpdateStatus(context.Background(), projectID, models.ProjectStatusIndexing, models.ProjectStatusIndexingFailed)
		}
	}()

	repos, err := s.projectRepoRepo.ListByProject(ctx, projectID)
	if err != nil {
		slog.Error("project indexing: list repos failed", slog.String("project_id", projectID.String()), slog.String("error", err.Error()))
		_ = s.projectRepo.UpdateStatus(ctx, projectID, models.ProjectStatusIndexing, models.ProjectStatusIndexingFailed)
		return
	}
	if len(repos) == 0 {
		// Проект без репо-реестра — индексировать нечего.
		_ = s.projectRepo.UpdateStatus(ctx, projectID, models.ProjectStatusIndexing, models.ProjectStatusReady)
		return
	}

	multi := len(repos) > 1
	anyFailed := false
	prefixes := make([]string, 0, len(repos))

	for i := range repos {
		repo := &repos[i]
		if repo.GitProvider == models.GitProviderLocal || repo.GitURL == "" {
			continue // local / без URL не индексируем
		}
		prefix := ""
		workDir := filepath.Join(s.importDir, projectID.String())
		if multi {
			prefix = repo.Slug
			workDir = filepath.Join(s.importDir, projectID.String(), repo.Slug)
			prefixes = append(prefixes, repo.Slug)
		}

		provider, perr := s.buildGitProvider(ctx, repo.GitProvider, repo.GitCredentialsID, ownerID, repo.GitIntegrationCredentialID)
		if perr != nil {
			anyFailed = true
			_ = s.projectRepoRepo.UpdateIndexStatus(ctx, repo.ID, models.ProjectStatusIndexingFailed, "")
			slog.Error("project indexing: build provider failed", slog.String("repo", repo.Slug), slog.String("error", perr.Error()))
			continue
		}

		branch := repo.GitDefaultBranch
		if branch == "" {
			branch = "main"
		}
		commitSHA := ""
		if sha, e := provider.GetLatestCommitSHA(ctx, repo.GitURL, branch); e == nil {
			commitSHA = sha
		}

		_ = s.projectRepoRepo.UpdateIndexStatus(ctx, repo.ID, models.ProjectStatusIndexing, "")
		if ierr := s.cloneAndIndexInto(ctx, provider, projectID, repo.GitURL, branch, workDir, prefix); ierr != nil {
			anyFailed = true
			_ = s.projectRepoRepo.UpdateIndexStatus(ctx, repo.ID, models.ProjectStatusIndexingFailed, "")
			_ = os.RemoveAll(workDir)
			slog.Error("project indexing: repo failed", slog.String("project_id", projectID.String()), slog.String("repo", repo.Slug), slog.String("error", ierr.Error()))
			continue
		}
		_ = s.projectRepoRepo.UpdateIndexStatus(ctx, repo.ID, models.ProjectStatusReady, commitSHA)
		slog.Info("project indexing: repo done", slog.String("project_id", projectID.String()), slog.String("repo", repo.Slug))
	}

	// Мульти-репо: вычищаем legacy не-префиксованные записи, оставшиеся от индексации
	// до перехода на мульти-репо (одно-разовая чистка; дальше — no-op).
	if multi && len(prefixes) > 0 {
		if perr := s.indexer.PruneToPrefixes(ctx, projectID, prefixes); perr != nil {
			slog.Warn("project indexing: prune to prefixes failed", slog.String("project_id", projectID.String()), slog.String("error", perr.Error()))
		}
	}

	final := models.ProjectStatusReady
	if anyFailed {
		final = models.ProjectStatusIndexingFailed
	}
	if uerr := s.projectRepo.UpdateStatus(ctx, projectID, models.ProjectStatusIndexing, final); uerr != nil {
		slog.Error("project indexing: final project status update failed", slog.String("project_id", projectID.String()), slog.String("error", uerr.Error()))
	}
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

// GetOwnerID возвращает user_id владельца проекта без ABAC-проверки.
// Используется внутренними продюсерами событий, у которых уже нет на руках
// userID (например, оркестратор v2 при `Transition`), но которым нужен
// user_id для user-scoped fan-out в WebSocket (см. Sprint 21 §7).
func (s *projectService) GetOwnerID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return uuid.Nil, mapProjectRepoErr(err)
	}
	return project.UserID, nil
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
	// Мульти-аккаунт: выбор/отвязка OAuth-аккаунта провайдера.
	switch {
	case req.RemoveGitIntegrationCredential:
		project.GitIntegrationCredentialID = nil
	case req.GitIntegrationCredentialID != nil:
		project.GitIntegrationCredentialID = req.GitIntegrationCredentialID
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

	needsRevalidation := req.GitURL != nil || req.GitProvider != nil || req.GitCredentialID != nil || req.RemoveGitCredential ||
		req.GitIntegrationCredentialID != nil || req.RemoveGitIntegrationCredential
	isRemote := project.GitProvider != models.GitProviderLocal
	var provider gitprovider.GitProvider

	// Эффективный OAuth-аккаунт для валидации: новый выбор из req (или сброс), иначе текущий проекта.
	effectiveIntegID := project.GitIntegrationCredentialID
	if req.RemoveGitIntegrationCredential {
		effectiveIntegID = nil
	} else if req.GitIntegrationCredentialID != nil {
		effectiveIntegID = req.GitIntegrationCredentialID
	}

	if needsRevalidation && isRemote {
		if project.GitURL != "" {
			provider, err = s.buildGitProvider(ctx, project.GitProvider, project.GitCredentialsID, userID, effectiveIntegID)
			if err != nil {
				return nil, err
			}
			if err := provider.ValidateAccess(ctx, project.GitURL); err != nil {
				return nil, mapGitProviderErr(err)
			}
		} else if project.GitCredentialsID != nil {
			if _, err := s.buildGitProvider(ctx, project.GitProvider, project.GitCredentialsID, userID, effectiveIntegID); err != nil {
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
				)
			} else {
				project.Status = models.ProjectStatusIndexing
				pid, gp, gu, br := project.ID, provider, project.GitURL, project.GitDefaultBranch
				s.startIndexing(pid, func() { s.runIndexingPipeline(gp, pid, gu, br, "") })
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

	// Удаляем локальный клон проекта
	if s.importDir != "" {
		_ = os.RemoveAll(filepath.Join(s.importDir, projectID.String()))
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

func (s *projectService) Reindex(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) error {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return mapProjectRepoErr(err)
	}
	if err := s.checkProjectAccess(project, userID, userRole); err != nil {
		return err
	}

	if project.Status == models.ProjectStatusIndexing {
		return ErrProjectIndexingConflict
	}

	if project.GitProvider == models.GitProviderLocal || project.GitURL == "" {
		return ErrProjectLocalCannotReindex
	}

	// Для remote-проектов нужен провайдер
	var provider gitprovider.GitProvider
	provider, err = s.buildGitProvider(ctx, project.GitProvider, project.GitCredentialsID, userID, project.GitIntegrationCredentialID)
	if err != nil {
		return err
	}
	if err := provider.ValidateAccess(ctx, project.GitURL); err != nil {
		return mapGitProviderErr(err)
	}

	latestSHA, err := provider.GetLatestCommitSHA(ctx, project.GitURL, project.GitDefaultBranch)
	if err != nil {
		slog.Warn("failed to fetch latest commit SHA during manual reindexing",
			slog.String("project_id", project.ID.String()),
			slog.String("error", err.Error()),
		)
	}

	// Обновляем статус на Indexing (CAS)
	if err := s.projectRepo.UpdateStatus(ctx, projectID, project.Status, models.ProjectStatusIndexing); err != nil {
		// Если не удалось обновить, возможно статус уже изменился
		return ErrProjectIndexingConflict
	}

	// Мульти-репо: проект из нескольких репо переиндексируем через per-repo оркестратор
	// (с префиксами + prune legacy-записей); одно-репо — прежний пайплайн.
	pid, uid := project.ID, userID
	if len(project.Repositories) > 1 {
		s.startIndexing(pid, func() { s.runProjectIndexing(pid, uid) })
	} else {
		gp, gu, br, sha := provider, project.GitURL, project.GitDefaultBranch, latestSHA
		s.startIndexing(pid, func() { s.runIndexingPipeline(gp, pid, gu, br, sha) })
	}

	return nil
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

func (s *projectService) RunBackgroundReindexing(ctx context.Context) error {
	slog.Info("starting periodic background reindexing check")

	projects, _, err := s.projectRepo.List(ctx, repository.ProjectFilter{
		Limit: 1000,
	})
	if err != nil {
		return fmt.Errorf("failed to list projects for background sync: %w", err)
	}

	for _, project := range projects {
		if project.GitProvider == models.GitProviderLocal || project.GitURL == "" {
			continue
		}

		if project.Status != models.ProjectStatusReady &&
			project.Status != models.ProjectStatusActive &&
			project.Status != models.ProjectStatusIndexingFailed {
			continue
		}

		// Change-detection по project-level git (primary-репо). Для дополнительных репо
		// мульти-репо проекта авто-детект изменений в фоне не выполняется — их покрывают
		// ручной Reindex и AddRepository. См. [[multi_repo_support]].
		provider, err := s.buildGitProvider(ctx, project.GitProvider, project.GitCredentialsID, project.UserID, project.GitIntegrationCredentialID)
		if err != nil {
			slog.Error("failed to build git provider for project background sync",
				slog.String("project_id", project.ID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		latestSHA, err := provider.GetLatestCommitSHA(ctx, project.GitURL, project.GitDefaultBranch)
		if err != nil {
			slog.Warn("failed to fetch latest commit SHA for background sync",
				slog.String("project_id", project.ID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		if project.LastIndexedCommit == latestSHA {
			continue
		}

		slog.Info("detected remote changes in project repository, triggering reindexing",
			slog.String("project_id", project.ID.String()),
			slog.String("old_sha", project.LastIndexedCommit),
			slog.String("new_sha", latestSHA),
		)

		if err := s.projectRepo.UpdateStatus(ctx, project.ID, project.Status, models.ProjectStatusIndexing); err != nil {
			slog.Info("project status changed concurrently, skipping background sync",
				slog.String("project_id", project.ID.String()),
				slog.String("error", err.Error()),
			)
			continue
		}

		// Мульти-репо: если проект состоит из нескольких репо — переиндексируем все с
		// префиксами; иначе прежний одно-репо пайплайн.
		multi := false
		if s.projectRepoRepo != nil {
			if repos, rerr := s.projectRepoRepo.ListByProject(ctx, project.ID); rerr == nil && len(repos) > 1 {
				multi = true
			}
		}
		pid, uid := project.ID, project.UserID
		if multi {
			s.startIndexing(pid, func() { s.runProjectIndexing(pid, uid) })
		} else {
			gp, gu, br, sha := provider, project.GitURL, project.GitDefaultBranch, latestSHA
			s.startIndexing(pid, func() { s.runIndexingPipeline(gp, pid, gu, br, sha) })
		}
	}

	return nil
}

func (s *projectService) initializeEmptyRepo(
	ctx context.Context,
	provider gitprovider.GitProvider,
	gitURL string,
	branch string,
	projectID uuid.UUID,
) error {
	tempDir, err := os.MkdirTemp(s.importDir, fmt.Sprintf("init-%s-*", projectID))
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	cmd = exec.CommandContext(ctx, "git", "config", "user.name", "PolyMaths AI")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config user.name: %w", err)
	}

	cmd = exec.CommandContext(ctx, "git", "config", "user.email", "ai@polymaths.local")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git config user.email: %w", err)
	}

	readmePath := filepath.Join(tempDir, "README.md")
	readmeContent := fmt.Sprintf("# Project %s\n\nInitial repository created automatically by PolyMaths AI.\n", projectID)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("write README.md: %w", err)
	}

	_, _, err = provider.Commit(ctx, tempDir, gitprovider.CommitOptions{
		Message: "Initial commit",
		Author: gitprovider.Author{
			Name:  "PolyMaths AI",
			Email: "ai@polymaths.local",
		},
	})
	if err != nil {
		return fmt.Errorf("provider commit: %w", err)
	}

	cmd = exec.CommandContext(ctx, "git", "remote", "add", "origin", gitURL)
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}

	cmd = exec.CommandContext(ctx, "git", "branch", "-M", branch)
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git branch rename: %w", err)
	}

	err = provider.Push(ctx, tempDir, gitprovider.PushOptions{
		Branch: branch,
		Remote: "origin",
	})
	if err != nil {
		return fmt.Errorf("provider push: %w", err)
	}

	return nil
}

// SearchCode выполняет контекстный поиск по проиндексированному коду проекта.
func (s *projectService) SearchCode(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, query string, limit int) ([]indexer.Chunk, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return nil, mapProjectRepoErr(err)
	}
	if err := s.checkProjectAccess(project, userID, userRole); err != nil {
		return nil, err
	}
	return s.indexer.SearchContext(ctx, projectID, query, limit)
}

// GetProjectRepoPath возвращает локальный путь к репозиторию проекта.
func (s *projectService) GetProjectRepoPath(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) (string, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return "", mapProjectRepoErr(err)
	}
	if err := s.checkProjectAccess(project, userID, userRole); err != nil {
		return "", err
	}
	if project.GitProvider == models.GitProviderLocal {
		return project.GitURL, nil
	}
	return filepath.Join(s.importDir, projectID.String()), nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Мульти-репо: управление репозиториями проекта
// ─────────────────────────────────────────────────────────────────────────────

// mapProjectRepoRepoErr маппит ошибки репо-слоя репозиториев в ошибки сервиса.
func mapProjectRepoRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrProjectRepoNotFound) {
		return ErrRepoNotFound
	}
	if errors.Is(err, repository.ErrProjectRepoSlugExists) {
		return ErrRepoSlugExists
	}
	return err
}

func (s *projectService) ListRepositories(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.ProjectRepository, error) {
	if err := s.HasAccess(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	repos, err := s.projectRepoRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, mapProjectRepoRepoErr(err)
	}
	return repos, nil
}

func (s *projectService) AddRepository(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.AddRepositoryRequest) (*models.ProjectRepository, error) {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		return nil, mapProjectRepoErr(err)
	}
	if err := s.checkProjectAccess(project, userID, userRole); err != nil {
		return nil, err
	}

	slug := strings.TrimSpace(req.Slug)
	if slug == "" {
		return nil, ErrRepoSlugRequired
	}
	gitURL := strings.TrimSpace(req.GitURL)
	if gitURL == "" {
		return nil, ErrRepoURLRequired
	}
	gp, err := parseGitProvider(req.GitProvider)
	if err != nil {
		return nil, err
	}
	branch := strings.TrimSpace(req.GitDefaultBranch)
	if branch == "" {
		branch = "main"
	}

	// Для remote-репо валидируем доступ (как при создании проекта).
	if gp != models.GitProviderLocal {
		provider, perr := s.buildGitProvider(ctx, gp, req.GitCredentialID, userID, req.GitIntegrationCredentialID)
		if perr != nil {
			return nil, perr
		}
		if verr := provider.ValidateAccess(ctx, gitURL); verr != nil {
			return nil, mapGitProviderErr(verr)
		}
	}

	repo := &models.ProjectRepository{
		ProjectID:                  projectID,
		Slug:                       slug,
		DisplayName:                strings.TrimSpace(req.DisplayName),
		RoleDescription:            req.RoleDescription,
		GitProvider:                gp,
		GitURL:                     gitURL,
		GitDefaultBranch:           branch,
		GitCredentialsID:           req.GitCredentialID,
		GitIntegrationCredentialID: req.GitIntegrationCredentialID,
		Status:                     models.ProjectStatusActive,
		IsPrimary:                  req.IsPrimary,
		SortOrder:                  req.SortOrder,
	}
	// Первый репозиторий проекта всегда становится primary.
	if len(project.Repositories) == 0 {
		repo.IsPrimary = true
	}

	err = s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
		if repo.IsPrimary {
			if cerr := s.projectRepoRepo.ClearPrimary(txCtx, projectID, uuid.Nil); cerr != nil {
				return cerr
			}
		}
		return s.projectRepoRepo.Create(txCtx, repo)
	})
	if err != nil {
		return nil, mapProjectRepoRepoErr(err)
	}

	// Индексируем добавленный репозиторий (переиндексация проекта — корректно обрабатывает
	// переход single→multi: префиксует пути и вычищает legacy не-префиксованные записи).
	// Через startIndexing: если индексация уже идёт — запрос коалесцируется в догоняющий
	// проход, который подхватит новый репо. Статус в indexing выставляем best-effort для
	// первого прохода; если уже idёт индексация (status=indexing) — CAS no-op, ловит coalesce.
	if s.importDir != "" && gp != models.GitProviderLocal {
		_ = s.projectRepo.UpdateStatus(ctx, projectID, project.Status, models.ProjectStatusIndexing)
		pid, uid := projectID, userID
		s.startIndexing(pid, func() { s.runProjectIndexing(pid, uid) })
	}
	return repo, nil
}

func (s *projectService) UpdateRepository(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, repoID uuid.UUID, req dto.UpdateRepositoryRequest) (*models.ProjectRepository, error) {
	if err := s.HasAccess(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	repo, err := s.projectRepoRepo.GetByID(ctx, repoID)
	if err != nil {
		return nil, mapProjectRepoRepoErr(err)
	}
	if repo.ProjectID != projectID {
		return nil, ErrRepoNotFound
	}
	// Не сохраняем association (иначе GORM Save попытается upsert'нуть git_credential).
	repo.GitCredential = nil

	if req.DisplayName != nil {
		repo.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.RoleDescription != nil {
		repo.RoleDescription = *req.RoleDescription
	}
	if req.GitProvider != nil {
		gp, perr := parseGitProvider(*req.GitProvider)
		if perr != nil {
			return nil, perr
		}
		repo.GitProvider = gp
	}
	if req.GitURL != nil {
		u := strings.TrimSpace(*req.GitURL)
		if u == "" {
			return nil, ErrRepoURLRequired
		}
		repo.GitURL = u
	}
	if req.GitDefaultBranch != nil && strings.TrimSpace(*req.GitDefaultBranch) != "" {
		repo.GitDefaultBranch = strings.TrimSpace(*req.GitDefaultBranch)
	}
	if req.GitCredentialID != nil {
		repo.GitCredentialsID = req.GitCredentialID
	}
	switch {
	case req.RemoveGitIntegrationCredential:
		repo.GitIntegrationCredentialID = nil
	case req.GitIntegrationCredentialID != nil:
		repo.GitIntegrationCredentialID = req.GitIntegrationCredentialID
	}
	if req.SortOrder != nil {
		repo.SortOrder = *req.SortOrder
	}
	makePrimary := req.IsPrimary != nil && *req.IsPrimary

	err = s.transactions.WithTransaction(ctx, func(txCtx context.Context) error {
		if makePrimary {
			if cerr := s.projectRepoRepo.ClearPrimary(txCtx, projectID, repoID); cerr != nil {
				return cerr
			}
			repo.IsPrimary = true
		}
		return s.projectRepoRepo.Update(txCtx, repo)
	})
	if err != nil {
		return nil, mapProjectRepoRepoErr(err)
	}
	return repo, nil
}

func (s *projectService) RemoveRepository(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, repoID uuid.UUID) error {
	if err := s.HasAccess(ctx, userID, userRole, projectID); err != nil {
		return err
	}
	repo, err := s.projectRepoRepo.GetByID(ctx, repoID)
	if err != nil {
		return mapProjectRepoRepoErr(err)
	}
	if repo.ProjectID != projectID {
		return ErrRepoNotFound
	}
	// Нельзя удалить primary, пока есть другие репо — сначала назначь другой primary.
	if repo.IsPrimary {
		all, lerr := s.projectRepoRepo.ListByProject(ctx, projectID)
		if lerr != nil {
			return mapProjectRepoRepoErr(lerr)
		}
		if len(all) > 1 {
			return ErrCannotRemovePrimaryRepo
		}
	}
	if err := s.projectRepoRepo.Delete(ctx, repoID); err != nil {
		return mapProjectRepoRepoErr(err)
	}
	// Записи удалённого репозитория в индексе вычистит ближайшая переиндексация проекта
	// (PruneToPrefixes по оставшимся репо) — ручная (Reindex) или фоновая при изменениях.
	return nil
}
