//go:build featuresmoke && featuresmoke_frontend

// Package featuresmoke — Go-обёртка для Flutter integration_test.
//
// Назначение
//
// PR-gate флоу для Phase 3 (см. docs/integration-tests-plan.md):
//  1. Поднять backend через harness (FakeLLM redirect + dummy-ключи +
//     ORCHESTRATOR_V2_WORKERS_ENABLED=false), на ФИКСИРОВАННОМ порту 8080
//     (FEATURESMOKE_FORCE_PORT=8080), потому что `lib/core/api/dio_providers.dart`
//     зашит на `http://localhost:8080/api/v1` и не читает dart-define.
//  2. Сделать snapshot `SELECT COUNT(*) FROM llm_logs` ДО прогона.
//  3. Спавнить `flutter test integration_test/<files>` как subprocess
//     и стримить stdout/stderr в test-output (чтобы видеть прогресс Flutter'а).
//  4. После завершения flutter — снова прочитать count.
//  5. **Cost-leak guard**: delta == 0 для Phase 3 P0/P1 (auth/projects/team/
//     tasks/full_flow). Любой LLM-вызов из этих флоу — регрессия: фейлим
//     тест с понятным сообщением.
//
// Build tag — отдельный (`featuresmoke_frontend`), чтобы обычный
// `make test-features-backend` его НЕ подхватывал (он не должен спавнить
// flutter; Flutter — отдельная фаза в make-flow).
//
// Список Flutter-тестов задаётся переменной `FEATURESMOKE_FRONTEND_TESTS`
// (space-separated). По умолчанию — все P0/P1 файлы, без assistant_e2e
// (он намеренно жжёт LLM и идёт в отдельном таргете).
//
// Запуск:
//
//	cd backend && \
//	  FEATURESMOKE_ENABLED=1 FEATURESMOKE_FORCE_PORT=8080 \
//	  DB_HOST=localhost DB_PORT=5433 DB_USER=yugabyte DB_PASSWORD=yugabyte DB_NAME=yugabyte \
//	  go test -tags featuresmoke,featuresmoke_frontend -timeout 1800s ./test/featuresmoke/... -run TestFrontendIntegration_Phase3 -v

package featuresmoke

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	// pgx stdlib driver: тот же, что использует основной backend.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// defaultPhase3FrontendTests — P0/P1 интеграционные сценарии (Phase 3 Tasks
// 3.1–3.3 + Sprint 14.2 full_flow). assistant_e2e_test.dart **не** в этом
// списке — он специально про LLM agent-loop и требует отдельного прогона
// под guard'ом «delta == calls_to_FakeLLM», а не «delta == 0».
var defaultPhase3FrontendTests = []string{
	"integration_test/auth_flow_test.dart",
	"integration_test/projects_flow_test.dart",
	"integration_test/team_settings_test.dart",
	"integration_test/task_lifecycle_test.dart",
	"integration_test/full_flow_test.dart",
}

func frontendTestsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("FEATURESMOKE_FRONTEND_TESTS"))
	if raw == "" {
		return defaultPhase3FrontendTests
	}
	out := make([]string, 0, 8)
	for _, p := range strings.Split(raw, " ") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// frontendRoot — путь к каталогу `frontend/` из backend/.
func frontendRoot(t *testing.T) string {
	t.Helper()
	// backend/test/featuresmoke/frontend_e2e_test.go → ../../.. = repo root.
	root := filepath.Clean(filepath.Join(backendRoot(), ".."))
	fp := filepath.Join(root, "frontend", "pubspec.yaml")
	if _, err := os.Stat(fp); err != nil {
		t.Fatalf("frontendRoot: pubspec.yaml не найден в %s: %v", fp, err)
	}
	return filepath.Join(root, "frontend")
}

// dbDSN строит DSN для прямого подключения к Yugabyte/Postgres (для llm_logs
// snapshot'а). Параметры берутся из ENV — те же, что harness прокидывает
// в backend через composeEnv.
func dbDSN() string {
	host := envOr("DB_HOST", "localhost")
	port := envOr("DB_PORT", "5433")
	user := envOr("DB_USER", "yugabyte")
	pass := envOr("DB_PASSWORD", "yugabyte")
	name := envOr("DB_NAME", "yugabyte")
	sslmode := envOr("DB_SSLMODE", "disable")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, name, sslmode)
}

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dbDSN())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = db.Close() })
	if err := db.PingContext(timeoutCtx(t, 5*time.Second)); err != nil {
		t.Fatalf("ping db: %v", err)
	}
	return db
}

