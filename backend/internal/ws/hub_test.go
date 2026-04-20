package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func newTestHub(t *testing.T) (*Hub, context.CancelFunc) {
	t.Helper()
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	t.Cleanup(func() {
		cancel()
		<-h.done
		goleak.VerifyNone(t)
	})
	return h, cancel
}

func newFakeClient(t *testing.T, id string, sendBuf int) *Client {
	t.Helper()
	return &Client{
		ID:     id,
		UserID: "user-" + id,
		Conn:   nil,
		Send:   make(chan []byte, sendBuf),
		Hub:    nil,
	}
}

// registerSynced — регистрация с ожиданием завершения addClient в Run (устраняет гонку «send unblocks до case body» у unbuffered register).
func registerSynced(h *Hub, c *Client, projectIDs []string) {
	ack := make(chan struct{})
	select {
	case h.register <- &RegisterMessage{Client: c, ProjectIDs: projectIDs, ack: ack}:
		<-ack
	case <-h.done:
	}
}

func recvOne(t *testing.T, c *Client) []byte {
	t.Helper()
	var got []byte
	assert.Eventually(t, func() bool {
		select {
		case got = <-c.Send:
			return true
		default:
			return false
		}
	}, 5*time.Second, time.Millisecond, "expected a message on client.Send")
	return got
}

// assertEventuallySendClosed ждёт, пока очередь Send опустеет и канал закроется (removeClient после slow-path).
func assertEventuallySendClosed(t *testing.T, c *Client) {
	t.Helper()
	assert.Eventually(t, func() bool {
		for {
			select {
			case _, ok := <-c.Send:
				if !ok {
					return true
				}
			default:
				return false
			}
		}
	}, 5*time.Second, time.Millisecond)
}

// --- A. Init & Lifecycle ---

func TestNewHub_InitializesEmptyState(t *testing.T) {
	defer goleak.VerifyNone(t)
	h := NewHub()
	require.NotNil(t, h.projects)
	require.NotNil(t, h.clientProjects)
	require.NotNil(t, h.clientsByID)
	require.NotNil(t, h.userConnCounts)
	require.Empty(t, h.projects)
	require.Empty(t, h.clientProjects)
	require.Empty(t, h.clientsByID)
	require.Empty(t, h.userConnCounts)
	require.Equal(t, 0, len(h.broadcast))
	require.Equal(t, 0, len(h.unicast))
}

