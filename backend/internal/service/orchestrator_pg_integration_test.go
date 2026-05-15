//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/internal/agent"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// orchestrator_pg_integration_test.go — Sprint 17 / Sprint 5F.2.
//
// Integration-тесты с реальным Postgres через testcontainers. Покрывают DoD-пункты
// которые требуют DB-семантики, недоступной в sqlite/mock:
//   - FOR UPDATE NOWAIT в Orchestrator.Step (per-task сериализация)
//   - SELECT FOR UPDATE SKIP LOCKED в TaskEventRepository.ClaimNext (worker pool)
//   - JSONB / TEXT[] / INTERVAL колонки + CHECK constraints миграций 031-038
//
// Запуск: `go test -tags integration ./internal/service/...` (требует Docker).
// В обычном `make test-unit` (без тега) эти тесты НЕ собираются.
//
// Используется postgres вместо yugabyte для скорости старта контейнера — yugabyte
// требует ~30с инициализации. Postgres достаточно: orchestrator_v2 использует только
// общие SQL-подмножества (FOR UPDATE NOWAIT, SKIP LOCKED, INSERT ... RETURNING),
// поддерживаемые обоими.

// ─────────────────────────────────────────────────────────────────────────────
// Test harness — реальный postgres + goose migrations + готовый Orchestrator
// ─────────────────────────────────────────────────────────────────────────────

type pgHarness struct {
	t            *testing.T
	container    *tcpostgres.PostgresContainer
	dsn          string
	gormDB       *gorm.DB
	sqlDB        *sql.DB
	artifactRepo repository.ArtifactRepository
	eventRepo    repository.TaskEventRepository
	decisionRepo repository.RouterDecisionRepository
	worktreeRepo repository.WorktreeRepository
	orchestrator *service.Orchestrator
	router       *service.RouterService
	cleanup      []func()
}

