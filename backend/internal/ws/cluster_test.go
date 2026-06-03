package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// marshalEnvelope — хелпер для сборки сырого сообщения, как его кладёт publish().
func marshalEnvelope(t *testing.T, origin, scope, key, msgType string, payload []byte) string {
	t.Helper()
	data, err := json.Marshal(clusterEnvelope{
		Origin:  origin,
		Scope:   scope,
		Key:     key,
		Type:    msgType,
		Payload: json.RawMessage(payload),
	})
	require.NoError(t, err)
	return string(data)
}

// assertNoMessage — клиент НЕ должен ничего получить в течение короткого окна.
func assertNoMessage(t *testing.T, c *Client) {
	t.Helper()
	select {
	case got := <-c.Send:
		t.Fatalf("expected no message, got %q", string(got))
	case <-time.After(100 * time.Millisecond):
	}
}

// handle с чужим origin и scope=project должен ре-доставить payload локальным клиентам проекта.
func TestClusterBridge_Handle_ProjectScope_DeliversToLocalClients(t *testing.T) {
	h, _ := newTestHub(t)
	client := newFakeClient(t, "c1", 4)
	registerSynced(h, client, []string{"p1"})

	bridge := NewClusterBridge(nil, h, "self-instance", nil)
	payload := []byte(`{"type":"task_status","project_id":"p1"}`)
	bridge.handle(marshalEnvelope(t, "other-instance", wsScopeProject, "p1", "task_status", payload))

	require.Equal(t, payload, recvOne(t, client))
}

// Сообщения с собственным origin уже доставлены локально в SendTo* — handle их обязан игнорировать
// (иначе клиент получит дубль, а при наивной реализации образуется петля).
func TestClusterBridge_Handle_SuppressesOwnEcho(t *testing.T) {
	h, _ := newTestHub(t)
	client := newFakeClient(t, "c1", 4)
	registerSynced(h, client, []string{"p1"})

	bridge := NewClusterBridge(nil, h, "self-instance", nil)
	payload := []byte(`{"type":"task_status","project_id":"p1"}`)
	bridge.handle(marshalEnvelope(t, "self-instance", wsScopeProject, "p1", "task_status", payload))

	assertNoMessage(t, client)
}

// handle с чужим origin и scope=user должен ре-доставить payload всем клиентам пользователя.
func TestClusterBridge_Handle_UserScope_DeliversToUserClients(t *testing.T) {
	h, _ := newTestHub(t)
	client := newFakeClient(t, "c1", 4) // UserID == "user-c1" (см. newFakeClient)
	registerSynced(h, client, []string{"p1"})

	bridge := NewClusterBridge(nil, h, "self-instance", nil)
	payload := []byte(`{"type":"integration_status"}`)
	bridge.handle(marshalEnvelope(t, "other-instance", wsScopeUser, "user-c1", "integration_status", payload))

	require.Equal(t, payload, recvOne(t, client))
}

// Неизвестный scope и битый JSON не должны паниковать и не доставляют ничего.
func TestClusterBridge_Handle_IgnoresUnknownScopeAndGarbage(t *testing.T) {
	h, _ := newTestHub(t)
	client := newFakeClient(t, "c1", 4)
	registerSynced(h, client, []string{"p1"})

	bridge := NewClusterBridge(nil, h, "self-instance", nil)
	bridge.handle(marshalEnvelope(t, "other-instance", "bogus-scope", "p1", "x", []byte(`{}`)))
	bridge.handle("not-json-at-all")

	assertNoMessage(t, client)
}

// SendToProject в одноинстансном режиме (cluster == nil) работает как раньше: локальная доставка,
// без обращения к Redis. Защищает от регресса рефактора enqueueProject/SendToProject.
func TestHub_SendToProject_NoClusterStillDeliversLocally(t *testing.T) {
	h, _ := newTestHub(t)
	client := newFakeClient(t, "c1", 4)
	registerSynced(h, client, []string{"p1"})

	payload := []byte(`{"type":"task_status"}`)
	require.NoError(t, h.SendToProject("p1", "task_status", payload))
	assert.Equal(t, payload, recvOne(t, client))
}
