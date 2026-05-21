//go:build featuresmoke

// Package featuresmoke — black-box HTTP smoke-тесты поверх живого backend.
//
// Контракт harness:
//   - StartServer(t) — поднимает один общий backend-процесс (lazy, ProcessOnce).
//     Миграции накатываются один раз, при первом старте. Все последующие тесты
//     переиспользуют этот процесс. Изоляция между тестами — Tenant-уровень:
//     уникальный user_id + project_id (UUID) per-test.
//
//   - h.NewUser(t) — регистрирует пользователя через POST /api/v1/auth/register,
//     возвращает токен. Email — `test-<uuid>@example.com`.
//
//   - h.Do(t, method, path, body, token) — JSON HTTP helper. Закрывает Body
//     самостоятельно после ReadAll; копию байтов возвращает в `Response.Body`.
//
//   - h.WS(t, token, projectID) — подключает websocket; t.Cleanup закрывает conn.
//
//   - h.FakeLLM(t) / h.FakeGit(t) — поднимают per-test stub'ы.
//     **Real-режим** (FEATURESMOKE_MODE=real) → fakes не используются,
//     backend читает реальные ключи из ENV; тесты, требующие FakeLLM, могут
//     вызывать t.Skip если key пуст.
//
// Все тесты в featuresmoke **обязаны** вызывать t.Parallel() — это часть
// Tenant-изоляции (см. docs/integration-tests-plan.md).
package featuresmoke

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/devteam/backend/test/featuresmoke/fakes"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Mode — режим прогона; настраивается через FEATURESMOKE_MODE.
type Mode string

const (
	ModeMock Mode = "mock" // default, PR-gate
	ModeReal Mode = "real" // nightly с настоящими LLM/Git
)

// Cheap-модели для тестов — экономим токены в nightly real-режиме и держим
// единую константу для mock-режима, чтобы записи в БД были консистентны.
// В mock-режиме FakeLLM перехватывает запросы и возвращает фиксированные ответы,
// поэтому имя модели влияет только на содержимое БД-записей и логов.
//
// Anthropic: Haiku 4.5 — самая дешёвая текущая Claude-модель.
// OpenAI:    gpt-4o-mini — массово используется как cheap default.
// DeepSeek:  deepseek-chat — единственная нашего интереса, уже дешёвая.
const (
	TestModelAnthropic = "claude-haiku-4-5-20251001"
	TestModelOpenAI    = "gpt-4o-mini"
	TestModelDeepSeek  = "deepseek-chat"
	// TestModelOpenRouter — после Phase 5 review (см. integration-tests-plan):
	// assistant + orchestrator + planner переехали на OpenRouter+v4-flash для
	// сокращения времени pipeline. Один и тот же ID в mock-режиме (FakeLLM
	// принимает любую модель) и в real-режиме (~$0.0000001/M tokens, дешевле
	// чем haiku в десятки раз).
	TestModelOpenRouter = "deepseek/deepseek-v4-flash"
)

// CurrentMode возвращает активный режим.
func CurrentMode() Mode {
	if strings.ToLower(strings.TrimSpace(os.Getenv("FEATURESMOKE_MODE"))) == "real" {
		return ModeReal
	}
	return ModeMock
}

// Harness — точка входа в тестовый API.
type Harness struct {
	BaseURL string
	hc      *http.Client

	// fakeLLM / fakeGit — lazy per-test, не глобальные.
	// Глобальные fakes использовать нельзя: они должны быть привязаны к *testing.T,
	// чтобы fast-fail срабатывал на правильном тесте.
	fakeOnce sync.Once
	fakeLLM  *fakes.FakeLLM
}

// User — учётка, зарегистрированная NewUser'ом.
type User struct {
	ID           string
	Email        string
	Password     string
	AccessToken  string
	RefreshToken string
}

// Response — результат Do, готовый для assert'ов. Body уже прочитан.
type Response struct {
	Status int
	Body   []byte
	Header http.Header
}

