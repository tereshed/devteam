package service

import (
	"strings"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
)

// router_prompt_content_test.go — контракт на содержимое user-prompt'а Router'а.
//
// Тесты вызывают `buildUserPrompt` напрямую (без HTTP / БД) — это pure-функция,
// её ассерты дешёвые и устойчивые.
//
// Контекст инцидента: Phase 2 review показал, что router prompt вырастал до
// 6.6k токенов на каждый вызов (5.3M токенов / $30 за день), потому что в
// каталог попадали 289 leaked-агентов с пустым role_description (созданных
// смоук-тестами через POST /api/v1/agents). buildUserPrompt просто конкатенирует
// `state.Agents` без фильтрации.
//
// Эти тесты фиксируют ОЖИДАЕМЫЙ контракт. Часть из них СЕЙЧАС упадёт —
// это нормально: значит backend требует фикса. См. TODO/Logf'ы.

// helperAgent — миниатюрный конструктор для тестов prompt-content.
func helperAgent(name, role, description string, kind models.AgentExecutionKind) *models.Agent {
	a := &models.Agent{
		ID:            uuid.New(),
		Name:          name,
		Role:          models.AgentRole(role),
		ExecutionKind: kind,
		IsActive:      true,
	}
	if description != "" {
		a.RoleDescription = &description
	}
	return a
}

// helperTask — минимально валидный task для теста.
func helperTask(title, description string) *models.Task {
	return &models.Task{
		ID:          uuid.New(),
		ProjectID:   uuid.New(),
		Title:       title,
		Description: description,
		State:       models.TaskStateActive,
	}
}

// TestBuildUserPrompt_IncludesTaskTitleAndDescription — позитивный sanity:
// то что мы передали, действительно идёт в prompt. Регрессия защищает от
// «случайно вырезали Title/Description» при рефакторинге.
func TestBuildUserPrompt_IncludesTaskTitleAndDescription(t *testing.T) {
	r := &RouterService{}
	state := RouterState{
		Task:   helperTask("Add JWT auth", "Implement JWT for the API."),
		Agents: []*models.Agent{helperAgent("planner", "planner", "creates plans", models.AgentExecutionKindLLM)},
	}
	out := r.buildUserPrompt(state, "")

	if !strings.Contains(out, "Add JWT auth") {
		t.Fatalf("ожидали title в prompt, не нашли. Output:\n%s", out)
	}
	if !strings.Contains(out, "Implement JWT for the API.") {
		t.Fatalf("ожидали description в prompt, не нашли")
	}
}

// TestBuildUserPrompt_DoesNotIncludeArtifactContent — фиксирует контракт из
// комментария buildUserPrompt: "НЕ включаем artifact.Content". Передаём артефакт
// с заведомо опознаваемым content'ом и проверяем, что его в выводе нет.
func TestBuildUserPrompt_DoesNotIncludeArtifactContent(t *testing.T) {
	const canaryContent = "ARTIFACT-CONTENT-MARKER-SHOULD-NEVER-LEAK-XYZ"
	r := &RouterService{}
	state := RouterState{
		Task:   helperTask("task", "desc"),
		Agents: []*models.Agent{helperAgent("planner", "planner", "creates plans", models.AgentExecutionKindLLM)},
		Artifacts: []models.Artifact{
			{
				ID:            uuid.New(),
				TaskID:        uuid.New(),
				Kind:          models.ArtifactKindPlan,
				ProducerAgent: "planner",
				Summary:       "test plan summary",
				Status:        models.ArtifactStatusReady,
				// Content специально содержит маркер. Если он попадёт в prompt
				// — buildUserPrompt протёк (LLM получает diff'ы / тексты планов,
				// которых не должен видеть на уровне Router'а).
				Content: []byte(`{"raw":"` + canaryContent + `"}`),
			},
		},
	}
	out := r.buildUserPrompt(state, "")

	if strings.Contains(out, canaryContent) {
		t.Fatalf("LEAK: artifact.Content попал в prompt. Output:\n%s", out)
	}
	// Но summary — должен.
	if !strings.Contains(out, "test plan summary") {
		t.Fatalf("ожидали artifact.Summary в prompt, не нашли")
	}
}

