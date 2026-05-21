//go:build integration

package service_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcwait "github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/datatypes"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/devteam/backend/internal/handler"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/internal/seed"
	"github.com/devteam/backend/pkg/llm"
)

// assistant_pg_integration_test.go — Sprint 21 §Verification (docs/tasks/21-assistant-sidebar.md).
//
// E2E-сценарии для Assistant Sidebar: реальный Postgres (testcontainers) +
// httptest-сервер + scripted LLM + scripted MCP-каталог. Покрывают полную
// связку handler → service → repository → agentloop → LLM-mock → tool-mock →
// БД и WS-broadcast. Никаких unit-моков на уровне service/repo — только на
// границах процесса (LLM API, MCP tool execute).
//
// Build tag `integration` — не собирается в `make test-unit`. Запуск:
//   go test -tags integration ./internal/service/ -run TestAssistantE2E -v
// требует Docker (testcontainers поднимет postgres:16-alpine).

// ─────────────────────────────────────────────────────────────────────────────
// Harness
// ─────────────────────────────────────────────────────────────────────────────

type assistantHarness struct {
	t         *testing.T
	container *tcpostgres.PostgresContainer
	gormDB    *gorm.DB
	sqlDB     *sql.DB

	repo      repository.AssistantSessionRepository
	taskRepo  repository.TaskRepository
	hub       *recordingHub
	llm       *scriptedLLMClient
	catalog   *scriptedCatalog
	svc       service.AssistantService

	server  *httptest.Server
	userID  uuid.UUID
	cleanup []func()
}

func (h *assistantHarness) Close() {
	for i := len(h.cleanup) - 1; i >= 0; i-- {
		h.cleanup[i]()
	}
}

func startAssistantHarness(t *testing.T) *assistantHarness {
	t.Helper()
	ctx := context.Background()

	// postgres:15-alpine — намеренно фиксируем версию, отличную от orchestrator_pg_integration_test
	// (там 16-alpine). Никакой Postgres-16-специфики у миграций assistant нет (jsonb/partial
	// indexes/UNIQUE-CHECK поддерживаются с 9.x). Снижаем требования к окружению — image,
	// который уже в кэше у разработчиков с других тестов, не качается заново.
	pgC, err := tcpostgres.Run(ctx,
		"postgres:15-alpine",
		tcpostgres.WithDatabase("devteam_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			tcwait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "start postgres container")

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	sqlDB, err := sql.Open("pgx", dsn)
	require.NoError(t, err)

	migrationsDir := findAssistantMigrationsDir(t)
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, migrationsDir), "goose up")

	gdb, err := gorm.Open(gormpostgres.New(gormpostgres.Config{DSN: dsn}), &gorm.Config{})
	require.NoError(t, err)

	// Seed: создаём пользователя и per-user agent role='assistant'.
	userID := seedTestUser(t, gdb)
	logger := slog.New(logging.NewHandler(slog.NewTextHandler(io.Discard, nil)))
	_ = logger
	require.NoError(t, seed.SeedRolePrompts(ctx, gdb, nil), "seed role prompts")
	agentSvc := service.NewAgentService(
		repository.NewAgentRepository(gdb),
		repository.NewAgentSecretRepository(gdb),
		service.NoopEncryptor{},
		repository.NewTransactionManager(gdb),
	)
	agentSvc.WithRolePromptRepo(repository.NewAgentRolePromptRepository(gdb)).
		WithApiKeyRepo(repository.NewApiKeyRepository(gdb))
	require.NoError(t, agentSvc.CreateDefaultAssistant(ctx, userID), "create assistant")

	h := &assistantHarness{
		t:         t,
		container: pgC,
		gormDB:    gdb,
		sqlDB:     sqlDB,
		userID:    userID,
		cleanup: []func(){
			func() { _ = sqlDB.Close() },
			func() { _ = pgC.Terminate(ctx) },
		},
	}

	h.repo = repository.NewAssistantSessionRepository(gdb)
	h.taskRepo = repository.NewTaskRepository(gdb)
	h.hub = newRecordingHub()
	h.llm = newScriptedLLMClient()
	h.catalog = newScriptedCatalog()

	exec := agentloop.NewExecutor(agentloop.Config{
		MaxIterations:      service.AssistantMaxIterations,
		MaxToolResultBytes: service.AssistantMaxToolResultBytes,
		MaxHistoryBytes:    service.AssistantMaxHistoryBytes,
		HistoryTailKeep:    service.AssistantHistoryTailKeep,
		// Per-call timeout в тестах ставим короче, чтобы fail-тесты не вешали билд.
		PerLLMCallTimeout: 5 * time.Second,
	}, logger)

	svc, err := service.NewAssistantService(service.AssistantServiceDeps{
		Repo:         h.repo,
		TaskRepo:     h.taskRepo,
		AgentLoader:  service.NewDBAgentLoader(gdb),
		AgentCreator: agentSvc,
		LLMResolver:  fixedLLMResolver{client: h.llm},
		ToolCatalog:  h.catalog,
		Hub:          h.hub,
		Executor:     exec,
		Logger:       logger,
	})
	require.NoError(t, err, "NewAssistantService")
	h.svc = svc

	// HTTP server.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	hh := handler.NewAssistantHandler(svc)
	// Маленький in-test «middleware»: ставим userID, как делает auth-слой.
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", string(models.RoleUser))
		c.Next()
	})
	g := r.Group("/api/v1/assistant")
	{
		g.POST("/sessions", hh.CreateSession)
		g.GET("/sessions", hh.ListSessions)
		g.GET("/sessions/:id", hh.GetSession)
		g.DELETE("/sessions/:id", hh.ArchiveSession)
		g.GET("/sessions/:id/messages", hh.GetMessages)
		g.POST("/sessions/:id/messages", hh.SendMessage)
		g.POST("/sessions/:id/confirm", hh.ConfirmToolCall)
		g.GET("/active-tasks", hh.ListActiveTasks)
	}
	h.server = httptest.NewServer(r)
	h.cleanup = append(h.cleanup, func() { h.server.Close() })
	return h
}

