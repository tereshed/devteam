package gitprovider

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type infiniteZeroReader struct{}

func (infiniteZeroReader) Read(p []byte) (int, error) {
	return len(p), nil
}

func TestLocalGitCLI_GetLocalDiff_success_mocked(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		if args[0] != "rev-parse" {
			return "", "", errors.New("unexpected")
		}
		if strings.Contains(strings.Join(args, " "), "main") {
			return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n", "", nil
		}
		return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n", "", nil
	}
	rr.pipeHook = func(ctx context.Context, workDir, token string, args []string) (io.ReadCloser, error) {
		if args[0] != "diff" {
			return nil, errors.New("bad")
		}
		return io.NopCloser(strings.NewReader("diff-out")), nil
	}
	cli := LocalGitCLI{creds: Credentials{}, runner: rr}
	rc, err := cli.GetLocalDiff(context.Background(), "/w", "main", "topic")
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "diff-out", string(b))
}

// TestLocalGitCLI_GetLocalDiff_streamingRead_allocs замеряет весь путь GetLocalDiff + чтение префикса
// (без bytes.Repeat на 10 MiB в подготовке данных: источник — infiniteZeroReader + LimitReader в pipeHook).
func TestLocalGitCLI_GetLocalDiff_streamingRead_allocs(t *testing.T) {
	// sequential: AllocsPerRun несовместим с t.Parallel()
	buf := make([]byte, 64*1024)
	nRun := 20
	if testing.Short() {
		nRun = 5
	}
	allocs := testing.AllocsPerRun(nRun, func() {
		rr := &recordingRunner{}
		var revN int
		rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
			if args[0] != "rev-parse" {
				return "", "", errors.New("unexpected")
			}
			revN++
			if revN == 1 {
				return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n", "", nil
			}
			return "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n", "", nil
		}
		rr.pipeHook = func(ctx context.Context, workDir, token string, args []string) (io.ReadCloser, error) {
			return io.NopCloser(io.LimitReader(infiniteZeroReader{}, 10<<20)), nil
		}
		cli := LocalGitCLI{creds: Credentials{}, runner: rr}
		rc, err := cli.GetLocalDiff(context.Background(), "/w", "a", "b")
		if err != nil {
			panic(err)
		}
		_, _ = rc.Read(buf)
		_ = rc.Close()
	})
	// Порог: не ожидаем порядка «прочитать весь объём в один слайс»; при регрессии в GetLocalDiff/pipe всплеск будет заметен.
	require.Less(t, allocs, 80.0, "GetLocalDiff+Read префикса не должны давать чрезмерный рост аллокаций на итерацию")
}

func TestLocalGitCLI_GetLocalFileContent_success_mocked(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{
		pipeHook: func(ctx context.Context, workDir, token string, args []string) (io.ReadCloser, error) {
			if len(args) >= 1 && args[0] == "cat-file" {
				return io.NopCloser(strings.NewReader("blob-data")), nil
			}
			return nil, errors.New("unexpected pipe")
		},
	}
	cli := LocalGitCLI{creds: Credentials{}, runner: rr}
	rc, err := cli.GetLocalFileContent(context.Background(), "/w", "HEAD", "README.md")
	require.NoError(t, err)
	defer func() { require.NoError(t, rc.Close()) }()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "blob-data", string(b))
}

func TestDeleteLocalBranch_errorMapping_mocked(t *testing.T) {
	t.Parallel()
	t.Run("checked_out", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
			return "", "error: cannot delete branch checked out", errors.New("exit 1")
		}}
		cli := LocalGitCLI{creds: Credentials{}, runner: rr}
		err := cli.DeleteLocalBranch(context.Background(), "/w", "b")
		require.ErrorIs(t, err, ErrConflict)
	})
	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
			return "", "branch not found", errors.New("exit 1")
		}}
		cli := LocalGitCLI{creds: Credentials{}, runner: rr}
		err := cli.DeleteLocalBranch(context.Background(), "/w", "b")
		require.ErrorIs(t, err, ErrBranchNotFound)
	})
}

