//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// tasks_smoke_test.go — P0 lifecycle задач.
//
// Покрытие:
//   - Create → GetByID → List → Update title.
//   - Pause → Resume → Cancel; terminal-state guard на повторных pause/resume.
//   - Correct → POST /correct добавляет feedback-сообщение.
//   - Messages: POST + List, type=feedback.
//   - Cross-tenant: чужие задачи невидимы.

type taskResponse struct {
	ID            string  `json:"id"`
	ProjectID     string  `json:"project_id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Status        string  `json:"status"`
	Priority      string  `json:"priority"`
	CreatedByType string  `json:"created_by_type"`
	CreatedByID   string  `json:"created_by_id"`
	BranchName    *string `json:"branch_name,omitempty"`
}

type taskListResponse struct {
	Tasks  []taskResponse `json:"tasks"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type taskMessageResponse struct {
	ID          string `json:"id"`
	TaskID      string `json:"task_id"`
	SenderType  string `json:"sender_type"`
	SenderID    string `json:"sender_id"`
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
}

type taskMessageListResponse struct {
	Messages []taskMessageResponse `json:"messages"`
	Total    int64                 `json:"total"`
}

// createTask — helper.
func createTask(t *testing.T, h *Harness, token, projectID, title string) taskResponse {
	t.Helper()
	resp := h.Do(t, "POST", "/api/v1/projects/"+projectID+"/tasks", map[string]any{
		"title":       title,
		"description": "featuresmoke task " + title,
		"priority":    "medium",
	}, token)
	if resp.Status != http.StatusCreated && resp.Status != http.StatusOK {
		t.Fatalf("create task: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var out taskResponse
	resp.JSON(t, &out)
	if out.ID == "" {
		t.Fatalf("create task: пустой id: %s", truncBody(resp.Body))
	}
	if out.ProjectID != projectID {
		t.Fatalf("create task: project_id=%q ожидали %q", out.ProjectID, projectID)
	}
	return out
}

// TestTasks_CRUDLifecycle — happy path жизненного цикла:
// create → list → get → pause → resume → cancel.
func TestTasks_CRUDLifecycle(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	task := createTask(t, h, user.AccessToken, p.ID, "smoke-"+uuid.NewString())
	// Свежесозданная задача = state="active".
	if task.Status != "active" {
		t.Fatalf("create: status=%q ожидали active", task.Status)
	}

	// GET /tasks/:id
	getResp := h.Do(t, "GET", "/api/v1/tasks/"+task.ID, nil, user.AccessToken)
	if getResp.Status != http.StatusOK {
		t.Fatalf("get task: status=%d body=%s", getResp.Status, truncBody(getResp.Body))
	}

	// LIST /projects/:id/tasks — задача присутствует.
	listResp := h.Do(t, "GET", "/api/v1/projects/"+p.ID+"/tasks", nil, user.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("list tasks: status=%d", listResp.Status)
	}
	var list taskListResponse
	listResp.JSON(t, &list)
	found := false
	for _, x := range list.Tasks {
		if x.ID == task.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list tasks: созданная задача %s не найдена среди %d", task.ID, list.Total)
	}

	// UPDATE — меняем title.
	newTitle := "renamed-" + uuid.NewString()
	upResp := h.Do(t, "PUT", "/api/v1/tasks/"+task.ID,
		map[string]any{"title": newTitle}, user.AccessToken)
	if upResp.Status != http.StatusOK {
		t.Fatalf("update task: status=%d body=%s", upResp.Status, truncBody(upResp.Body))
	}
	var updated taskResponse
	upResp.JSON(t, &updated)
	if updated.Title != newTitle {
		t.Fatalf("update task: title=%q ожидали %q", updated.Title, newTitle)
	}

	// PAUSE — active → paused.
	pauseResp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/pause", nil, user.AccessToken)
	if pauseResp.Status != http.StatusOK {
		t.Fatalf("pause: status=%d body=%s", pauseResp.Status, truncBody(pauseResp.Body))
	}
	var paused taskResponse
	pauseResp.JSON(t, &paused)
	if paused.Status != "paused" {
		t.Fatalf("pause: status=%q ожидали paused", paused.Status)
	}

	// Повторный pause — terminal или invalid_transition (paused → paused запрещён).
	pause2 := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/pause", nil, user.AccessToken)
	if pause2.Status >= 500 {
		t.Fatalf("second pause: server error status=%d body=%s",
			pause2.Status, truncBody(pause2.Body))
	}
	if pause2.Status == http.StatusOK {
		t.Fatalf("second pause: status=200 (ожидали 4xx — paused→paused запрещён)")
	}

	// RESUME — paused → active.
	resResp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/resume", nil, user.AccessToken)
	if resResp.Status != http.StatusOK {
		t.Fatalf("resume: status=%d body=%s", resResp.Status, truncBody(resResp.Body))
	}
	var resumed taskResponse
	resResp.JSON(t, &resumed)
	if resumed.Status != "active" {
		t.Fatalf("resume: status=%q ожидали active", resumed.Status)
	}

	// CANCEL — active → cancelled. ВАЖНО: между resume и cancel может вклиниться
	// v2 step-worker (PollInterval=500ms), увидеть active-task без агентов и
	// финализировать её (failed/done). Тогда наш cancel получит 409 task_already_terminal —
	// это валидное состояние, не баг.
	cancelResp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/cancel", nil, user.AccessToken)
	switch cancelResp.Status {
	case http.StatusOK:
		var cancelled taskResponse
		cancelResp.JSON(t, &cancelled)
		if cancelled.Status != "cancelled" {
			t.Fatalf("cancel: status=%q ожидали cancelled", cancelled.Status)
		}
	case http.StatusConflict:
		// worker уже завершил задачу — допустимый исход.
		t.Logf("cancel: 409 task_already_terminal (worker завершил задачу раньше)")
	default:
		t.Fatalf("cancel: status=%d body=%s", cancelResp.Status, truncBody(cancelResp.Body))
	}

	// После terminal-состояния — pause должен ответить 409/400.
	pauseAfter := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/pause", nil, user.AccessToken)
	if pauseAfter.Status != http.StatusConflict && pauseAfter.Status != http.StatusBadRequest {
		t.Fatalf("pause after terminal: status=%d (ожидали 409/400)", pauseAfter.Status)
	}
}

// TestTasks_CancelImmediately — отдельный тест на cancel: создаём задачу и сразу
// её отменяем, до того как worker сможет её подхватить. Без resume — orchestrator
// не имеет шанса финализировать paused/needs_human задачу за пределами нашего контроля.
func TestTasks_CancelImmediately(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	task := createTask(t, h, user.AccessToken, p.ID, "cancel-"+uuid.NewString())
	cancelResp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/cancel", nil, user.AccessToken)
	// Race с v2 worker'ом: либо мы успели (200 cancelled), либо worker уже
	// финализировал (409). 5xx — баг.
	if cancelResp.Status >= 500 {
		t.Fatalf("cancel: server error status=%d body=%s",
			cancelResp.Status, truncBody(cancelResp.Body))
	}
	if cancelResp.Status != http.StatusOK && cancelResp.Status != http.StatusConflict {
		t.Fatalf("cancel: status=%d (ожидали 200 или 409 task_already_terminal)",
			cancelResp.Status)
	}
}

// TestTasks_CorrectAddsFeedbackMessage — POST /correct => появляется сообщение
// в /messages.
func TestTasks_CorrectAddsFeedbackMessage(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "correct-"+uuid.NewString())

	const correction = "пожалуйста, обнови README"
	resp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/correct",
		map[string]any{"text": correction}, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("correct: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}

	// Сообщения должны существовать. correct превращает текст в feedback-сообщение.
	listResp := h.Do(t, "GET", "/api/v1/tasks/"+task.ID+"/messages", nil, user.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("list messages: status=%d body=%s", listResp.Status, truncBody(listResp.Body))
	}
	var msgs taskMessageListResponse
	listResp.JSON(t, &msgs)
	if msgs.Total == 0 {
		t.Fatalf("correct: сообщений нет, ожидался хотя бы один (feedback)")
	}
}

// TestTasks_AddAndListMessages — POST /messages создаёт запись, она видна в GET.
func TestTasks_AddAndListMessages(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "msg-"+uuid.NewString())

	const content = "hello from smoke test"
	addResp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/messages", map[string]any{
		"content":      content,
		"message_type": "feedback",
	}, user.AccessToken)
	if addResp.Status != http.StatusCreated && addResp.Status != http.StatusOK {
		t.Fatalf("add message: status=%d body=%s", addResp.Status, truncBody(addResp.Body))
	}

	listResp := h.Do(t, "GET", "/api/v1/tasks/"+task.ID+"/messages", nil, user.AccessToken)
	if listResp.Status != http.StatusOK {
		t.Fatalf("list messages: status=%d", listResp.Status)
	}
	var msgs taskMessageListResponse
	listResp.JSON(t, &msgs)
	found := false
	for _, m := range msgs.Messages {
		if m.Content == content && m.MessageType == "feedback" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("list messages: только что добавленное сообщение не найдено (total=%d)", msgs.Total)
	}
}

