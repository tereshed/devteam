//go:build featuresmoke

package featuresmoke

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/devteam/backend/test/featuresmoke/fakes"
	"github.com/google/uuid"
)

// prompt_content_smoke_test.go — контракт на содержимое payload'а, который
// backend отправляет в LLM (Anthropic/OpenAI/...).
//
// Мотивация: после Phase 2 review обнаружилось, что один день featuresmoke-
// прогонов сжёг ~$30 на реальный Anthropic, т.к. (a) harness не редиректил
// провайдерские base_url на FakeLLM и (b) router prompt раздувался до 6.6k
// токенов на каждый вызов (мусорные тестовые агенты + накопленные артефакты).
// Эти тесты — регресс-гард на оба класса проблем.
//
// Поверхность тестирования:
//   - Перехват: harness.GlobalFakeLLM() — FakeLLM поднят ДО backend'а,
//     redirect через ANTHROPIC_BASE_URL/OPENAI_BASE_URL/... В mock-режиме
//     ЛЮБОЙ LLM-вызов backend'а проходит через него и оседает в Calls().
//   - Триггер: assistant (POST /assistant/sessions/.../messages) — единственный
//     путь к LLM при ORCHESTRATOR_V2_WORKERS_ENABLED=false (PR-gate смоук).
//     Real-режим использует настоящий LLM, поэтому tests t.Skip'аются.

// canaryEnvSecrets — значения, которые harness.composeEnv проставляет как fake-ключи.
// Они НЕ ДОЛЖНЫ оказаться в bodies — даже фейковые ключи в prompt'е это leak
// (на проде там был бы реальный ANTHROPIC_API_KEY).
var canaryEnvSecrets = []string{
	"fake-anthropic-featuresmoke-key",
	"fake-openai-featuresmoke-key",
	"fake-deepseek-featuresmoke-key",
	"fake-gemini-featuresmoke-key",
	"fake-qwen-featuresmoke-key",
	"fake-openrouter-featuresmoke-key",
	"featuresmoke-jwt-secret-1234567890abcdef",
}

// suspiciousPathPrefixes — фрагменты, которые означают, что в prompt протёк
// абсолютный path с локальной dev-машины / CI-агента. На LLM такое отправлять
// нельзя (информация об инфре).
var suspiciousPathPrefixes = []string{
	"/Users/",      // macOS home (developer machines)
	"/home/",       // linux home (CI runners)
	"/var/folders/", // macOS temp
	"/private/var/",
	"/root/",
}

