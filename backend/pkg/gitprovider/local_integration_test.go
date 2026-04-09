//go:build integration

package gitprovider

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestLocalGit_real_repo_GetLocalDiff(t *testing.T) {
	// sequential: t.Setenv несовместим с t.Parallel
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	t.Setenv("GIT_ASKPASS", "")
	dir := t.TempDir()
	steps := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "a@b.c"},
		{"git", "config", "user.name", "n"},
		{"git", "commit", "--allow-empty", "-m", "a"},
		{"git", "commit", "--allow-empty", "-m", "b"},
	}
	for _, argv := range steps {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "argv=%v out=%s", argv, out)
	}

	logCmd := exec.Command("git", "log", "--format=%H", "-2")
	logCmd.Dir = dir
	logCmd.Env = append(logCmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	logOut, err := logCmd.Output()
	require.NoError(t, err)
	lines := strings.Fields(strings.TrimSpace(string(logOut)))
	require.Len(t, lines, 2, "need 2 commits for diff, got: %q", logOut)
	headSHA, baseSHA := lines[0], lines[1]

	cli := LocalGitCLI{creds: Credentials{}}
	rc, err := cli.GetLocalDiff(context.Background(), dir, baseSHA, headSHA)
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()
	_, err = io.ReadAll(rc)
	require.NoError(t, err)
}

// Реальный git + readCloserWithWait: читаем только префикс diff и закрываем — без утечек горутин и без ложной ошибки от SIGPIPE.
func TestLocalGit_GetLocalDiff_partialReadClose_goleak(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("раннее закрытие git diff / goleak проверяем на Unix (exit 141)")
	}
	defer goleak.VerifyNone(t)
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	dir := t.TempDir()
	blob := bytes.Repeat([]byte("z"), 128*1024)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), blob, 0o644))
	for _, argv := range [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "a@b.c"},
		{"git", "config", "user.name", "n"},
		{"git", "add", "f.txt"},
		{"git", "commit", "-m", "a"},
	} {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = dir
		cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%v %s", argv, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), append(blob, []byte("x")...), 0o644))
	cmd := exec.Command("git", "commit", "-am", "b")
	cmd.Dir = dir
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	_, err := cmd.CombinedOutput()
	require.NoError(t, err)

	logCmd := exec.Command("git", "log", "--format=%H", "-2")
	logCmd.Dir = dir
	logCmd.Env = append(logCmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	logOut, err := logCmd.Output()
	require.NoError(t, err)
	lines := strings.Fields(strings.TrimSpace(string(logOut)))
	require.Len(t, lines, 2)
	headSHA, baseSHA := lines[0], lines[1]

	cli := LocalGitCLI{creds: Credentials{}}
	rc, err := cli.GetLocalDiff(context.Background(), dir, baseSHA, headSHA)
	require.NoError(t, err)
	buf := make([]byte, 256)
	_, err = rc.Read(buf)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
}

func TestLocalGit_contextDeadline_execGitRunner(t *testing.T) {
	t.Setenv("GIT_TERMINAL_PROMPT", "0")
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond)
	r := NewExecGitRunner()
	_, stderr, err := r.RunGit(ctx, t.TempDir(), "version")
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded), "stderr=%q err=%v", stderr, err)
}
