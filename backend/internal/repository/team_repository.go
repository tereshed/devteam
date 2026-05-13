package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/datatypes"
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
	GetAgentInProject(ctx context.Context, projectID, agentID uuid.UUID) (*models.Agent, error)
	GetAgentByID(ctx context.Context, agentID uuid.UUID) (*models.Agent, error)
	// GetAgentOwnerUserID возвращает user_id владельца проекта, к команде которого принадлежит агент.
	// Sprint 15.B (B4): нужен для ownership-check в /agents/:id/settings и MCP-инструментах.
	// Возвращает ErrTeamAgentNotFound, если агент не найден или у него нет team_id/project_id.
	GetAgentOwnerUserID(ctx context.Context, agentID uuid.UUID) (uuid.UUID, error)
	SaveAgent(ctx context.Context, agent *models.Agent) error
	// SaveAgentWithToolBindings атомарно сохраняет агента и при replaceBindings полностью заменяет agent_tool_bindings.
	SaveAgentWithToolBindings(ctx context.Context, agent *models.Agent, replaceBindings bool, bindingToolDefIDs []uuid.UUID) error
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
		Preload("Agents.ToolBindings.ToolDefinition").
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
		Preload("Agents.ToolBindings.ToolDefinition").
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

// GetAgentByID возвращает агента по ID без привязки к проекту (Sprint 15.23 — settings handler).
func (r *teamRepository) GetAgentByID(ctx context.Context, agentID uuid.UUID) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).Where("id = ?", agentID).First(&agent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent by id: %w", err)
	}
	return &agent, nil
}

// GetAgentOwnerUserID джойнит agents → teams → projects и возвращает projects.user_id.
// Используется handler'ом /agents/:id/settings и MCP-инструментами для ownership-check.
func (r *teamRepository) GetAgentOwnerUserID(ctx context.Context, agentID uuid.UUID) (uuid.UUID, error) {
	var row struct{ UserID uuid.UUID }
	err := r.db.WithContext(ctx).
		Table("agents").
		Select("projects.user_id AS user_id").
		Joins("INNER JOIN teams ON teams.id = agents.team_id").
		Joins("INNER JOIN projects ON projects.id = teams.project_id").
		Where("agents.id = ?", agentID).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, ErrTeamAgentNotFound
		}
		return uuid.Nil, fmt.Errorf("failed to resolve agent owner: %w", err)
	}
	return row.UserID, nil
}

// GetAgentInProject возвращает агента, если он принадлежит команде указанного проекта.
func (r *teamRepository) GetAgentInProject(ctx context.Context, projectID, agentID uuid.UUID) (*models.Agent, error) {
	var agent models.Agent
	err := r.db.WithContext(ctx).
		Joins("INNER JOIN teams ON teams.id = agents.team_id").
		Where("teams.project_id = ? AND agents.id = ?", projectID, agentID).
		First(&agent).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTeamAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent in project: %w", err)
	}
	return &agent, nil
}

// SaveAgent сохраняет поля агента без каскада на связи.
func (r *teamRepository) SaveAgent(ctx context.Context, agent *models.Agent) error {
	if err := r.db.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: false}).Save(agent).Error; err != nil {
		return fmt.Errorf("failed to save agent: %w", err)
	}
	return nil
}

// SaveAgentWithToolBindings сохраняет агента; при replaceBindings удаляет все bindings и вставляет новые (config='{}').
// replaceBindings: сейчас сервис всегда передаёт true при PATCH tool_bindings (13.3.1). Значение false зарезервировано под будущие сценарии без полной замены (TODO: см. редактируемый config в задаче 13.3.1 A.4).
func (r *teamRepository) SaveAgentWithToolBindings(ctx context.Context, agent *models.Agent, replaceBindings bool, bindingToolDefIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		sess := tx.Session(&gorm.Session{FullSaveAssociations: false})
		if err := sess.Save(agent).Error; err != nil {
			return fmt.Errorf("failed to save agent: %w", err)
		}
		if !replaceBindings {
			return nil
		}
		if err := tx.Where("agent_id = ?", agent.ID).Delete(&models.AgentToolBinding{}).Error; err != nil {
			return fmt.Errorf("failed to delete tool bindings: %w", err)
		}
		emptyCfg := datatypes.JSON([]byte("{}"))
		if len(bindingToolDefIDs) > 0 {
			rows := make([]models.AgentToolBinding, len(bindingToolDefIDs))
			for i, tid := range bindingToolDefIDs {
				rows[i] = models.AgentToolBinding{
					AgentID:          agent.ID,
					ToolDefinitionID: tid,
					Config:           emptyCfg,
				}
			}
			if err := tx.Create(&rows).Error; err != nil {
				return fmt.Errorf("failed to insert tool bindings: %w", err)
			}
		}
		// touch updated_at: у models.Agent.UpdatedAt нет тега autoUpdateTime (workflow.go), gorm Save
		// может не изменить updated_at при отсутствии изменённых полей; инвариант A.5 13.3.1 требует bump
		// и для tool_bindings: [] / совпадающего множества id — явный UPDATE обязателен (не «дубль autoUpdateTime»).
		if err := tx.Model(&models.Agent{}).Where("id = ?", agent.ID).Update("updated_at", gorm.Expr("CURRENT_TIMESTAMP")).Error; err != nil {
			return fmt.Errorf("failed to touch agent updated_at: %w", err)
		}
		return nil
	})
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