func findAssistantMigrationsDir(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	// Восходим до backend/db/migrations.
	for dir := wd; dir != "/"; dir = parentDir(dir) {
		for _, sub := range []string{
			"db/migrations",
			"backend/db/migrations",
		} {
			candidate := joinPath(dir, sub)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate
			}
		}
	}
	t.Fatalf("migrations dir not found from %s", wd)
	return ""
}

func parentDir(p string) string {
	// Без strings.LastIndex для path: используем strings, чтобы не тащить filepath
	// (он есть в orchestrator_pg_integration_test.go — берём оттуда semantics).
	idx := strings.LastIndex(p, "/")
	if idx <= 0 {
		return "/"
	}
	return p[:idx]
}

func joinPath(a, b string) string {
	if strings.HasSuffix(a, "/") {
		return a + b
	}
	return a + "/" + b
}

func seedTestUser(t *testing.T, gdb *gorm.DB) uuid.UUID {
	t.Helper()
	var row struct{ ID string }
	err := gdb.Raw(`
		INSERT INTO users (id, email, password_hash, role, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, 'x', 'user', NOW(), NOW())
		RETURNING id`,
		fmt.Sprintf("assistant-e2e-%s@test", uuid.NewString()[:8]),
	).Scan(&row).Error
	require.NoError(t, err)
	return uuid.MustParse(row.ID)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test doubles for process boundaries
// ─────────────────────────────────────────────────────────────────────────────

// recordingHub — реализует service.WSBroadcaster. Записывает все
// SendToUser-вызовы, чтобы тесты проверили эмиссию событий.
type recordingHub struct {
	mu     sync.Mutex
	events []hubEvent
}

type hubEvent struct {
	UserID  string
	Type    string
	Payload []byte
}

func newRecordingHub() *recordingHub { return &recordingHub{} }

func (h *recordingHub) SendToUser(userID, msgType string, payload []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, hubEvent{UserID: userID, Type: msgType, Payload: append([]byte(nil), payload...)})
	return nil
}

func (h *recordingHub) Snapshot() []hubEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]hubEvent, len(h.events))
	copy(out, h.events)
	return out
}

func (h *recordingHub) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = nil
}

// scriptedLLMClient — реализует llm.Client. Каждый Chat-вызов выдаёт
// следующий ответ из очереди. Pause-канал позволяет тесту «придерживать»
// LLM, чтобы воспроизвести busy/timeout-сценарии.
type scriptedLLMClient struct {
	mu        sync.Mutex
	responses []*llm.Response
	calls     int
	pauseCh   chan struct{} // если != nil, Chat блокируется до закрытия канала
}

func newScriptedLLMClient() *scriptedLLMClient { return &scriptedLLMClient{} }

func (c *scriptedLLMClient) SetResponses(responses ...*llm.Response) {
	c.mu.Lock()
	c.responses = responses
	c.calls = 0
	c.mu.Unlock()
}

func (c *scriptedLLMClient) SetPause(ch chan struct{}) {
	c.mu.Lock()
	c.pauseCh = ch
	c.mu.Unlock()
}

func (c *scriptedLLMClient) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

