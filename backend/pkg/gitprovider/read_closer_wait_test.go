package gitprovider

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func infiniteStdoutCmd(ctx context.Context) *exec.Cmd {
	if _, err := exec.LookPath("yes"); err == nil {
		return exec.CommandContext(ctx, "yes")
	}
	return exec.CommandContext(ctx, "sh", "-c", "while true; do printf 'y\\n'; done")
}

// Проверяем сценарий «процесс забит записью в stdout»: закрытие ReadCloser до Wait не должно вешать тест.
func TestReadCloserWithWait_Close_unblocksBlockedWriter(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := infiniteStdoutCmd(ctx)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r := &readCloserWithWait{ReadCloser: stdout, cmd: cmd, stderr: stderr, token: ""}
	// Даём процессу заполнить буфер пайпа (типично ~64 KiB), после чего он блокируется на write.
	time.Sleep(150 * time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- r.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Logf("Close: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("deadlock: Close() не завершился — ожидался разрыв пайпа и Wait()")
	}
}

// Раннее закрытие stdout: писатель получает SIGPIPE (exit 141 на Unix) — не считаем это ошибкой Close().
func TestReadCloserWithWait_Close_sigPIPEAfterShortRead(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("exit 141 / SIGPIPE после закрытия пайпа — проверяем на Unix")
	}
	var cmd *exec.Cmd
	if _, err := exec.LookPath("yes"); err == nil {
		cmd = exec.CommandContext(context.Background(), "yes")
	} else {
		cmd = exec.CommandContext(context.Background(), "sh", "-c", "while true; do printf 'x\\n'; done")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r := &readCloserWithWait{ReadCloser: stdout, cmd: cmd, stderr: stderr, token: ""}
	buf := make([]byte, 16)
	if _, err := r.Read(buf); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestReadCloserWithWait_Close_stderrBlobMissing(t *testing.T) {
	t.Parallel()
	cmd := exec.CommandContext(context.Background(), "sh", "-c", `echo "fatal: path foo does not exist" >&2; exit 1`)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr := new(bytes.Buffer)
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r := &readCloserWithWait{ReadCloser: stdout, cmd: cmd, stderr: stderr, token: "tok"}
	err = r.Close()
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("%v", err)
	}
}