func TestRun_BlocksUntilContextCancelled(t *testing.T) {
	defer goleak.VerifyNone(t)
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	select {
	case <-h.done:
		t.Fatal("Run must not exit before cancel")
	default:
	}
	cancel()
	assert.Eventually(t, func() bool {
		select {
		case <-h.done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond)
}

func TestRun_ShutdownClosesAllSendChannels(t *testing.T) {
	h, cancel := newTestHub(t)
	c := newFakeClient(t, "c1", 16)
	registerSynced(h, c, []string{"p1"})
	require.NoError(t, h.SendToProject("p1", "ping", []byte("sync")))
	recvOne(t, c)

	cancel()
	<-h.done

	for {
		_, ok := <-c.Send
		if !ok {
			break
		}
	}
}

func TestRun_ShutdownSendsCloseMessage(t *testing.T) {
	h, cancel := newTestHub(t)
	c := newFakeClient(t, "c1", 64)
	registerSynced(h, c, []string{"p1"})
	require.NoError(t, h.SendToProject("p1", "ping", []byte("sync")))
	recvOne(t, c)

	cancel()

	var raw []byte
	assert.Eventually(t, func() bool {
		select {
		case raw = <-c.Send:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond)

	var env map[string]any
	require.NoError(t, json.Unmarshal(raw, &env))
	require.Equal(t, "close", env["type"])
	require.Equal(t, "server_shutdown", env["reason"])
}

// --- B. Register / Unregister ---

func TestRegister_AddsClientToProject(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1"})
	require.NoError(t, h.SendToProject("p1", "t", []byte("x")))
	require.Equal(t, []byte("x"), recvOne(t, c))

	// Register синхронизирует с Run (unbuffered канал); далее Hub ждёт на select — чтение индексов без гонки с мутациями.
	require.Contains(t, h.projects["p1"], c)
	require.Same(t, c, h.clientsByID["c1"])
	require.True(t, h.clientProjects[c]["p1"])
}

func TestRegister_AddsClientToMultipleProjects(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1", "p2", "p3"})

	require.Contains(t, h.projects["p1"], c)
	require.Contains(t, h.projects["p2"], c)
	require.Contains(t, h.projects["p3"], c)
	require.True(t, h.clientProjects[c]["p1"])
	require.True(t, h.clientProjects[c]["p2"])
	require.True(t, h.clientProjects[c]["p3"])
}

func TestUnregister_RemovesClientFromAllProjects(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1", "p2"})
	h.Unregister(c)

	_, ok := <-c.Send
	require.False(t, ok)

	// После Unregister канал Send закрыт; unicast на удалённый ID — no-op без паники.
	require.NoError(t, h.SendToClient("c1", "u", []byte("ghost")))
}

func TestUnregister_Idempotent(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1"})
	h.Unregister(c)
	h.Unregister(c)
}

func TestRegister_AfterShutdown_DoesNotBlock(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done

	done := make(chan struct{})
	go func() {
		c := newFakeClient(t, "late", 1)
		registerSynced(h, c, []string{"p1"})
		close(done)
	}()

	assert.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "Register blocked after shutdown")
	goleak.VerifyNone(t)
}

func TestUnregister_AfterShutdown_DoesNotBlock(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	c := newFakeClient(t, "c1", 1)
	registerSynced(h, c, []string{"p1"})
	cancel()
	<-h.done

	done := make(chan struct{})
	go func() {
		h.Unregister(c)
		close(done)
	}()

	assert.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "Unregister blocked after shutdown")
	goleak.VerifyNone(t)
}

// --- C. Broadcast ---

func TestSendToProject_DeliversToAllSubscribers(t *testing.T) {
	h, _ := newTestHub(t)
	payload := []byte("payload-all")
	clients := []*Client{
		newFakeClient(t, "a", 8),
		newFakeClient(t, "b", 8),
		newFakeClient(t, "c", 8),
	}
	for _, cl := range clients {
		registerSynced(h, cl, []string{"p1"})
	}
	require.NoError(t, h.SendToProject("p1", "evt", payload))
	for _, cl := range clients {
		require.Equal(t, payload, recvOne(t, cl))
	}
}

func TestSendToProject_DoesNotDeliverToOtherProjects(t *testing.T) {
	h, _ := newTestHub(t)
	c1 := newFakeClient(t, "c1", 8)
	c2 := newFakeClient(t, "c2", 8)
	registerSynced(h, c1, []string{"p1"})
	registerSynced(h, c2, []string{"p2"})
	require.NoError(t, h.SendToProject("p1", "evt", []byte("only-p1")))
	require.Equal(t, []byte("only-p1"), recvOne(t, c1))

	select {
	case <-c2.Send:
		t.Fatal("p2 client must not receive p1 broadcast")
	default:
	}
}

func TestSendToProject_NoSubscribers_NoOp(t *testing.T) {
	h, _ := newTestHub(t)
	require.NoError(t, h.SendToProject("ghost", "evt", []byte("x")))
}

func TestSendToProject_EmptyProjectID_ReturnsError(t *testing.T) {
	h, _ := newTestHub(t)
	require.ErrorIs(t, h.SendToProject("", "t", []byte("x")), ErrEmptyProjectID)
}

func TestSendToProject_BroadcastChannelFull_DropsSilently(t *testing.T) {
	h := NewHub()
	for {
		require.NoError(t, h.SendToProject("p1", "f", []byte("fill")))
		if len(h.broadcast) >= cap(h.broadcast) {
			break
		}
	}
	n := len(h.broadcast)
	require.NoError(t, h.SendToProject("p1", "drop", []byte("z")))
	require.Equal(t, n, len(h.broadcast))

	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done
	goleak.VerifyNone(t)
}

func TestSendToProject_PayloadIntegrity(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1"})
	want := []byte{0x00, 0xff, 0x01, 0x02}
	require.NoError(t, h.SendToProject("p1", "bin", want))
	require.Equal(t, want, recvOne(t, c))
}

func TestSendToProject_MultipleProjectsRouting(t *testing.T) {
	type routeRow struct {
		name      string
		targetPID string
		payload   string
		wantIDs   []string
	}
	h, _ := newTestHub(t)
	byID := map[string]*Client{
		"c1": newFakeClient(t, "c1", 8),
		"c2": newFakeClient(t, "c2", 8),
		"c3": newFakeClient(t, "c3", 8),
		"c4": newFakeClient(t, "c4", 8),
		"c5": newFakeClient(t, "c5", 8),
	}
	registerSynced(h, byID["c1"], []string{"A"})
	registerSynced(h, byID["c2"], []string{"A", "B"})
	registerSynced(h, byID["c3"], []string{"B", "C"})
	registerSynced(h, byID["c4"], []string{"C"})
	registerSynced(h, byID["c5"], []string{"A", "C"})

	rows := []routeRow{
		{"to A", "A", "m1", []string{"c1", "c2", "c5"}},
		{"to B", "B", "m2", []string{"c2", "c3"}},
		{"to C", "C", "m3", []string{"c3", "c4", "c5"}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			require.NoError(t, h.SendToProject(row.targetPID, "t", []byte(row.payload)))
			for _, id := range row.wantIDs {
				cl := byID[id]
				require.Equal(t, row.payload, string(recvOne(t, cl)), "client %s", id)
			}
		})
	}
}

func TestSendToProject_ConcurrentSenders(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 256)
	registerSynced(h, c, []string{"p1"})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = h.SendToProject("p1", "t", []byte(strconv.Itoa(i)))
		}(i)
	}
	wg.Wait()

	received := 0
	assert.Eventually(t, func() bool {
		for {
			select {
			case <-c.Send:
				received++
				if received == 100 {
					return true
				}
			default:
				return false
			}
		}
	}, 5*time.Second, time.Millisecond, "expected 100 deliveries, got %d", received)
}