// JSON парсит Body в out (или t.Fatalf).
func (r *Response) JSON(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, out); err != nil {
		t.Fatalf("Response.JSON: %v (status=%d body=%q)", err, r.Status, truncBody(r.Body))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Server bootstrap: один shared backend на TestMain.
// ─────────────────────────────────────────────────────────────────────────────

var (
	startOnce      sync.Once
	startErr       error
	baseURL        string
	backendLogPath string

	// globalFakeLLM — поднятый ДО bootstrapServer'а fake-сервер LLM (только mock-mode).
	// Его URL прописывается в ANTHROPIC_BASE_URL / OPENAI_BASE_URL / DEEPSEEK_BASE_URL /
	// GEMINI_BASE_URL / QWEN_BASE_URL — backend ходит туда, а не на api.anthropic.com.
	// В real-режиме обе переменные остаются nil/пустыми.
	//
	// Доступен из тестов через Harness.GlobalFakeLLM() — нужен для prompt-content
	// тестов (захват реальных запросов от backend'а к LLM и assertion над их shape).
	globalFakeLLM    *fakes.FakeLLM
	globalFakeLLMURL string
)

// BackendLogPath возвращает путь к файлу stdout/stderr запущенного backend'а.
// Пустая строка — если StartServer ещё не вызывался.
// Используется secret_scrub_smoke_test'ом для grep'а на канареечные секреты.
func BackendLogPath() string { return backendLogPath }

// StartServer возвращает Harness, привязанный к единому shared backend.
// На каждый вызов сервер НЕ перезапускается; cleanup общий через TestMain.
func StartServer(t *testing.T) *Harness {
	t.Helper()
	startOnce.Do(func() {
		bu, cleanup, err := bootstrapServer()
		if err != nil {
			startErr = err
			return
		}
		baseURL = bu
		// Кешируем cleanup для TestMain'а / os.Exit.
		registerGlobalCleanup(cleanup)
		// FEATURESMOKE_BACKEND_LOG также выставляем — это контракт для secret_scrub_smoke_test.go.
		_ = os.Setenv("FEATURESMOKE_BACKEND_LOG", backendLogPath)
	})
	if startErr != nil {
		t.Skipf("featuresmoke: backend не запущен: %v", startErr)
	}
	return &Harness{
		BaseURL: baseURL,
		hc:      &http.Client{Timeout: 30 * time.Second},
	}
}

// bootstrapServer компилирует и запускает cmd/api на случайном порту.
// Возвращает baseURL и cleanup, который останавливает процесс.
//
// Параметры окружения берутся из текущего env (env-файл — забота make-таргета),
// SERVER_PORT перетирается на случайный.
//
// КРИТИЧНО (cost-leak prevention): в mock-режиме ДО запуска backend'а мы
// поднимаем глобальный FakeLLM и проставляем его URL в ANTHROPIC_BASE_URL /
// OPENAI_BASE_URL / ... — иначе backend ходит на api.anthropic.com с реальным
// ключом из родительского env и жжёт деньги на каждый orchestrator-вызов.
// См. секцию «Что пошло не так» в docs/integration-tests-plan.md (Phase 2 review).
func bootstrapServer() (string, func(), error) {
	if !envBool("FEATURESMOKE_ENABLED") {
		return "", nil, errors.New("FEATURESMOKE_ENABLED=1 не выставлен — пропускаем (запусти через `make test-features-backend`)")
	}

	// 1) Поднять FakeLLM ДО backend (только в mock-режиме). Real-режим оставляет
	// провайдерские URL'ы как есть.
	if CurrentMode() == ModeMock {
		globalFakeLLM = fakes.NewFakeLLMGlobal()
		globalFakeLLMURL = globalFakeLLM.URL()
		registerGlobalCleanup(globalFakeLLM.Close)
	}

	port, err := pickServerPort()
	if err != nil {
		return "", nil, fmt.Errorf("pickServerPort: %w", err)
	}

	binPath, err := buildBinary()
	if err != nil {
		return "", nil, fmt.Errorf("buildBinary: %w", err)
	}

	cmd := exec.Command(binPath)
	cmd.Env = composeEnv(port)
	cmd.Dir = backendRoot()
	logFile, err := os.CreateTemp("", "featuresmoke-backend-*.log")
	if err != nil {
		return "", nil, fmt.Errorf("create log: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// Setpgid (только Unix) ставится в harness_unix.go — чтобы killTree мог
	// сигналить всю process-group. На Windows такого механизма нет, поэтому
	// fallback — обычный cmd.Process.Kill (см. harness_windows.go).
	configureSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return "", nil, fmt.Errorf("start backend: %w", err)
	}

	bu := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitHealth(bu, 60*time.Second); err != nil {
		_ = killTree(cmd)
		_ = logFile.Close()
		return "", nil, fmt.Errorf("backend не отвечает на /health: %w (log: %s)", err, logFile.Name())
	}

	backendLogPath = logFile.Name()
	cleanup := func() {
		_ = killTree(cmd)
		_ = logFile.Close()
		// FEATURESMOKE_KEEP_LOG=1 — оставить файл для пост-мортема.
		if !envBool("FEATURESMOKE_KEEP_LOG") {
			// Удаляем temp-файл: при активной TDD-локалке /tmp иначе быстро
			// разрастается до сотен МБ за десяток прогонов.
			_ = os.Remove(logFile.Name())
		}
	}
	return bu, cleanup, nil
}

func composeEnv(port int) []string {
	// Базовый env родителя + перетёртые SERVER_PORT и доп. параметры.
	parent := os.Environ()
	overrides := map[string]string{
		"SERVER_HOST":            "127.0.0.1",
		"SERVER_PORT":            fmt.Sprintf("%d", port),
		"DB_HOST":                envOr("DB_HOST", "localhost"),
		"DB_PORT":                envOr("DB_PORT", "5433"),
		"DB_USER":                envOr("DB_USER", "yugabyte"),
		"DB_PASSWORD":            envOr("DB_PASSWORD", "yugabyte"),
		"DB_NAME":                envOr("DB_NAME", "yugabyte"),
		"DB_SSLMODE":             envOr("DB_SSLMODE", "disable"),
		"WS_ALLOWED_ORIGINS":     envOr("WS_ALLOWED_ORIGINS", "*"),
		"JWT_SECRET_KEY":         envOr("JWT_SECRET_KEY", "featuresmoke-jwt-secret-1234567890abcdef"),
		"WORKFLOW_WORKER_ENABLED": envOr("WORKFLOW_WORKER_ENABLED", "false"),
		// Дешёвые модели по умолчанию (см. TestModel* выше). В nightly real-режиме
		// экономит токены; в mock-режиме просто держит консистентные имена.
		// Пользовательский ANTHROPIC_MODEL/OPENAI_MODEL/DEEPSEEK_MODEL не перетираем —
		// envOr оставит то, что выставил вызывающий.
		"ANTHROPIC_MODEL": envOr("ANTHROPIC_MODEL", TestModelAnthropic),
		"OPENAI_MODEL":    envOr("OPENAI_MODEL", TestModelOpenAI),
		"DEEPSEEK_MODEL":  envOr("DEEPSEEK_MODEL", TestModelDeepSeek),
		// PR-gate смоук проверяет CRUD/API-контракт, не реальный pipeline →
		// отключаем v2 step/agent воркеры. Без этого они на 500ms-интервале конкурируют
		// с пользовательскими pause/cancel/correct: либо успевают перевести задачу в
		// failed (нет LLM-ключа) до того, как мы её паузим (409 task_already_terminal),
		// либо ловят серию SQLSTATE 40001 (YugabyteDB snapshot-isolation read-restart),
		// перегружая retry-бюджет. В real-режиме (FEATURESMOKE_MODE=real) флаг не
		// выставляется — там воркеры нужны для прогона полного pipeline.
		"ORCHESTRATOR_V2_WORKERS_ENABLED": envOr("ORCHESTRATOR_V2_WORKERS_ENABLED",
			func() string {
				if CurrentMode() == ModeReal {
					return "true"
				}
				return "false"
			}()),
		"ENV":             "test",
		"ADMIN_EMAIL":     envOr("ADMIN_EMAIL", "admin@example.com"),
		"ADMIN_PASSWORD":  envOr("ADMIN_PASSWORD", "admin-featuresmoke-password-123"),
	}

	// КРИТИЧНО (cost-leak prevention): в mock-режиме редиректим все провайдерские
	// base_url на FakeLLM и подсовываем dummy-ключи. Без этого ЛЮБОЙ LLM-вызов
	// от backend'а (orchestrator workers / assistant / direct chat) пойдёт на
	// реальный api.anthropic.com / api.openai.com с настоящим ключом из родительского
	// env и сожжёт деньги. Real-режим (FEATURESMOKE_MODE=real) НЕ перетирает —
	// тесты должны идти к настоящим API.
	if CurrentMode() == ModeMock && globalFakeLLMURL != "" {
		overrides["ANTHROPIC_BASE_URL"] = globalFakeLLMURL
		overrides["OPENAI_BASE_URL"] = globalFakeLLMURL + "/v1"
		overrides["DEEPSEEK_BASE_URL"] = globalFakeLLMURL + "/v1"
		overrides["GEMINI_BASE_URL"] = globalFakeLLMURL
		overrides["QWEN_BASE_URL"] = globalFakeLLMURL + "/v1"
		// OpenRouter (Phase 5 review): без редиректа backend на openrouter ходил
		// бы на реальный openrouter.ai с fake-key → 401 → llm_logs grow → frontend
		// cost-leak guard падает. FakeLLM матчит по суффиксу `*/chat/completions`,
		// поэтому /api/v1/chat/completions из OpenRouter-клиента попадает в OpenAI-
		// ветку handler'а автоматически.
		overrides["OPENROUTER_BASE_URL"] = globalFakeLLMURL + "/api/v1"
		// Dummy-ключи. Без них config.go.createProvider не зарегистрирует провайдеров
		// (`if pCfg.APIKey != ""`). Префикс «fake-» делает их распознаваемыми в логах
		// и грепе. РОДИТЕЛЬСКИЕ настоящие ключи перетираются — это и есть защита.
		overrides["ANTHROPIC_API_KEY"] = "fake-anthropic-featuresmoke-key"
		overrides["OPENAI_API_KEY"] = "fake-openai-featuresmoke-key"
		overrides["DEEPSEEK_API_KEY"] = "fake-deepseek-featuresmoke-key"
		overrides["GEMINI_API_KEY"] = "fake-gemini-featuresmoke-key"
		overrides["QWEN_API_KEY"] = "fake-qwen-featuresmoke-key"
		overrides["OPENROUTER_API_KEY"] = "fake-openrouter-featuresmoke-key"
	}
	// Слияние: overrides перетирают parent.
	idx := make(map[string]int, len(parent))
	for i, kv := range parent {
		k := kv
		if eq := strings.IndexByte(kv, '='); eq >= 0 {
			k = kv[:eq]
		}
		idx[k] = i
	}
	for k, v := range overrides {
		entry := k + "=" + v
		if i, ok := idx[k]; ok {
			parent[i] = entry
		} else {
			parent = append(parent, entry)
		}
	}
	return parent
}

// buildBinary компилирует cmd/api один раз за тестовый процесс.
// Возвращает путь к временному бинарю; удаляется в TestMain-cleanup.
var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

func buildBinary() (string, error) {
	buildOnce.Do(func() {
		// MkdirTemp + filepath.Join вместо CreateTemp + Remove: убираем
		// TOCTOU-окно, в которое другой процесс мог занять освобождённое имя.
		// Директория эксклюзивна для нас, бинарник внутри — детерминированное
		// имя.
		dir, err := os.MkdirTemp("", "featuresmoke-bin-*")
		if err != nil {
			buildErr = err
			return
		}
		path := filepath.Join(dir, "api")
		if runtime.GOOS == "windows" {
			path += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", path, "./cmd/api")
		cmd.Dir = backendRoot()
		cmd.Env = os.Environ()
		raw, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = fmt.Errorf("go build: %w\n%s", err, string(raw))
			_ = os.RemoveAll(dir)
			return
		}
		binPath = path
		registerGlobalCleanup(func() { _ = os.RemoveAll(dir) })
	})
	return binPath, buildErr
}

// backendRoot — корень backend-модуля (директория с go.mod).
// Определяем относительно текущего файла (test/featuresmoke/harness.go).
func backendRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func waitHealth(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	hc := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := hc.Get(baseURL + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status=%d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

// freePort находит свободный порт через bind на :0 + Close.
//
// TODO: TOCTOU vulnerability — между l.Close() и cmd.Start()+bind проходит
// окно в несколько миллисекунд, в которое другой процесс (особенно в CI с
// высокой параллельностью) может занять этот порт → flaky tests.
//
// Правильное решение: SERVER_PORT=0 на стороне backend'а + чтение реального
// порта из stdout/специального файла. Переделка cmd/api под это сейчас вне
// scope Phase 1 (затрагивает server.Start, который форсит конкретный порт).
// При наблюдении flakes — поднять в Phase 5 (CI/CD).
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// pickServerPort выбирает порт для backend'а:
//   - если выставлен `FEATURESMOKE_FORCE_PORT` (например, 8080) — используем его;
//     это нужно для Flutter integration_test, где `lib/core/api/dio_providers.dart`
//     зашит на `http://localhost:8080/api/v1` и не читает dart-define.
//   - иначе — случайный свободный порт (поведение по умолчанию).
//
// Проверяем, что forced-port реально свободен: если нет — фейлимся явно
// (ошибка «port busy» лучше, чем тихое падение Flutter-теста на TCP-RST).
func pickServerPort() (int, error) {
	if raw := strings.TrimSpace(os.Getenv("FEATURESMOKE_FORCE_PORT")); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil || p <= 0 || p > 65535 {
			return 0, fmt.Errorf("FEATURESMOKE_FORCE_PORT=%q: not a valid port", raw)
		}
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			return 0, fmt.Errorf("port %d is busy (FEATURESMOKE_FORCE_PORT): %w; "+
				"остановите wibe_backend (`docker compose down app`) или выберите другой порт",
				p, err)
		}
		_ = l.Close()
		return p, nil
	}
	return freePort()
}

// killTree — graceful shutdown через os.Interrupt (SIGINT на Unix, CTRL_BREAK
// на Windows реализован cmd.Process.Kill'ом — там Interrupt не поддерживается),
// затем жёсткий cmd.Process.Kill через 5 секунд.
//
// На Unix процесс стартует с Setpgid=true (см. harness_unix.go), но мы НЕ шлём
// сигнал процесс-группе через syscall.Kill(-pid) — это код, который не
// компилируется на Windows. Кроссплатформенный cmd.Process.Kill убивает
// только корневой процесс, но cmd/api сам останавливает воркеры в SIGINT-handler'е
// (см. cmd/api/main.go: signal.Notify + waitGroup), так что детям время уходит
// штатно.
func killTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// На Windows os.Interrupt не имплементирован для Process.Signal —
	// сразу пойдём через Kill.
	if runtime.GOOS != "windows" {
		_ = cmd.Process.Signal(os.Interrupt)
	}
	// done — буферизованный канал; горутина cmd.Wait() не залипнет на send
	// даже если мы не дождёмся её здесь.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		// ⚠ Намеренно НЕ ждём второй раз <-done. Если процесс ушёл в zombie
		// (D-state в ядре), повторный wait может зависнуть навсегда → весь
		// TestMain застрянет на cleanup. Лучше оставить горутину висеть в фоне
		// (буферизованный канал не даст утечь сверх одной горутины), чем
		// заблокировать весь suite.
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Глобальный cleanup. Регистрируется через registerGlobalCleanup; вызывается
// из TestMain.
// ─────────────────────────────────────────────────────────────────────────────

var (
	globalCleanups   []func()
	globalCleanupsMu sync.Mutex
)

func registerGlobalCleanup(fn func()) {
	globalCleanupsMu.Lock()
	globalCleanups = append(globalCleanups, fn)
	globalCleanupsMu.Unlock()
}

// RunGlobalCleanup вызывает все зарегистрированные cleanup'ы.
// Использовать только из TestMain — Go-runtime гарантирует, что после m.Run()
// активных тестов уже нет.
func RunGlobalCleanup() {
	globalCleanupsMu.Lock()
	fns := globalCleanups
	globalCleanups = nil
	globalCleanupsMu.Unlock()
	// LIFO — сначала kill процесса, потом удаление бинарника.
	for i := len(fns) - 1; i >= 0; i-- {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		done := make(chan struct{})
		go func(f func()) {
			// defer гарантирует close(done) даже если f() запаникует
			// (удалить залоченный файл, разыменовать nil и т.п.). Без этого
			// select висел бы все 5 секунд на каждом падающем cleanup.
			defer close(done)
			f()
		}(fns[i])
		select {
		case <-done:
		case <-ctx.Done():
		}
		cancel()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP helpers.
// ─────────────────────────────────────────────────────────────────────────────

// Do делает HTTP-запрос к backend. body может быть nil / []byte / любой
// JSON-сериализуемый тип. token (если не пустой) — Bearer Authorization.
func (h *Harness) Do(t *testing.T, method, path string, body any, token string) *Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		switch v := body.(type) {
		case []byte:
			reader = bytes.NewReader(v)
		case string:
			reader = strings.NewReader(v)
		default:
			raw, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("Harness.Do: marshal body: %v", err)
			}
			reader = bytes.NewReader(raw)
		}
	}
	url := h.BaseURL + path
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("Harness.Do: new request: %v", err)
	}
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := h.hc.Do(req)
	if err != nil {
		t.Fatalf("Harness.Do: %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Harness.Do: read body: %v", err)
	}
	return &Response{Status: resp.StatusCode, Body: raw, Header: resp.Header.Clone()}
}

