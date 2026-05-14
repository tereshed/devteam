package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// agent_repository.go — Sprint 17 / Sprint 5 — CRUD по агентам.
//
// Существующий код читает агентов напрямую через *gorm.DB в нескольких местах
// (agent_dispatcher, conversation_service и т.д.). Sprint 5 централизует это
// для:
//   - MCP-инструментов (list_agents, create_agent, update_agent — добавляются в Sprint 5B)
//   - Frontend Agents Management screen (Sprint 5F)
//   - Будущих внутренних сервисов (например, RouterService.AgentLoader через repo вместо DBAgentLoader)
//
// AgentSecretRepository (Sprint 1) для секретов остаётся отдельным —
// secret-операции имеют свой шифрованный pipeline (pkg/crypto + AAD).

// ErrAgentNameTaken — попытка Create/UPDATE с уже существующим name (unique constraint).
// Sprint 5: ErrAgentNotFound уже объявлен в errors.go (используется legacy team-кодом);
// переиспользуем его, не дублируем.
var ErrAgentNameTaken = errors.New("agent name already taken")

// ErrAgentConcurrentUpdate — optimistic concurrency violation: expected updated_at
// не совпал с тем что в БД (другой процесс изменил запись параллельно).
// Sprint 5 review fix #2: защита от lost-update.
var ErrAgentConcurrentUpdate = errors.New("agent was modified concurrently, please retry")

// AgentFilter — фильтры для List. Все поля опциональные.
type AgentFilter struct {
	// OnlyActive — true вернёт только is_active=true.
	OnlyActive bool
	// ExecutionKind — фильтр по llm|sandbox (см. models.AgentExecutionKind).
	ExecutionKind *models.AgentExecutionKind
	// Role — фильтр по AgentRole.
	Role *models.AgentRole
	// NameLike — частичный поиск по name (case-insensitive LIKE с escape).
	NameLike string
	// Limit/Offset — пагинация. Limit<=0 — default 50; >200 — clamp to 200.
	Limit  int
	Offset int
}

// AgentRepository CRUD по агентам.
//
// Внимание: списковые методы используют `agentListColumns` чтобы НЕ тащить
// system_prompt (большое поле) при перечислении. GetByID/GetByName возвращают
// полную запись.
//
// Sprint 5 review fix #2: все методы tx-aware (используют `gormDB(ctx, r.db)`)
// — caller'ы (service-слой) могут оборачивать вызовы в TransactionManager.WithTransaction.
type AgentRepository interface {
	Create(ctx context.Context, agent *models.Agent) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	// GetByIDForUpdate — SELECT ... FOR UPDATE (внутри транзакции блокирует строку
	// до commit'а). Sprint 5: используется AgentService.Update для защиты от
	// Lost-Update в Read-Modify-Write цикле.
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	GetByName(ctx context.Context, name string) (*models.Agent, error)
	List(ctx context.Context, filter AgentFilter) ([]models.Agent, int64, error)
	// Update — optimistic concurrency: WHERE id=? AND updated_at=expectedUpdatedAt.
	// Sprint 5 review fix #2: возвращает ErrAgentConcurrentUpdate если другой
	// процесс изменил запись между чтением и записью.
	Update(ctx context.Context, agent *models.Agent, expectedUpdatedAt time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// agentListColumns — НЕ включаем system_prompt (может быть большим);
// при необходимости caller вызывает GetByID/GetByName.
const agentListColumns = `
	id, name, role, team_id, model, prompt_id, skills,
	code_backend, settings, model_config,
	provider_kind, code_backend_settings, sandbox_permissions,
	is_active, requires_code_context,
	execution_kind, role_description, temperature, max_tokens,
	created_at, updated_at
`

type agentRepository struct {
	db *gorm.DB
}

// NewAgentRepository — конструктор.
func NewAgentRepository(db *gorm.DB) AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) Create(ctx context.Context, a *models.Agent) error {
	db := gormDB(ctx, r.db)
	if err := db.WithContext(ctx).Create(a).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrAgentNameTaken
		}
		return fmt.Errorf("failed to create agent: %w", err)
	}
	return nil
}

func (r *agentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	db := gormDB(ctx, r.db)
	var a models.Agent
	err := db.WithContext(ctx).Where("id = ?", id).First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent %s: %w", id, err)
	}
	return &a, nil
}

// GetByIDForUpdate — SELECT ... FOR UPDATE. Должен вызываться ВНУТРИ транзакции
// (через TransactionManager); вне tx FOR UPDATE — no-op (lock тут же снимается).
//
// Yugabyte поддерживает FOR UPDATE (см. план §5g — мы заменили advisory locks на
// row-locks как Yugabyte-friendly примитив).
func (r *agentRepository) GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	db := gormDB(ctx, r.db)
	var a models.Agent
	err := db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent %s for update: %w", id, err)
	}
	return &a, nil
}