// TestBuildUserPrompt_DoesNotIncludeTaskUUID — TaskID нужен системе, но НЕ нужен
// LLM в prompt (LLM работает по Title + Agents + Artifacts, ID для него — шум).
// Защита от случайного `fmt.Fprintf(&b, "task %s", task.ID)` при будущих правках.
func TestBuildUserPrompt_DoesNotIncludeTaskUUID(t *testing.T) {
	task := helperTask("title", "desc")
	r := &RouterService{}
	state := RouterState{
		Task:   task,
		Agents: []*models.Agent{helperAgent("planner", "planner", "creates plans", models.AgentExecutionKindLLM)},
	}
	out := r.buildUserPrompt(state, "")

	if strings.Contains(out, task.ID.String()) {
		t.Fatalf("LEAK: task.ID (%s) попал в prompt:\n%s", task.ID, out)
	}
}

// TestBuildUserPrompt_OmitsAgentsWithEmptyDescription — КЛЮЧЕВОЙ тест на
// cost-leak регрессию. Агенты без role_description (то, что наши смоук-тесты
// создавали 289 штук) НЕ должны попадать в "# Available Agents" каталог,
// потому что LLM не может ничего полезного выбрать из агента с пустым описанием —
// это чистый шум, который раздувает input до 7k+ токенов на каждый Router-вызов.
//
// ⚠ Этот тест СЕЙЧАС упадёт — backend (orchestrator_v2.go:284) грузит ВСЕХ
// `is_active=true` без фильтра по role_description. Когда фикс мерджнут —
// тест станет зелёным. Не маскируем багу t.Skip'ом, чтобы CI явно её показывал.
func TestBuildUserPrompt_OmitsAgentsWithEmptyDescription(t *testing.T) {
	r := &RouterService{}
	state := RouterState{
		Task: helperTask("task", "desc"),
		Agents: []*models.Agent{
			helperAgent("router", "router", "makes routing decisions", models.AgentExecutionKindLLM),
			helperAgent("planner", "planner", "creates plans", models.AgentExecutionKindLLM),
			// Мусорные leaked-агенты — должны быть отфильтрованы при подаче в prompt.
			helperAgent("ag-1111111111111111", "developer", "", models.AgentExecutionKindLLM),
			helperAgent("ag-2222222222222222", "developer", "", models.AgentExecutionKindLLM),
		},
	}
	out := r.buildUserPrompt(state, "")

	// Должны быть.
	if !strings.Contains(out, "router") {
		t.Errorf("ожидали router в каталоге, не нашли")
	}
	if !strings.Contains(out, "planner") {
		t.Errorf("ожидали planner в каталоге, не нашли")
	}
	// НЕ должны быть.
	if strings.Contains(out, "ag-1111111111111111") || strings.Contains(out, "ag-2222222222222222") {
		t.Fatalf("LEAK: leaked-агенты с пустым role_description попали в Router prompt — "+
			"это и есть cost-leak из Phase 2 review (раздувает input до 7k+ токенов). "+
			"Фикс: в orchestrator_v2.loadRouterState либо в buildUserPrompt отфильтровать "+
			"`role_description != ''`. Output:\n%s", out)
	}
}

// TestBuildUserPrompt_BoundedSizeWithManyAgents — даже если backend случайно
// загрузит 500 leaked-агентов, prompt не должен превышать здравый размер.
// Текущий buildUserPrompt просто конкатенирует — лимита нет.
//
// ⚠ Тест СЕЙЧАС упадёт — нет cap'а. Фикс: либо отфильтровать в loadRouterState,
// либо ограничить топ-N агентов в buildUserPrompt.
func TestBuildUserPrompt_BoundedSizeWithManyAgents(t *testing.T) {
	r := &RouterService{}
	manyAgents := make([]*models.Agent, 500)
	for i := range manyAgents {
		// Реалистичный сценарий cost-leak: leaked-агенты + один canonical.
		manyAgents[i] = helperAgent("ag-"+uuid.NewString()[:24], "developer", "", models.AgentExecutionKindLLM)
	}
	manyAgents = append(manyAgents, helperAgent("router", "router", "router", models.AgentExecutionKindLLM))

	state := RouterState{
		Task:   helperTask("task", "desc"),
		Agents: manyAgents,
	}
	out := r.buildUserPrompt(state, "")

	// 50KB ≈ 12k токенов — щедрый верхний предел для healthy prompt'а.
	// Выше — точно прорезаны границы.
	const maxBytes = 50 * 1024
	if len(out) > maxBytes {
		t.Fatalf("oversize: Router prompt %d bytes на 500 leaked-агентах (cap %d). "+
			"Нужен фильтр / трим в loadRouterState или buildUserPrompt.",
			len(out), maxBytes)
	}
}