// --- D. Unicast ---

func TestSendToClient_DeliversToSpecificClient(t *testing.T) {
	h, _ := newTestHub(t)
	a := newFakeClient(t, "a", 8)
	b := newFakeClient(t, "b", 8)
	registerSynced(h, a, []string{"p1"})
	registerSynced(h, b, []string{"p1"})
	require.NoError(t, h.SendToClient("b", "u", []byte("only-b")))
	require.Equal(t, []byte("only-b"), recvOne(t, b))
	select {
	case <-a.Send:
		t.Fatal("a must not receive unicast for b")
	default:
	}
}

func TestSendToClient_UnknownClientID_NoOp(t *testing.T) {
	h, _ := newTestHub(t)
	require.NoError(t, h.SendToClient("missing", "u", []byte("x")))
}

func TestSendToClient_EmptyClientID_ReturnsErrEmptyClientID(t *testing.T) {
	h, _ := newTestHub(t)
	require.ErrorIs(t, h.SendToClient("", "u", []byte("x")), ErrEmptyClientID)
}

func TestSendToClient_UnicastChannelFull_DropsSilently(t *testing.T) {
	h := NewHub()
	for {
		require.NoError(t, h.SendToClient("x", "f", []byte("fill")))
		if len(h.unicast) >= cap(h.unicast) {
			break
		}
	}
	n := len(h.unicast)
	require.NoError(t, h.SendToClient("x", "d", []byte("z")))
	require.Equal(t, n, len(h.unicast))

	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done
	goleak.VerifyNone(t)
}