// awaitFakeLLMCall ждёт пока FakeLLM получит хотя бы один новый call'е с
// content'ом, содержащим маркер `marker`. Возвращает все такие calls'ы.
// Падает по таймауту.
func awaitFakeLLMCall(t *testing.T, fake *fakes.FakeLLM, marker string, timeout time.Duration) []fakes.LLMCall {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var matched []fakes.LLMCall
		for _, c := range fake.Calls() {
			if bytes.Contains(c.Body, []byte(marker)) {
				matched = append(matched, c)
			}
		}
		if len(matched) > 0 {
			return matched
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("FakeLLM: за %s не пришло ни одного call'а с маркером %q (всего calls=%d)",
		timeout, marker, fake.CallCount())
	return nil
}

// triggerAssistantMessage создаёт сессию ассистента и шлёт сообщение с уникальным
// маркером. Возвращает marker — каноничный fingerprint, по которому потом ищем
// конкретный наш call в общем журнале FakeLLM.
func triggerAssistantMessage(t *testing.T, h *Harness, user User) string {
	t.Helper()
	h.ConfigureUserAssistant(t, user, "anthropic", "claude-3-5-haiku-20241022")
	createResp := h.Do(t, "POST", "/api/v1/assistant/sessions", nil, user.AccessToken)
	if createResp.Status != http.StatusCreated && createResp.Status != http.StatusOK {
		t.Skipf("assistant не доступен: POST /assistant/sessions = %d (%s) — пропускаем prompt-content смоук",
			createResp.Status, truncBody(createResp.Body))
	}
	var sess struct {
		ID string `json:"id"`
	}
	createResp.JSON(t, &sess)
	if sess.ID == "" {
		t.Skipf("assistant сессия не создалась (body=%s) — пропускаем", truncBody(createResp.Body))
	}

	// Уникальный маркер в сообщении — по нему вычленяем именно наш call.
	marker := "PROMPT-CONTENT-CANARY-" + uuid.NewString()
	sendResp := h.Do(t, "POST", "/api/v1/assistant/sessions/"+sess.ID+"/messages",
		map[string]any{"content": marker},
		user.AccessToken)
	if sendResp.Status != http.StatusAccepted && sendResp.Status != http.StatusOK {
		t.Skipf("assistant отказался принять сообщение: status=%d body=%s — пропускаем",
			sendResp.Status, truncBody(sendResp.Body))
	}
	return marker
}

// TestPromptContent_FakeLLMReceivesNoCallsForPureCRUDFlow — cost-leak regression
// gate. CRUD-операции (auth, projects, tasks без assistant и без orchestrator-
// воркеров) НЕ должны порождать НИ ОДНОГО LLM-вызова. Если этот тест упал —
// значит либо появилась новая фоновая петля, либо случайно включили воркеры,
// либо где-то лишний вызов /llm/chat.
//
// Точно отлавливает Phase-2 cost-leak (5,271 вызовов от смоук-сьюита, $30).
//
// ВАЖНО: НЕ t.Parallel(). Sequential-тест в go test'е выполняется ПЕРЕД
// parallel-batch'ем, поэтому к моменту его прогона параллельные assistant-тесты
// (которые сами шлют LLM-вызовы) ещё не стартовали. Иначе их calls'ы прилетают
// в общий FakeLLM-журнал между моими before/after snapshot'ами и дают false-
// positive «cost-leak».
func TestPromptContent_FakeLLMReceivesNoCallsForPureCRUDFlow(t *testing.T) {
	// БЕЗ t.Parallel() — см. комментарий выше.
	h := StartServer(t)
	fake := h.GlobalFakeLLM(t)

	// Snapshot до — другие parallel-тесты ещё не стартовали (паузятся на своих t.Parallel'ах).
	before := fake.CallCount()
	t.Logf("FakeLLM до прогона CRUD: %d calls", before)

	// Полный CRUD-цикл: register → project → task → pause → resume → cancel.
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "no-llm-flow-"+uuid.NewString())

	for _, action := range []string{"pause", "resume", "cancel"} {
		resp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/"+action, nil, user.AccessToken)
		if resp.Status >= 500 {
			t.Fatalf("%s: status=%d body=%s", action, resp.Status, truncBody(resp.Body))
		}
	}

	// Пол-секунды на eventual flush WS-/background-горутин (если бы они были).
	// Если за это время LLM-вызовы появились — мы их увидим.
	time.Sleep(500 * time.Millisecond)

	after := fake.CallCount()
	delta := after - before
	if delta != 0 {
		// Найти эти call'ы, чтобы дать диагностику.
		recent := fake.Calls()
		dump := ""
		if len(recent) > 0 {
			lastBody := recent[len(recent)-1].Body
			dump = truncBody(lastBody)
		}
		t.Fatalf("LLM cost-leak: CRUD-флоу породил %d LLM-вызовов (всего before=%d after=%d). Последний body: %s",
			delta, before, after, dump)
	}
}

// TestPromptContent_AssistantPromptHasNoEnvSecrets — assistant LLM-payload
// не должен содержать ANTHROPIC_API_KEY / JWT_SECRET_KEY / прочие env-секреты.
// Защита от: (a) случайного forwarding'а env в prompt-строитель, (b) утечки
// через стек-трейсы в metadata, (c) включения config-dump в системный prompt.
func TestPromptContent_AssistantPromptHasNoEnvSecrets(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	fake := h.GlobalFakeLLM(t)
	user := h.NewUser(t)

	marker := triggerAssistantMessage(t, h, user)
	calls := awaitFakeLLMCall(t, fake, marker, 30*time.Second)

	for _, c := range calls {
		for _, secret := range canaryEnvSecrets {
			if bytes.Contains(c.Body, []byte(secret)) {
				t.Fatalf("LEAK: assistant LLM-call (%s) содержит env-секрет %q: %s",
					c.Path, secret, truncBody(c.Body))
			}
		}
	}
}