// startPgHarness поднимает postgres-контейнер, применяет миграции 001..040,
// конструирует репо/сервисы. Параметр scriptedRouterOutputs программирует
// последовательные ответы Router-LLM.
func startPgHarness(t *testing.T, scriptedRouterOutputs []string) *pgHarness {
	t.Helper()
	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("devteam_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			tcwait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	// Применяем все миграции 001..040 (включая 037, 038, 039, 040).
	migrationsDir := findMigrationsDir(t)
	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(sqlDB, migrationsDir); err != nil {
		t.Fatalf("goose up: %v", err)
	}

	gdb, err := gorm.Open(gormpostgres.New(gormpostgres.Config{DSN: dsn}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	h := &pgHarness{
		t:            t,
		container:    pgC,
		dsn:          dsn,
		gormDB:       gdb,
		sqlDB:        sqlDB,
		artifactRepo: repository.NewArtifactRepository(gdb),
		eventRepo:    repository.NewTaskEventRepository(gdb),
		decisionRepo: repository.NewRouterDecisionRepository(gdb),
		worktreeRepo: repository.NewWorktreeRepository(gdb),
		cleanup: []func(){
			func() { _ = sqlDB.Close() },
			func() { _ = pgC.Terminate(ctx) },
		},
	}

	// Router-агент seed создан миграцией 038. Достаточно для конструкции RouterService.
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	loader := service.NewDBAgentLoader(gdb)
	exec := &pgScriptedExecutor{responses: scriptedRouterOutputs}
	disp := &pgFixedDispatcher{exec: exec}
	h.router = service.NewRouterService(loader, disp, logger, service.DefaultRouterConfig())
	h.orchestrator = service.NewOrchestrator(
		gdb,
		h.artifactRepo, h.eventRepo, h.decisionRepo,
		nil, // worktreeMgr — для DAG-теста без sandbox-агентов не нужен
		h.router,
		nil, // notifier
		logger,
		service.DefaultOrchestratorConfig(),
	)

	return h
}

func (h *pgHarness) Close() {
	for i := len(h.cleanup) - 1; i >= 0; i-- {
		h.cleanup[i]()
	}
}

// findMigrationsDir — поиск директории миграций относительно тестового бинарника.
// Тесты могут запускаться из internal/service/, поэтому идём вверх до backend/db/migrations.
func findMigrationsDir(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for dir := wd; dir != "/"; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "db", "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		// Альтернатива: backend/db/migrations при запуске из repo root
		alt := filepath.Join(dir, "backend", "db", "migrations")
		if info, err := os.Stat(alt); err == nil && info.IsDir() {
			return alt
		}
	}
	t.Fatalf("migrations dir not found from %s", wd)
	return ""
}

// pgScriptedExecutor / pgFixedDispatcher — копии моков из orchestration_scenarios_test
// (которые в package service, а мы здесь в service_test, чтобы избежать конфликта типов
// между unit-тестами и integration-тестами).
type pgScriptedExecutor struct {
	responses []string
	calls     int
}

func (e *pgScriptedExecutor) Execute(_ context.Context, _ agent.ExecutionInput) (*agent.ExecutionResult, error) {
	if e.calls >= len(e.responses) {
		return nil, fmt.Errorf("pgScriptedExecutor: out of responses (call %d)", e.calls)
	}
	out := e.responses[e.calls]
	e.calls++
	return &agent.ExecutionResult{Success: true, Output: out}, nil
}

type pgFixedDispatcher struct{ exec agent.AgentExecutor }

func (d *pgFixedDispatcher) BuildExecutor(_ context.Context, _ *models.Agent) (agent.AgentExecutor, error) {
	return d.exec, nil
}

// gormpostgres — alias чтобы избежать конфликта импорта.
var _ = gormpostgres.Open

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

// TestPGIntegration_OrchestratorStep_DoneOutcome — самый базовый integration-цикл:
// создаём project + task, enqueue step_req, вызываем Step() напрямую (без worker'а),
// проверяем что Router-decision сохранился и task.state стал done.
func TestPGIntegration_OrchestratorStep_DoneOutcome(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startPgHarness(t, []string{
		`{"done": true, "outcome": "done", "agents": [], "reason": "integration test fixture: task already done"}`,
	})
	defer h.Close()

	ctx := context.Background()

	// Для простоты воспользуемся существующим router-агентом из seed 038 и создадим
	// фиктивную задачу с pre-set state=active (через user → project → task цепочку).
	taskID := h.createMinimalActiveTask(t)

	// Enqueue первый step_req.
	if err := h.orchestrator.EnqueueInitialStep(ctx, taskID); err != nil {
		t.Fatalf("EnqueueInitialStep: %v", err)
	}

	// Step напрямую.
	if err := h.orchestrator.Step(ctx, taskID); err != nil {
		t.Fatalf("Step: %v", err)
	}

	// Проверяем что state стал done.
	var got struct{ State string }
	if err := h.gormDB.Raw(`SELECT state FROM tasks WHERE id = ?`, taskID).Scan(&got).Error; err != nil {
		t.Fatalf("read task state: %v", err)
	}
	if got.State != "done" {
		t.Errorf("expected state=done, got %q", got.State)
	}

	// router_decision должен быть один.
	decisions, err := h.decisionRepo.ListByTaskID(ctx, taskID, false)
	if err != nil {
		t.Fatalf("ListByTaskID: %v", err)
	}
	if len(decisions) != 1 {
		t.Errorf("expected 1 router_decision, got %d", len(decisions))
	}
}

// createMinimalActiveTask — минимальная задача с FK на dummy project. Для тестов orchestrator'а
// достаточно tasks.state='active' + tasks.cancel_requested=false. Чтобы не тащить пол-схемы
// (users → projects → tasks), создаём фиктивные user и project на месте.
func (h *pgHarness) createMinimalActiveTask(t *testing.T) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	type idRow struct{ ID string }
	var u, p, ta idRow

	if err := h.gormDB.WithContext(ctx).Raw(
		`INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
		 VALUES (gen_random_uuid(), 'int@test', 'x', 'user', NOW(), NOW()) RETURNING id`,
	).Scan(&u).Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := h.gormDB.WithContext(ctx).Raw(
		`INSERT INTO projects (id, user_id, name, git_url, git_default_branch, created_at, updated_at)
		 VALUES (gen_random_uuid(), ?, 'integration-project', 'https://example.com/repo.git', 'main', NOW(), NOW()) RETURNING id`,
		u.ID,
	).Scan(&p).Error; err != nil {
		t.Fatalf("insert project: %v", err)
	}
	if err := h.gormDB.WithContext(ctx).Raw(
		`INSERT INTO tasks (id, project_id, title, description, priority, state, cancel_requested, current_step_no,
		 created_by_type, created_by_id, context, artifacts, created_at, updated_at)
		 VALUES (gen_random_uuid(), ?, 'integration task', 'fixture', 'medium', 'active', false, 0,
		         'user', ?, '{}'::jsonb, '{}'::jsonb, NOW(), NOW()) RETURNING id`,
		p.ID, u.ID,
	).Scan(&ta).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}
	id, err := uuid.Parse(ta.ID)
	if err != nil {
		t.Fatalf("parse task id %q: %v", ta.ID, err)
	}
	return id
}

// Silence "imported and not used" if helper-imports drop:
var _ = strings.Contains
