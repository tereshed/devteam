package sandbox

import (
	"context"
	"encoding/binary"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func muxDockerFrame(streamType byte, payload []byte) []byte {
	h := make([]byte, 8)
	h[0] = streamType
	binary.BigEndian.PutUint32(h[4:], uint32(len(payload)))
	return append(h, payload...)
}

func registerTrackedInstance(t *testing.T, r *DockerSandboxRunner, cid string) *instanceState {
	t.Helper()
	st := newInstanceState("550e8400-e29b-41d4-a716-446655440000")
	st.containerID = cid
	r.mu.Lock()
	r.instances[cid] = st
	r.mu.Unlock()
	return st
}

func TestStreamLogs_SecondStreamReturnsErrStreamAlreadyActive(t *testing.T) {
	payload := muxDockerFrame(1, []byte("ok\n"))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/"+testSandboxID+"/logs"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/"+testSandboxID):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	r := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	registerTrackedInstance(t, r, testSandboxID)

	ch1, err := r.StreamLogs(context.Background(), testSandboxID)
	require.NoError(t, err)
	_, err = r.StreamLogs(context.Background(), testSandboxID)
	require.ErrorIs(t, err, ErrStreamAlreadyActive)

	for range ch1 {
	}
}

func TestStreamLogs_NotTrackedReturnsErrSandboxNotFound(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	r := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	_, err := r.StreamLogs(context.Background(), testSandboxID)
	require.ErrorIs(t, err, ErrSandboxNotFound)
}

func TestStreamLogs_DemuxStdoutStderr(t *testing.T) {
	payload := append(muxDockerFrame(1, []byte("out\n")), muxDockerFrame(2, []byte("err\n"))...)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/"+testSandboxID+"/logs") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
			return
		}
		http.NotFound(w, r)
	})
	r := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	registerTrackedInstance(t, r, testSandboxID)

	ch, err := r.StreamLogs(context.Background(), testSandboxID)
	require.NoError(t, err)
	var lines []struct {
		line   string
		stderr bool
	}
	for e := range ch {
		require.Nil(t, e.Error)
		if e.Line != "" {
			lines = append(lines, struct {
				line   string
				stderr bool
			}{e.Line, e.Stderr})
		}
	}
	require.Len(t, lines, 2)
	assert.Equal(t, "out", lines[0].line)
	assert.False(t, lines[0].stderr)
	assert.Equal(t, "err", lines[1].line)
	assert.True(t, lines[1].stderr)
}

func TestStreamLogs_ChunkLongLineTimestampOnlyOnFirstChunk(t *testing.T) {
	long := strings.Repeat("x", LogEntryMaxLineBytes+100)
	payload := muxDockerFrame(1, []byte(long+"\n"))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/"+testSandboxID+"/logs") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
			return
		}
		http.NotFound(w, r)
	})
	r := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	registerTrackedInstance(t, r, testSandboxID)

	ch, err := r.StreamLogs(context.Background(), testSandboxID)
	require.NoError(t, err)
	var entries []LogEntry
	for e := range ch {
		if e.Line != "" {
			entries = append(entries, e)
		}
	}
	require.GreaterOrEqual(t, len(entries), 2)
	assert.False(t, entries[0].Timestamp.IsZero())
	assert.True(t, entries[1].Timestamp.IsZero())
	for _, e := range entries {
		assert.LessOrEqual(t, len(e.Line), LogEntryMaxLineBytes)
	}
}

func TestStreamLogs_SlowConsumerSeesDropMarker(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		b.WriteString("x\n")
	}
	payload := muxDockerFrame(1, []byte(b.String()))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/"+testSandboxID+"/logs") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
			return
		}
		http.NotFound(w, r)
	})
	r := NewDockerSandboxRunner(newTestDockerClient(t, h), nil, WithStreamLogsEntryBuffer(8))
	registerTrackedInstance(t, r, testSandboxID)

	ch, err := r.StreamLogs(context.Background(), testSandboxID)
	require.NoError(t, err)
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	var sawDrop bool
	for e := range ch {
		if strings.Contains(e.Line, logStreamDroppedLine) {
			sawDrop = true
		}
		<-tick.C
	}
	assert.True(t, sawDrop, "expected slow-consumer drop marker")
}

func TestStreamLogs_CleanupThenNoLeak(t *testing.T) {
	payload := muxDockerFrame(1, []byte("done\n"))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/containers/"+testSandboxID+"/logs"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(payload)
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/"+testSandboxID):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	r := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	registerTrackedInstance(t, r, testSandboxID)

	ch, err := r.StreamLogs(context.Background(), testSandboxID)
	require.NoError(t, err)
	require.NoError(t, r.Cleanup(context.Background(), testSandboxID))
	for range ch {
	}
}

func TestStreamLogs_OpenErrorExposesSandboxDockerOnly(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/"+testSandboxID+"/logs") {
			http.Error(w, "engine internal xyzzy detail", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	})
	r := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	registerTrackedInstance(t, r, testSandboxID)

	ch, err := r.StreamLogs(context.Background(), testSandboxID)
	require.NoError(t, err)
	var gotErr error
	for e := range ch {
		if e.Error != nil {
			gotErr = e.Error
		}
	}
	require.Error(t, gotErr)
	assert.ErrorIs(t, gotErr, ErrSandboxDocker)
	assert.NotContains(t, gotErr.Error(), "xyzzy")
}
