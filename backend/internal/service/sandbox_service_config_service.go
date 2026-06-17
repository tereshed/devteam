package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/google/uuid"
)

// Дефолты декларации сервис-сайдкара (применяются при пустых полях upsert).
const (
	sandboxServiceDefaultImage        = "postgres:16-alpine"
	sandboxServiceDefaultDBName       = "app"
	sandboxServiceDefaultDBUser       = "postgres"
	sandboxServiceDefaultPort         = 5432
	sandboxServiceDefaultReadyTimeout = 60
	sandboxServiceMaxSeedBytes        = 256 * 1024
)

var (
	// ErrSandboxServiceInvalidAlias — alias не DNS-safe.
	ErrSandboxServiceInvalidAlias = errors.New("alias must match ^[a-z][a-z0-9-]{0,62}$")
	// ErrSandboxServiceInvalidKind — неизвестный kind.
	ErrSandboxServiceInvalidKind = errors.New("invalid sandbox service kind")
	// ErrSandboxServiceInvalidSeedKind — неизвестный seed_kind.
	ErrSandboxServiceInvalidSeedKind = errors.New("invalid sandbox service seed_kind")
	// ErrSandboxServiceInvalidImage — образ вне allowlist.
	ErrSandboxServiceInvalidImage = errors.New("sandbox service image is not allowed")
	// ErrSandboxServiceInvalidPort — порт вне диапазона.
	ErrSandboxServiceInvalidPort = errors.New("port must be between 1 and 65535")
	// ErrSandboxServiceInvalidTimeout — ready_timeout вне диапазона.
	ErrSandboxServiceInvalidTimeout = errors.New("ready_timeout_seconds must be between 10 and 600")
	// ErrSandboxServiceInvalidField — пустое обязательное поле (db_name/db_user).
	ErrSandboxServiceInvalidField = errors.New("db_name and db_user are required")
	// ErrSandboxServiceInvalidSeedValue — сид слишком большой / противоречит seed_kind.
	ErrSandboxServiceInvalidSeedValue = errors.New("invalid seed_value for seed_kind")
	// ErrSandboxServiceNotFound — декларация не найдена.
	ErrSandboxServiceNotFound = errors.New("sandbox service config not found")
)

var sandboxServiceAliasRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
var sandboxServiceDBTokenRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// SandboxServiceConfigService — CRUD деклараций сервис-сайдкаров проекта (Sprint 22).
type SandboxServiceConfigService interface {
	List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.SandboxServiceConfig, error)
	Upsert(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpsertSandboxServiceRequest) (*models.SandboxServiceConfig, error)
	Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID) error
}

// SandboxServiceConfigDeps — DI-bag конструктора.
type SandboxServiceConfigDeps struct {
	Repo       repository.SandboxServiceRepository
	ProjectSvc ProjectService
	// AllowedImages — allowlist образов для валидации на этапе конфигурации.
	// Пусто → sandbox.DefaultAllowedSandboxServiceImages().
	AllowedImages []string
}

type sandboxServiceConfigService struct {
	deps          SandboxServiceConfigDeps
	allowedImages []string
}

// NewSandboxServiceConfigService создаёт сервис конфигов сервис-сайдкаров.
func NewSandboxServiceConfigService(deps SandboxServiceConfigDeps) (SandboxServiceConfigService, error) {
	if deps.Repo == nil {
		return nil, errors.New("SandboxServiceConfigService: Repo is required")
	}
	if deps.ProjectSvc == nil {
		return nil, errors.New("SandboxServiceConfigService: ProjectSvc is required")
	}
	allowed := deps.AllowedImages
	if len(allowed) == 0 {
		allowed = sandbox.DefaultAllowedSandboxServiceImages()
	}
	return &sandboxServiceConfigService{deps: deps, allowedImages: allowed}, nil
}

func (s *sandboxServiceConfigService) List(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID) ([]models.SandboxServiceConfig, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}
	return s.deps.Repo.ListByProject(ctx, projectID)
}

