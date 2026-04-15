package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestRunOpts() SandboxOptions {
	return SandboxOptions{
		TaskID:       "550e8400-e29b-41d4-a716-446655440000",
		Backend:      CodeBackendClaudeCode,
		Image:        "devteam/sandbox-claude:local",
		RepoURL:      "https://github.com/example/example.git",
		Branch:       "feature/runner-test",
		Instruction:  "do the thing",
		Context:      "",
		EnvVars:      nil,
		DisableNetwork: true,
	}
}

func dockerAPIError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func newCountingMux(t *testing.T, inner http.HandlerFunc) (http.HandlerFunc, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		inner(w, r)
	}), &n
}

// runTaskSuccessMux — минимальные ответы Engine для RunTask с DisableNetwork (без NetworkCreate).
func runTaskSuccessMux(t *testing.T, taskID, containerID string) http.HandlerFunc {
	t.Helper()
	cname := taskContainerName(taskID)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.Contains(p, "/containers/") && strings.HasSuffix(p, "/json"):
			if strings.Contains(p, "/containers/"+cname+"/") {
				dockerAPIError(w, http.StatusNotFound, "no such container")
				return
			}
			if !strings.Contains(p, "/containers/"+containerID+"/") {
				dockerAPIError(w, http.StatusNotFound, "no such container")
				return
			}
			writeContainerInspect(w, containerID, taskID, "running", 0, true)

		case r.Method == http.MethodGet && strings.Contains(p, "/images/") && strings.HasSuffix(p, "/json"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Id":"sha256:imgtest"}`))

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/create"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"` + containerID + `"}`))

		case r.Method == http.MethodPut && strings.Contains(p, "/containers/"+containerID+"/archive"):
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/"+containerID+"/start"):
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/"+containerID+"/wait"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"StatusCode":0}`))

		case r.Method == http.MethodGet && strings.Contains(p, "/containers/"+containerID+"/archive"):
			dockerAPIError(w, http.StatusNotFound, "Could not find the file")

		case r.Method == http.MethodDelete && strings.Contains(p, "/containers/"+containerID):
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	})
}

func writeContainerInspect(w http.ResponseWriter, id, taskID, status string, exitCode int, running bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	m := map[string]interface{}{
		"Id": id,
		"Config": map[string]interface{}{
			"Labels": map[string]string{
				"devteam.sandbox":      "1",
				"devteam.task_id":      taskID,
				"devteam.timeout_secs": "3600",
				"devteam.host_tmp":     "",
				"devteam.network_id":   "",
			},
		},
		"State": map[string]interface{}{
			"Status":    status,
			"Running":   running,
			"ExitCode":  exitCode,
			"OOMKilled": false,
		},
	}
	b, _ := json.Marshal(m)
	_, _ = w.Write(b)
}