// timeoutCtx — короткий child-ctx; cancel вызывается через t.Cleanup, не
// сразу — иначе ctx закроется до использования.
func timeoutCtx(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func countLLMLogs(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRowContext(
		timeoutCtx(t, 5*time.Second),
		"SELECT COUNT(*) FROM llm_logs",
	).Scan(&n); err != nil {
		t.Fatalf("count llm_logs: %v", err)
	}
	return n
}

// detailLLMLogs — пишет в лог последние N записей `llm_logs` для постмортема
// (что именно «утекло»). Не падает на ошибках — best-effort.
func detailLLMLogs(t *testing.T, db *sql.DB, limit int) {
	t.Helper()
	rows, err := db.QueryContext(
		timeoutCtx(t, 5*time.Second),
		`SELECT created_at, provider, model, input_tokens, output_tokens,
		        COALESCE(error_message, '') AS err
		 FROM llm_logs ORDER BY created_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		t.Logf("detailLLMLogs: query failed (best-effort): %v", err)
		return
	}
	defer rows.Close()
	t.Logf("recent llm_logs entries (up to %d):", limit)
	for rows.Next() {
		var (
			ts                                  time.Time
			provider, model, errMsg             string
			inputTokens, outputTokens           int
		)
		if err := rows.Scan(&ts, &provider, &model, &inputTokens, &outputTokens, &errMsg); err != nil {
			t.Logf("  scan err: %v", err)
			continue
		}
		t.Logf("  %s provider=%s model=%s in=%d out=%d err=%q",
			ts.Format(time.RFC3339), provider, model, inputTokens, outputTokens, errMsg)
	}
}

// flutterBinary возвращает путь к `flutter` (через `which flutter`), либо
// фейлится с понятной ошибкой. Без полного PATH (например, GitHub Actions
// без flutter-action) тест должен явно сказать «поставь flutter», а не
// падать на «exec: flutter: not found».
func flutterBinary(t *testing.T) string {
	t.Helper()
	cmd := "flutter"
	if runtime.GOOS == "windows" {
		cmd = "flutter.bat"
	}
	path, err := exec.LookPath(cmd)
	if err != nil {
		t.Fatalf("flutter not found in PATH: %v; установите Flutter SDK", err)
	}
	return path
}

// runStreaming запускает команду со стримингом stdout/stderr в t.Log —
// тесту видно прогресс flutter test (long-running).
func runStreaming(t *testing.T, cmd *exec.Cmd) error {
	t.Helper()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	done := make(chan struct{}, 2)
	go pipeToLog(t, stdout, "stdout", done)
	go pipeToLog(t, stderr, "stderr", done)
	<-done
	<-done
	return cmd.Wait()
}

func pipeToLog(t *testing.T, r io.Reader, prefix string, done chan<- struct{}) {
	t.Helper()
	defer func() { done <- struct{}{} }()
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scan.Scan() {
		t.Logf("[flutter %s] %s", prefix, scan.Text())
	}
	if err := scan.Err(); err != nil && !errors.Is(err, io.EOF) {
		t.Logf("[flutter %s] scan err: %v", prefix, err)
	}
}

// TestFrontendIntegration_Phase3 — главный «boss»-тест для Phase 3.
//
// Что делает:
//  1. Бутстрап backend через harness (forced port 8080).
//  2. Snapshot count(llm_logs).
//  3. Прогон Flutter integration tests (perimeter Phase 3 P0/P1).
//  4. Snapshot count(llm_logs) повторно → delta строго 0, иначе FAIL.
//
// Никаких t.Parallel() — этот тест ЭКСКЛЮЗИВНО держит backend на порту 8080
// и весь Flutter-prozess; параллелить его с другими нельзя.
func TestFrontendIntegration_Phase3(t *testing.T) {
	if envOr("FEATURESMOKE_FRONTEND_FORCE_PORT_SKIP", "") != "" {
		// safety-valve на случай, если CI хочет запустить wrapper без
		// форсированного порта (например, dry-run без backend на 8080).
		// По умолчанию НЕ выставлен.
		t.Skip("FEATURESMOKE_FRONTEND_FORCE_PORT_SKIP=1 — пропуск")
	}
	if os.Getenv("FEATURESMOKE_FORCE_PORT") == "" {
		// harness уважает FEATURESMOKE_FORCE_PORT — без этой переменной
		// backend поднимется на случайном порту, и Flutter не достучится.
		t.Skip("FEATURESMOKE_FORCE_PORT не выставлен; запусти через `make test-features-frontend`")
	}

	h := StartServer(t)

	// Sanity: backend поднялся именно на http://127.0.0.1:8080 (что ожидает Flutter).
	if !strings.Contains(h.BaseURL, ":8080") {
		t.Fatalf("ожидали backend на :8080 (Flutter hardcoded), но получили %s", h.BaseURL)
	}

	db := openDB(t)

	llmBefore := countLLMLogs(t, db)
	t.Logf("llm_logs count BEFORE flutter run: %d", llmBefore)

	flutter := flutterBinary(t)
	frontDir := frontendRoot(t)

	// 1) flutter pub get + codegen — гарантируем, что .g.dart актуальны
	//    (после правок моделей может отсутствовать сборка).
	// Используем bounded timeout: codegen может идти 1-2 минуты на первой сборке.
	pubGet := exec.Command(flutter, "pub", "get")
	pubGet.Dir = frontDir
	pubGet.Env = os.Environ()
	if err := runStreaming(t, pubGet); err != nil {
		t.Fatalf("flutter pub get: %v", err)
	}

	// 2) Прогон integration_test/. Используем web-server device — он не требует
	//    macOS-entitlements и стабилен в CI. На локалке можно переопределить через
	//    FEATURESMOKE_FRONTEND_DEVICE (например, =macos).
	device := envOr("FEATURESMOKE_FRONTEND_DEVICE", "web-server")
	tests := frontendTestsFromEnv()
	args := []string{"test", "-d", device}
	args = append(args, tests...)
	t.Logf("running flutter %s (cwd=%s)", strings.Join(args, " "), frontDir)

	flutterCmd := exec.Command(flutter, args...)
	flutterCmd.Dir = frontDir
	// Прокидываем переменные окружения, чтобы Flutter-тесты могли:
	//   - FEATURESMOKE_REQUIRE_BACKEND=1 → fail вместо skip при недоступности
	//     backend (CI-контракт).
	// Также передаём API_BASE через --dart-define, чтобы в будущем, когда
	// dioClient научится его читать, тесты автоматически подхватили.
	childEnv := append(os.Environ(), "FEATURESMOKE_REQUIRE_BACKEND=1")
	flutterCmd.Env = childEnv

	// Конкатенируем dart-define после позиционных тестов — flutter test
	// принимает --dart-define где угодно после `test`.
	flutterCmd.Args = append(flutterCmd.Args,
		"--dart-define=API_BASE="+h.BaseURL)

	if err := runStreaming(t, flutterCmd); err != nil {
		// Flutter упал — посмотрим, не утекло ли в llm_logs до фейла.
		detailLLMLogs(t, db, 5)
		t.Fatalf("flutter test FAILED: %v", err)
	}

	llmAfter := countLLMLogs(t, db)
	delta := llmAfter - llmBefore
	t.Logf("llm_logs count AFTER flutter run: %d (delta=%d)", llmAfter, delta)

	// Cost-leak guard. Phase 3 P0/P1 не должен спровоцировать НИ одного
	// LLM-вызова. Если delta>0 — это утечка: либо backend пошёл на api.anthropic.com
	// с dummy-ключом (видим status=401 в `error_message`), либо тесты затянули
	// LLM-фичу (assistant chat, и т.п.) без явного мока.
	if delta != 0 {
		detailLLMLogs(t, db, 10)
		t.Fatalf("LLM cost-leak detected: llm_logs выросла на %d записей за "+
			"прогон Phase 3 integration tests. Это значит, что какой-то Flutter-"+
			"тест дернул LLM-эндпоинт. Phase 3 P0/P1 не должны вообще касаться "+
			"LLM (см. docs/integration-tests-plan.md). Проверь recent_llm_logs "+
			"выше — каждый row показывает provider/model/error.", delta)
	}
}