// TestTasks_InvalidMessageTypeReturns400.
func TestTasks_InvalidMessageTypeReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)
	task := createTask(t, h, user.AccessToken, p.ID, "bad-msg-"+uuid.NewString())

	resp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/messages", map[string]any{
		"content":      "x",
		"message_type": "not-a-real-type",
	}, user.AccessToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("bad message type: status=%d (ожидали 400) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestTasks_CrossTenantTaskInvisible.
func TestTasks_CrossTenantTaskInvisible(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)
	p := createLocalProject(t, h, alice.AccessToken)
	task := createTask(t, h, alice.AccessToken, p.ID, "private-"+uuid.NewString())

	// Bob дёргает /tasks/:id Alice'ин.
	bobGet := h.Do(t, "GET", "/api/v1/tasks/"+task.ID, nil, bob.AccessToken)
	if bobGet.Status != http.StatusNotFound && bobGet.Status != http.StatusForbidden {
		t.Fatalf("cross-tenant task get: status=%d (ожидали 404/403)", bobGet.Status)
	}

	// Bob дёргает project tasks list — проекта Алисы не видит, должен 404.
	bobList := h.Do(t, "GET", "/api/v1/projects/"+p.ID+"/tasks", nil, bob.AccessToken)
	if bobList.Status != http.StatusNotFound && bobList.Status != http.StatusForbidden {
		t.Fatalf("cross-tenant tasks list: status=%d", bobList.Status)
	}
}

// TestTasks_RequireTitle — без title = 400.
func TestTasks_RequireTitle(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	resp := h.Do(t, "POST", "/api/v1/projects/"+p.ID+"/tasks",
		map[string]any{"description": "no title"}, user.AccessToken)
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("no title: status=%d (ожидали 400)", resp.Status)
	}
}