// TestRunTask_ValidationFailsBeforeDocker — RunTask не обращается к Engine, пока не пройдены Validate* / allowlist.
func TestRunTask_ValidationFailsBeforeDocker(t *testing.T) {
	t.Parallel()

	allowedLocalOnly := []string{"devteam/sandbox-claude:local"}

	cases := []struct {
		name            string
		mutate          func(*SandboxOptions)
		runnerAllowed   []string // nil → allowedLocalOnly
		wantErrContains []error // все должны удовлетворять errors.Is
	}{
		{
			name: "empty_task_id",
			mutate: func(o *SandboxOptions) {
				o.TaskID = ""
			},
			wantErrContains: []error{ErrInvalidOptions, ErrInvalidTaskID},
		},
		{
			name: "invalid_branch_path_traversal",
			mutate: func(o *SandboxOptions) {
				o.Branch = "feature/../evil"
			},
			wantErrContains: []error{ErrInvalidOptions, ErrInvalidBranchName},
		},
		{
			name: "empty_image",
			mutate: func(o *SandboxOptions) {
				o.Image = ""
			},
			wantErrContains: []error{ErrInvalidOptions},
		},
		{
			name: "image_not_in_allowlist",
			mutate: func(o *SandboxOptions) {
				o.Image = "nginx:latest"
			},
			runnerAllowed:   allowedLocalOnly,
			wantErrContains: []error{ErrInvalidOptions},
		},
		{
			name: "empty_backend",
			mutate: func(o *SandboxOptions) {
				o.Backend = ""
			},
			wantErrContains: []error{ErrInvalidOptions},
		},
		{
			name: "unsupported_backend",
			mutate: func(o *SandboxOptions) {
				o.Backend = CodeBackendType("unknown-backend")
			},
			wantErrContains: []error{ErrInvalidOptions},
		},
		{
			name: "empty_instruction",
			mutate: func(o *SandboxOptions) {
				o.Instruction = ""
			},
			wantErrContains: []error{ErrInvalidOptions},
		},
		{
			name: "repo_url_file_scheme",
			mutate: func(o *SandboxOptions) {
				o.RepoURL = "file:///etc/passwd"
			},
			wantErrContains: []error{ErrInvalidOptions, ErrInvalidRepoURL},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts := validTestRunOpts()
			tc.mutate(&opts)

			allowed := tc.runnerAllowed
			if allowed == nil {
				allowed = allowedLocalOnly
			}

			h, cnt := newCountingMux(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "unexpected", http.StatusInternalServerError)
			}))
			rn := NewDockerSandboxRunner(newTestDockerClient(t, h), allowed)

			_, err := rn.RunTask(context.Background(), opts)
			require.Error(t, err)
			for _, target := range tc.wantErrContains {
				assert.ErrorIs(t, err, target, "errors.Is: want %v", target)
			}
			require.Equal(t, int32(0), cnt.Load(), "mux must not see HTTP traffic before validation/allowlist")
		})
	}
}

func TestRunTask_HappyPath_ReturnsFullIDAndRunning(t *testing.T) {
	t.Parallel()
	opts := validTestRunOpts()
	mux := runTaskSuccessMux(t, opts.TaskID, testFullContainerHex)
	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), []string{opts.Image},
		WithDefaultTaskTimeout(time.Hour))

	inst, err := rn.RunTask(context.Background(), opts)
	require.NoError(t, err)
	require.NotNil(t, inst)
	assert.Equal(t, testFullContainerHex, inst.ID)
	assert.Equal(t, opts.TaskID, inst.TaskID)
	assert.Equal(t, SandboxStatusRunning, inst.Status)
	assert.NoError(t, ValidateSandboxID(inst.ID))
}

func TestRunTask_CopyToContainerFails_RemovesContainer(t *testing.T) {
	t.Parallel()
	opts := validTestRunOpts()
	taskID := opts.TaskID
	cname := taskContainerName(taskID)
	var removeCalls atomic.Int32

	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.Contains(p, "/containers/") && strings.HasSuffix(p, "/json"):
			if strings.Contains(p, "/containers/"+cname+"/") {
				dockerAPIError(w, http.StatusNotFound, "no such container")
				return
			}
			writeContainerInspect(w, testFullContainerHex, taskID, "created", 0, false)

		case r.Method == http.MethodGet && strings.Contains(p, "/images/") && strings.HasSuffix(p, "/json"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Id":"sha256:x"}`))

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/create"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"` + testFullContainerHex + `"}`))

		case r.Method == http.MethodPut && strings.Contains(p, "/containers/"+testFullContainerHex+"/archive"):
			dockerAPIError(w, http.StatusInternalServerError, "copy failed")

		case r.Method == http.MethodDelete && strings.Contains(p, "/containers/"+testFullContainerHex):
			removeCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	})

	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), []string{opts.Image})
	_, err := rn.RunTask(context.Background(), opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSandboxDocker))
	require.GreaterOrEqual(t, removeCalls.Load(), int32(1), "expected ContainerRemove after copy failure")

	rn.mu.Lock()
	_, still := rn.instances[testFullContainerHex]
	rn.mu.Unlock()
	assert.False(t, still, "instance must not be registered after failed RunTask")
}

