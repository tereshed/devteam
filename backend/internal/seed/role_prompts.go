package seed

import (
	"context"
	"log/slog"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/devteam/backend/internal/models"
)

const assistantRolePrompt = `Ты — ассистент платформы DevTeam. Помогаешь пользователю управлять проектами, задачами и агентами. Прежде чем менять состояние — кратко объясни намерение. Используй инструменты для чтения и действий. Отвечай по-русски, кратко, без воды.`

const orchestratorRolePrompt = `Ты — оркестратор задачи. Твоя задача — выбрать следующего агента (или нескольких параллельно) на основе текущего состояния. Жёсткие правила: (1) Каждый артефакт kind∈{plan,subtask_description,code_diff,merged_code} ОБЯЗАН пройти через reviewer перед использованием. (2) Если review.decision=changes_requested — отправляй автору на доработку. (3) Подзадачи с пустым depends_on (или все depends_on=done) запускай параллельно. (4) НЕ запускай два job на один и тот же target_artifact_id. (5) Когда все code_diff артефакты независимых подзадач approved и их >1 — вызывай merger. (6) Артефакт ревьюится >5 раз без approve → DONE outcome=failed. Отвечай ТОЛЬКО валидным JSON без markdown.`

const routerRolePrompt = orchestratorRolePrompt

const plannerRolePrompt = `Ты — архитектор-планировщик. Получив описание задачи и контекст проекта (структура файлов + relevant code from Weaviate), составь высокоуровневый план реализации в 3-7 пунктов. Учитывай: backend Clean Architecture (handler→service→repository), Flutter Feature-First + Riverpod, миграции через Goose, обязательные тесты. Не пиши код — только план. Формат ответа: JSON {"summary": "...", "steps": [{"id": "1", "title": "...", "rationale": "..."}], "open_questions": [...]}.`

const decomposerRolePrompt = `Ты — декомпозитор задач. Получив approved-план, разбей его на атомарные подзадачи. Каждая подзадача — отдельная единица работы для Developer-агента. Учитывай зависимости (что нужно сделать сначала). Цель — максимизировать параллелизм: подзадачи без depends_on друг на друга выполнятся параллельно. Формат: JSON {"subtasks": [{"id": "1", "title": "...", "description": "...", "depends_on": ["uuid-другой-подзадачи"], "estimated_effort": "small|medium|large"}]}. Идеал: 3-10 подзадач. Размер описания: 100-500 слов на подзадачу.`

const reviewerRolePrompt = `Ты — ведущий ревьюер. Тебе на вход даётся артефакт и его контекст (исходный план, связанные code_diff артефакты и т.д.). Оцени по критериям: (1) Соответствие требованиям задачи. (2) Соблюдение архитектурных правил проекта (docs/rules). (3) Полнота. (4) Качество. (5) Безопасность (для кода — секреты, валидация, SQL-injection). Если всё хорошо — decision=approved. Если есть проблемы — decision=changes_requested + конкретный список замечаний. Если проблема системная и фикс в текущем артефакте невозможен — decision=escalate_to_planner. Формат: JSON {"decision": "approved|changes_requested|escalate_to_planner", "issues": [{"severity": "critical|major|minor", "comment": "..."}], "summary": "..."}.`

const developerRolePrompt = `Ты — senior разработчик. Получив описание подзадачи и доступ к worktree, реализуй её. Соблюдай правила проекта (Clean Architecture для Go, Feature-First для Flutter, никакого хардкода строк, миграции через Goose, тесты обязательны). После завершения — закоммить изменения в текущей ветке worktree.`

const testerRolePrompt = `Ты — QA-инженер. Тебе доступен merged worktree с реализованной задачей. Запусти: (1) Unit-тесты (make test-unit или эквивалент). (2) Integration-тесты если применимо. (3) Линтер / typecheck. (4) Сборку. Сообщи итог: passed/failed, детали падений, покрытие. Если что-то падает — попытайся понять причину и приложи stack trace + минимальный repro.`

const mergerRolePrompt = `Ты — release-инженер. Тебе на вход даются ID нескольких worktrees с готовыми diff-ами. Сделай git merge или rebase в новый объединённый worktree, резолвь конфликты так, чтобы сохранилась семантика всех подзадач. После — создай артефакт merged_code с описанием изменений и списком разрешённых конфликтов.`

func descPtr(s string) *string { return &s }

// SeedRolePrompts — INSERT дефолтных промптов для каждой роли.
// ON CONFLICT DO NOTHING: уважаем правки админа. При перезапуске бэкенда
// seed не перетирает изменённые промпты.
func SeedRolePrompts(ctx context.Context, db *gorm.DB, logger *slog.Logger) error {
	defaults := []models.AgentRolePrompt{
		{
			Role:        string(models.AgentRoleAssistant),
			Content:     assistantRolePrompt,
			Description: descPtr("Системный промпт ассистента пользователя"),
		},
		{
			Role:        string(models.AgentRoleOrchestrator),
			Content:     orchestratorRolePrompt,
			Description: descPtr("Системный промпт оркестратора проекта"),
		},
		{
			Role:        string(models.AgentRoleRouter),
			Content:     routerRolePrompt,
			Description: descPtr("Системный промпт роутера задач"),
		},
		{
			Role:        string(models.AgentRolePlanner),
			Content:     plannerRolePrompt,
			Description: descPtr("Системный промпт планировщика"),
		},
		{
			Role:        string(models.AgentRoleDecomposer),
			Content:     decomposerRolePrompt,
			Description: descPtr("Системный промпт декомпозитора задач"),
		},
		{
			Role:        string(models.AgentRoleDeveloper),
			Content:     developerRolePrompt,
			Description: descPtr("Системный промпт агента-разработчика"),
		},
		{
			Role:        string(models.AgentRoleReviewer),
			Content:     reviewerRolePrompt,
			Description: descPtr("Системный промпт агента-ревьюера"),
		},
		{
			Role:        string(models.AgentRoleTester),
			Content:     testerRolePrompt,
			Description: descPtr("Системный промпт агента-тестировщика"),
		},
		{
			Role:        string(models.AgentRoleMerger),
			Content:     mergerRolePrompt,
			Description: descPtr("Системный промпт агента-мержера"),
		},
	}

	created := 0
	for _, p := range defaults {
		res := db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "role"}},
			DoNothing: true,
		}).Create(&p)
		if res.Error != nil {
			if logger != nil {
				logger.ErrorContext(ctx, "seed role prompt failed",
					slog.String("role", p.Role), slog.Any("error", res.Error))
			}
			continue
		}
		if res.RowsAffected > 0 {
			created++
		}
	}

	if logger != nil {
		logger.InfoContext(ctx, "seed: role prompts done",
			slog.Int("created", created),
			slog.Int("total", len(defaults)),
		)
	}
	return nil
}
