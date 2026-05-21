//go:build featuresmoke

package featuresmoke

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// assistant_smoke_test.go — P2 контракт /api/v1/assistant/* (Sprint 21).
//
// Что покрываем:
//   - POST /assistant/sessions — 201, session.id != "".
//   - GET /assistant/sessions — содержит свежесозданную сессию.
//   - GET /assistant/sessions/:id — детали; busy=false для свежей сессии.
//   - GET /assistant/sessions/:id/messages — пустой список до отправки.
//   - DELETE /assistant/sessions/:id — soft-delete (status=archived).
//   - POST /assistant/sessions/:id/messages — 202, эхо user-сообщения в payload.
//   - GET /assistant/active-tasks — 200, массив (возможно пустой).
//   - 401 без токена для всех ручек.
//
// Реальный assistant-loop (LLM-вызов + agent_tool_calls) проверяется через
// llm_logs (см. TestAssistant_SendMessage_RealLLMLogIsSane), а не через
// FakeLLM.Calls(): assistant работает асинхронно, и в моке-режиме (PR-gate)
// у глобального FakeLLM fastFail=false и нет per-rule prompts'ов, поэтому
// гарантия одна — backend сделал HTTP-запрос на наш стаб (полное «адекватно
// сформирован prompt» отдаётся real-режиму с проверкой llm_logs).
//
// КРИТИЧНО (cost-leak prevention): даже в mock-режиме мы не шлём «настоящий»
// длинный prompt — content="ping" (5 байт), и assistantSession отсекается
// в течение нескольких секунд (assistant agent в seed.go настроен на cheap
// модель + temperature=0.2 + maxIterations=12). При cost-leak'е через
// orchestrator-воркеры (всё ещё OFF в mock, см. ORCHESTRATOR_V2_WORKERS_ENABLED)
// этот тест не добавит расходов.

type assistantSession struct {
	ID                string  `json:"id"`
	UserID            string  `json:"user_id"`
	Title             *string `json:"title,omitempty"`
	Status            string  `json:"status"`
	Busy              bool    `json:"busy"`
	PendingToolCallID *string `json:"pending_tool_call_id,omitempty"`
}

type assistantSessionsList struct {
	Sessions []assistantSession `json:"sessions"`
}

func createAssistantSession(t *testing.T, h *Harness, token string) assistantSession {
	t.Helper()
	resp := h.Do(t, "POST", "/api/v1/assistant/sessions", map[string]any{}, token)
	if resp.Status != http.StatusCreated {
		t.Fatalf("create assistant session: status=%d body=%s",
			resp.Status, truncBody(resp.Body))
	}
	var s assistantSession
	resp.JSON(t, &s)
	if s.ID == "" {
		t.Fatalf("create assistant session: пустой id: %s", truncBody(resp.Body))
	}
	if s.Status != "active" {
		t.Fatalf("create assistant session: status=%q ожидали active", s.Status)
	}
	if s.Busy {
		t.Fatalf("create assistant session: busy=true для свежей сессии")
	}
	return s
}

// TestAssistant_SessionLifecycle — create → get → list → archive → 4xx после archive.
func TestAssistant_SessionLifecycle(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	created := createAssistantSession(t, h, user.AccessToken)

	getResp := h.Do(t, "GET", "/api/v1/assistant/sessions/"+created.ID, nil, user.AccessToken)
	if getResp.Status != http.StatusOK {
		t.Fatalf("get session: status=%d body=%s", getResp.Status, truncBody(getResp.Body))
	}
	var got assistantSession
	getResp.JSON(t, &got)
	if got.ID != created.ID {
		t.Fatalf("get session: id=%q ожидали %q", got.ID, created.ID)
	}

	listResp := h.Do(t, "GET", "/api/v1/assistant/sessions", nil, user.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("list sessions: status=%d", listResp.Status)
	}
	var list assistantSessionsList
	listResp.JSON(t, &list)
	if !containsSessionID(list.Sessions, created.ID) {
		t.Fatalf("list sessions: %s не найден среди %d", created.ID, len(list.Sessions))
	}

	// Пустая история до отправки.
	msgsResp := h.Do(t, "GET", "/api/v1/assistant/sessions/"+created.ID+"/messages",
		nil, user.AccessToken)
	if msgsResp.Status != http.StatusOK {
		t.Fatalf("get messages: status=%d body=%s", msgsResp.Status, truncBody(msgsResp.Body))
	}
	var msgs struct {
		Messages []json.RawMessage `json:"messages"`
		HasMore  bool              `json:"has_more"`
	}
	msgsResp.JSON(t, &msgs)
	if len(msgs.Messages) != 0 {
		t.Fatalf("get messages: ожидали пусто для свежей сессии, получили %d", len(msgs.Messages))
	}

	// Archive.
	delResp := h.Do(t, "DELETE", "/api/v1/assistant/sessions/"+created.ID, nil, user.AccessToken)
	if delResp.Status != http.StatusNoContent {
		t.Fatalf("archive: status=%d body=%s", delResp.Status, truncBody(delResp.Body))
	}

	// После archive в обычном list (без include_archived=true) её быть не должно.
	listResp2 := h.Do(t, "GET", "/api/v1/assistant/sessions", nil, user.AccessToken)
	if listResp2.Status != http.StatusOK {
		t.Fatalf("list after archive: status=%d", listResp2.Status)
	}
	var list2 assistantSessionsList
	listResp2.JSON(t, &list2)
	if containsSessionID(list2.Sessions, created.ID) {
		t.Fatalf("list after archive: %s всё ещё в active-списке", created.ID)
	}
}