func (c *scriptedLLMClient) Chat(ctx context.Context, req llm.Request) (*llm.Response, error) {
	c.mu.Lock()
	pause := c.pauseCh
	c.mu.Unlock()

	if pause != nil {
		select {
		case <-pause:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls >= len(c.responses) {
		return nil, fmt.Errorf("scriptedLLMClient: out of responses (call %d)", c.calls)
	}
	out := c.responses[c.calls]
	c.calls++
	return out, nil
}

func (c *scriptedLLMClient) Embed(_ context.Context, _ llm.EmbedRequest) (*llm.EmbedResponse, error) {
	return nil, fmt.Errorf("Embed: not implemented in scriptedLLMClient")
}

func (c *scriptedLLMClient) HealthCheck(_ context.Context) error  { return nil }
func (c *scriptedLLMClient) ResolveBaseURL() string                { return "http://scripted-llm.test" }

// fixedLLMResolver — реализует service.AssistantLLMClientResolver. Игнорирует
// agent.ProviderKind и всегда возвращает scripted-клиента.
type fixedLLMResolver struct {
	client llm.Client
}

func (r fixedLLMResolver) ResolveAssistantClient(_ context.Context, _ *models.Agent, _ uuid.UUID) (llm.Client, error) {
	return r.client, nil
}

// scriptedCatalog — реализует service.AssistantToolCatalogProvider. Тесты
// объявляют ровно те tools, что нужны сценарию: handler — это func, который
// возвращает заранее заданный JSON.
type scriptedCatalog struct {
	mu    sync.Mutex
	tools []agentloop.Tool
	calls map[string]int
}

func newScriptedCatalog() *scriptedCatalog {
	return &scriptedCatalog{calls: make(map[string]int)}
}

func (c *scriptedCatalog) Catalog() []agentloop.Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]agentloop.Tool, len(c.tools))
	copy(out, c.tools)
	return out
}

func (c *scriptedCatalog) Register(name string, requiresConfirmation bool, result []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tools = append(c.tools, agentloop.Tool{
		Name:                 name,
		Description:          "test tool " + name,
		InputSchema:          json.RawMessage(`{"type":"object"}`),
		RequiresConfirmation: requiresConfirmation,
		Handler: func(_ context.Context, _ agentloop.AuthContext, _ json.RawMessage) (json.RawMessage, error) {
			c.mu.Lock()
			c.calls[name]++
			c.mu.Unlock()
			return json.RawMessage(append([]byte(nil), result...)), nil
		},
	})
}

func (c *scriptedCatalog) CallCount(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[name]
}

// ─────────────────────────────────────────────────────────────────────────────
// HTTP helpers
// ─────────────────────────────────────────────────────────────────────────────

