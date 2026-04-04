package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrProjectNotFound   = errors.New("project not found")
	ErrProjectNameExists = errors.New("project with this name already exists")
)

const (
	projectListDefaultLimit = 20
	projectListMaxLimit     = 100
)

var allowedProjectOrderColumns = map[string]bool{
	"created_at": true,
	"updated_at": true,
	"name":       true,
	"status":     true,
}

// escapeILIKEWildcards экранирует \, % и _ для ILIKE с ESCAPE '\', чтобы ввод не работал как шаблон.
func escapeILIKEWildcards(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// normalizeProjectListLimit защищает от GORM Limit(0) («без лимита») и от слишком больших страниц.
// Дефолты дублируют контракт сервиса — сервис всё равно должен выставлять лимит; здесь — defense in depth.
func normalizeProjectListLimit(limit int) int {
	if limit <= 0 {
		return projectListDefaultLimit
	}
	if limit > projectListMaxLimit {
		return projectListMaxLimit
	}
	return limit
}

func sanitizeProjectOrder(orderBy, orderDir string) string {
	if !allowedProjectOrderColumns[orderBy] {
		orderBy = "created_at"
	}
	if orderDir != "ASC" && orderDir != "asc" {
		orderDir = "DESC"
	}
	return orderBy + " " + orderDir
}

// ProjectFilter фильтры и пагинация для списка проектов
type ProjectFilter struct {
	Status      *models.ProjectStatus
	GitProvider *models.GitProvider
	Search      *string
	Limit       int
	Offset      int
	OrderBy     string
	OrderDir    string
}

// ProjectRepository CRUD по проектам
type ProjectRepository interface {
	Create(ctx context.Context, project *models.Project) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
	List(ctx context.Context, filter ProjectFilter) ([]models.Project, int64, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, filter ProjectFilter) ([]models.Project, int64, error)
	Update(ctx context.Context, project *models.Project) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type projectRepository struct {
	db *gorm.DB
}

// NewProjectRepository создаёт репозиторий проектов
func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) scopedQuery(ctx context.Context, filter ProjectFilter, userID *uuid.UUID) *gorm.DB {
	q := r.db.WithContext(ctx).Model(&models.Project{})
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	}
	if filter.Status != nil {
		q = q.Where("status = ?", *filter.Status)
	}
	if filter.GitProvider != nil {
		q = q.Where("git_provider = ?", *filter.GitProvider)
	}
	if filter.Search != nil && *filter.Search != "" {
		pattern := "%" + escapeILIKEWildcards(*filter.Search) + "%"
		q = q.Where("(name ILIKE ? ESCAPE '\\' OR description ILIKE ? ESCAPE '\\')", pattern, pattern)
	}
	return q
}

// Create создаёт проект; дубликат имени у того же пользователя → ErrProjectNameExists.
// Внутри TransactionManager.WithTransaction использует ту же транзакцию, что и другие репозитории.
func (r *projectRepository) Create(ctx context.Context, project *models.Project) error {
	return r.createWithDB(ctx, gormDB(ctx, r.db), project)
}

func (r *projectRepository) createWithDB(ctx context.Context, db *gorm.DB, project *models.Project) error {
	if err := db.WithContext(ctx).Create(project).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrProjectNameExists
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrProjectNameExists
		}
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return ErrProjectNameExists
		}
		return fmt.Errorf("failed to create project: %w", err)
	}
	return nil
}

// GetByID возвращает проект с Preload GitCredential
func (r *projectRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	var project models.Project
	if err := r.db.WithContext(ctx).Preload("GitCredential").Where("id = ?", id).First(&project).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return &project, nil
}

// List все проекты (для admin) с пагинацией и фильтрами
func (r *projectRepository) List(ctx context.Context, filter ProjectFilter) ([]models.Project, int64, error) {
	return r.listWithUserScope(ctx, nil, filter)
}

// ListByUserID проекты пользователя
func (r *projectRepository) ListByUserID(ctx context.Context, userID uuid.UUID, filter ProjectFilter) ([]models.Project, int64, error) {
	return r.listWithUserScope(ctx, &userID, filter)
}

func (r *projectRepository) listWithUserScope(ctx context.Context, userID *uuid.UUID, filter ProjectFilter) ([]models.Project, int64, error) {
	base := r.scopedQuery(ctx, filter, userID)
	var count int64
	if err := base.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	var projects []models.Project
	q := r.scopedQuery(ctx, filter, userID)
	order := sanitizeProjectOrder(filter.OrderBy, filter.OrderDir)
	limit := normalizeProjectListLimit(filter.Limit)
	if err := q.Order(order).Limit(limit).Offset(filter.Offset).Find(&projects).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list projects: %w", err)
	}
	return projects, count, nil
}

// Update полная перезапись строки через Save (контракт: сервис передаёт полную модель)
func (r *projectRepository) Update(ctx context.Context, project *models.Project) error {
	if err := r.db.WithContext(ctx).Save(project).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrProjectNameExists
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrProjectNameExists
		}
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return ErrProjectNameExists
		}
		return fmt.Errorf("failed to update project: %w", err)
	}
	return nil
}

// Delete удаляет проект (жёстко; каскады в БД)
func (r *projectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.Project{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}
