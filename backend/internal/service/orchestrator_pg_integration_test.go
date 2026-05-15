//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// pgHarnessOpts — необязательные опции для startPgHarness.
type pgHarnessOpts struct {
	// orchestratorCfg — переопределение OrchestratorConfig (например, маленький
	// MaxStepsPerTask для теста overflow). nil-указатель = default.
	orchestratorCfg *service.OrchestratorConfig
	// execCallback — вызывается перед каждым Execute scripted-executor'а
	// (с номером вызова, начиная с 0). Используется в cancel-тесте чтобы
	// триггерить RequestCancel из середины Step'а.
	execCallback func(callNo int)
	// scriptedExec — если не nil, переиспользуется (нужно для restart-теста,
	// где новый harness работает поверх существующего контейнера).
	scriptedExec *pgScriptedExecutor
}

// startPgHarness поднимает postgres-контейнер, применяет миграции 001..040,
// конструирует репо/сервисы. Параметр scriptedRouterOutputs программирует
// последовательные ответы Router-LLM.
func startPgHarness(t *testing.T, scriptedRouterOutputs []string) *pgHarness {
	return startPgHarnessWithOpts(t, scriptedRouterOutputs, pgHarnessOpts{})
}

func startPgHarnessWithOpts(t *testing.T, scriptedRouterOutputs []string, opts pgHarnessOpts) *pgHarness {
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

	return reuseHarness(t, dsn, scriptedRouterOutputs, opts, []func(){
		func() { _ = pgC.Terminate(ctx) },
	}, pgC)
}

// reuseHarness — общая часть конструкции harness'а. Может вызываться:
//   - startPgHarnessWithOpts: с новым postgres-контейнером
//   - повторно из restart-теста с тем же DSN (контейнер не пересоздаётся)
func reuseHarness(t *testing.T, dsn string, scriptedRouterOutputs []string, opts pgHarnessOpts,
	extraCleanup []func(), pgC *tcpostgres.PostgresContainer) *pgHarness {
	t.Helper()

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}

	// Применяем все миграции (идемпотентно при reuse: goose видит существующую
	// goose_db_version таблицу и пропускает уже применённые).
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

	cleanup := []func(){
		func() { _ = sqlDB.Close() },
	}
	cleanup = append(cleanup, extraCleanup...)

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
		cleanup:      cleanup,
	}

	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	loader := service.NewDBAgentLoader(gdb)
	exec := opts.scriptedExec
	if exec == nil {
		exec = &pgScriptedExecutor{responses: scriptedRouterOutputs, callback: opts.execCallback}
	}
	disp := &pgFixedDispatcher{exec: exec}
	h.router = service.NewRouterService(loader, disp, logger, service.DefaultRouterConfig())

	cfg := service.DefaultOrchestratorConfig()
	if opts.orchestratorCfg != nil {
		cfg = *opts.orchestratorCfg
	}
	h.orchestrator = service.NewOrchestrator(
		gdb,
		h.artifactRepo, h.eventRepo, h.decisionRepo,
		nil, // worktreeMgr — для текущих интеграционных тестов без sandbox-агентов не нужен
		h.router,
		nil, // notifier
		logger,
		cfg,
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
	mu        sync.Mutex
	responses []string
	calls     int
	// callback — необязательный hook, вызываемый перед выдачей ответа на N-м
	// вызове (N = текущее значение e.calls, начиная с 0). Позволяет тесту
	// триггерить внешние операции (RequestCancel) ровно посередине Step'а.
	callback func(callNo int)
}