func (h *assistantHarness) post(path string, body any) *http.Response {
	h.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(h.t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(http.MethodPost, h.server.URL+path, &buf)
	require.NoError(h.t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.server.Client().Do(req)
	require.NoError(h.t, err)
	return resp
}

func (h *assistantHarness) get(path string) *http.Response {
	h.t.Helper()
	req, err := http.NewRequest(http.MethodGet, h.server.URL+path, nil)
	require.NoError(h.t, err)
	resp, err := h.server.Client().Do(req)
	require.NoError(h.t, err)
	return resp
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var out T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

// waitFor опрашивает condition() каждые 20мс до timeout. Используется вместо
// time.Sleep — петля ассистента живёт в фоновой горутине, и тесты не должны
// угадывать её скорость.
func waitFor(t *testing.T, timeout time.Duration, msg string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waitFor timeout: %s", msg)
}

func (h *assistantHarness) reloadSession(sessionID uuid.UUID) *models.AssistantSession {
	h.t.Helper()
	sess, err := h.repo.GetSession(context.Background(), sessionID, h.userID)
	require.NoError(h.t, err)
	return sess
}

func (h *assistantHarness) listMessages(sessionID uuid.UUID) []*models.AssistantMessage {
	h.t.Helper()
	msgs, err := h.repo.ListMessages(context.Background(), sessionID, 200, time.Time{}, uuid.Nil)
	require.NoError(h.t, err)
	// Reverse → хронологический порядок для удобства проверок.
	out := make([]*models.AssistantMessage, len(msgs))
	for i := range msgs {
		out[len(msgs)-1-i] = msgs[i]
	}
	return out
}

func (h *assistantHarness) eventsOfType(msgType string) []hubEvent {
	h.t.Helper()
	out := make([]hubEvent, 0)
	for _, e := range h.hub.Snapshot() {
		if e.Type == msgType {
			out = append(out, e)
		}
	}
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers for crafting LLM responses
// ─────────────────────────────────────────────────────────────────────────────

func finalTextResponse(text string) *llm.Response {
	return &llm.Response{Content: text}
}

func toolCallResponse(callID, toolName string, args map[string]any) *llm.Response {
	argsJSON, _ := json.Marshal(args)
	return &llm.Response{
		ToolCalls: []llm.ToolCall{{
			ID:   callID,
			Type: "function",
			Function: llm.Function{
				Name:      toolName,
				Arguments: string(argsJSON),
			},
		}},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────────────────────────────────────

// TestAssistantE2E_HappyPath_ProjectList — самый базовый сценарий:
//   1. POST /sessions — создаём сессию.
//   2. POST /messages "Покажи проекты" — стартует агент-петлю.
//   3. LLM-mock: первый вызов → tool_call(project_list); второй → final text.
//   4. project_list возвращает фикс-payload {"items":[...],"next_cursor":null}.
//   5. Проверяем БД: user-msg, assistant-msg-с-tool_call, tool-row, final assistant.
//   6. Проверяем WS-broadcast: tool_call, tool_result, message-события улетели.
//   7. busy снят (session_updated busy=false в конце).
func TestAssistantE2E_HappyPath_ProjectList(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.catalog.Register("project_list", false, []byte(`{"items":[{"id":"p1","name":"Demo"}],"next_cursor":null}`))
	h.llm.SetResponses(
		toolCallResponse("call_1", "project_list", map[string]any{"limit": 20}),
		finalTextResponse("У тебя 1 проект: Demo."),
	)

	// 1. Create session.
	respCreate := h.post("/api/v1/assistant/sessions", nil)
	require.Equal(t, http.StatusCreated, respCreate.StatusCode)
	sess := decodeJSON[dto.AssistantSessionResponse](t, respCreate)
	require.NotEqual(t, uuid.Nil, sess.ID)

	// 2. Send message.
	clientMsgID := uuid.NewString()
	respSend := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "Покажи проекты", "client_message_id": clientMsgID})
	require.Equal(t, http.StatusAccepted, respSend.StatusCode)
	sendBody := decodeJSON[dto.SendAssistantMessageResponse](t, respSend)
	require.False(t, sendBody.Duplicate)

	// 3. Wait for agent loop to complete.
	waitFor(t, 5*time.Second, "agent loop completed (session not busy)", func() bool {
		s := h.reloadSession(sess.ID)
		return !s.Busy
	})

	// 4. Check tool call counter (handler executed exactly once).
	assert.Equal(t, 1, h.catalog.CallCount("project_list"))
	assert.Equal(t, 2, h.llm.Calls(), "LLM Chat called twice (tool_call, then final)")

	// 5. Check DB: user → assistant(tool_call) → tool → assistant(final).
	msgs := h.listMessages(sess.ID)
	require.GreaterOrEqual(t, len(msgs), 4, "expected at least 4 messages: %+v", msgRoles(msgs))

	// user
	assert.Equal(t, models.AssistantMessageRoleUser, msgs[0].Role)
	require.NotNil(t, msgs[0].Content)
	assert.Equal(t, "Покажи проекты", *msgs[0].Content)
	require.NotNil(t, msgs[0].ClientMessageID)
	assert.Equal(t, clientMsgID, *msgs[0].ClientMessageID)

	// assistant(tool_call)
	assert.Equal(t, models.AssistantMessageRoleAssistant, msgs[1].Role)
	require.NotNil(t, msgs[1].ToolName)
	assert.Equal(t, "project_list", *msgs[1].ToolName)
	require.NotNil(t, msgs[1].ToolCallID)
	assert.Equal(t, "call_1", *msgs[1].ToolCallID)

	// tool result row
	var toolRow *models.AssistantMessage
	for _, m := range msgs {
		if m.Role == models.AssistantMessageRoleTool {
			toolRow = m
			break
		}
	}
	require.NotNil(t, toolRow, "tool row must exist")
	assert.NotEmpty(t, []byte(toolRow.ToolResult), "tool_result populated")

	// final assistant text — последнее сообщение.
	final := msgs[len(msgs)-1]
	assert.Equal(t, models.AssistantMessageRoleAssistant, final.Role)
	require.NotNil(t, final.Content)
	assert.Contains(t, *final.Content, "Demo")

	// 6. WS broadcasts.
	assert.NotEmpty(t, h.eventsOfType("assistant.tool_call"), "assistant.tool_call broadcast")
	assert.NotEmpty(t, h.eventsOfType("assistant.tool_result"), "assistant.tool_result broadcast")
	msgEvents := h.eventsOfType("assistant.message")
	assert.GreaterOrEqual(t, len(msgEvents), 3, "user/assistant tool_call/final assistant message events")

	// 7. Сессия свободна, session_updated с busy=false есть в WS-эмиссии.
	sessAfter := h.reloadSession(sess.ID)
	assert.False(t, sessAfter.Busy, "session.busy=false after loop")
	sessionUpdates := h.eventsOfType("assistant.session_updated")
	require.NotEmpty(t, sessionUpdates, "at least one session_updated event")
	lastUpd := sessionUpdates[len(sessionUpdates)-1]
	assert.Contains(t, string(lastUpd.Payload), `"busy":false`)
}

// TestAssistantE2E_SessionBusy_409 — пока агент-петля висит на LLM, второй
// POST /messages должен вернуть 409 session_busy без второго запуска loop'а.
func TestAssistantE2E_SessionBusy_409(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	// LLM «висит» до закрытия канала — busy остаётся TRUE.
	gate := make(chan struct{})
	h.llm.SetPause(gate)
	h.llm.SetResponses(finalTextResponse("ok"))

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))

	respFirst := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "первое"})
	require.Equal(t, http.StatusAccepted, respFirst.StatusCode)

	// Ждём, пока busy=TRUE (горутина успела сделать AcquireBusy).
	waitFor(t, 2*time.Second, "session.busy=true", func() bool {
		return h.reloadSession(sess.ID).Busy
	})

	// Второе сообщение — должно упасть с 409.
	respSecond := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "второе"})
	require.Equal(t, http.StatusConflict, respSecond.StatusCode)
	body := decodeJSON[map[string]any](t, respSecond)
	assert.Equal(t, "session_busy", body["error"])

	// Освобождаем LLM, проверяем, что петля корректно завершилась.
	close(gate)
	waitFor(t, 3*time.Second, "session released", func() bool {
		return !h.reloadSession(sess.ID).Busy
	})
}