func TestWait_InvalidSandboxID_NoHTTP(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
	})
	rn := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	_, err := rn.Wait(context.Background(), "short")
	require.ErrorIs(t, err, ErrInvalidSandboxID)
	require.Equal(t, int32(0), n.Load())
}

func TestWait_ContextCancelled(t *testing.T) {
	t.Parallel()
	opts := validTestRunOpts()
	mux := runTaskSuccessMux(t, opts.TaskID, testFullContainerHex)
	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), []string{opts.Image},
		WithDefaultTaskTimeout(time.Hour))

	inst, err := rn.RunTask(context.Background(), opts)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = rn.Wait(ctx, inst.ID)
	require.ErrorIs(t, err, context.Canceled)
}

func TestGetStatus_RunningAndNotFound(t *testing.T) {
	t.Parallel()

	t.Run("running", func(t *testing.T) {
		t.Parallel()
		taskID := "550e8400-e29b-41d4-a716-446655440001"
		mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			if r.Method == http.MethodGet && strings.Contains(p, "/containers/"+testSandboxID+"/json") {
				writeContainerInspect(w, testSandboxID, taskID, "running", 0, true)
				return
			}
			http.NotFound(w, r)
		})
		rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), nil)
		st, err := rn.GetStatus(context.Background(), testSandboxID)
		require.NoError(t, err)
		require.NotNil(t, st)
		assert.Equal(t, SandboxStatusRunning, st.Status)
	})

	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			dockerAPIError(w, http.StatusNotFound, "no such container")
		})
		rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), nil)
		_, err := rn.GetStatus(context.Background(), testSandboxID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSandboxNotFound))
	})
}

func TestStop_IdempotentAndNotFoundIsOK(t *testing.T) {
	t.Parallel()
	var stopCalls atomic.Int32
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if r.Method == http.MethodPost && (strings.Contains(p, "/containers/"+testSandboxID+"/stop") ||
			strings.Contains(p, "/containers/"+testSandboxID+"/kill")) {
			stopCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(p, "/containers/"+testSandboxID+"/json") {
			writeContainerInspect(w, testSandboxID, "550e8400-e29b-41d4-a716-446655440002", "running", 0, true)
			return
		}
		http.NotFound(w, r)
	})
	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), nil)
	registerTrackedInstance(t, rn, testSandboxID)

	require.NoError(t, rn.Stop(context.Background(), testSandboxID))
	require.NoError(t, rn.Stop(context.Background(), testSandboxID))
	require.GreaterOrEqual(t, stopCalls.Load(), int32(1))

	// Stop when inspect says not found -> nil
	h404 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dockerAPIError(w, http.StatusNotFound, "no such container")
	})
	rn404 := NewDockerSandboxRunner(newTestDockerClient(t, h404), nil)
	require.NoError(t, rn404.Stop(context.Background(), testSandboxID))
}

func TestCleanup_IdempotentAndNotFoundLogged(t *testing.T) {
	t.Parallel()
	var removeCalls atomic.Int32
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if r.Method == http.MethodDelete && strings.Contains(p, "/containers/"+testSandboxID) {
			removeCalls.Add(1)
			dockerAPIError(w, http.StatusNotFound, "no such container")
			return
		}
		http.NotFound(w, r)
	})
	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), nil)
	registerTrackedInstance(t, rn, testSandboxID)

	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	require.NoError(t, rn.Cleanup(context.Background(), testSandboxID))
	require.NoError(t, rn.Cleanup(context.Background(), testSandboxID))
	require.GreaterOrEqual(t, removeCalls.Load(), int32(1))

	out := buf.String()
	assert.Contains(t, out, "container_remove")
	assert.Contains(t, out, testSandboxID)
}