func (r *agentRepository) GetByName(ctx context.Context, name string) (*models.Agent, error) {
	db := gormDB(ctx, r.db)
	var a models.Agent
	err := db.WithContext(ctx).Where("name = ?", name).First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, fmt.Errorf("failed to get agent by name %q: %w", name, err)
	}
	return &a, nil
}

func (r *agentRepository) List(ctx context.Context, filter AgentFilter) ([]models.Agent, int64, error) {
	db := gormDB(ctx, r.db)
	limit := normalizeLimit(filter.Limit, 50, 200)
	offset := normalizeOffset(filter.Offset)

	q := db.WithContext(ctx).Model(&models.Agent{})
	if filter.OnlyActive {
		q = q.Where("is_active = ?", true)
	}
	if filter.ExecutionKind != nil {
		q = q.Where("execution_kind = ?", string(*filter.ExecutionKind))
	}
	if filter.Role != nil {
		q = q.Where("role = ?", string(*filter.Role))
	}
	if filter.NameLike != "" {
		pattern := "%" + escapeILIKEWildcards(filter.NameLike) + "%"
		q = q.Where(`name ILIKE ? ESCAPE '\'`, pattern)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count agents: %w", err)
	}

	var agents []models.Agent
	err := q.Select(agentListColumns).
		Order("name ASC").
		Limit(limit).
		Offset(offset).
		Find(&agents).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list agents: %w", err)
	}
	return agents, total, nil
}

// Update — optimistic concurrency через `WHERE id=? AND updated_at=expectedUpdatedAt`.
// Sprint 5 review fix #2: защита от Lost-Update. Если другой процесс обновил
// запись между чтением и записью — RowsAffected будет 0 → ErrAgentConcurrentUpdate.
//
// Параметр a содержит ВСЕ обновляемые поля (caller должен сначала прочитать
// через GetByIDForUpdate, изменить нужные поля, передать сюда).
func (r *agentRepository) Update(ctx context.Context, a *models.Agent, expectedUpdatedAt time.Time) error {
	db := gormDB(ctx, r.db)
	now := time.Now().UTC()
	updates := map[string]any{
		"name":                   a.Name,
		"role":                   a.Role,
		"team_id":                a.TeamID,
		"model":                  a.Model,
		"prompt_id":              a.PromptID,
		"skills":                 a.Skills,
		"code_backend":           a.CodeBackend,
		"settings":               a.Settings,
		"model_config":           a.ModelConfig,
		"provider_kind":          a.ProviderKind,
		"code_backend_settings":  a.CodeBackendSettings,
		"sandbox_permissions":    a.SandboxPermissions,
		"is_active":              a.IsActive,
		"requires_code_context":  a.RequiresCodeContext,
		"execution_kind":         a.ExecutionKind,
		"role_description":       a.RoleDescription,
		"system_prompt":          a.SystemPrompt,
		"temperature":            a.Temperature,
		"max_tokens":             a.MaxTokens,
		"updated_at":             now,
	}
	result := db.WithContext(ctx).Model(&models.Agent{}).
		Where("id = ? AND updated_at = ?", a.ID, expectedUpdatedAt).
		Updates(updates)
	if result.Error != nil {
		if isUniqueViolation(result.Error) {
			return ErrAgentNameTaken
		}
		return fmt.Errorf("failed to update agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// Либо запись исчезла, либо updated_at не совпал. Различить сложно без
		// доп. запроса; маппим в ErrAgentConcurrentUpdate (caller-friendly: retry).
		return ErrAgentConcurrentUpdate
	}
	return nil
}

func (r *agentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	db := gormDB(ctx, r.db)
	// Hard delete — это семантически "удалить из реестра". Для soft-disable
	// есть is_active=false (правильный способ для backward-compat с in-flight задачами).
	result := db.WithContext(ctx).Where("id = ?", id).Delete(&models.Agent{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete agent %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAgentNotFound
	}
	return nil
}


// isUniqueViolation — проверка через pgconn.PgError (SQLSTATE 23505).
// Sprint 5 review fix #3: ранее парсили текст err.Error() (хрупко к локали /
// версии Postgres). pgconn уже в транзитивных зависимостях GORM, используем его.
//
// Мы НЕ ограничиваем по ConstraintName/ColumnName "agents_name_key" — единственная
// уникальная колонка в `agents` это name, других unique нет; любой 23505 в этой
// таблице — это коллизия имени.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
