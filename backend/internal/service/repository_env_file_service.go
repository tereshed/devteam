package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/sandbox"
	"github.com/google/uuid"
)

var (
	ErrRepoEnvFileNotFound   = errors.New("repository env file not found")
	ErrRepoEnvFileValidation = errors.New("repository env file validation failed")
	// ErrRepoEnvFileRepoMismatch — repoID не принадлежит указанному проекту (или не существует).
	ErrRepoEnvFileRepoMismatch = errors.New("repository does not belong to project")
)

// RepositoryEnvFileService управляет «инъекцией env-файла» уровня репозитория:
// шифрует содержимое (AES-256-GCM, как project_secrets), валидирует имя/папку и
// проверяет принадлежность репозитория проекту. Контент возвращается дешифрованным
// только авторизованному владельцу проекта (для редактирования в UI).
type RepositoryEnvFileService struct {
	repo     repository.RepositoryEnvFileRepository
	repoRepo repository.ProjectRepoRepository
	secrets  *SecretService
	logger   *slog.Logger
}

func NewRepositoryEnvFileService(
	repo repository.RepositoryEnvFileRepository,
	repoRepo repository.ProjectRepoRepository,
	secrets *SecretService,
	logger *slog.Logger,
) *RepositoryEnvFileService {
	return &RepositoryEnvFileService{
		repo:     repo,
		repoRepo: repoRepo,
		secrets:  secrets,
		logger:   logger,
	}
}

// RepoEnvFileView — представление env-файла для API. Содержимое НЕ возвращается
// (write-only, как у project_secrets): редактирование = полная перезапись файла.
type RepoEnvFileView struct {
	ID                  uuid.UUID `json:"id"`
	ProjectRepositoryID uuid.UUID `json:"project_repository_id"`
	FileName            string    `json:"file_name"`
	TargetDir           string    `json:"target_dir"`
	CreatedAt           string    `json:"created_at"`
	UpdatedAt           string    `json:"updated_at"`
}

type SetRepoEnvFileInput struct {
	ProjectID uuid.UUID
	RepoID    uuid.UUID
	FileName  string
	TargetDir string
	Content   string
}

const timeFmtRFC3339 = "2006-01-02T15:04:05Z07:00"

// assertRepoInProject fail-loud'ит, если репозиторий не принадлежит проекту.
func (s *RepositoryEnvFileService) assertRepoInProject(ctx context.Context, projectID, repoID uuid.UUID) error {
	repo, err := s.repoRepo.GetByID(ctx, repoID)
	if err != nil {
		return ErrRepoEnvFileRepoMismatch
	}
	if repo.ProjectID != projectID {
		return ErrRepoEnvFileRepoMismatch
	}
	return nil
}

// Get возвращает метаданные env-файла репозитория (имя/папка/тайминги), БЕЗ содержимого
// (write-only). nil, nil — файл не настроен.
func (s *RepositoryEnvFileService) Get(ctx context.Context, projectID, repoID uuid.UUID) (*RepoEnvFileView, error) {
	if err := s.assertRepoInProject(ctx, projectID, repoID); err != nil {
		return nil, err
	}
	f, err := s.repo.GetByRepo(ctx, repoID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return s.toView(f), nil
}

// Set создаёт или обновляет env-файл репозитория (один на репо).
func (s *RepositoryEnvFileService) Set(ctx context.Context, in SetRepoEnvFileInput) (*RepoEnvFileView, error) {
	if in.ProjectID == uuid.Nil || in.RepoID == uuid.Nil {
		return nil, fmt.Errorf("%w: project_id and repo_id are required", ErrRepoEnvFileValidation)
	}
	if err := s.assertRepoInProject(ctx, in.ProjectID, in.RepoID); err != nil {
		return nil, err
	}
	if in.Content == "" {
		return nil, fmt.Errorf("%w: content must be non-empty", ErrRepoEnvFileValidation)
	}
	if err := sandbox.ValidateInjectedEnvFile(in.FileName, in.TargetDir); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRepoEnvFileValidation, err)
	}

	s.logger.Info("repository env file set",
		slog.String("repo_id", in.RepoID.String()),
		slog.String("project_id", in.ProjectID.String()),
		slog.String("file_name", in.FileName),
		slog.String("target_dir", in.TargetDir),
		slog.Int("content_len", len(in.Content)),
	)

	// Стабильный ID: при обновлении переиспользуем существующий (encrypt привязан к ID).
	existing, err := s.repo.GetByRepo(ctx, in.RepoID)
	if err != nil && !errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
		return nil, fmt.Errorf("check existing repository env file: %w", err)
	}
	id := uuid.New()
	if existing != nil {
		id = existing.ID
	}
	blob, encErr := s.secrets.Encrypt(id, in.Content)
	if encErr != nil {
		return nil, encErr
	}
	f := &models.RepositoryEnvFile{
		ID:                  id,
		ProjectRepositoryID: in.RepoID,
		FileName:            in.FileName,
		TargetDir:           in.TargetDir,
		EncryptedContent:    blob,
	}
	if upErr := s.repo.Upsert(ctx, f); upErr != nil {
		return nil, fmt.Errorf("persist repository env file: %w", upErr)
	}
	return s.toView(f), nil
}

// Delete удаляет env-файл репозитория.
func (s *RepositoryEnvFileService) Delete(ctx context.Context, projectID, repoID uuid.UUID) error {
	if err := s.assertRepoInProject(ctx, projectID, repoID); err != nil {
		return err
	}
	if err := s.repo.DeleteByRepo(ctx, repoID); err != nil {
		if errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
			return ErrRepoEnvFileNotFound
		}
		return fmt.Errorf("delete repository env file: %w", err)
	}
	return nil
}

// GetInjectedFileForRepo возвращает дешифрованную спеку для инъекции в sandbox
// (ContextBuilder). nil, nil — файл не настроен. Без проверки проекта — внутренний путь.
func (s *RepositoryEnvFileService) GetInjectedFileForRepo(ctx context.Context, repoID uuid.UUID) (*sandbox.InjectedEnvFileSpec, error) {
	f, err := s.repo.GetByRepo(ctx, repoID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
			return nil, nil
		}
		return nil, err
	}
	plain, decErr := s.secrets.Decrypt(f.ID, f.EncryptedContent)
	if decErr != nil {
		return nil, fmt.Errorf("decrypt repository env file %s: %w", f.ID, decErr)
	}
	return &sandbox.InjectedEnvFileSpec{
		FileName:  f.FileName,
		TargetDir: f.TargetDir,
		Content:   plain,
	}, nil
}

func (s *RepositoryEnvFileService) toView(f *models.RepositoryEnvFile) *RepoEnvFileView {
	return &RepoEnvFileView{
		ID:                  f.ID,
		ProjectRepositoryID: f.ProjectRepositoryID,
		FileName:            f.FileName,
		TargetDir:           f.TargetDir,
		CreatedAt:           f.CreatedAt.Format(timeFmtRFC3339),
		UpdatedAt:           f.UpdatedAt.Format(timeFmtRFC3339),
	}
}
