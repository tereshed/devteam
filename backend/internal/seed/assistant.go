// Package seed — bootstrap-функции, гарантирующие наличие системных записей
// в БД при старте бэкенда. Все функции идемпотентны.
package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devteam/backend/internal/models"
)

// AssistantAgentName — имя seed-агента глобального ассистента.
// ДОЛЖНО совпадать с service.AssistantAgentName: assistant_loop.go тащит
// agent по этому имени через AgentLoader.GetAgentByName. Дублируем как
// const, чтобы пакет seed не зависел от service (избегаем import-cycle).
const AssistantAgentName = "assistant"

// assistantDefaultSystemPrompt — дефолтный system prompt для роли assistant
// (план §6). Может быть переопределён через UI редактирования агентов:
// seed уважает существующую запись (ON CONFLICT DO NOTHING).
const assistantDefaultSystemPrompt = `Ты — ассистент платформы DevTeam. Помогаешь пользователю управлять проектами, задачами и агентами. Прежде чем менять состояние — кратко объясни намерение. Используй инструменты для чтения и действий. Отвечай по-русски, кратко, без воды.`

// Дефолтные параметры LLM для assistant-агента.
//
// Phase 5 review: переехал с anthropic+claude-haiku-4-5 на
// openrouter+deepseek/deepseek-v4-flash. Причина — assistant гоняет до
// AssistantMaxIterations=12 итераций на одно user-сообщение, и v4-flash
// (~$0.0000001/M tokens, ~10× быстрее haiku) даёт по-другому ощутимое UX:
// первый ответ за <1s вместо ~5s, плюс существенная экономия токенов.
// Качество tool-calling сопоставимо для управляющих задач уровня assistant'а.
//
// Оператор может перебить через UI / `PUT /api/v1/agents/:id` если нужна
// другая модель — seed уважает существующую запись (ON CONFLICT DO NOTHING).
//
// Температура низкая (0.2): assistant — управляющий агент, нам нужны
// предсказуемые tool_call'ы, а не креативные ответы.
const (
	assistantDefaultModel        = "deepseek/deepseek-v4-flash"
	assistantDefaultProviderKind = models.AgentProviderKindOpenRouter
)

var (
	assistantDefaultTemperature = float64(0.2)
	assistantDefaultMaxTokens   = int(4096)
)

// SeedAssistantAgent — INSERT записи agent с role='assistant', если её ещё
// нет. Идемпотентна: повторный вызов на ту же БД no-op'ит за счёт
// `ON CONFLICT (name) WHERE team_id IS NULL DO NOTHING` (partial unique
// index `idx_agents_global_name` из миграции 038).
//
// Контракт: вызывать ПОСЛЕ runMigrations — иначе нет ни таблицы agents,
// ни chk_agents_role, разрешающего 'assistant' (миграция 046).
//
// Логирует факт создания/no-op'а через переданный logger; если logger=nil,
// логи молча отбрасываются (тесты передают nil).
func SeedAssistantAgent(ctx context.Context, db *gorm.DB, logger *slog.Logger) error {
	if db == nil {
		return errors.New("seed: db is required")
	}

	agent := &models.Agent{
		Name:          AssistantAgentName,
		Role:          models.AgentRoleAssistant,
		ExecutionKind: models.AgentExecutionKindLLM,
		Model:         strPtr(assistantDefaultModel),
		ProviderKind:  providerKindPtr(assistantDefaultProviderKind),
		SystemPrompt:  strPtr(assistantDefaultSystemPrompt),
		Temperature:   &assistantDefaultTemperature,
		MaxTokens:     &assistantDefaultMaxTokens,
		IsActive:      true,

		// Обязательные JSONB-поля с NOT NULL DEFAULT в схеме: GORM при INSERT
		// шлёт NULL, если поле = nil, что нарушит NOT NULL. Передаём явные
		// пустые JSON-литералы (то же самое делает миграция 038).
		Skills:              datatypes.JSON([]byte(`[]`)),
		Settings:            datatypes.JSON([]byte(`{}`)),
		ModelConfig:         datatypes.JSON([]byte(`{}`)),
		CodeBackendSettings: datatypes.JSON([]byte(`{}`)),
		SandboxPermissions:  datatypes.JSON([]byte(`{}`)),
		// team_id = NULL → системный (глобальный) агент. Под partial unique
		// `idx_agents_global_name`, на который опирается ON CONFLICT ниже.
	}

	// WHERE-clause обязателен: arbiter — partial unique index
	// idx_agents_global_name (team_id IS NULL). Без него PG требует
	// неусечённого UNIQUE(name), которого после миграции 016 нет.
	//
	// Сырое clause.Expr вместо clause.Eq{Value: nil}: GORM сейчас неявно
	// конвертирует nil→IS NULL, но это implementation detail. Явный SQL
	// 1:1 совпадает с предикатом partial-индекса (см. миграция 038), что
	// гарантирует, что planner возьмёт именно его как arbiter — даже после
	// мажорных апдейтов GORM.
	res := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "team_id IS NULL"},
		}},
		DoNothing: true,
	}).Create(agent)
	if res.Error != nil {
		return fmt.Errorf("seed assistant agent: %w", res.Error)
	}

	if logger != nil {
		if res.RowsAffected == 0 {
			logger.InfoContext(ctx, "seed: assistant agent already present, skipped",
				slog.String("name", AssistantAgentName),
			)
		} else {
			logger.InfoContext(ctx, "seed: assistant agent created",
				slog.String("name", AssistantAgentName),
				slog.String("role", string(models.AgentRoleAssistant)),
			)
		}
	}
	return nil
}

func strPtr(s string) *string { return &s }

func providerKindPtr(k models.AgentProviderKind) *models.AgentProviderKind {
	return &k
}
