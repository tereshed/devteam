package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectRepoRepository CRUD по репозиториям проекта (мульти-репо).
type ProjectRepoRepository interface {
	Create(ctx context.Context, repo *models.ProjectRepository) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.ProjectRepository, error)
	GetBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.ProjectRepository, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ProjectRepository, error)
	Update(ctx context.Context, repo *models.ProjectRepository) error
	UpdateStatusAndCommit(ctx context.Context, id uuid.UUID, oldStatus, newStatus models.ProjectStatus, commitSHA string) error
	// UpdateIndexStatus безусловно (без CAS) обновляет статус репо и last_indexed_commit.
	// Используется индексатором: статус репо в пайплайне может стартовать из разных значений.
	UpdateIndexStatus(ctx context.Context, id uuid.UUID, status models.ProjectStatus, commitSHA string) error
	// ClearPrimary снимает флаг is_primary со всех репо проекта, кроме exceptID
	// (передай uuid.Nil чтобы снять со всех). Нужно перед назначением нового primary,
	// т.к. частичный уникальный индекс uq_project_primary_repo допускает один primary на проект.
	ClearPrimary(ctx context.Context, projectID, exceptID uuid.UUID) error
	// ReleaseStuckIndexing сбрасывает осиротевшие status='indexing' старше cutoff в
	// 'indexing_failed' (по updated_at: UpdateIndexStatus освежает его при переходе в indexing).
	ReleaseStuckIndexing(ctx context.Context, cutoff time.Time) (int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type projectRepoRepository struct {
	db *gorm.DB
}

// NewProjectRepoRepository создаёт репозиторий репозиториев проекта.
func NewProjectRepoRepository(db *gorm.DB) ProjectRepoRepository {
	return &projectRepoRepository{db: db}
}

func (r *projectRepoRepository) Create(ctx context.Context, repo *models.ProjectRepository) error {
	if err := gormDB(ctx, r.db).WithContext(ctx).Create(repo).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrProjectRepoSlugExists
		}
		return fmt.Errorf("failed to create project repository: %w", err)
	}
	return nil
}

func (r *projectRepoRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ProjectRepository, error) {
	var repo models.ProjectRepository
	if err := r.db.WithContext(ctx).Preload("GitCredential").Where("id = ?", id).First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectRepoNotFound
		}
		return nil, fmt.Errorf("failed to get project repository: %w", err)
	}
	return &repo, nil
}

func (r *projectRepoRepository) GetBySlug(ctx context.Context, projectID uuid.UUID, slug string) (*models.ProjectRepository, error) {
	var repo models.ProjectRepository
	if err := r.db.WithContext(ctx).Preload("GitCredential").
		Where("project_id = ? AND slug = ?", projectID, slug).First(&repo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectRepoNotFound
		}
		return nil, fmt.Errorf("failed to get project repository by slug: %w", err)
	}
	return &repo, nil
}

func (r *projectRepoRepository) ListByProject(ctx context.Context, projectID uuid.UUID) ([]models.ProjectRepository, error) {
	var repos []models.ProjectRepository
	if err := r.db.WithContext(ctx).Preload("GitCredential").
		Where("project_id = ?", projectID).
		Order("sort_order ASC, created_at ASC").
		Find(&repos).Error; err != nil {
		return nil, fmt.Errorf("failed to list project repositories: %w", err)
	}
	return repos, nil
}

func (r *projectRepoRepository) Update(ctx context.Context, repo *models.ProjectRepository) error {
	if err := gormDB(ctx, r.db).WithContext(ctx).Save(repo).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrProjectRepoSlugExists
		}
		return fmt.Errorf("failed to update project repository: %w", err)
	}
	return nil
}

// UpdateStatusAndCommit безопасно обновляет статус репозитория и хэш последнего индексированного коммита (CAS по статусу).
func (r *projectRepoRepository) UpdateStatusAndCommit(ctx context.Context, id uuid.UUID, oldStatus, newStatus models.ProjectStatus, commitSHA string) error {
	updates := map[string]interface{}{"status": newStatus}
	if commitSHA != "" {
		updates["last_indexed_commit"] = commitSHA
	}
	result := gormDB(ctx, r.db).WithContext(ctx).Model(&models.ProjectRepository{}).
		Where("id = ? AND status = ?", id, oldStatus).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update project repository status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("project repository status update failed: not found or status changed (expected %s)", oldStatus)
	}
	return nil
}

func (r *projectRepoRepository) UpdateIndexStatus(ctx context.Context, id uuid.UUID, status models.ProjectStatus, commitSHA string) error {
	updates := map[string]interface{}{"status": status}
	if commitSHA != "" {
		updates["last_indexed_commit"] = commitSHA
	}
	if err := gormDB(ctx, r.db).WithContext(ctx).Model(&models.ProjectRepository{}).
		Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update project repository index status: %w", err)
	}
	return nil
}

func (r *projectRepoRepository) ClearPrimary(ctx context.Context, projectID, exceptID uuid.UUID) error {
	q := gormDB(ctx, r.db).WithContext(ctx).Model(&models.ProjectRepository{}).
		Where("project_id = ? AND is_primary = ?", projectID, true)
	if exceptID != uuid.Nil {
		q = q.Where("id <> ?", exceptID)
	}
	if err := q.Update("is_primary", false).Error; err != nil {
		return fmt.Errorf("failed to clear primary repository: %w", err)
	}
	return nil
}

// ReleaseStuckIndexing сбрасывает осиротевшие status='indexing' старше cutoff в
// 'indexing_failed'. Маркер давности — updated_at: репо-строку в indexing переводит
// только UpdateIndexStatus (что освежает updated_at), а ручное редактирование
// настроек репо лишь отодвигает recovery, не давая ложных срабатываний.
func (r *projectRepoRepository) ReleaseStuckIndexing(ctx context.Context, cutoff time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.ProjectRepository{}).
		Where("status = ? AND updated_at < ?", models.ProjectStatusIndexing, cutoff).
		Update("status", models.ProjectStatusIndexingFailed)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to release stuck indexing repositories: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *projectRepoRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.ProjectRepository{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete project repository: %w", err)
	}
	return nil
}