func (s *sandboxServiceConfigService) Upsert(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID uuid.UUID, req dto.UpsertSandboxServiceRequest) (*models.SandboxServiceConfig, error) {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return nil, err
	}

	cfg, err := s.deps.Repo.GetByProjectAndAlias(ctx, projectID, strings.TrimSpace(req.Alias))
	created := false
	if err != nil {
		if !errors.Is(err, repository.ErrSandboxServiceConfigNotFound) {
			return nil, err
		}
		cfg = &models.SandboxServiceConfig{
			ID:        uuid.New(),
			ProjectID: projectID,
			CreatedBy: userID,
		}
		created = true
	}

	if err := applySandboxServiceRequest(cfg, req); err != nil {
		return nil, err
	}
	if err := s.validate(cfg); err != nil {
		return nil, err
	}

	if created {
		if err := s.deps.Repo.Create(ctx, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err := s.deps.Repo.Update(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *sandboxServiceConfigService) Delete(ctx context.Context, userID uuid.UUID, userRole models.UserRole, projectID, id uuid.UUID) error {
	if _, err := s.deps.ProjectSvc.GetByID(ctx, userID, userRole, projectID); err != nil {
		return err
	}
	cfg, err := s.deps.Repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrSandboxServiceConfigNotFound) {
			return ErrSandboxServiceNotFound
		}
		return err
	}
	// Защита от cross-project удаления по UUID.
	if cfg.ProjectID != projectID {
		return ErrSandboxServiceNotFound
	}
	if err := s.deps.Repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrSandboxServiceConfigNotFound) {
			return ErrSandboxServiceNotFound
		}
		return err
	}
	return nil
}

// applySandboxServiceRequest накладывает запрос на модель с дефолтами для пустых полей.
func applySandboxServiceRequest(cfg *models.SandboxServiceConfig, req dto.UpsertSandboxServiceRequest) error {
	cfg.IsEnabled = req.IsEnabled
	cfg.Alias = strings.TrimSpace(req.Alias)

	cfg.Kind = models.SandboxServiceKind(strings.TrimSpace(req.Kind))
	if cfg.Kind == "" {
		cfg.Kind = models.SandboxServiceKindPostgres
	}
	cfg.Image = strings.TrimSpace(req.Image)
	if cfg.Image == "" {
		cfg.Image = sandboxServiceDefaultImage
	}
	cfg.DBName = strings.TrimSpace(req.DBName)
	if cfg.DBName == "" {
		cfg.DBName = sandboxServiceDefaultDBName
	}
	cfg.DBUser = strings.TrimSpace(req.DBUser)
	if cfg.DBUser == "" {
		cfg.DBUser = sandboxServiceDefaultDBUser
	}
	cfg.Port = req.Port
	if cfg.Port == 0 {
		cfg.Port = sandboxServiceDefaultPort
	}
	cfg.SeedKind = models.SandboxServiceSeedKind(strings.TrimSpace(req.SeedKind))
	if cfg.SeedKind == "" {
		cfg.SeedKind = models.SandboxSeedNone
	}
	cfg.SeedValue = req.SeedValue
	cfg.ReadyTimeoutSeconds = req.ReadyTimeoutSeconds
	if cfg.ReadyTimeoutSeconds == 0 {
		cfg.ReadyTimeoutSeconds = sandboxServiceDefaultReadyTimeout
	}
	return nil
}

func (s *sandboxServiceConfigService) validate(cfg *models.SandboxServiceConfig) error {
	if !sandboxServiceAliasRE.MatchString(cfg.Alias) {
		return ErrSandboxServiceInvalidAlias
	}
	if !cfg.Kind.IsValid() {
		return ErrSandboxServiceInvalidKind
	}
	if !cfg.SeedKind.IsValid() {
		return ErrSandboxServiceInvalidSeedKind
	}
	if err := sandbox.ValidateAllowedImage(cfg.Image, s.allowedImages); err != nil {
		return fmt.Errorf("%w: %v", ErrSandboxServiceInvalidImage, err)
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return ErrSandboxServiceInvalidPort
	}
	if cfg.ReadyTimeoutSeconds < 10 || cfg.ReadyTimeoutSeconds > 600 {
		return ErrSandboxServiceInvalidTimeout
	}
	if !sandboxServiceDBTokenRE.MatchString(cfg.DBName) || !sandboxServiceDBTokenRE.MatchString(cfg.DBUser) {
		return ErrSandboxServiceInvalidField
	}
	// Сид: inline ограничен размером; none не должен нести значение; repo_file — путь без traversal.
	switch cfg.SeedKind {
	case models.SandboxSeedNone:
		cfg.SeedValue = ""
	case models.SandboxSeedInline:
		if len(cfg.SeedValue) > sandboxServiceMaxSeedBytes {
			return fmt.Errorf("%w: inline seed exceeds %d bytes", ErrSandboxServiceInvalidSeedValue, sandboxServiceMaxSeedBytes)
		}
	case models.SandboxSeedRepoFile:
		p := strings.TrimSpace(cfg.SeedValue)
		if p == "" || strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
			return fmt.Errorf("%w: repo_file path must be relative and without '..'", ErrSandboxServiceInvalidSeedValue)
		}
		cfg.SeedValue = p
	}
	return nil
}