func TestSendToClient_AfterUnregister_NoDelivery(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1"})
	h.Unregister(c)
	require.NoError(t, h.SendToClient("c1", "u", []byte("late")))
}

// --- E. Slow client ---

func TestSlowClient_GetsDisconnectedOnFullSend(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "slow", 1)
	registerSynced(h, c, []string{"p1"})
	// Два сообщения при buf=1 снимают клиента; часть SendToProject может дропнуться при заполненном
	// broadcast-канале, поэтому шлём серию — пока не сработает removeClient.
	for i := 0; i < 32; i++ {
		require.NoError(t, h.SendToProject("p1", "m", []byte(strconv.Itoa(i))))
	}
	assertEventuallySendClosed(t, c)
}

func TestSlowClient_DoesNotBlockOtherClients(t *testing.T) {
	h, _ := newTestHub(t)
	slow := newFakeClient(t, "slow", 1)
	f1 := newFakeClient(t, "f1", 64)
	f2 := newFakeClient(t, "f2", 64)
	registerSynced(h, slow, []string{"p1"})
	registerSynced(h, f1, []string{"p1"})
	registerSynced(h, f2, []string{"p1"})

	for i := 0; i < 32; i++ {
		require.NoError(t, h.SendToProject("p1", "m", []byte(strconv.Itoa(i))))
	}
	drainClientSend := func(cl *Client) {
		for {
			select {
			case _, ok := <-cl.Send:
				if !ok {
					return
				}
			default:
				return
			}
		}
	}
	drainClientSend(slow)

	assertEventuallySendClosed(t, slow)

	require.NoError(t, h.SendToProject("p1", "m", []byte("c")))

	waitForC := func(cl *Client) {
		for {
			select {
			case msg, ok := <-cl.Send:
				if !ok {
					return
				}
				if string(msg) == "c" {
					return
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for 'c'")
			}
		}
	}
	waitForC(f1)
	waitForC(f2)
}

func TestSlowClient_DoesNotBlockHubLoop(t *testing.T) {
	h, _ := newTestHub(t)
	slow := newFakeClient(t, "slow", 1)
	registerSynced(h, slow, []string{"p1"})
	for i := 0; i < 32; i++ {
		require.NoError(t, h.SendToProject("p1", "m", []byte(strconv.Itoa(i))))
	}
	drainSlow := func() {
		for {
			select {
			case _, ok := <-slow.Send:
				if !ok {
					return
				}
			default:
				return
			}
		}
	}
	drainSlow()
	assertEventuallySendClosed(t, slow)

	fast := newFakeClient(t, "fast", 8)
	registerSynced(h, fast, []string{"p2"})
	require.NoError(t, h.SendToProject("p2", "m", []byte("ok")))
	require.Equal(t, []byte("ok"), recvOne(t, fast))
}

func TestSlowClient_RemoveIsAtomic(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "slow", 0)
	registerSynced(h, c, []string{"p1"})
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = h.SendToClient("slow", "u", []byte("x"))
			}
		}()
	}
	wg.Wait()
	assertEventuallySendClosed(t, c)
}

func TestSlowClient_OnUnicast_AlsoDisconnects(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "u1", 1)
	registerSynced(h, c, []string{"p1"})
	require.NoError(t, h.SendToClient("u1", "a", []byte("1")))
	require.NoError(t, h.SendToClient("u1", "b", []byte("2")))
	recvOne(t, c)
	assertEventuallySendClosed(t, c)
}

// --- F. Concurrency ---

