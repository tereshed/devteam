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
	ErrTeamNotFound      = errors.New("team not found")
	ErrTeamAlreadyExists = errors.New("team for this project already exists")
)

// TeamRepository CRUD по командам проекта (1 проект = 1 команда)
type TeamRepository interface {
	Create(ctx context.Context, team *models.Team) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error)
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error)
	Update(ctx context.Context, team *models.Team) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type teamRepository struct {
	db *gorm.DB
}

// NewTeamRepository создаёт репозиторий команд
func NewTeamRepository(db *gorm.DB) TeamRepository {
	return &teamRepository{db: db}
}

func (r *teamRepository) createWithDB(ctx context.Context, db *gorm.DB, team *models.Team) error {
	if err := db.WithContext(ctx).Create(team).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrTeamAlreadyExists
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrTeamAlreadyExists
		}
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "23505") {
			return ErrTeamAlreadyExists
		}
		return fmt.Errorf("failed to create team: %w", err)
	}
	return nil
}

// Create создаёт команду; вторая команда для того же project_id → ErrTeamAlreadyExists.
// Внутри TransactionManager.WithTransaction использует ту же транзакцию, что и другие репозитории.
func (r *teamRepository) Create(ctx context.Context, team *models.Team) error {
	return r.createWithDB(ctx, gormDB(ctx, r.db), team)
}

// GetByID возвращает команду с агентами, отсортированными по role ASC
func (r *teamRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Team, error) {
	var team models.Team
	err := r.db.WithContext(ctx).
		Preload("Agents", func(db *gorm.DB) *gorm.DB {
			return db.Order("role ASC")
		}).
		Where("id = ?", id).
		First(&team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team by id: %w", err)
	}
	return &team, nil
}

// GetByProjectID возвращает команду проекта с агентами и вложенным Prompt
func (r *teamRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	var team models.Team
	err := r.db.WithContext(ctx).
		Preload("Agents", func(db *gorm.DB) *gorm.DB {
			return db.Order("role ASC")
		}).
		Preload("Agents.Prompt").
		Where("project_id = ?", projectID).
		First(&team).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, fmt.Errorf("failed to get team by project id: %w", err)
	}
	return &team, nil
}

// Update перезаписывает строку через Save (все поля модели).
// Контракт слоя сервиса: передаётся полностью загруженная сущность (Get* → правки → Update);
// неполная модель может обнулить колонки в БД. При смене контракта предпочтительнее Updates по полям/map.
func (r *teamRepository) Update(ctx context.Context, team *models.Team) error {
	if err := r.db.WithContext(ctx).Save(team).Error; err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}
	return nil
}

// Delete удаляет команду по id (агенты получают team_id = NULL по ON DELETE SET NULL)
func (r *teamRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Delete(&models.Team{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}
