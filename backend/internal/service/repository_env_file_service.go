package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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
	// ErrRepoEnvFileDuplicate — в репозитории уже есть файл с таким путём (target_dir + file_name).
	ErrRepoEnvFileDuplicate = errors.New("repository env file with this path already exists")
)

// RepositoryEnvFileService управляет «инъекцией env-файлов» уровня репозитория. На один
// репозиторий допускается НЕСКОЛЬКО файлов (уникальность по target_dir+file_name).
// Содержимое шифруется AES-256-GCM и write-only — наружу не возвращается.
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

// RepoEnvFileView — метаданные env-файла для API (без содержимого, write-only).
type RepoEnvFileView struct {
	ID                  uuid.UUID `json:"id"`
	ProjectRepositoryID uuid.UUID `json:"project_repository_id"`
	FileName            string    `json:"file_name"`
	TargetDir           string    `json:"target_dir"`
	CreatedAt           string    `json:"created_at"`
	UpdatedAt           string    `json:"updated_at"`
}

type CreateRepoEnvFileInput struct {
	ProjectID uuid.UUID
	RepoID    uuid.UUID
	FileName  string
	TargetDir string
	Content   string
}

type UpdateRepoEnvFileInput struct {
	ProjectID uuid.UUID
	RepoID    uuid.UUID
	FileID    uuid.UUID
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

// List возвращает метаданные всех env-файлов репозитория (без содержимого).
func (s *RepositoryEnvFileService) List(ctx context.Context, projectID, repoID uuid.UUID) ([]RepoEnvFileView, error) {
	if err := s.assertRepoInProject(ctx, projectID, repoID); err != nil {
		return nil, err
	}
	files, err := s.repo.ListByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	out := make([]RepoEnvFileView, 0, len(files))
	for i := range files {
		out = append(out, *s.toView(&files[i]))
	}
	return out, nil
}

// Create добавляет новый env-файл в репозиторий.
func (s *RepositoryEnvFileService) Create(ctx context.Context, in CreateRepoEnvFileInput) (*RepoEnvFileView, error) {
	if in.ProjectID == uuid.Nil || in.RepoID == uuid.Nil {
		return nil, fmt.Errorf("%w: project_id and repo_id are required", ErrRepoEnvFileValidation)
	}
	if err := s.assertRepoInProject(ctx, in.ProjectID, in.RepoID); err != nil {
		return nil, err
	}
	if err := s.validatePayload(in.FileName, in.TargetDir, in.Content); err != nil {
		return nil, err
	}
	if err := s.assertUniquePath(ctx, in.RepoID, in.FileName, in.TargetDir, uuid.Nil); err != nil {
		return nil, err
	}

	id := uuid.New()
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
	s.logCreate(in)
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("persist repository env file: %w", err)
	}
	return s.toView(f), nil
}

// Update заменяет содержимое/имя/папку существующего env-файла (полная перезапись).
func (s *RepositoryEnvFileService) Update(ctx context.Context, in UpdateRepoEnvFileInput) (*RepoEnvFileView, error) {
	if in.ProjectID == uuid.Nil || in.RepoID == uuid.Nil || in.FileID == uuid.Nil {
		return nil, fmt.Errorf("%w: project_id, repo_id and file_id are required", ErrRepoEnvFileValidation)
	}
	if err := s.assertRepoInProject(ctx, in.ProjectID, in.RepoID); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByID(ctx, in.FileID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
			return nil, ErrRepoEnvFileNotFound
		}
		return nil, err
	}
	if existing.ProjectRepositoryID != in.RepoID {
		return nil, ErrRepoEnvFileNotFound
	}
	if err := s.validatePayload(in.FileName, in.TargetDir, in.Content); err != nil {
		return nil, err
	}
	if err := s.assertUniquePath(ctx, in.RepoID, in.FileName, in.TargetDir, in.FileID); err != nil {
		return nil, err
	}

	blob, encErr := s.secrets.Encrypt(existing.ID, in.Content)
	if encErr != nil {
		return nil, encErr
	}
	existing.FileName = in.FileName
	existing.TargetDir = in.TargetDir
	existing.EncryptedContent = blob
	if err := s.repo.Update(ctx, existing); err != nil {
		if errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
			return nil, ErrRepoEnvFileNotFound
		}
		return nil, fmt.Errorf("update repository env file: %w", err)
	}
	return s.toView(existing), nil
}