func TestLocalGitCLI_Commit_invalidAuthor(t *testing.T) {
	t.Parallel()
	cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
	_, _, err := cli.Commit(context.Background(), "/w", CommitOptions{
		Message: "m",
		Author:  Author{Name: "OnlyName", Email: ""},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "author")
}

func TestExecuteCommit_rejectsUnsafeFilePath(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	_, _, err := executeCommit(context.Background(), rr, "", "/w", CommitOptions{
		Message: "m",
		Files:   []string{"../etc/passwd"},
	})
	require.ErrorIs(t, err, ErrUnsafePath)
}

func TestLocalGitCLI_CreateBranch_table(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t.Run("empty_name", func(t *testing.T) {
		t.Parallel()
		cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
		require.ErrorContains(t, cli.CreateBranch(ctx, "/w", BranchOptions{BranchName: "  "}), "branch name is required")
	})
	t.Run("upload_pack_injection", func(t *testing.T) {
		t.Parallel()
		cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
		require.ErrorIs(t, cli.CreateBranch(ctx, "/w", BranchOptions{BranchName: "x--upload-pack=id"}), ErrUnsafePath)
	})
	t.Run("receive_pack_in_base", func(t *testing.T) {
		t.Parallel()
		cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
		require.ErrorIs(t, cli.CreateBranch(ctx, "/w", BranchOptions{BranchName: "ok", BaseBranch: "main--receive-pack=pwn"}), ErrUnsafePath)
	})
}

func TestLocalGitCLI_GetLocalDiff_invalidRef(t *testing.T) {
	t.Parallel()
	cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
	_, err := cli.GetLocalDiff(context.Background(), "/w", "", "HEAD")
	require.Error(t, err)
}

func TestLocalGitCLI_GetLocalFileContent_invalidPath(t *testing.T) {
	t.Parallel()
	cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
	_, err := cli.GetLocalFileContent(context.Background(), "/w", "HEAD", "../x")
	require.ErrorIs(t, err, ErrUnsafePath)
}

func TestLocalGitCLI_nilContext(t *testing.T) {
	t.Parallel()
	var nilCtx context.Context
	cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
	require.ErrorContains(t, cli.CreateBranch(nilCtx, "/w", BranchOptions{BranchName: "b"}), "nil context")
	_, err := cli.ListLocalBranches(nilCtx, "/w", "")
	require.ErrorContains(t, err, "nil context")
	require.ErrorContains(t, cli.DeleteLocalBranch(nilCtx, "/w", "b"), "nil context")
	_, _, err = cli.Commit(nilCtx, "/w", CommitOptions{Message: "m"})
	require.ErrorContains(t, err, "nil context")
	_, err = cli.GetLocalDiff(nilCtx, "/w", "a", "b")
	require.ErrorContains(t, err, "nil context")
	_, err = cli.GetLocalFileContent(nilCtx, "/w", "HEAD", "f")
	require.ErrorContains(t, err, "nil context")
}

func TestRunGit_errorsAsExitError(t *testing.T) {
	t.Parallel()
	var exitErr error
	if runtime.GOOS == "windows" {
		exitErr = exec.Command("cmd", "/c", "exit 128").Run()
	} else {
		exitErr = exec.Command("sh", "-c", "exit 128").Run()
	}
	var want *exec.ExitError
	require.True(t, errors.As(exitErr, &want))
	rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
		return "", "fatal", exitErr
	}}
	_, err := runGit(context.Background(), rr, "", "/w", "status")
	require.Error(t, err)
	var ee *exec.ExitError
	require.True(t, errors.As(err, &ee))
	require.Equal(t, 128, ee.ExitCode())
}

func TestLocalGitCLI_ListLocalBranches_and_DeleteLocalBranch_smoke(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{defaultOut: "main\nfeature\n"}
	cli := LocalGitCLI{creds: Credentials{}, runner: rr}
	ctx := context.Background()
	names, err := cli.ListLocalBranches(ctx, "/w", "feat")
	require.NoError(t, err)
	require.Equal(t, []string{"feature"}, names)
	rr2 := &recordingRunner{}
	rr2.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		return "", "", nil
	}
	cli2 := LocalGitCLI{creds: Credentials{}, runner: rr2}
	require.NoError(t, cli2.DeleteLocalBranch(ctx, "/w", "topic"))
}