// TestAssistantE2E_Idempotency_DuplicateClientMessageID — повтор POST /messages
// с тем же client_message_id не запускает вторую петлю и возвращает duplicate=true.
func TestAssistantE2E_Idempotency_DuplicateClientMessageID(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.llm.SetResponses(finalTextResponse("hi"))

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))
	clientMsgID := uuid.NewString()

	first := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "hi", "client_message_id": clientMsgID})
	require.Equal(t, http.StatusAccepted, first.StatusCode)
	body1 := decodeJSON[dto.SendAssistantMessageResponse](t, first)
	assert.False(t, body1.Duplicate)

	// Дождёмся, пока первая петля завершит работу (busy → false).
	waitFor(t, 3*time.Second, "first loop done", func() bool {
		return !h.reloadSession(sess.ID).Busy
	})

	// Повтор — duplicate=true, новая петля НЕ стартует.
	llmCallsBefore := h.llm.Calls()
	second := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "hi", "client_message_id": clientMsgID})
	require.Equal(t, http.StatusAccepted, second.StatusCode)
	body2 := decodeJSON[dto.SendAssistantMessageResponse](t, second)
	assert.True(t, body2.Duplicate)
	assert.Equal(t, body1.Message.ID.String(), body2.Message.ID.String())

	// Подождём чуть-чуть и убедимся, что LLM-счётчик не вырос.
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, llmCallsBefore, h.llm.Calls(), "no second loop started")
}

// TestAssistantE2E_DestructiveConfirm_ApprovedFlow — полный destructive-цикл:
//   1. LLM зовёт project_delete (RequiresConfirmation=true).
//   2. Сессия паркуется: busy=TRUE, pending_tool_call_id=set, WS confirm_request.
//   3. POST /confirm approved=true → handler исполняет tool → петля резюмится →
//      LLM выдаёт финальный текст → busy=false.
func TestAssistantE2E_DestructiveConfirm_ApprovedFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.catalog.Register("project_delete", true, []byte(`{"status":"ok","deleted":true}`))
	h.llm.SetResponses(
		toolCallResponse("call_d1", "project_delete", map[string]any{"id": "p1"}),
		finalTextResponse("Проект удалён."),
	)

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))

	respSend := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "удали проект p1"})
	require.Equal(t, http.StatusAccepted, respSend.StatusCode)

	// Сессия должна запарковаться: busy=TRUE, pending_tool_call_id=call_d1.
	waitFor(t, 3*time.Second, "session parked on confirm", func() bool {
		s := h.reloadSession(sess.ID)
		return s.Busy && s.PendingToolCallID != nil && *s.PendingToolCallID == "call_d1"
	})

	// До confirm — tool НЕ исполнен.
	assert.Equal(t, 0, h.catalog.CallCount("project_delete"), "tool not executed before confirm")
	confirmEvents := h.eventsOfType("assistant.confirm_request")
	assert.NotEmpty(t, confirmEvents, "confirm_request WS broadcast")

	// Approve.
	respConfirm := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/confirm", sess.ID),
		map[string]any{"tool_call_id": "call_d1", "approved": true})
	require.Equal(t, http.StatusAccepted, respConfirm.StatusCode)

	// Дожидаемся завершения резюм-петли.
	waitFor(t, 5*time.Second, "loop resumed and completed", func() bool {
		s := h.reloadSession(sess.ID)
		return !s.Busy && s.PendingToolCallID == nil
	})

	assert.Equal(t, 1, h.catalog.CallCount("project_delete"), "tool executed exactly once")

	// Финальный текст в истории.
	msgs := h.listMessages(sess.ID)
	final := msgs[len(msgs)-1]
	require.Equal(t, models.AssistantMessageRoleAssistant, final.Role)
	require.NotNil(t, final.Content)
	assert.Contains(t, *final.Content, "удалён")

	// Tool-row закрыт реальным payload'ом.
	var toolRow *models.AssistantMessage
	for _, m := range msgs {
		if m.Role == models.AssistantMessageRoleTool && m.ToolCallID != nil && *m.ToolCallID == "call_d1" {
			toolRow = m
		}
	}
	require.NotNil(t, toolRow)
	// Postgres jsonb нормализует whitespace в `"key": value` — допускаем оба варианта.
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(toolRow.ToolResult, &parsed))
	assert.Equal(t, true, parsed["deleted"])
}