func containsSessionID(s []assistantSession, id string) bool {
	for _, x := range s {
		if x.ID == id {
			return true
		}
	}
	return false
}

// TestAssistant_GetMissingReturns404.
func TestAssistant_GetMissingReturns404(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	resp := h.Do(t, "GET", "/api/v1/assistant/sessions/"+uuid.NewString(),
		nil, user.AccessToken)
	if resp.Status != http.StatusNotFound {
		t.Fatalf("get missing: status=%d (ожидали 404)", resp.Status)
	}
}

// TestAssistant_CrossTenantIsolation — чужая сессия не видна.
func TestAssistant_CrossTenantIsolation(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)

	s := createAssistantSession(t, h, alice.AccessToken)

	resp := h.Do(t, "GET", "/api/v1/assistant/sessions/"+s.ID, nil, bob.AccessToken)
	if resp.Status != http.StatusNotFound && resp.Status != http.StatusForbidden {
		t.Fatalf("cross-tenant get: status=%d (ожидали 404/403)", resp.Status)
	}
}

// TestAssistant_ActiveTasksList — возвращает массив (возможно пустой) для свежего юзера.
func TestAssistant_ActiveTasksList(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	resp := h.Do(t, "GET", "/api/v1/assistant/active-tasks", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("active-tasks: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out struct {
		Tasks []json.RawMessage `json:"tasks"`
	}
	resp.JSON(t, &out)
	// Для свежего юзера ожидаем пусто, но не падаем, если что-то затесалось
	// (shared backend между тестами — сюда могут попасть задачи из других прогонов).
	_ = out
}

// TestAssistant_RequireAuthentication.
func TestAssistant_RequireAuthentication(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	cases := []struct {
		method string
		path   string
		body   any
	}{
		{"POST", "/api/v1/assistant/sessions", map[string]any{}},
		{"GET", "/api/v1/assistant/sessions", nil},
		{"GET", "/api/v1/assistant/sessions/" + uuid.NewString(), nil},
		{"DELETE", "/api/v1/assistant/sessions/" + uuid.NewString(), nil},
		{"GET", "/api/v1/assistant/sessions/" + uuid.NewString() + "/messages", nil},
		{"POST", "/api/v1/assistant/sessions/" + uuid.NewString() + "/messages", map[string]any{
			"content": "x",
		}},
		{"POST", "/api/v1/assistant/sessions/" + uuid.NewString() + "/confirm", map[string]any{
			"tool_call_id": "call_x", "approved": true,
		}},
		{"GET", "/api/v1/assistant/active-tasks", nil},
	}
	for _, tc := range cases {
		resp := h.Do(t, tc.method, tc.path, tc.body, "")
		if resp.Status != http.StatusUnauthorized {
			t.Fatalf("%s %s no token: status=%d (ожидали 401)",
				tc.method, tc.path, resp.Status)
		}
	}
}

// TestAssistant_SendMessage_ReturnsAcceptedEcho — happy-path POST /messages.
// Мы НЕ ждём ответа агента (он асинхронный, может занять несколько секунд +
// потенциально совершит LLM-вызов); проверяем только синхронный 202-контракт:
//   - status=202;
//   - message.role=user, message.content=наш контент;
//   - duplicate=false (первый POST);
//   - повторный POST с тем же client_message_id → 202 + duplicate=true.
func TestAssistant_SendMessage_ReturnsAcceptedEcho(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	sess := createAssistantSession(t, h, user.AccessToken)

	clientMsgID := uuid.NewString()
	content := "ping " + uuid.NewString()
	resp := h.Do(t, "POST", "/api/v1/assistant/sessions/"+sess.ID+"/messages",
		map[string]any{
			"content":           content,
			"client_message_id": clientMsgID,
		}, user.AccessToken)
	if resp.Status != http.StatusAccepted {
		t.Fatalf("send: status=%d (ожидали 202) body=%s",
			resp.Status, truncBody(resp.Body))
	}
	var out struct {
		Message struct {
			ID        string `json:"id"`
			SessionID string `json:"session_id"`
			Role      string `json:"role"`
			Content   string `json:"content"`
		} `json:"message"`
		Duplicate bool `json:"duplicate"`
	}
	resp.JSON(t, &out)
	if out.Message.SessionID != sess.ID {
		t.Fatalf("send: message.session_id=%q ожидали %q", out.Message.SessionID, sess.ID)
	}
	if out.Message.Role != "user" {
		t.Fatalf("send: message.role=%q ожидали user", out.Message.Role)
	}
	if out.Message.Content != content {
		t.Fatalf("send: message.content=%q ожидали %q", out.Message.Content, content)
	}
	if out.Duplicate {
		t.Fatalf("send: duplicate=true для первого POST")
	}

	// Повтор с тем же client_message_id → duplicate=true.
	resp2 := h.Do(t, "POST", "/api/v1/assistant/sessions/"+sess.ID+"/messages",
		map[string]any{
			"content":           content,
			"client_message_id": clientMsgID,
		}, user.AccessToken)
	// Допустимо 200 ИЛИ 202 для дубликата (см. swagger annotation: оба заявлены).
	if resp2.Status != http.StatusAccepted && resp2.Status != http.StatusOK {
		t.Fatalf("send duplicate: status=%d (ожидали 200/202) body=%s",
			resp2.Status, truncBody(resp2.Body))
	}
	var out2 struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
		Duplicate bool `json:"duplicate"`
	}
	resp2.JSON(t, &out2)
	if !out2.Duplicate {
		t.Fatalf("send duplicate: duplicate=false (ожидали true)")
	}
	if out2.Message.ID != out.Message.ID {
		t.Fatalf("send duplicate: вернулся другой message id=%q ожидали %q",
			out2.Message.ID, out.Message.ID)
	}
}

