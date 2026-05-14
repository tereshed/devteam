package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLlmCredentialsPatchRateLimiter_429AndRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lim := NewLlmCredentialsPatchRateLimiter(3, time.Minute)
	t.Cleanup(func() { lim.Close() })
	h := lim.Handler()
	uid := uuid.New()

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("userID", uid)
		c.Request = httptest.NewRequest(http.MethodPatch, "/x", nil)
		h(c)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.False(t, c.IsAborted())
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uid)
	c.Request = httptest.NewRequest(http.MethodPatch, "/x", nil)
	h(c)
	require.True(t, c.IsAborted())
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, apierror.ErrTooManyRequests, body.Error)
}

func TestLlmCredentialsPatchRateLimiter_GCPrunesMapBeforeReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// `cur` шарится между тестовой goroutine (write) и gcLoop-goroutine,
	// которая дёргает clock-closure. Read-modify-write плюс concurrent reader
	// без синхронизации = data race (поймано `go test -race`). atomic.Pointer
	// гарантирует видимость записей и атомарную замену.
	var cur atomic.Pointer[time.Time]
	t0 := time.Unix(1_700_000_000, 0)
	cur.Store(&t0)
	advance := func(d time.Duration) {
		next := cur.Load().Add(d)
		cur.Store(&next)
	}
	window := 40 * time.Millisecond
	lim := NewLlmCredentialsPatchRateLimiter(2, window,
		WithPatchRateLimitClock(func() time.Time { return *cur.Load() }),
		WithPatchRateLimitGCInterval(12*time.Millisecond),
	)
	t.Cleanup(func() { lim.Close() })
	h := lim.Handler()
	uid := uuid.New()

	call := func() (code int, aborted bool) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("userID", uid)
		c.Request = httptest.NewRequest(http.MethodPatch, "/x", nil)
		h(c)
		return w.Code, c.IsAborted()
	}

	code, aborted := call()
	require.Equal(t, http.StatusOK, code)
	require.False(t, aborted)
	advance(time.Millisecond)
	code, aborted = call()
	require.Equal(t, http.StatusOK, code)
	require.False(t, aborted)
	advance(time.Millisecond)
	code, aborted = call()
	require.Equal(t, http.StatusTooManyRequests, code)
	require.True(t, aborted)

	advance(2 * window)
	time.Sleep(45 * time.Millisecond)

	lim.mu.Lock()
	nAfterGC := len(lim.hits)
	lim.mu.Unlock()
	assert.Equal(t, 0, nAfterGC, "gcLoop must drop stale uid entries before next request")

	advance(time.Millisecond)
	code, aborted = call()
	require.Equal(t, http.StatusOK, code)
	require.False(t, aborted)

	lim.mu.Lock()
	n := len(lim.hits)
	lim.mu.Unlock()
	assert.Equal(t, 1, n)
}