// TestBuildUserPrompt_IncludesCorrectionAtEnd — corrective retry append'ит сообщение
// об ошибке для повторного запроса. Проверяем, что текст коррекции реально идёт.
func TestBuildUserPrompt_IncludesCorrectionAtEnd(t *testing.T) {
	const correction = "your previous response had an invalid agent name"
	r := &RouterService{}
	state := RouterState{
		Task:   helperTask("task", "desc"),
		Agents: []*models.Agent{helperAgent("router", "router", "x", models.AgentExecutionKindLLM)},
	}
	out := r.buildUserPrompt(state, correction)

	if !strings.Contains(out, correction) {
		t.Fatalf("correction не попала в prompt:\n%s", out)
	}
	// Sanity: correction идёт в КОНЦЕ (после response-format-инструкций).
	corrIdx := strings.LastIndex(out, correction)
	respFmtIdx := strings.Index(out, "Response Format")
	if respFmtIdx < 0 {
		t.Fatalf("response-format block отсутствует в prompt")
	}
	if corrIdx < respFmtIdx {
		t.Fatalf("correction появилась РАНЬШЕ response-format блока — нарушение порядка")
	}
}

// TestBuildUserPrompt_NoArtifactsHandled — пустой список артефактов = явный
// маркер "no artifacts yet" (LLM ничего не должен выдумывать). Sanity.
func TestBuildUserPrompt_NoArtifactsHandled(t *testing.T) {
	r := &RouterService{}
	state := RouterState{
		Task:   helperTask("task", "desc"),
		Agents: []*models.Agent{helperAgent("planner", "planner", "x", models.AgentExecutionKindLLM)},
	}
	out := r.buildUserPrompt(state, "")
	if !strings.Contains(out, "no artifacts yet") {
		t.Fatalf("ожидали маркер 'no artifacts yet', не нашли:\n%s", out)
	}
}

// TestBuildUserPrompt_DescriptionPassedAsIs — task.Description идёт в prompt
// БЕЗ scrub'инга секретов. Это документирует ТЕКУЩЕЕ поведение: если пользователь
// в Description написал свой токен — он улетит в LLM как есть.
//
// Информационный warning, не assertion — фикс scrubber'а описания вне scope
// этого слоя тестов (нужен на уровне task.Update / task.Create handler'ов).
func TestBuildUserPrompt_DescriptionPassedAsIs_LeakWarning(t *testing.T) {
	const sensitive = "ANTHROPIC_API_KEY=sk-ant-leaked-token-12345"
	r := &RouterService{}
	state := RouterState{
		Task:   helperTask("task", sensitive),
		Agents: []*models.Agent{helperAgent("planner", "planner", "x", models.AgentExecutionKindLLM)},
	}
	out := r.buildUserPrompt(state, "")

	if strings.Contains(out, sensitive) {
		// Это не Errorf — поведение сейчас именно такое (нет scrub'а Description'а
		// в buildUserPrompt). Тест фиксирует факт; реальный фикс — в handler'ах
		// task.Create/Update, где Description проходит через secrets.Scrubber до записи в БД.
		t.Logf("⚠ INFO: task.Description передаётся в LLM как есть, без scrub'а. "+
			"Если пользователь поместит токен в описание — он попадёт в prompt. "+
			"Контракт scrub'а должен быть на уровне task.Create/Update handler'ов; "+
			"buildUserPrompt сам не должен этим заниматься (он не знает что в строке секрет).")
	} else {
		t.Errorf("test setup error: ожидали что Description дойдёт до prompt'а, но он отсутствует")
	}
}