// TestAssistant_SendMessage_LLMRequestIsSane — проверка адекватности
// LLM-запроса, который отправляет backend, когда юзер пишет в assistant.
//
// Стратегия:
//   - Регистрируемся как обычный юзер.
//   - Создаём session, отправляем короткий «ping».
//   - Ждём, пока в llm_logs появится свежая запись (assistant-агент работает
//     асинхронно; на cheap-модели и коротком prompt'е укладывается в ~10 сек).
//   - Достаём prompt_snapshot и проверяем:
//       a) provider/model — anthropic + claude-haiku (assistant seed) ИЛИ
//          какой явно проставлен; для PR-gate fakeAnthropic тоже подойдёт.
//       b) snapshot содержит system prompt assistant'а («ассистент платформы»).
//       c) snapshot содержит наш user content.
//   - Snapshot — это ровно то, что backend отправил в LLM, поэтому это и
//     есть «адекватность запроса», которую ревьюверу нужно глазами смотреть.
//
func TestAssistant_GetStatus_AutoConfigure(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// 1. Initially, should not be configured
	statusResp := h.Do(t, "GET", "/api/v1/assistant/status", nil, user.AccessToken)
	if statusResp.Status != http.StatusOK {
		t.Fatalf("get assistant status: status=%d body=%s", statusResp.Status, truncBody(statusResp.Body))
	}
	var status struct {
		IsConfigured     bool   `json:"is_configured"`
		RequiredProvider string `json:"required_provider"`
	}
	statusResp.JSON(t, &status)
	if status.IsConfigured {
		t.Fatalf("expected assistant to be unconfigured initially")
	}
	if status.RequiredProvider != "openrouter" {
		t.Fatalf("expected required_provider to be openrouter, got %q", status.RequiredProvider)
	}

	// 2. Add OpenRouter API key
	fakeKey := "sk-openrouter-featuresmoke-test-fake-key-must-be-long-enough"
	patchResp := h.Do(t, "PATCH", "/api/v1/me/llm-credentials", map[string]any{
		"openrouter_api_key": fakeKey,
	}, user.AccessToken)
	if patchResp.Status != http.StatusOK {
		t.Fatalf("patch credentials failed: status=%d body=%s", patchResp.Status, truncBody(patchResp.Body))
	}

	// 3. Now GET /api/v1/assistant/status should be configured: true
	statusResp2 := h.Do(t, "GET", "/api/v1/assistant/status", nil, user.AccessToken)
	if statusResp2.Status != http.StatusOK {
		t.Fatalf("get assistant status after credentials: status=%d body=%s", statusResp2.Status, truncBody(statusResp2.Body))
	}
	statusResp2.JSON(t, &status)
	if !status.IsConfigured {
		t.Fatalf("expected assistant to be configured after adding openrouter key")
	}
	if status.RequiredProvider != "openrouter" {
		t.Fatalf("expected required_provider to be openrouter, got %q", status.RequiredProvider)
	}
}