// NewUser создаёт уникального пользователя через POST /auth/register.
// Email = `test-<uuid>@example.com`, password ≥16 символов.
// Не делает t.Parallel — это ответственность теста.
func (h *Harness) NewUser(t *testing.T) User {
	t.Helper()
	email := fmt.Sprintf("test-%s@example.com", uuid.NewString())
	password := "Pass-" + uuid.NewString() // ≥36 символов
	resp := h.Do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	if resp.Status != http.StatusCreated && resp.Status != http.StatusOK {
		t.Fatalf("NewUser: register failed: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("NewUser: decode response: %v body=%s", err, truncBody(resp.Body))
	}
	if out.AccessToken == "" {
		// Часть API возвращает только токены без user объекта — попробуем альтернативный shape.
		var alt struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.Unmarshal(resp.Body, &alt)
		out.AccessToken = alt.AccessToken
		out.RefreshToken = alt.RefreshToken
	}
	if out.AccessToken == "" {
		t.Fatalf("NewUser: пустой access_token в ответе: %s", truncBody(resp.Body))
	}
	user := User{
		ID:           out.User.ID,
		Email:        email,
		Password:     password,
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
	}
	// /auth/register не возвращает user.id — вытаскиваем его через /auth/me
	// (нужен тестам, которые сравнивают user-id в payload'ах).
	if user.ID == "" {
		meResp := h.Do(t, "GET", "/api/v1/auth/me", nil, user.AccessToken)
		if meResp.Status == http.StatusOK {
			var me struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(meResp.Body, &me)
			user.ID = me.ID
		}
	}
	return user
}

// AdminUser водит админа через POST /api/v1/auth/login.
func (h *Harness) AdminUser(t *testing.T) User {
	t.Helper()
	email := envOr("ADMIN_EMAIL", "admin@example.com")
	password := envOr("ADMIN_PASSWORD", "admin-featuresmoke-password-123")
	resp := h.Do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	if resp.Status != http.StatusOK {
		t.Fatalf("AdminUser: login failed: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		t.Fatalf("AdminUser: decode response: %v body=%s", err, truncBody(resp.Body))
	}
	if out.AccessToken == "" {
		t.Fatalf("AdminUser: пустой access_token в ответе: %s", truncBody(resp.Body))
	}
	user := User{
		ID:           out.User.ID,
		Email:        email,
		Password:     password,
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
	}
	if user.ID == "" {
		meResp := h.Do(t, "GET", "/api/v1/auth/me", nil, user.AccessToken)
		if meResp.Status == http.StatusOK {
			var me struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(meResp.Body, &me)
			user.ID = me.ID
		}
	}
	return user
}

// WS подключается к /api/v1/projects/:id/ws с Bearer-токеном.
// t.Cleanup закрывает соединение.
func (h *Harness) WS(t *testing.T, token, projectID string) *websocket.Conn {
	t.Helper()
	u, err := url.Parse(h.BaseURL)
	if err != nil {
		t.Fatalf("WS: parse base url: %v", err)
	}
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/api/v1/projects/%s/ws", scheme, u.Host, projectID)
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("WS: dial %s: %v", wsURL, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// FakeLLM возвращает per-test (lazy) стаб LLM. В real-режиме fakes не используются.
// Чтобы перевести backend на этот стаб — выставь его URL в env *до* StartServer.
func (h *Harness) FakeLLM(t *testing.T) *fakes.FakeLLM {
	t.Helper()
	if CurrentMode() == ModeReal {
		t.Skip("FakeLLM недоступен в real-режиме (используются настоящие провайдеры)")
	}
	h.fakeOnce.Do(func() {
		h.fakeLLM = fakes.NewFakeLLM(t)
	})
	return h.fakeLLM
}

// GlobalFakeLLM возвращает FakeLLM, поднятый ДО bootstrapServer'а (один на пакет).
// На него уже редиректнуты ANTHROPIC_BASE_URL/OPENAI_BASE_URL/... — любой LLM-вызов
// backend'а (assistant, /llm/chat, MCP, оркестратор если включены воркеры) попадёт
// в его Calls(). Используется prompt-content смоук-тестами для assertion'ов
// над shape реальных запросов.
//
// В real-режиме nil — тесты должны t.Skip самостоятельно (или использовать FakeLLM(t)).
func (h *Harness) GlobalFakeLLM(t *testing.T) *fakes.FakeLLM {
	t.Helper()
	if CurrentMode() == ModeReal {
		t.Skip("GlobalFakeLLM недоступен в real-режиме (используются настоящие провайдеры)")
	}
	if globalFakeLLM == nil {
		t.Fatal("GlobalFakeLLM: nil — StartServer не был вызван или mock-режим не активен")
	}
	return globalFakeLLM
}

// FakeGit возвращает клиента к локальному Gitea.
// В real-режиме fakes не используются — тесты должны t.Skip самостоятельно.
func (h *Harness) FakeGit(t *testing.T) *fakes.FakeGit {
	t.Helper()
	if CurrentMode() == ModeReal {
		t.Skip("FakeGit недоступен в real-режиме (используется настоящий GitHub)")
	}
	return fakes.NewFakeGit(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared assertion helpers для смоук-тестов.
//
// Назначение: убрать копипаст «список ручек → проверить 401/403» из
// prompts/workflows/api-keys/assistant и т.п. Каждая ручка — это маленький
// контракт; набор контрактов крутится в одном месте, новые ручки добавляются
// одной строкой в slice (см. review Phase 4 §3 DRY).
// ─────────────────────────────────────────────────────────────────────────────

// EndpointCase описывает один эндпоинт для assert-хелперов. `Name` опционален —
// если задан, идёт в имя sub-test'а (parallel-friendly диагностика при
// падениях). Если пуст, имя строится из method+path.
type EndpointCase struct {
	Name   string
	Method string
	Path   string
	Body   any
}

// caseName возвращает стабильное имя для sub-test'а — без пробелов, чтобы
// `go test -run …/<name>` работал без экранирования.
func (ec EndpointCase) caseName() string {
	if ec.Name != "" {
		return ec.Name
	}
	// Приводим path к ascii-safe виду: «/api/v1/foo» → «api_v1_foo».
	safe := strings.NewReplacer(
		"/", "_",
		"-", "_",
		":", "_",
	).Replace(strings.TrimPrefix(ec.Path, "/"))
	return ec.Method + "_" + safe
}

// AssertRequiresAuth проверяет, что без Authorization-токена каждая ручка
// отвечает 401. Запускает по sub-test'у на кейс — при падении видно,
// какая именно ручка пропустила запрос.
//
// Использование:
//
//	h.AssertRequiresAuth(t, []EndpointCase{
//	    {Method: "GET",  Path: "/api/v1/prompts"},
//	    {Method: "POST", Path: "/api/v1/prompts", Body: map[string]any{"name":"x"}},
//	})
func (h *Harness) AssertRequiresAuth(t *testing.T, cases []EndpointCase) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.caseName()+"_no_token_returns_401", func(t *testing.T) {
			t.Parallel()
			resp := h.Do(t, tc.Method, tc.Path, tc.Body, "")
			if resp.Status != http.StatusUnauthorized {
				t.Fatalf("%s %s no token: status=%d (ожидали 401) body=%s",
					tc.Method, tc.Path, resp.Status, truncBody(resp.Body))
			}
		})
	}
}

// AssertRequiresAdmin проверяет, что обычный (не-admin) пользователь
// получает 403 на каждой из ручек, защищённых middleware.AdminOnlyMiddleware().
// Запускает по sub-test'у на кейс — диагностика как у AssertRequiresAuth.
//
// `token` — access_token обычного юзера; вызывающий код обязан передать
// валидный токен, иначе фейл будет 401 (auth-middleware срабатывает раньше
// admin-middleware), и сообщение будет misleading.
func (h *Harness) AssertRequiresAdmin(t *testing.T, token string, cases []EndpointCase) {
	t.Helper()
	if token == "" {
		t.Fatalf("AssertRequiresAdmin: token пуст — кейс деградирует в auth-check")
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.caseName()+"_as_user_returns_403", func(t *testing.T) {
			t.Parallel()
			resp := h.Do(t, tc.Method, tc.Path, tc.Body, token)
			if resp.Status != http.StatusForbidden {
				t.Fatalf("%s %s as non-admin: status=%d (ожидали 403) body=%s",
					tc.Method, tc.Path, resp.Status, truncBody(resp.Body))
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Utils.
// ─────────────────────────────────────────────────────────────────────────────

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}

func envBool(name string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	return v == "1" || v == "true" || v == "yes"
}

func truncBody(b []byte) string {
	const max = 500
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + fmt.Sprintf("...(+%d bytes)", len(b)-max)
}

// ConfigureUserAssistant sets up LLM credentials for the user and configures their assistant agent
// with a provider and model, so that LLM-based actions (like assistant chat) can proceed.
func (h *Harness) ConfigureUserAssistant(t *testing.T, user User, providerKind, model string) {
	t.Helper()

	// 1. PATCH /api/v1/me/llm-credentials to set the fake API key for the provider
	credField := ""
	switch providerKind {
	case "openai":
		credField = "openai_api_key"
	case "anthropic":
		credField = "anthropic_api_key"
	case "gemini":
		credField = "gemini_api_key"
	case "deepseek":
		credField = "deepseek_api_key"
	case "qwen":
		credField = "qwen_api_key"
	case "openrouter":
		credField = "openrouter_api_key"
	default:
		t.Fatalf("ConfigureUserAssistant: unknown provider_kind %q", providerKind)
	}

	fakeKey := fmt.Sprintf("sk-%s-featuresmoke-test-fake-key-must-be-long-enough", providerKind)
	patchResp := h.Do(t, "PATCH", "/api/v1/me/llm-credentials", map[string]any{
		credField: fakeKey,
	}, user.AccessToken)
	if patchResp.Status != http.StatusOK {
		t.Fatalf("ConfigureUserAssistant: patch credentials failed: status=%d body=%s",
			patchResp.Status, truncBody(patchResp.Body))
	}

	// 2. GET /api/v1/me/agents to retrieve assistant's ID
	listResp := h.Do(t, "GET", "/api/v1/me/agents", nil, user.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("ConfigureUserAssistant: list agents failed: status=%d body=%s",
			listResp.Status, truncBody(listResp.Body))
	}

	var list struct {
		Items []struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body, &list); err != nil {
		t.Fatalf("ConfigureUserAssistant: decode list: %v body=%s", err, truncBody(listResp.Body))
	}

	var assistantID string
	for _, item := range list.Items {
		if item.Role == "assistant" {
			assistantID = item.ID
			break
		}
	}
	if assistantID == "" {
		t.Fatalf("ConfigureUserAssistant: assistant agent not found in list")
	}

	// 3. PUT /api/v1/me/agents/:id to configure provider_kind and model
	putResp := h.Do(t, "PUT", "/api/v1/me/agents/"+assistantID, map[string]any{
		"provider_kind": providerKind,
		"model":         model,
	}, user.AccessToken)
	if putResp.Status != http.StatusOK {
		t.Fatalf("ConfigureUserAssistant: configure assistant agent failed: status=%d body=%s",
			putResp.Status, truncBody(putResp.Body))
	}
}