// TestAssistantE2E_DestructiveConfirm_Denied — Deny path: тулза НЕ
// исполняется, в историю пишется synthetic deny payload, LLM получает его
// и выдаёт финальный текст.
func TestAssistantE2E_DestructiveConfirm_Denied(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.catalog.Register("project_delete", true, []byte(`{"status":"ok","deleted":true}`))
	h.llm.SetResponses(
		toolCallResponse("call_d2", "project_delete", map[string]any{"id": "p1"}),
		finalTextResponse("Окей, отменяю."),
	)

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))
	require.Equal(t, http.StatusAccepted, h.post(
		fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "удали"}).StatusCode)

	waitFor(t, 3*time.Second, "parked", func() bool {
		s := h.reloadSession(sess.ID)
		return s.Busy && s.PendingToolCallID != nil
	})

	respConfirm := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/confirm", sess.ID),
		map[string]any{"tool_call_id": "call_d2", "approved": false})
	require.Equal(t, http.StatusAccepted, respConfirm.StatusCode)

	waitFor(t, 3*time.Second, "loop finished after deny", func() bool {
		return !h.reloadSession(sess.ID).Busy
	})

	assert.Equal(t, 0, h.catalog.CallCount("project_delete"), "deny → tool NOT executed")

	msgs := h.listMessages(sess.ID)
	final := msgs[len(msgs)-1]
	require.NotNil(t, final.Content)
	assert.Contains(t, *final.Content, "отменяю")

	// tool-row содержит synthetic deny payload.
	var toolRow *models.AssistantMessage
	for _, m := range msgs {
		if m.Role == models.AssistantMessageRoleTool && m.ToolCallID != nil && *m.ToolCallID == "call_d2" {
			toolRow = m
		}
	}
	require.NotNil(t, toolRow)
	assert.Contains(t, string(toolRow.ToolResult), `"denied"`)
}

// TestAssistantE2E_DoubleConfirm_AlreadyConfirmed — повторный confirm на тот
// же tool_call_id должен вернуть 409 already_confirmed (атомарный UPDATE).
func TestAssistantE2E_DoubleConfirm_AlreadyConfirmed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.catalog.Register("project_delete", true, []byte(`{"status":"ok"}`))
	h.llm.SetResponses(
		toolCallResponse("call_dd", "project_delete", map[string]any{}),
		finalTextResponse("done"),
	)

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))
	h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "удали"})
	waitFor(t, 3*time.Second, "parked", func() bool {
		s := h.reloadSession(sess.ID)
		return s.PendingToolCallID != nil
	})

	r1 := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/confirm", sess.ID),
		map[string]any{"tool_call_id": "call_dd", "approved": true})
	require.Equal(t, http.StatusAccepted, r1.StatusCode)

	// Повтор: уже закрыт.
	r2 := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/confirm", sess.ID),
		map[string]any{"tool_call_id": "call_dd", "approved": true})
	require.Equal(t, http.StatusConflict, r2.StatusCode)
	body := decodeJSON[map[string]any](t, r2)
	// Может быть already_confirmed или no_pending_confirmation в зависимости от того,
	// успела ли резюм-петля сбросить pending_tool_call_id. Оба — корректное 409.
	errCode, _ := body["error"].(string)
	assert.Contains(t, []string{"already_confirmed", "no_pending_confirmation"}, errCode,
		"got error=%q", errCode)
}

