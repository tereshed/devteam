//go:build featuresmoke

package featuresmoke

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ws_smoke_test.go — P0 WebSocket-стрим для /api/v1/projects/:id/ws.
//
// Контракт (internal/ws/types.go):
//   - Envelope: { type, v, ts, project_id, data }
//   - Project-scoped события: task_status, task_message, agent_log, error
//
// В smoke-режиме мы валидируем:
//   1) хендшейк с валидным токеном проходит, без токена — 401;
//   2) при создании task'а приходит хотя бы один envelope с непустым type
//      и project_id, совпадающим с проектом;
//   3) при pause'е task'а — приходит task_status с status=paused.

type wsEnvelope struct {
	Type      string          `json:"type"`
	Version   int             `json:"v"`
	Timestamp time.Time       `json:"ts"`
	ProjectID string          `json:"project_id"`
	UserID    string          `json:"user_id,omitempty"`
	Data      json.RawMessage `json:"data"`
}

// awaitEnvelopeOfType читает сообщения, пока не встретит нужный type, или
// падает по deadline. Полезен на пути «create task → ждём task_status»: туда
// прилетают и другие envelope'ы (assistant.task_update и т.п.), которые надо
// проскипать.
//
// Контракт чтения:
//   - SetReadDeadline-ошибка → t.Fatalf (показывает реальную проблему сокета,
//     не маскируем под таймаут);
//   - не-TextMessage фреймы (ping, бинарные) → continue;
//   - TextMessage с невалидным JSON → t.Fatalf (бэкенд обязан слать JSON-envelope'ы;
//     любой битый кадр — баг, который надо чинить, а не проглатывать).
func awaitEnvelopeOfType(t *testing.T, conn *websocket.Conn, wantType string, total time.Duration) wsEnvelope {
	t.Helper()
	deadline := time.Now().Add(total)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("ws: не дождались envelope type=%q за %s", wantType, total)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			t.Fatalf("ws: deadline expired for type=%q", wantType)
		}
		if err := conn.SetReadDeadline(time.Now().Add(remaining)); err != nil {
			t.Fatalf("ws SetReadDeadline (waiting for %q): %v", wantType, err)
		}
		msgType, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ws read while waiting for %q: %v", wantType, err)
		}
		// Бинарные фреймы / control-frame'ы (ping/pong/close gorilla'е сам
		// обрабатывает в ReadMessage). На уровне приложения мы ждём TextMessage.
		if msgType != websocket.TextMessage {
			continue
		}
		var env wsEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatalf("ws: невалидный JSON в TextMessage (ожидали envelope): %v raw=%s",
				err, truncBody(raw))
		}
		if env.Type == wantType {
			return env
		}
	}
}

// TestWS_ConnectAuthorizedReceivesTaskStatusOnCreate — после создания таски
// прилетает task_status (active) или task_message с правильным project_id.
func TestWS_ConnectAuthorizedReceivesTaskStatusOnCreate(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	conn := h.WS(t, user.AccessToken, p.ID)

	// Создаём задачу — backend опубликует task_status (active).
	task := createTask(t, h, user.AccessToken, p.ID, "ws-"+uuid.NewString())

	env := awaitEnvelopeOfType(t, conn, "task_status", 10*time.Second)
	if env.Version != 1 {
		t.Fatalf("ws envelope v=%d ожидали 1", env.Version)
	}
	if env.ProjectID != p.ID {
		t.Fatalf("ws envelope project_id=%q ожидали %q", env.ProjectID, p.ID)
	}

	var data struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("ws task_status data decode: %v", err)
	}
	if data.TaskID != task.ID {
		t.Fatalf("ws task_status: task_id=%q ожидали %q", data.TaskID, task.ID)
	}
}

// TestWS_PauseEmitsTaskStatusPaused — после POST /pause приходит task_status
// со status=paused.
func TestWS_PauseEmitsTaskStatusPaused(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	conn := h.WS(t, user.AccessToken, p.ID)

	task := createTask(t, h, user.AccessToken, p.ID, "ws-pause-"+uuid.NewString())
	// Дренируем «active»-стартовый envelope, чтобы не путал последующее ожидание.
	_ = awaitEnvelopeOfType(t, conn, "task_status", 10*time.Second)

	resp := h.Do(t, "POST", "/api/v1/tasks/"+task.ID+"/pause", nil, user.AccessToken)
	if resp.Status != http.StatusOK {
		t.Fatalf("pause: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}

	env := awaitEnvelopeOfType(t, conn, "task_status", 10*time.Second)
	var data struct {
		TaskID string `json:"task_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("ws pause envelope decode: %v", err)
	}
	if data.Status != "paused" {
		t.Fatalf("ws pause envelope status=%q ожидали paused", data.Status)
	}
	if data.TaskID != task.ID {
		t.Fatalf("ws pause envelope task_id=%q ожидали %q", data.TaskID, task.ID)
	}
}

// TestWS_RejectsConnectionWithoutToken — без Bearer = HTTP 401 в апгрейде.
func TestWS_RejectsConnectionWithoutToken(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	p := createLocalProject(t, h, user.AccessToken)

	// Без Authorization — Dial должен упасть с 401 в response.
	wsURL := "ws://" + h.BaseURL[len("http://"):] + "/api/v1/projects/" + p.ID + "/ws"
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, resp, err := dialer.Dial(wsURL, http.Header{})
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatalf("ws dial without token: ожидали ошибку, получили connect")
	}
	if resp == nil {
		t.Fatalf("ws dial without token: response=nil err=%v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ws dial without token: status=%d (ожидали 401)", resp.StatusCode)
	}
}

// TestWS_RejectsCrossTenantProject — Bob подключается к WS Alice'ого проекта.
func TestWS_RejectsCrossTenantProject(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	alice := h.NewUser(t)
	bob := h.NewUser(t)
	p := createLocalProject(t, h, alice.AccessToken)

	wsURL := "ws://" + h.BaseURL[len("http://"):] + "/api/v1/projects/" + p.ID + "/ws"
	header := http.Header{}
	header.Set("Authorization", "Bearer "+bob.AccessToken)
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, resp, err := dialer.Dial(wsURL, header)
	if conn != nil {
		_ = conn.Close()
	}
	if err == nil {
		t.Fatalf("ws cross-tenant: ожидали ошибку, получили connect")
	}
	if resp == nil {
		t.Fatalf("ws cross-tenant: response=nil err=%v", err)
	}
	if resp.StatusCode != http.StatusForbidden &&
		resp.StatusCode != http.StatusNotFound &&
		resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("ws cross-tenant: status=%d (ожидали 401/403/404)", resp.StatusCode)
	}
}
