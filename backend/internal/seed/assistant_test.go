//go:build integration

package seed_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/seed"
)

// setupTestDB подключается к интеграционной БД. Mirrors helpers из
// internal/repository/*_test.go (та же конвенция и тот же дефолтный DSN),
// но повторяем здесь — кросс-пакетные test-helpers Go запрещает.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "connect to test DB")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping(), "ping test DB")
	return db
}

// cleanupAssistantSeed удаляет ровно ту запись, что создаёт SeedAssistantAgent.
// Не трогаем чужих агентов: тесты могут гонять параллельно с уже-засеянной
// staging-БД, и DELETE FROM agents без WHERE снесёт production seed
// миграции 038 (router/planner/...).
func cleanupAssistantSeed(t *testing.T, db *gorm.DB) {
	t.Helper()
	err := db.Exec(`DELETE FROM agents WHERE name = ? AND team_id IS NULL`, seed.AssistantAgentName).Error
	require.NoError(t, err)
}

// TestSeedAssistantAgent_CreatesOnEmptyDB — happy path: запись отсутствует,
// после вызова появляется с дефолтными значениями (план §6).
func TestSeedAssistantAgent_CreatesOnEmptyDB(t *testing.T) {
	db := setupTestDB(t)
	cleanupAssistantSeed(t, db)
	defer cleanupAssistantSeed(t, db)

	ctx := context.Background()
	require.NoError(t, seed.SeedAssistantAgent(ctx, db, nil))

	var agent models.Agent
	require.NoError(t,
		db.Where("name = ? AND team_id IS NULL", seed.AssistantAgentName).First(&agent).Error)

	assert.Equal(t, models.AgentRoleAssistant, agent.Role)
	assert.Equal(t, models.AgentExecutionKindLLM, agent.ExecutionKind)
	require.NotNil(t, agent.Model)
	assert.Equal(t, "claude-sonnet-4-6", *agent.Model)
	require.NotNil(t, agent.ProviderKind)
	assert.Equal(t, models.AgentProviderKindAnthropic, *agent.ProviderKind)
	require.NotNil(t, agent.SystemPrompt)
	assert.NotEmpty(t, *agent.SystemPrompt, "system_prompt должен быть проставлен")
	assert.True(t, agent.IsActive)
	assert.Nil(t, agent.TeamID, "глобальный seed-агент: team_id NULL")
	assert.Nil(t, agent.CodeBackend, "llm-агент: code_backend ОБЯЗАН быть NULL (chk_agents_kind_requirements)")
}

// TestSeedAssistantAgent_Idempotent — повторный вызов на той же БД не падает
// и не дублирует запись. Защита от рестарта бэкенда.
func TestSeedAssistantAgent_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	cleanupAssistantSeed(t, db)
	defer cleanupAssistantSeed(t, db)

	ctx := context.Background()
	require.NoError(t, seed.SeedAssistantAgent(ctx, db, nil))
	require.NoError(t, seed.SeedAssistantAgent(ctx, db, nil), "second call must be no-op")
	require.NoError(t, seed.SeedAssistantAgent(ctx, db, nil), "third call must be no-op")

	var count int64
	require.NoError(t,
		db.Model(&models.Agent{}).
			Where("name = ? AND team_id IS NULL", seed.AssistantAgentName).
			Count(&count).Error)
	assert.Equal(t, int64(1), count, "ON CONFLICT DO NOTHING — не должно быть дублей")
}

// TestSeedAssistantAgent_DoesNotOverwriteCustomPrompt — главное обещание
// плана §6 «Prompt можно потом править через существующий UI». Если
// оператор перепрошил system_prompt, рестарт бэкенда не должен его
// перетереть. Защищает контракт DO NOTHING.
func TestSeedAssistantAgent_DoesNotOverwriteCustomPrompt(t *testing.T) {
	db := setupTestDB(t)
	cleanupAssistantSeed(t, db)
	defer cleanupAssistantSeed(t, db)

	ctx := context.Background()

	// 1) seed создаёт запись с дефолтным промптом.
	require.NoError(t, seed.SeedAssistantAgent(ctx, db, nil))

	// 2) Имитируем правку оператора через UI.
	const customPrompt = "Кастомный промпт от оператора, не трогать"
	customTemp := 0.7
	res := db.Model(&models.Agent{}).
		Where("name = ? AND team_id IS NULL", seed.AssistantAgentName).
		Updates(map[string]any{
			"system_prompt": customPrompt,
			"temperature":   customTemp,
		})
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected)

	// 3) Повторный seed (имитируем рестарт бэкенда) — DO NOTHING.
	require.NoError(t, seed.SeedAssistantAgent(ctx, db, nil))

	// 4) Поля оператора сохранились.
	var after models.Agent
	require.NoError(t,
		db.Where("name = ? AND team_id IS NULL", seed.AssistantAgentName).First(&after).Error)
	require.NotNil(t, after.SystemPrompt)
	assert.Equal(t, customPrompt, *after.SystemPrompt, "seed не должен перетирать пользовательский промпт")
	require.NotNil(t, after.Temperature)
	assert.InDelta(t, customTemp, *after.Temperature, 0.001, "seed не должен сбрасывать пользовательскую температуру")
}