// TestAssistantE2E_LimitExceeded — LLM возвращает tool_call без конца, лимит
// итераций срабатывает, петля завершается с ошибкой в истории и busy снимается.
func TestAssistantE2E_LimitExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.catalog.Register("project_list", false, []byte(`{"items":[]}`))
	// Каждый ответ — это tool_call (никогда не возвращает финальный текст).
	loopResponses := make([]*llm.Response, service.AssistantMaxIterations+2)
	for i := range loopResponses {
		loopResponses[i] = toolCallResponse(fmt.Sprintf("call_%d", i), "project_list", map[string]any{})
	}
	h.llm.SetResponses(loopResponses...)

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))
	h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "loop"})

	waitFor(t, 10*time.Second, "loop hits MaxIterations and busy released", func() bool {
		return !h.reloadSession(sess.ID).Busy
	})

	assert.Equal(t, service.AssistantMaxIterations, h.llm.Calls(),
		"LLM called exactly MaxIterations times")

	// Последнее сообщение — error-text про лимит.
	msgs := h.listMessages(sess.ID)
	final := msgs[len(msgs)-1]
	require.NotNil(t, final.Content)
	assert.Contains(t, *final.Content, "лимит")
}

// TestAssistantE2E_GetHistory_PaginationStable — INSERT'им пачку сообщений
// «одним залпом» (созданием через repo), читаем через REST с курсором —
// проверяем, что (created_at, id)-курсор не теряет/не дублирует строки при
// одинаковых timestamp'ах.
func TestAssistantE2E_GetHistory_PaginationStable(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))
	sessionID := sess.ID

	// Создаём 5 сообщений напрямую через repo (быстро, без агент-петли).
	for i := 0; i < 5; i++ {
		content := fmt.Sprintf("msg-%d", i)
		err := h.repo.AppendMessage(context.Background(), &models.AssistantMessage{
			SessionID: sessionID,
			Role:      models.AssistantMessageRoleUser,
			Content:   &content,
		})
		require.NoError(t, err)
	}

	// Читаем страницами по 2.
	r1 := h.get(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages?limit=2", sessionID))
	require.Equal(t, http.StatusOK, r1.StatusCode)
	page1 := decodeJSON[dto.AssistantMessageListResponse](t, r1)
	require.Len(t, page1.Messages, 2)
	require.True(t, page1.HasMore)
	require.NotNil(t, page1.NextBeforeCreatedAt)
	require.NotNil(t, page1.NextBeforeID)

	r2 := h.get(fmt.Sprintf(
		"/api/v1/assistant/sessions/%s/messages?limit=2&before_id=%s&before_created_at=%s",
		sessionID, page1.NextBeforeID.String(), page1.NextBeforeCreatedAt.UTC().Format(time.RFC3339Nano)))
	require.Equal(t, http.StatusOK, r2.StatusCode)
	page2 := decodeJSON[dto.AssistantMessageListResponse](t, r2)
	require.Len(t, page2.Messages, 2)

	// Никаких пересечений между страницами.
	page1IDs := map[string]bool{}
	for _, m := range page1.Messages {
		page1IDs[m.ID.String()] = true
	}
	for _, m := range page2.Messages {
		assert.False(t, page1IDs[m.ID.String()],
			"page2 contains duplicate ID %s already on page1", m.ID)
	}
}

// TestAssistantE2E_ActiveTasks_PerUserFilter — создаём task в чужом проекте
// и в проекте юзера; GET /active-tasks возвращает только свой.
func TestAssistantE2E_ActiveTasks_PerUserFilter(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	// Свой проект → свой task.
	ownProjectID := insertProject(t, h.gormDB, h.userID, "own")
	ownTaskID := insertActiveTask(t, h.gormDB, ownProjectID, "my task", h.userID)

	// Чужой пользователь → чужой проект → чужой task.
	otherUserID := seedTestUser(t, h.gormDB)
	otherProjectID := insertProject(t, h.gormDB, otherUserID, "other")
	_ = insertActiveTask(t, h.gormDB, otherProjectID, "other task", otherUserID)

	resp := h.get("/api/v1/assistant/active-tasks")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeJSON[dto.AssistantActiveTasksResponse](t, resp)

	require.Len(t, body.Tasks, 1, "exactly one task for current user, got %d", len(body.Tasks))
	assert.Equal(t, ownTaskID.String(), body.Tasks[0].TaskID.String())
}