// Delete удаляет env-файл репозитория по его id.
func (s *RepositoryEnvFileService) Delete(ctx context.Context, projectID, repoID, fileID uuid.UUID) error {
	if err := s.assertRepoInProject(ctx, projectID, repoID); err != nil {
		return err
	}
	existing, err := s.repo.GetByID(ctx, fileID)
	if err != nil {
		if errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
			return ErrRepoEnvFileNotFound
		}
		return err
	}
	if existing.ProjectRepositoryID != repoID {
		return ErrRepoEnvFileNotFound
	}
	if err := s.repo.Delete(ctx, fileID); err != nil {
		if errors.Is(err, repository.ErrRepositoryEnvFileNotFound) {
			return ErrRepoEnvFileNotFound
		}
		return fmt.Errorf("delete repository env file: %w", err)
	}
	return nil
}

// GetInjectedFilesForRepo возвращает дешифрованные спеки ВСЕХ env-файлов репо для
// инъекции в sandbox (ContextBuilder). Пустой срез — файлов нет. Внутренний путь.
func (s *RepositoryEnvFileService) GetInjectedFilesForRepo(ctx context.Context, repoID uuid.UUID) ([]sandbox.InjectedEnvFileSpec, error) {
	files, err := s.repo.ListByRepoWithContent(ctx, repoID)
	if err != nil {
		return nil, err
	}
	out := make([]sandbox.InjectedEnvFileSpec, 0, len(files))
	for i := range files {
		f := &files[i]
		plain, decErr := s.secrets.Decrypt(f.ID, f.EncryptedContent)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt repository env file %s: %w", f.ID, decErr)
		}
		out = append(out, sandbox.InjectedEnvFileSpec{
			FileName:  f.FileName,
			TargetDir: f.TargetDir,
			Content:   plain,
		})
	}
	return out, nil
}

func (s *RepositoryEnvFileService) validatePayload(fileName, targetDir, content string) error {
	if content == "" {
		return fmt.Errorf("%w: content must be non-empty", ErrRepoEnvFileValidation)
	}
	if err := sandbox.ValidateInjectedEnvFile(fileName, targetDir); err != nil {
		return fmt.Errorf("%w: %v", ErrRepoEnvFileValidation, err)
	}
	return nil
}

// assertUniquePath не допускает двух файлов с одинаковым путём (target_dir+file_name) в
// одном репо. excludeID — id обновляемой записи (Nil при создании). DB-constraint —
// бэкстоп, но дружелюбную ошибку отдаём на уровне сервиса.
func (s *RepositoryEnvFileService) assertUniquePath(ctx context.Context, repoID uuid.UUID, fileName, targetDir string, excludeID uuid.UUID) error {
	files, err := s.repo.ListByRepo(ctx, repoID)
	if err != nil {
		return err
	}
	for i := range files {
		if files[i].ID == excludeID {
			continue
		}
		if strings.EqualFold(files[i].FileName, fileName) && files[i].TargetDir == targetDir {
			return fmt.Errorf("%w: %s/%s", ErrRepoEnvFileDuplicate, targetDir, fileName)
		}
	}
	return nil
}

func (s *RepositoryEnvFileService) logCreate(in CreateRepoEnvFileInput) {
	s.logger.Info("repository env file created",
		slog.String("repo_id", in.RepoID.String()),
		slog.String("project_id", in.ProjectID.String()),
		slog.String("file_name", in.FileName),
		slog.String("target_dir", in.TargetDir),
		slog.Int("content_len", len(in.Content)),
	)
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