func TestCleanup_CancelledUserCtxStillRemoves(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var sawDelete atomic.Bool
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/containers/"+testSandboxID) {
			sawDelete.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})
	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), nil)
	registerTrackedInstance(t, rn, testSandboxID)

	require.NoError(t, rn.Cleanup(ctx, testSandboxID))
	assert.True(t, sawDelete.Load())
}

func TestStopTask_DuringSlowRunTask_InitCancelled(t *testing.T) {
	t.Parallel()
	opts := validTestRunOpts()
	taskID := opts.TaskID
	cname := taskContainerName(taskID)
	block := make(chan struct{})

	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.Contains(p, "/containers/") && strings.HasSuffix(p, "/json"):
			if strings.Contains(p, "/containers/"+cname+"/") {
				dockerAPIError(w, http.StatusNotFound, "no such container")
				return
			}
			writeContainerInspect(w, testFullContainerHex, taskID, "running", 0, true)

		case r.Method == http.MethodGet && strings.Contains(p, "/images/") && strings.HasSuffix(p, "/json"):
			<-block
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Id":"sha256:x"}`))

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/create"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"` + testFullContainerHex + `"}`))

		case r.Method == http.MethodPut && strings.Contains(p, "/containers/"+testFullContainerHex+"/archive"):
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/"+testFullContainerHex+"/start"):
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	})

	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), []string{opts.Image})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		_ = rn.StopTask(context.Background(), taskID)
		close(block)
	}()

	_, err := rn.RunTask(context.Background(), opts)
	wg.Wait()
	require.ErrorIs(t, err, ErrSandboxInitCancelled)
}

func TestWait_EmptySandboxID(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
	})
	rn := NewDockerSandboxRunner(newTestDockerClient(t, h), nil)
	_, err := rn.Wait(context.Background(), "")
	require.ErrorIs(t, err, ErrInvalidSandboxID)
	require.Equal(t, int32(0), n.Load())
}

func TestBuildPromptContextTar_OnlyLiteralFileNames(t *testing.T) {
	t.Parallel()
	instruction := `; rm -rf / --upload-pack=id`
	ctx := `../../../etc/passwd`
	rc, err := buildPromptContextTar(instruction, ctx)
	require.NoError(t, err)
	defer rc.Close()

	tr := tar.NewReader(rc)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		names = append(names, hdr.Name)
		_, err = io.Copy(io.Discard, tr)
		require.NoError(t, err)
	}
	require.Equal(t, []string{"prompt.txt", "context.txt"}, names)
}

func TestRunTask_StartFails_RemovesContainer(t *testing.T) {
	t.Parallel()
	opts := validTestRunOpts()
	taskID := opts.TaskID
	cname := taskContainerName(taskID)
	var removeCalls atomic.Int32

	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.Contains(p, "/containers/") && strings.HasSuffix(p, "/json"):
			if strings.Contains(p, "/containers/"+cname+"/") {
				dockerAPIError(w, http.StatusNotFound, "no such container")
				return
			}
			writeContainerInspect(w, testFullContainerHex, taskID, "created", 0, false)

		case r.Method == http.MethodGet && strings.Contains(p, "/images/") && strings.HasSuffix(p, "/json"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Id":"sha256:x"}`))

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/create"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"Id":"` + testFullContainerHex + `"}`))

		case r.Method == http.MethodPut && strings.Contains(p, "/containers/"+testFullContainerHex+"/archive"):
			w.WriteHeader(http.StatusOK)

		case r.Method == http.MethodPost && strings.Contains(p, "/containers/"+testFullContainerHex+"/start"):
			dockerAPIError(w, http.StatusInternalServerError, "start failed")

		case r.Method == http.MethodDelete && strings.Contains(p, "/containers/"+testFullContainerHex):
			removeCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	})

	rn := NewDockerSandboxRunner(newTestDockerClient(t, mux), []string{opts.Image})
	_, err := rn.RunTask(context.Background(), opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSandboxDocker))
	require.GreaterOrEqual(t, removeCalls.Load(), int32(1))
}