func TestConcurrent_RegisterUnregisterBroadcast(t *testing.T) {
	h, _ := newTestHub(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		id := strconv.Itoa(i)
		go func() {
			defer wg.Done()
			cl := newFakeClient(t, id, 32)
			h.Register(cl, []string{"px"})
		}()
		go func() {
			defer wg.Done()
			cl := newFakeClient(t, id, 32)
			h.Unregister(cl)
		}()
		go func() {
			defer wg.Done()
			_ = h.SendToProject("px", "t", []byte("b"))
		}()
	}
	wg.Wait()
}

func TestConcurrent_DoubleUnregisterFromMultipleGoroutines(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1"})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.Unregister(c)
		}()
	}
	wg.Wait()
}

func TestConcurrent_RegisterDuringShutdown(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cl := newFakeClient(t, strconv.Itoa(1000+i), 2)
			h.Register(cl, []string{"p1"})
		}(i)
	}
	cancel()
	wg.Wait()
	<-h.done
	goleak.VerifyNone(t)
}

func TestConcurrent_SendToProjectVsRegister(t *testing.T) {
	h, _ := newTestHub(t)
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(2)
		id := "cl-" + strconv.Itoa(i)
		go func() {
			defer wg.Done()
			cl := newFakeClient(t, id, 16)
			h.Register(cl, []string{"mix"})
		}()
		go func() {
			defer wg.Done()
			_ = h.SendToProject("mix", "t", []byte("x"))
		}()
	}
	wg.Wait()
}

func TestConcurrent_NoGoroutineLeak(t *testing.T) {
	h, _ := newTestHub(t)
	for round := 0; round < 20; round++ {
		c := newFakeClient(t, fmt.Sprintf("c%d", round), 8)
		registerSynced(h, c, []string{"p1"})
		require.NoError(t, h.SendToProject("p1", "t", []byte("x")))
		recvOne(t, c)
		h.Unregister(c)
	}
}

// --- G. Edge cases (table-driven where noted) ---

func TestEdge_RegisterWithEmptyProjectIDList(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "solo", 4)
	registerSynced(h, c, []string{})
	require.NoError(t, h.SendToProject("any", "t", []byte("x")))
	select {
	case <-c.Send:
		t.Fatal("client without projects must not get broadcast")
	default:
	}
	require.NoError(t, h.SendToClient("solo", "u", []byte("direct")))
	require.Equal(t, []byte("direct"), recvOne(t, c))
	require.Empty(t, h.clientProjects[c])
}

func TestEdge_RegisterWithDuplicateProjectIDs(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 4)
	registerSynced(h, c, []string{"p1", "p1", "p1"})
	require.Len(t, h.projects["p1"], 1)
	require.True(t, h.clientProjects[c]["p1"])
}

func TestEdge_RegisterSameClientTwice(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 8)
	registerSynced(h, c, []string{"p1"})
	registerSynced(h, c, []string{"p2"})
	require.True(t, h.clientProjects[c]["p1"])
	require.True(t, h.clientProjects[c]["p2"])
	require.Contains(t, h.projects["p1"], c)
	require.Contains(t, h.projects["p2"], c)
}