// TestPromptContent_AssistantPromptHasNoUserPassword — пароль зарегистрированного
// пользователя НИКОГДА не должен оказаться в payload'е (он хешируется при register,
// в БД его plain-формы нет, и через assistant он точно не должен утечь).
func TestPromptContent_AssistantPromptHasNoUserPassword(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	fake := h.GlobalFakeLLM(t)
	user := h.NewUser(t)

	marker := triggerAssistantMessage(t, h, user)
	calls := awaitFakeLLMCall(t, fake, marker, 30*time.Second)

	for _, c := range calls {
		if bytes.Contains(c.Body, []byte(user.Password)) {
			t.Fatalf("LEAK: пароль пользователя оказался в assistant LLM-call (%s)", c.Path)
		}
	}
}

// TestPromptContent_AssistantPromptHasNoFilesystemPaths — фрагменты абсолютных
// путей хост-машины не должны утекать в LLM. Это значит что-то в системном
// промпте или metadata случайно сериализует runtime.Caller / file-paths.
func TestPromptContent_AssistantPromptHasNoFilesystemPaths(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	fake := h.GlobalFakeLLM(t)
	user := h.NewUser(t)

	marker := triggerAssistantMessage(t, h, user)
	calls := awaitFakeLLMCall(t, fake, marker, 30*time.Second)

	for _, c := range calls {
		for _, prefix := range suspiciousPathPrefixes {
			if bytes.Contains(c.Body, []byte(prefix)) {
				idx := bytes.Index(c.Body, []byte(prefix))
				end := idx + 80
				if end > len(c.Body) {
					end = len(c.Body)
				}
				t.Fatalf("LEAK: подозрительный path-префикс %q в LLM-payload (%s): %q",
					prefix, c.Path, string(c.Body[idx:end]))
			}
		}
	}
}

// TestPromptContent_AssistantPromptIsBoundedSize — отдельный assistant-вызов
// не должен превышать здравый лимит. Под 50KB (≈ 12k токенов) — щедро, но
// fail если кто-то начнёт пихать весь project context / огромный системный
// промт / историю всех сообщений сразу.
func TestPromptContent_AssistantPromptIsBoundedSize(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	fake := h.GlobalFakeLLM(t)
	user := h.NewUser(t)

	marker := triggerAssistantMessage(t, h, user)
	calls := awaitFakeLLMCall(t, fake, marker, 30*time.Second)

	const maxBytes = 50 * 1024
	for _, c := range calls {
		if len(c.Body) > maxBytes {
			t.Fatalf("oversize: assistant LLM-call (%s) %d bytes > %d. Head: %s",
				c.Path, len(c.Body), maxBytes, truncBody(c.Body[:min(500, len(c.Body))]))
		}
	}
}

// TestPromptContent_AssistantPromptContainsUserMessage — позитивный sanity:
// то, что мы ПЕРЕДАЁМ ассистенту, действительно идёт в LLM. Защита от
// обратной поломки — если кто-то случайно начнёт scrub'ить user-content
// или пропустит его мимо prompt-builder'а, этот тест словит.
func TestPromptContent_AssistantPromptContainsUserMessage(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	fake := h.GlobalFakeLLM(t)
	user := h.NewUser(t)

	marker := triggerAssistantMessage(t, h, user)
	calls := awaitFakeLLMCall(t, fake, marker, 30*time.Second)
	// awaitFakeLLMCall уже фильтрует по marker'у — если дошли сюда, значит
	// в body есть наша строка. Доп. ассерт на формат (JSON-payload, не просто
	// строка/массив байт):
	for _, c := range calls {
		// Anthropic /v1/messages и OpenAI /v1/chat/completions — оба JSON-объекты.
		if !bytes.HasPrefix(bytes.TrimSpace(c.Body), []byte("{")) {
			t.Fatalf("LLM-payload не похож на JSON-объект (%s): head=%q",
				c.Path, truncBody(c.Body[:min(120, len(c.Body))]))
		}
		// И что Content-Type обещан application/json — это контракт SDK.
		if ct := c.Headers.Get("Content-Type"); !strings.Contains(strings.ToLower(ct), "json") {
			t.Fatalf("LLM-payload Content-Type=%q (ожидали JSON)", ct)
		}
	}
}

