package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func miniredisClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	c := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func doPatch(h gin.HandlerFunc, uid uuid.UUID) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uid)
	c.Request = httptest.NewRequest(http.MethodPatch, "/x", nil)
	h(c)
	return w
}

// Redis-путь: лимит shared между двумя лимитерами на одном Redis (эмуляция двух реплик).
// Суммарно по обоим инстансам допускается ровно max запросов, дальше — 429.
func TestLlmCredentialsPatchRateLimiter_RedisSharedAcrossInstances(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rc := miniredisClient(t)
	uid := uuid.New()

	limA := NewLlmCredentialsPatchRateLimiter(3, time.Minute, WithPatchRateLimitRedis(rc))
	limB := NewLlmCredentialsPatchRateLimiter(3, time.Minute, WithPatchRateLimitRedis(rc))
	t.Cleanup(func() { limA.Close(); limB.Close() })
	hA, hB := limA.Handler(), limB.Handler()

	// 3 запроса вперемешку по «репликам» — все проходят (общий счётчик = 3).
	assert.Equal(t, http.StatusOK, doPatch(hA, uid).Code)
	assert.Equal(t, http.StatusOK, doPatch(hB, uid).Code)
	assert.Equal(t, http.StatusOK, doPatch(hA, uid).Code)

	// 4-й (на любой реплике) — превышение общего лимита.
	w := doPatch(hB, uid)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))

	// Другой пользователь не затронут.
	assert.Equal(t, http.StatusOK, doPatch(hA, uuid.New()).Code)
}

// Fail-open: при недоступном Redis лимитер пропускает запросы, не роняя легитимный трафик.
func TestLlmCredentialsPatchRateLimiter_RedisFailOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr, err := miniredis.Run()
	require.NoError(t, err)
	// Короткий dial timeout без ретраев — чтобы недоступный Redis отказывал быстро.
	rc := redis.NewClient(&redis.Options{
		Addr:        mr.Addr(),
		DialTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { _ = rc.Close() })
	mr.Close() // Redis недоступен

	lim := NewLlmCredentialsPatchRateLimiter(1, time.Minute, WithPatchRateLimitRedis(rc))
	t.Cleanup(func() { lim.Close() })
	h := lim.Handler()
	uid := uuid.New()

	// Даже сверх лимита запросы проходят (fail-open).
	assert.Equal(t, http.StatusOK, doPatch(h, uid).Code)
	assert.Equal(t, http.StatusOK, doPatch(h, uid).Code)
	assert.Equal(t, http.StatusOK, doPatch(h, uid).Code)
}