func TestEdge_BroadcastBeforeRun(t *testing.T) {
	h := NewHub()
	for {
		require.NoError(t, h.SendToProject("p1", "f", []byte("x")))
		if len(h.broadcast) >= cap(h.broadcast) {
			break
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done
	goleak.VerifyNone(t)
}

func TestEdge_BroadcastAfterShutdown(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done
	require.NoError(t, h.SendToProject("p1", "t", []byte("late")))
	goleak.VerifyNone(t)
}

func TestEdge_NilPayload(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "c1", 4)
	registerSynced(h, c, []string{"p1"})
	require.NoError(t, h.SendToProject("p1", "t", nil))
	got := recvOne(t, c)
	require.Nil(t, got)
}

func TestEdge_RegisterWithMassiveProjectList(t *testing.T) {
	h, _ := newTestHub(t)
	ids := make([]string, 100_000)
	for i := range ids {
		ids[i] = "proj-" + strconv.Itoa(i)
	}
	c := newFakeClient(t, "big", 8)
	registerSynced(h, c, ids)

	other := newFakeClient(t, "other", 4)
	registerSynced(h, other, []string{"iso"})
	require.NoError(t, h.SendToProject("iso", "t", []byte("ok")))
	require.Equal(t, []byte("ok"), recvOne(t, other))

	require.Len(t, h.clientProjects[c], maxProjectsPerClient)
	require.NoError(t, h.SendToProject("proj-0", "t", []byte("head")))
	require.Equal(t, []byte("head"), recvOne(t, c))
	require.NoError(t, h.SendToProject("proj-99999", "t", []byte("tail")))
	select {
	case got := <-c.Send:
		t.Fatalf("client must not be subscribed past cap, got %q", got)
	default:
	}
}

func TestEdge_CasesTableDriven(t *testing.T) {
	type row struct {
		name string
		run  func(t *testing.T)
	}
	rows := []row{
		{
			name: "broadcast to unknown project is noop",
			run: func(t *testing.T) {
				h, _ := newTestHub(t)
				require.NoError(t, h.SendToProject("no-such-project", "t", []byte("z")))
			},
		},
		{
			name: "register duplicate project IDs in one call dedupes map size",
			run: func(t *testing.T) {
				h, _ := newTestHub(t)
				cl := newFakeClient(t, "dx", 2)
				registerSynced(h, cl, []string{"same", "same", "same"})
				require.Len(t, h.projects["same"], 1)
				require.Len(t, h.clientProjects[cl], 1)
			},
		},
	}
	for _, tc := range rows {
		t.Run(tc.name, tc.run)
	}
}

// --- Extra hub API (coverage / sign-off) ---

func TestRegisterIfUnderLimit_RegistersWhenUnderMax(t *testing.T) {
	h, _ := newTestHub(t)
	c := newFakeClient(t, "u1", 4)
	ok := h.RegisterIfUnderLimit(c, []string{"p1"}, 2)
	require.True(t, ok)
	require.Equal(t, 1, h.CountUserConnections("user-u1", "p1"))
}

func TestRegisterIfUnderLimit_ReturnsFalseWhenOverMax(t *testing.T) {
	h, _ := newTestHub(t)
	c1 := &Client{ID: "a", UserID: "shared-user", Conn: nil, Send: make(chan []byte, 2), Hub: nil}
	c2 := &Client{ID: "b", UserID: "shared-user", Conn: nil, Send: make(chan []byte, 2), Hub: nil}
	require.True(t, h.RegisterIfUnderLimit(c1, []string{"p1"}, 1))
	ok := h.RegisterIfUnderLimit(c2, []string{"p1"}, 1)
	require.False(t, ok)
}

func TestCountUserConnections_ReturnsZeroAfterShutdown(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done
	require.Equal(t, 0, h.CountUserConnections("u", "p"))
	goleak.VerifyNone(t)
}

func TestRegisterIfUnderLimit_ReturnsFalseAfterShutdown(t *testing.T) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	cancel()
	<-h.done

	c := newFakeClient(t, "u", 2)
	ok := h.RegisterIfUnderLimit(c, []string{"p"}, 10)
	require.False(t, ok)
	goleak.VerifyNone(t)
}

func BenchmarkHub_BroadcastTo1000Clients(b *testing.B) {
	h := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	defer func() {
		cancel()
		<-h.done
	}()

	clients := make([]*Client, 1000)
	for i := range clients {
		id := strconv.Itoa(i)
		clients[i] = &Client{
			ID:     id,
			UserID: "user-" + id,
			Conn:   nil,
			Send:   make(chan []byte, 1),
			Hub:    nil,
		}
		h.Register(clients[i], []string{"bench"})
	}
	payload := []byte(`{"ok":true}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.SendToProject("bench", "evt", payload)
		for _, cl := range clients {
			select {
			case <-cl.Send:
			default:
			}
		}
	}
}
