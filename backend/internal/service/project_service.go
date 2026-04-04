package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"gorm.io/datatypes"
)

const devTeamDefaultName = "Development Team"

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
}

type projectService struct {
	projectRepo    repository.ProjectRepository
	teamRepo       repository.TeamRepository
	gitCredRepo    repository.GitCredentialRepository
	transactions   repository.TransactionManager
	encryptionKey  []byte
}

// NewProjectService создаёт сервис проектов. encryptionKey зарезервирован для будущего шифрования на уровне сервиса.
func NewProjectService(
	projectRepo repository.ProjectRepository,
	teamRepo repository.TeamRepository,
	gitCredRepo repository.GitCredentialRepository,
	transactions repository.TransactionManager,
	encryptionKey []byte,
) ProjectService {
	return &projectService{
		projectRepo:   projectRepo,
		teamRepo:      teamRepo,
		gitCredRepo:   gitCredRepo,
		transactions:  transactions,
		encryptionKey: encryptionKey,
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

func (s *projectService) ensureGitCredentialOwned(ctx context.Context, userID uuid.UUID, credID uuid.UUID) error {
	cred, err := s.gitCredRepo.GetByID(ctx, credID)
	if err != nil {
		return mapGitCredRepoErr(err)
	}
	if cred.UserID != userID {
		return ErrGitCredentialForbidden
	}
	return nil
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
	if req.GitCredentialID != nil {
		if err := s.ensureGitCredentialOwned(ctx, userID, *req.GitCredentialID); err != nil {
			return nil, err
		}
	}

	branch := strings.TrimSpace(req.GitDefaultBranch)
	if branch == "" {
		branch = "main"
	}

	project := &models.Project{
		Name:             name,
		Description:      req.Description,
		GitProvider:      gp,
		GitURL:           req.GitURL,
		GitDefaultBranch: branch,
		GitCredentialsID: req.GitCredentialID,
		VectorCollection: req.VectorCollection,
		TechStack:        req.TechStack,
		Status:           status,
		Settings:         req.Settings,
		UserID:           userID,
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
	return project, nil
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
		project.GitURL = *req.GitURL
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
		if err := s.ensureGitCredentialOwned(ctx, userID, *req.GitCredentialID); err != nil {
			return nil, err
		}
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

	if err := s.projectRepo.Update(ctx, project); err != nil {
		return nil, mapProjectRepoErr(err)
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
	return s.projectRepo.Delete(ctx, projectID)
}