// В mock-режиме (PR-gate) backend ходит на FakeLLM, llm_logs всё равно
// заполняется (см. LLM repository — INSERT перед/после вызова провайдера).
// Если запись не появляется в течение 30s — что-то сломалось в сервис-слое
// (заявка не дошла до executor), и это надо чинить — тест валит.
//
// В real-режиме на anthropic уходит реальный запрос (≈1¢ за вызов на haiku).
// Это и есть «осторожная проверка» — один реальный запрос за прогон.
func TestAssistant_SendMessage_LLMRequestIsSane(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	h.ConfigureUserAssistant(t, user, "anthropic", "claude-3-5-haiku-20241022")
	sess := createAssistantSession(t, h, user.AccessToken)

	// Зафиксируем «до»-границу по llm_logs. Параллельные тесты могут
	// плодить свои записи, поэтому ниже фильтруем именно по нашему уникальному
	// контенту, а не «по самому раннему за startedAt».
	db := directDB(t)
	startedAt := time.Now().UTC().Add(-2 * time.Second)

	// Уникальный маркер, гарантирующий, что мы найдём именно нашу запись:
	// UUID без дефисов, чтобы пройти любые транформации JSON-сериализации.
	uniqueMarker := "smoke-ping-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	content := uniqueMarker
	resp := h.Do(t, "POST", "/api/v1/assistant/sessions/"+sess.ID+"/messages",
		map[string]any{
			"content":           content,
			"client_message_id": uuid.NewString(),
		}, user.AccessToken)
	if resp.Status != http.StatusAccepted {
		t.Fatalf("send: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}

	// Polling: ждём появления записи, где prompt_snapshot содержит наш маркер.
	// 30s хватает: запуск горутины + 1 HTTP roundtrip до FakeLLM/Anthropic + INSERT.
	// Фильтр по маркеру решает race с другими параллельными тестами,
	// которые тоже шлют /messages.
	deadline := time.Now().Add(30 * time.Second)
	var (
		gotProvider string
		gotModel    string
		gotSnap     string
	)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		row := db.QueryRowContext(ctx, `
			SELECT provider, model, COALESCE(prompt_snapshot::text, '')
			FROM llm_logs
			WHERE created_at >= $1
			  AND prompt_snapshot::text LIKE $2
			ORDER BY created_at ASC
			LIMIT 1`, startedAt, "%"+uniqueMarker+"%")
		err := row.Scan(&gotProvider, &gotModel, &gotSnap)
		cancel()
		if err == nil && gotSnap != "" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if gotSnap == "" {
		t.Fatalf("llm_logs: запись с маркером %q не появилась за 30s — backend "+
			"не дошёл до LLM либо отправил без нашего контента. "+
			"Проверь, что agent role='assistant' заведён в seed и LLMResolver настроен.",
			uniqueMarker)
	}

	// 1. Provider/model — у assistant seed это anthropic + claude-haiku.
	//    В real-режиме оператор может перепереключить на другой провайдер
	//    через UI, поэтому строгий equals не делаем, а просто требуем
	//    непустые значения.
	if gotProvider == "" {
		t.Fatalf("llm_logs: пустой provider")
	}
	if gotModel == "" {
		t.Fatalf("llm_logs: пустой model")
	}

	// 2. Snapshot обязан содержать system prompt assistant'а — это инвариант
	//    AgentService.CreateDefaultAssistant. Если он пропал, ассистент будет
	//    работать «как general chatbot», что ломает product-behavior.
	if !strings.Contains(gotSnap, "ассистент платформы") &&
		!strings.Contains(gotSnap, "ассистент") {
		t.Fatalf("llm_logs: prompt_snapshot не содержит assistant system prompt. "+
			"provider=%q model=%q snapshot[первые 400 байт]=%s",
			gotProvider, gotModel, truncStr(gotSnap, 400))
	}

	// 3. Snapshot обязан содержать user-контент — это значит история
	//    действительно дошла до LLM, а не «пустое сообщение».
	if !strings.Contains(gotSnap, content) {
		t.Fatalf("llm_logs: prompt_snapshot не содержит наш user-content %q. "+
			"snapshot[первые 400 байт]=%s",
			content, truncStr(gotSnap, 400))
	}
}