func (e *pgScriptedExecutor) Execute(_ context.Context, _ agent.ExecutionInput) (*agent.ExecutionResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.callback != nil {
		e.callback(e.calls)
	}
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

// ─────────────────────────────────────────────────────────────────────────────
// TestPGIntegration_DAG_DependsOn
// ─────────────────────────────────────────────────────────────────────────────

// Проверяет что postgres-семантика JSONB корректно отдаёт артефакты с depends_on в
// content и что state-loader Orchestrator'а собирает их по `ORDER BY created_at`.
// Прямой доступ к loadRouterState невозможен (он приватный), поэтому используем
// факт: Router в каждом Step видит ровно артефакты задачи (через ListMetadataByTaskID)
// и принимает решения; scripted Router возвращает ожидаемые ответы, и Step не падает.
// В конце проверяем что depends_on массив достаём из БД "как есть" (через ArtifactRepository),
// и что все 3 subtask_description видны в metadata-листинге в правильном порядке.
func TestPGIntegration_DAG_DependsOn(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	// Router отдаст 2 ответа: первый "decompose дальше" (Step 1), второй DONE (Step 2).
	// Между ними мы вручную создаём цепочку артефактов чтобы убедиться что
	// loadRouterState их корректно подхватывает в Step 2.
	h := startPgHarness(t, []string{
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "decompose"}`,
		`{"done": true, "outcome": "done", "agents": [], "reason": "all subtasks chained correctly"}`,
	})
	defer h.Close()

	ctx := context.Background()
	taskID := h.createMinimalActiveTask(t)

	// Создаём 4 артефакта в правильном порядке (ORDER BY created_at гарантируется
	// разными NOW() в отдельных INSERT'ах + serial PK сортировкой при равных таймстампах).
	// 1) plan
	// 2) subtask_description #1 (depends_on=[])
	// 3) subtask_description #2 (depends_on=[id-of-#1])
	// 4) subtask_description #3 (depends_on=[id-of-#1, id-of-#2])
	type idRow struct{ ID string }

	insertArt := func(kind string, contentJSON string) string {
		var r idRow
		err := h.gormDB.WithContext(ctx).Raw(
			`INSERT INTO artifacts (id, task_id, producer_agent, kind, summary, content, status, iteration, created_at)
			 VALUES (gen_random_uuid(), ?, 'planner', ?, 'fixture', ?::jsonb, 'ready', 0, NOW())
			 RETURNING id`,
			taskID, kind, contentJSON,
		).Scan(&r).Error
		if err != nil {
			t.Fatalf("insert artifact %s: %v", kind, err)
		}
		// Микросдвиг во времени, чтобы ORDER BY created_at был стабилен между
		// артефактами (NOW() в одном statement может выдать одинаковое значение).
		time.Sleep(2 * time.Millisecond)
		return r.ID
	}

	planID := insertArt("plan", `{"steps":["a","b","c"]}`)
	st1 := insertArt("subtask_description", `{"depends_on":[],"title":"s1"}`)
	st2 := insertArt("subtask_description", fmt.Sprintf(`{"depends_on":[%q],"title":"s2"}`, st1))
	st3 := insertArt("subtask_description", fmt.Sprintf(`{"depends_on":[%q,%q],"title":"s3"}`, st1, st2))
	_ = planID

	// Enqueue + первый Step.
	if err := h.orchestrator.EnqueueInitialStep(ctx, taskID); err != nil {
		t.Fatalf("EnqueueInitialStep: %v", err)
	}
	if err := h.orchestrator.Step(ctx, taskID); err != nil {
		t.Fatalf("Step 1: %v", err)
	}
	// Второй Step — Router должен вернуть DONE.
	if err := h.orchestrator.Step(ctx, taskID); err != nil {
		t.Fatalf("Step 2: %v", err)
	}

	// Проверка 1: state=done.
	var got struct{ State string }
	if err := h.gormDB.Raw(`SELECT state FROM tasks WHERE id = ?`, taskID).Scan(&got).Error; err != nil {
		t.Fatalf("read task state: %v", err)
	}
	if got.State != "done" {
		t.Errorf("expected state=done, got %q", got.State)
	}

	// Проверка 2: state-loader (через прямой вызов того же артефакт-репо что и Orchestrator)
	// возвращает все 4 артефакта в порядке создания.
	arts, err := h.artifactRepo.ListMetadataByTaskID(ctx, taskID, true)
	if err != nil {
		t.Fatalf("ListMetadataByTaskID: %v", err)
	}
	if len(arts) != 4 {
		t.Fatalf("expected 4 artifacts, got %d", len(arts))
	}
	wantKinds := []string{"plan", "subtask_description", "subtask_description", "subtask_description"}
	for i, a := range arts {
		if string(a.Kind) != wantKinds[i] {
			t.Errorf("arts[%d].Kind = %q, want %q", i, a.Kind, wantKinds[i])
		}
	}

	// Проверка 3: depends_on в JSONB достаётся для последнего subtask_description.
	var depRow struct {
		Content []byte
	}
	if err := h.gormDB.Raw(
		`SELECT content FROM artifacts WHERE id = ?`, st3,
	).Scan(&depRow).Error; err != nil {
		t.Fatalf("read content: %v", err)
	}
	if !strings.Contains(string(depRow.Content), st1) || !strings.Contains(string(depRow.Content), st2) {
		t.Errorf("expected depends_on to reference %s and %s, got content=%s", st1, st2, string(depRow.Content))
	}

	// Проверка 4: было ровно 2 router_decisions (по числу Step'ов).
	decisions, err := h.decisionRepo.ListByTaskID(ctx, taskID, false)
	if err != nil {
		t.Fatalf("ListByTaskID: %v", err)
	}
	if len(decisions) != 2 {
		t.Errorf("expected 2 router_decisions, got %d", len(decisions))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPGIntegration_CancelMidFlight
// ─────────────────────────────────────────────────────────────────────────────

// Проверяет кооперативную отмену: после RequestCancel следующий Step должен:
//   - выставить state='cancelled' (cancel_requested=true сохраняется в UPDATE задачи)
//   - не enqueue'ить новые agent_jobs
//
// Дизайн: первый Step консумит первый scripted-ответ Router'а и enqueue'ит agent_job.
// Затем тест вызывает RequestCancel, второй Step не должен дойти до Router (cancel
// checked before Router call) — agent_job_count остаётся прежним.
func TestPGIntegration_CancelMidFlight(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startPgHarness(t, []string{
		// Первый Step: Router возвращает 1 agent_job. На второй Step Router НЕ должен
		// быть вызван — мы намеренно не даём ему ответа.
		`{"done": false, "agents": [{"agent": "planner"}], "reason": "kick off"}`,
	})
	defer h.Close()

	ctx := context.Background()
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	lifecycle := service.NewTaskLifecycleService(h.gormDB, nil, logger)

	taskID := h.createMinimalActiveTask(t)
	if err := h.orchestrator.EnqueueInitialStep(ctx, taskID); err != nil {
		t.Fatalf("EnqueueInitialStep: %v", err)
	}

	// Step 1 — нормальное продолжение, enqueue'ит agent_job.
	if err := h.orchestrator.Step(ctx, taskID); err != nil {
		t.Fatalf("Step 1: %v", err)
	}

	// Замер agent_jobs до отмены.
	countAgentJobs := func() int64 {
		var n int64
		if err := h.gormDB.Raw(
			`SELECT COUNT(*) FROM task_events WHERE task_id = ? AND kind = 'agent_job'`, taskID,
		).Scan(&n).Error; err != nil {
			t.Fatalf("count agent_jobs: %v", err)
		}
		return n
	}
	beforeCancel := countAgentJobs()
	if beforeCancel != 1 {
		t.Fatalf("expected 1 agent_job after Step 1, got %d", beforeCancel)
	}

	// Запрос отмены — гонкой не страдаем т.к. Step 1 уже завершился (per-task lock снят).
	if err := lifecycle.RequestCancel(ctx, taskID); err != nil {
		t.Fatalf("RequestCancel: %v", err)
	}

	// Step 2 — должен увидеть cancel_requested=true и финализировать.
	if err := h.orchestrator.Step(ctx, taskID); err != nil {
		t.Fatalf("Step 2 (after cancel): %v", err)
	}

	// Проверка 1: state=cancelled, cancel_requested=true.
	var got struct {
		State           string
		CancelRequested bool
	}
	if err := h.gormDB.Raw(
		`SELECT state, cancel_requested FROM tasks WHERE id = ?`, taskID,
	).Scan(&got).Error; err != nil {
		t.Fatalf("read task after cancel: %v", err)
	}
	if got.State != "cancelled" {
		t.Errorf("expected state=cancelled, got %q", got.State)
	}
	if !got.CancelRequested {
		t.Errorf("expected cancel_requested=true")
	}

	// Проверка 2: НЕТ новых agent_jobs после отмены.
	afterCancel := countAgentJobs()
	if afterCancel != beforeCancel {
		t.Errorf("expected no new agent_jobs after cancel, before=%d after=%d", beforeCancel, afterCancel)
	}

	// Проверка 3: worktrees (если бы они были — у нас worktreeMgr=nil, поэтому ни одного
	// не создано) — проверяем что репозиторий возвращает пустой список (sanity).
	wts, err := h.worktreeRepo.ListByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("worktrees list: %v", err)
	}
	for _, wt := range wts {
		if wt.State != models.WorktreeStateReleased {
			t.Errorf("expected all worktrees released after cancel, got %s id=%s", wt.State, wt.ID)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPGIntegration_RestartMidTask
// ─────────────────────────────────────────────────────────────────────────────

// Симулирует kill процесса посреди обработки task_event:
//  1. Claim'аем event (выставляется locked_by/locked_at) — это состояние воркера в момент kill.
//  2. Закрываем gorm/sql соединения (имитация выхода процесса).
//  3. Открываем новые соединения к тому же контейнеру (новый "процесс").
//  4. Сразу ClaimNext не должен видеть событие — оно ещё locked.
//  5. Вызываем ReleaseStuckLocks с cutoff=NOW (recovery-примитив) — освобождает lock.
//  6. ClaimNext снова возвращает event — recovery успешен.
//
// Реального startup-cleanup в кодовой базе пока нет (grep по ReleaseStuckLocks не
// показал её вызовов), поэтому тест проверяет именно сам примитив recovery,
// который должен быть запущен из cmd/api/main.go при старте (это будущий TODO).
func TestPGIntegration_RestartMidTask(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startPgHarness(t, nil)
	defer h.Close()

	ctx := context.Background()
	taskID := h.createMinimalActiveTask(t)

	// 1. Enqueue step_req.
	if err := h.orchestrator.EnqueueInitialStep(ctx, taskID); err != nil {
		t.Fatalf("EnqueueInitialStep: %v", err)
	}

	// 2. Claim event — имитация что воркер начал обработку.
	ev, err := h.eventRepo.ClaimNext(ctx, models.TaskEventKindStepReq, "worker-A")
	if err != nil {
		t.Fatalf("ClaimNext initial: %v", err)
	}
	if ev == nil || ev.TaskID != taskID {
		t.Fatalf("unexpected claimed event: %+v", ev)
	}

	// 3. "Kill" процесса — закрываем gorm/sql соединения. Контейнер не трогаем.
	//    После этого locked_by='worker-A' и locked_at=<10мс назад> остаются в БД.
	if err := h.sqlDB.Close(); err != nil {
		t.Logf("sql.Close warning: %v", err)
	}
	// gormDB поверх pgx pool тоже теряет соединения. Создадим новые ниже.

	// 4. "Перезапуск процесса" — открываем новый gorm/sql на том же DSN.
	sqlDB2, err := sql.Open("pgx", h.dsn)
	if err != nil {
		t.Fatalf("sql.Open after restart: %v", err)
	}
	defer func() { _ = sqlDB2.Close() }()
	gdb2, err := gorm.Open(gormpostgres.New(gormpostgres.Config{DSN: h.dsn}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open after restart: %v", err)
	}
	eventRepo2 := repository.NewTaskEventRepository(gdb2)

	// 5. Сразу ClaimNext — НЕ должен видеть event (он locked).
	if got, err := eventRepo2.ClaimNext(ctx, models.TaskEventKindStepReq, "worker-B"); err == nil {
		t.Errorf("expected ErrNoTaskEventAvailable while event is locked, got event=%+v", got)
	}

	// 6. Recovery: освобождаем locks старше now() (включая наш свежий lock).
	//    В реальной системе startup-cleanup использовал бы cutoff = now() - lockTTL;
	//    мы передаём NOW()+1s чтобы освободить ВСЕ existing locks для теста.
	released, err := eventRepo2.ReleaseStuckLocks(ctx, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("ReleaseStuckLocks: %v", err)
	}
	if released != 1 {
		t.Errorf("expected 1 lock released, got %d", released)
	}

	// 7. Теперь новый воркер забирает событие.
	revived, err := eventRepo2.ClaimNext(ctx, models.TaskEventKindStepReq, "worker-B")
	if err != nil {
		t.Fatalf("ClaimNext after recovery: %v", err)
	}
	if revived == nil || revived.ID != ev.ID {
		t.Errorf("expected revived event id=%d, got %+v", ev.ID, revived)
	}

	// 8. Task state по-прежнему active (не было финализации).
	var got struct{ State string }
	if err := gdb2.Raw(`SELECT state FROM tasks WHERE id = ?`, taskID).Scan(&got).Error; err != nil {
		t.Fatalf("read task state: %v", err)
	}
	if got.State != "active" {
		t.Errorf("expected state=active after restart, got %q", got.State)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPGIntegration_MaxStepsPerTask_NeedsHuman
// ─────────────────────────────────────────────────────────────────────────────

// Проверяет hard safety §5: при достижении MaxStepsPerTask задача переходит в
// needs_human, и последующие Step'ы — no-op.
func TestPGIntegration_MaxStepsPerTask_NeedsHuman(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	// Router всегда говорит "продолжай" — но cfg.MaxStepsPerTask=2 остановит после 2 шагов.
	// Готовим 3 ответа на всякий случай (если бы безопасность не сработала, 3-й Step
	// тоже спросил бы Router'а; тест провалится через "got router_decisions != 2").
	loopResp := `{"done": false, "agents": [{"agent": "planner"}], "reason": "keep going"}`
	cfg := service.DefaultOrchestratorConfig()
	cfg.MaxStepsPerTask = 2
	h := startPgHarnessWithOpts(t, []string{loopResp, loopResp, loopResp}, pgHarnessOpts{
		orchestratorCfg: &cfg,
	})
	defer h.Close()

	ctx := context.Background()
	taskID := h.createMinimalActiveTask(t)
	if err := h.orchestrator.EnqueueInitialStep(ctx, taskID); err != nil {
		t.Fatalf("EnqueueInitialStep: %v", err)
	}

	// Прогоняем 4 Step'а. Шаги 0 и 1 — Router-driven (current_step_no 0→1, 1→2).
	// Шаг 2 — current_step_no=2 >= MaxStepsPerTask=2 → finalize needs_human.
	// Шаги 3+ — state уже не active, ранний выход.
	for i := 0; i < 4; i++ {
		if err := h.orchestrator.Step(ctx, taskID); err != nil {
			t.Fatalf("Step %d: %v", i, err)
		}
	}

	// Проверка 1: state=needs_human.
	var got struct {
		State        string
		ErrorMessage *string `gorm:"column:error_message"`
	}
	if err := h.gormDB.Raw(
		`SELECT state, error_message FROM tasks WHERE id = ?`, taskID,
	).Scan(&got).Error; err != nil {
		t.Fatalf("read task: %v", err)
	}
	if got.State != "needs_human" {
		t.Errorf("expected state=needs_human, got %q", got.State)
	}
	if got.ErrorMessage == nil || !strings.Contains(*got.ErrorMessage, "max_steps_per_task") {
		t.Errorf("expected error_message to mention max_steps_per_task, got %v", got.ErrorMessage)
	}

	// Проверка 2: ровно 2 router_decisions (Step'ы 0 и 1; Step 2 финализирует БЕЗ Router'а).
	decisions, err := h.decisionRepo.ListByTaskID(ctx, taskID, false)
	if err != nil {
		t.Fatalf("ListByTaskID: %v", err)
	}
	if len(decisions) != 2 {
		t.Errorf("expected exactly 2 router_decisions (Steps 0 and 1), got %d", len(decisions))
	}

	// Проверка 3: дополнительные Step'ы — no-op (state не меняется).
	if err := h.orchestrator.Step(ctx, taskID); err != nil {
		t.Fatalf("Step after finalize: %v", err)
	}
	var stateAfter string
	if err := h.gormDB.Raw(`SELECT state FROM tasks WHERE id = ?`, taskID).Scan(&stateAfter).Error; err != nil {
		t.Fatalf("read task state: %v", err)
	}
	if stateAfter != "needs_human" {
		t.Errorf("state changed after finalize: %q", stateAfter)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestPGIntegration_CancelAfterDone_Returns409
// ─────────────────────────────────────────────────────────────────────────────

// Race condition: между чтением state='active' на фронте и POST /tasks/:id/cancel
// задача может перейти в терминальное состояние (worker завершил, сделал UPDATE state='done'),
// либо worker прямо сейчас держит row-lock на финализации. taskService.Cancel должен:
//   - под параллельным SELECT FOR UPDATE: получить SQLSTATE 55P03 → ErrTaskLocked →
//     ErrTaskAlreadyTerminal (HTTP 409, не 500).
//   - на уже терминальном state'е: GetByIDForUpdate отдаст task, проверка
//     isTerminalTaskState → ErrTaskAlreadyTerminal (HTTP 409, не молчаливый overwrite).
//
// Тест прогоняет оба сценария на реальном postgres — sqlite/mock не моделирует
// 55P03 для NOWAIT.
func TestPGIntegration_CancelAfterDone_Returns409(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startPgHarness(t, nil)
	defer h.Close()

	ctx := context.Background()
	taskRepo := repository.NewTaskRepository(h.gormDB)

	// ── Сценарий A: row уже terminal (state='done') ────────────────────────────
	taskID := h.createMinimalActiveTask(t)
	if err := h.gormDB.Exec(
		`UPDATE tasks SET state = 'done', completed_at = NOW(), updated_at = NOW() WHERE id = ?`, taskID,
	).Error; err != nil {
		t.Fatalf("UPDATE state=done: %v", err)
	}
	got, err := taskRepo.GetByIDForUpdate(ctx, taskID)
	if err != nil {
		t.Fatalf("GetByIDForUpdate on terminal task: %v", err)
	}
	if got.State != models.TaskStateDone {
		t.Errorf("expected state=done, got %q", got.State)
	}
	// (Сервисный слой проверит isTerminalTaskState и вернёт ErrTaskAlreadyTerminal —
	// это покрыто unit-тестами TestTaskCancel_FromTerminal. Здесь убеждаемся, что
	// сам репо корректно отдаёт залоченную row без ошибки, когда она свободна.)

	// ── Сценарий B: row занят другой транзакцией (SELECT FOR UPDATE) ───────────
	taskID2 := h.createMinimalActiveTask(t)

	// Открываем «воркер»-транзакцию через отдельный sql.Tx и держим FOR UPDATE.
	tx, err := h.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx (worker): %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	var lockedID string
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM tasks WHERE id = $1 FOR UPDATE`, taskID2,
	).Scan(&lockedID); err != nil {
		t.Fatalf("worker SELECT FOR UPDATE: %v", err)
	}

	// В это время другой ctx пытается взять lock NOWAIT — должен мгновенно отказать.
	if _, lockErr := taskRepo.GetByIDForUpdate(ctx, taskID2); !errors.Is(lockErr, repository.ErrTaskLocked) {
		t.Fatalf("expected ErrTaskLocked, got %v", lockErr)
	}

	// После COMMIT воркера lock снимается, NOWAIT снова работает.
	if err := tx.Commit(); err != nil {
		t.Fatalf("worker COMMIT: %v", err)
	}
	if _, err := taskRepo.GetByIDForUpdate(ctx, taskID2); err != nil {
		t.Fatalf("GetByIDForUpdate after lock release: %v", err)
	}
}