// TestAssistantE2E_ArchiveSession — POST /sessions → DELETE → GET возвращает 404
// (или 200 при include_archived=true).
func TestAssistantE2E_ArchiveSession(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	sess := decodeJSON[dto.AssistantSessionResponse](t, h.post("/api/v1/assistant/sessions", nil))

	req, _ := http.NewRequest(http.MethodDelete, h.server.URL+"/api/v1/assistant/sessions/"+sess.ID.String(), nil)
	resp, err := h.server.Client().Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Listing с дефолтным фильтром — пусто.
	listResp := h.get("/api/v1/assistant/sessions")
	require.Equal(t, http.StatusOK, listResp.StatusCode)
	list := decodeJSON[dto.AssistantSessionListResponse](t, listResp)
	assert.Empty(t, list.Sessions)

	// С include_archived=true — есть.
	listAll := h.get("/api/v1/assistant/sessions?include_archived=true")
	require.Equal(t, http.StatusOK, listAll.StatusCode)
	listAllBody := decodeJSON[dto.AssistantSessionListResponse](t, listAll)
	assert.Len(t, listAllBody.Sessions, 1)
	assert.Equal(t, "archived", listAllBody.Sessions[0].Status)
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func msgRoles(msgs []*models.AssistantMessage) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = string(m.Role)
	}
	return out
}

func insertProject(t *testing.T, gdb *gorm.DB, userID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var row struct{ ID string }
	err := gdb.Raw(`
		INSERT INTO projects (id, user_id, name, git_url, git_default_branch, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, ?, 'https://example.com/r.git', 'main', NOW(), NOW())
		RETURNING id`, userID, name).Scan(&row).Error
	require.NoError(t, err)
	return uuid.MustParse(row.ID)
}

func insertActiveTask(t *testing.T, gdb *gorm.DB, projectID uuid.UUID, title string, createdBy uuid.UUID) uuid.UUID {
	t.Helper()
	var row struct{ ID string }
	err := gdb.Raw(`
		INSERT INTO tasks (id, project_id, title, description, priority, state,
		                   cancel_requested, current_step_no, created_by_type, created_by_id,
		                   context, artifacts, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, ?, '', 'medium', 'active',
		        false, 0, 'user', ?, '{}'::jsonb, '{}'::jsonb, NOW(), NOW())
		RETURNING id`, projectID, title, createdBy).Scan(&row).Error
	require.NoError(t, err)
	return uuid.MustParse(row.ID)
}

func TestAssistantE2E_AutoGenerateTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.llm.SetResponses(
		finalTextResponse("Ответ ассистента"),
		finalTextResponse("Название проекта"),
	)

	// 1. Create session
	respCreate := h.post("/api/v1/assistant/sessions", nil)
	require.Equal(t, http.StatusCreated, respCreate.StatusCode)
	sess := decodeJSON[dto.AssistantSessionResponse](t, respCreate)
	assert.Empty(t, sess.Title)

	// 2. Send message
	respSend := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": "Создай мне проект под названием Название проекта"})
	require.Equal(t, http.StatusAccepted, respSend.StatusCode)

	// 3. Wait for title update
	waitFor(t, 5*time.Second, "session title is updated", func() bool {
		s := h.reloadSession(sess.ID)
		return s.Title != nil && *s.Title != ""
	})

	// 4. Verify title
	s := h.reloadSession(sess.ID)
	require.NotNil(t, s.Title)
	assert.Equal(t, "Название проекта", *s.Title)

	// 5. Verify broadcast
	sessionUpdates := h.eventsOfType("assistant.session_updated")
	require.NotEmpty(t, sessionUpdates)
	lastUpd := sessionUpdates[len(sessionUpdates)-1]
	assert.Contains(t, string(lastUpd.Payload), `"title":"Название проекта"`)
}

func TestAssistantE2E_AutoGenerateTitle_Fallback(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test (requires Docker)")
	}
	h := startAssistantHarness(t)
	defer h.Close()

	h.llm.SetResponses(
		finalTextResponse("Ответ ассистента"),
	)

	// 1. Create session
	respCreate := h.post("/api/v1/assistant/sessions", nil)
	require.Equal(t, http.StatusCreated, respCreate.StatusCode)
	sess := decodeJSON[dto.AssistantSessionResponse](t, respCreate)

	// 2. Send long message
	longMsg := "Это очень длинное первое сообщение пользователя, которое должно быть обрезано до сорока символов"
	respSend := h.post(fmt.Sprintf("/api/v1/assistant/sessions/%s/messages", sess.ID),
		map[string]any{"content": longMsg})
	require.Equal(t, http.StatusAccepted, respSend.StatusCode)

	// 3. Wait for title update fallback
	waitFor(t, 5*time.Second, "session title is updated via fallback", func() bool {
		s := h.reloadSession(sess.ID)
		return s.Title != nil && *s.Title != ""
	})

	// 4. Verify title
	s := h.reloadSession(sess.ID)
	require.NotNil(t, s.Title)
	expectedTitle := string([]rune(longMsg)[:40]) + "..."
	assert.Equal(t, expectedTitle, *s.Title)
}

// unused-imports guards (компилятор иначе ругается, если ветка кода не пошла).
var _ = datatypes.JSON(nil)
