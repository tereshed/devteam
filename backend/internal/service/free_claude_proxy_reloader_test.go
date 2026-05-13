package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sprint 15.N3 — Reload coalescing + reload HTTP path.

func newReloaderForTest(t *testing.T, baseURL, reloadPath string) (*FreeClaudeProxyReloader, string) {
	t.Helper()
	repo := &mockLLMProvidersForProxy{}
	builder := NewFreeClaudeProxyConfigBuilder(repo, staticSecrets{key: "k"}, 0, "svc")
	dir := t.TempDir()
	target := filepath.Join(dir, "config.yaml")
	return NewFreeClaudeProxyReloader(builder, target, baseURL, reloadPath, "svc-token", nil), target
}

func TestReloader_Reload_WritesAndPOSTs(t *testing.T) {
	var hits int32
	var sawAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		sawAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	rl, target := newReloaderForTest(t, srv.URL, "/reload")
	rl.Reload(context.Background())

	_, err := os.Stat(target)
	require.NoError(t, err, "config.yaml must be written")
	assert.Equal(t, int32(1), atomic.LoadInt32(&hits))
	assert.Equal(t, "Bearer svc-token", sawAuth)
}

func TestReloader_Reload_NoOpWhenBaseURLEmpty(t *testing.T) {
	rl, target := newReloaderForTest(t, "", "/reload")
	rl.Reload(context.Background())
	_, err := os.Stat(target)
	require.NoError(t, err, "config.yaml must still be written")
}

func TestReloader_Reload_TolerantToProxyErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	rl, _ := newReloaderForTest(t, srv.URL, "/reload")
	// Sprint 15.N3: апстрим без /reload → warn, без паники / leaked goroutines.
	rl.Reload(context.Background())
}

// Sprint 15.Major — connect refused / transport error не зависает Reload.
func TestReloader_Reload_TolerantToConnectRefused(t *testing.T) {
	rl, _ := newReloaderForTest(t, "http://127.0.0.1:1/", "/reload")
	done := make(chan struct{})
	go func() { rl.Reload(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("Reload deadlocked on connect refused")
	}
}

// Sprint 15.Major — прямые Reload() сериализованы через mu.
func TestReloader_Reload_DirectCalls_AreSerialized(t *testing.T) {
	var inProgress int32
	var maxConcurrent int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cur := atomic.AddInt32(&inProgress, 1)
		defer atomic.AddInt32(&inProgress, -1)
		for {
			prev := atomic.LoadInt32(&maxConcurrent)
			if cur <= prev || atomic.CompareAndSwapInt32(&maxConcurrent, prev, cur) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()
	rl, _ := newReloaderForTest(t, srv.URL, "/reload")
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rl.Reload(context.Background())
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), atomic.LoadInt32(&maxConcurrent),
		"Reload calls must be serialized via mu")
}

func TestReloader_AsyncReloadHook_CoalescesConcurrent(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		time.Sleep(40 * time.Millisecond) // эмулируем медленный reload
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	rl, _ := newReloaderForTest(t, srv.URL, "/reload")
	hook := rl.AsyncReloadHook()

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			hook(context.Background())
		}()
	}
	wg.Wait()

	// Дожидаемся завершения «последнего» reload (он стартует уже после wg).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && atomic.LoadInt32(&rl.asyncRunning) == 1 {
		time.Sleep(10 * time.Millisecond)
	}

	// Coalescing: реальных HTTP-вызовов должно быть ≤ 2 (первый + один coalesced).
	got := atomic.LoadInt32(&hits)
	assert.GreaterOrEqual(t, got, int32(1), "must hit at least once")
	assert.LessOrEqual(t, got, int32(2), "AsyncReloadHook must coalesce; got %d", got)
}
