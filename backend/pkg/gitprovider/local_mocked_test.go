package gitprovider

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestLocalGitProvider_ValidateAccess_and_Clone_mocked(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		if args[0] == "ls-remote" {
			return "deadbeef\tHEAD\n", "", nil
		}
		if args[0] == "clone" {
			return "", "", nil
		}
		return "", "", errors.New("unexpected")
	}
	l := &LocalGitProvider{LocalGitCLI: LocalGitCLI{creds: Credentials{Token: "ghp_x"}, runner: rr}}
	ctx := context.Background()
	if err := l.ValidateAccess(ctx, "https://github.com/o/r.git"); err != nil {
		t.Fatal(err)
	}
	if err := l.Clone(ctx, "https://github.com/o/r.git", CloneOptions{DestPath: "/d", Branch: "main"}); err != nil {
		t.Fatal(err)
	}
}

func TestLocalGitProvider_Push_and_CommitAndPush_mocked(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		switch args[0] {
		case "remote":
			return "https://github.com/o/r.git", "", nil
		case "push":
			return "", "", nil
		case "add":
			return "", "", nil
		case "rev-parse":
			if len(args) >= 2 && args[1] == "--verify" {
				return "", "fatal", errors.New("exit 128")
			}
			return "cafecafe", "", nil
		case "status":
			return "M a\n", "", nil
		case "commit":
			return "", "", nil
		default:
			return "", "", errors.New("unexpected")
		}
	}
	l := &LocalGitProvider{LocalGitCLI: LocalGitCLI{creds: Credentials{Token: "ghp_x"}, runner: rr}}
	ctx := context.Background()
	if err := l.Push(ctx, "/w", PushOptions{Branch: "b"}); err != nil {
		t.Fatal(err)
	}
	sha, ch, err := l.CommitAndPush(ctx, "/w", CommitOptions{Message: "m"}, PushOptions{Branch: "b"})
	if err != nil || !ch || sha != "cafecafe" {
		t.Fatalf("%v %q %v", err, sha, ch)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "diff-out" {
		t.Fatalf("%q", b)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "blob-data" {
		t.Fatalf("%q", b)
	}
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
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("%v", err)
		}
	})
	t.Run("not_found", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
			return "", "branch not found", errors.New("exit 1")
		}}
		cli := LocalGitCLI{creds: Credentials{}, runner: rr}
		err := cli.DeleteLocalBranch(context.Background(), "/w", "b")
		if !errors.Is(err, ErrBranchNotFound) {
			t.Fatalf("%v", err)
		}
	})
}

func TestLocalGitCLI_Commit_invalidAuthor(t *testing.T) {
	t.Parallel()
	cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
	_, _, err := cli.Commit(context.Background(), "/w", CommitOptions{
		Message: "m",
		Author:  Author{Name: "OnlyName", Email: ""},
	})
	if err == nil || !strings.Contains(err.Error(), "author") {
		t.Fatalf("%v", err)
	}
}

func TestLocalGitProvider_ValidateAccess_authFailed_stderr(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
		return "", "authentication failed for user", errors.New("exit 128")
	}}
	l := &LocalGitProvider{LocalGitCLI: LocalGitCLI{creds: Credentials{Token: "t"}, runner: rr}}
	err := l.ValidateAccess(context.Background(), "https://github.com/o/r.git")
	if !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("%v", err)
	}
}

func TestExecuteCommit_rejectsUnsafeFilePath(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	_, _, err := executeCommit(context.Background(), rr, "", "/w", CommitOptions{
		Message: "m",
		Files:   []string{"../etc/passwd"},
	})
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("%v", err)
	}
}
