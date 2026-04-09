package gitprovider

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newLocalProviderWithRunner(t *testing.T, creds Credentials, rr GitCommandRunner) *LocalGitProvider {
	t.Helper()
	return &LocalGitProvider{LocalGitCLI: LocalGitCLI{creds: creds, runner: rr}}
}

func TestNewLocalGitProvider(t *testing.T) {
	t.Parallel()
	l := NewLocalGitProvider(Credentials{Token: "x"})
	require.NotNil(t, l)
	require.Nil(t, l.runner)
}

func TestLocalGitProvider_meta(t *testing.T) {
	t.Parallel()
	l := NewLocalGitProvider(Credentials{})
	require.Equal(t, "local", l.ProviderType())
	require.False(t, l.SupportsPullRequests())
}

func TestLocalGitProvider_ErrNotImplemented_remoteMethods(t *testing.T) {
	t.Parallel()
	l := NewLocalGitProvider(Credentials{})
	ctx := context.Background()
	repo := "https://github.com/o/r"
	type call struct {
		name string
		fn   func() error
	}
	calls := []call{
		{"ListBranches", func() error { _, err := l.ListBranches(ctx, repo, ""); return err }},
		{"DeleteBranch", func() error { return l.DeleteBranch(ctx, repo, "b") }},
		{"GetDiff", func() error { _, err := l.GetDiff(ctx, repo, "a", "b"); return err }},
		{"GetFileContent", func() error { _, err := l.GetFileContent(ctx, repo, "m", "p"); return err }},
		{"GetRepoInfo", func() error { _, err := l.GetRepoInfo(ctx, repo); return err }},
		{"CreatePullRequest", func() error { _, err := l.CreatePullRequest(ctx, repo, PRCreateOptions{}); return err }},
		{"UpdatePullRequest", func() error { _, err := l.UpdatePullRequest(ctx, repo, 1, PRUpdateOptions{}); return err }},
		{"GetPullRequest", func() error { _, err := l.GetPullRequest(ctx, repo, 1); return err }},
		{"ListPullRequests", func() error { _, err := l.ListPullRequests(ctx, repo, PROptions{}); return err }},
		{"ListPRFiles", func() error { _, err := l.ListPRFiles(ctx, repo, 1); return err }},
		{"ListPRComments", func() error { _, err := l.ListPRComments(ctx, repo, 1); return err }},
		{"AddPRComment", func() error { return l.AddPRComment(ctx, repo, 1, "x") }},
		{"AddPRReviewComment", func() error { return l.AddPRReviewComment(ctx, repo, 1, PRReviewCommentOptions{}) }},
		{"SubmitPRReview", func() error { return l.SubmitPRReview(ctx, repo, 1, PRReviewOptions{Event: ReviewEventApprove}) }},
		{"MergePullRequest", func() error { return l.MergePullRequest(ctx, repo, 1, PRMergeOptions{SHA: "x"}) }},
	}
	require.Len(t, calls, 15, "ожидаем ровно 15 ErrNotImplemented у LocalGitProvider")
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.ErrorIs(t, tc.fn(), ErrNotImplemented)
		})
	}
}

func TestLocalGitProvider_nilContext(t *testing.T) {
	t.Parallel()
	l := NewLocalGitProvider(Credentials{})
	var nilCtx context.Context
	require.ErrorContains(t, l.ValidateAccess(nilCtx, "https://x/y.git"), "nil context")
	require.ErrorContains(t, l.Clone(nilCtx, "https://x/y.git", CloneOptions{DestPath: t.TempDir()}), "nil context")
	require.ErrorContains(t, l.Push(nilCtx, "/w", PushOptions{Branch: "b"}), "nil context")
	_, _, err := l.CommitAndPush(nilCtx, "/w", CommitOptions{Message: "m"}, PushOptions{Branch: "b"})
	require.ErrorContains(t, err, "nil context")
	_, err2 := l.ListBranches(nilCtx, "u", "")
	require.ErrorContains(t, err2, "nil context")
}

func TestLocalGitProvider_ValidateAccess_table(t *testing.T) {
	t.Parallel()
	t.Run("empty_repo_url_ErrRepoNotFound", func(t *testing.T) {
		t.Parallel()
		l := NewLocalGitProvider(Credentials{})
		require.ErrorIs(t, l.ValidateAccess(context.Background(), "   "), ErrRepoNotFound)
	})
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{}
		rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
			require.Equal(t, "ls-remote", args[0])
			require.Equal(t, "--", args[1])
			return "deadbeef\tHEAD\n", "", nil
		}
		l := newLocalProviderWithRunner(t, Credentials{Token: "ghp_x"}, rr)
		require.NoError(t, l.ValidateAccess(context.Background(), "https://github.com/o/r.git"))
	})
	t.Run("auth_stderr_wrapped", func(t *testing.T) {
		t.Parallel()
		tok := "ghp_secret"
		rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
			return "", "authentication failed for user", errors.New("exit 128")
		}}
		l := newLocalProviderWithRunner(t, Credentials{Token: tok}, rr)
		err := l.ValidateAccess(context.Background(), "https://github.com/o/r.git")
		require.ErrorIs(t, err, ErrAuthFailed)
		require.NotContains(t, err.Error(), tok)
		require.Contains(t, err.Error(), "details:")
	})
	t.Run("other_stderr_ErrRepoNotFound_wrapped", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
			return "", "repository not found on server", errors.New("exit 128")
		}}
		l := newLocalProviderWithRunner(t, Credentials{}, rr)
		err := l.ValidateAccess(context.Background(), "https://github.com/o/r.git")
		require.ErrorIs(t, err, ErrRepoNotFound)
		require.Contains(t, err.Error(), "details:")
	})
}

func TestLocalGitProvider_Clone_table(t *testing.T) {
	t.Parallel()
	destOK := filepath.Join(t.TempDir(), "clone-target")
	ctx := context.Background()

	t.Run("success_argv", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{}
		rr.runHook = func(c context.Context, workDir string, args []string) (string, string, error) {
			require.Equal(t, "clone", args[0])
			require.Equal(t, "--branch=main", args[1])
			i := len(args) - 3
			require.GreaterOrEqual(t, i, 1)
			require.Equal(t, "--", args[i])
			return "", "", nil
		}
		l := newLocalProviderWithRunner(t, Credentials{Token: "ghp_x"}, rr)
		require.NoError(t, l.Clone(ctx, "https://github.com/o/r.git", CloneOptions{DestPath: destOK, Branch: "main"}))
	})

	t.Run("empty_dest", func(t *testing.T) {
		t.Parallel()
		l := newLocalProviderWithRunner(t, Credentials{}, &recordingRunner{})
		err := l.Clone(ctx, "https://x/y.git", CloneOptions{DestPath: "  "})
		require.ErrorContains(t, err, "empty clone destination")
	})

	t.Run("unsafe_dest_ErrUnsafePath", func(t *testing.T) {
		t.Parallel()
		l := newLocalProviderWithRunner(t, Credentials{}, &recordingRunner{})
		err := l.Clone(ctx, "https://x/y.git", CloneOptions{DestPath: "/etc/passwd"})
		require.ErrorIs(t, err, ErrUnsafePath)
	})

	t.Run("branch_injection_ErrUnsafePath", func(t *testing.T) {
		t.Parallel()
		l := newLocalProviderWithRunner(t, Credentials{}, &recordingRunner{})
		err := l.Clone(ctx, "https://x/y.git", CloneOptions{DestPath: destOK, Branch: "main--upload-pack=id"})
		require.ErrorIs(t, err, ErrUnsafePath)
	})

	t.Run("clone_failed_ErrCloneFailed", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
			return "", "network down", errors.New("exit 1")
		}}
		l := newLocalProviderWithRunner(t, Credentials{}, rr)
		err := l.Clone(ctx, "https://x/y.git", CloneOptions{DestPath: destOK})
		require.ErrorIs(t, err, ErrCloneFailed)
		require.Contains(t, err.Error(), "details:")
	})

	t.Run("token_masked_in_clone_error", func(t *testing.T) {
		t.Parallel()
		tok := "ghp_LEAK_TEST"
		rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
			return "", "error: " + tok + " bad", errors.New("exit 1")
		}}
		l := newLocalProviderWithRunner(t, Credentials{Token: tok}, rr)
		err := l.Clone(ctx, "https://github.com/o/r.git", CloneOptions{DestPath: destOK})
		require.ErrorIs(t, err, ErrCloneFailed)
		require.NotContains(t, err.Error(), tok)
	})
}

func TestLocalGitProvider_Push_table(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty_workdir", func(t *testing.T) {
		t.Parallel()
		l := NewLocalGitProvider(Credentials{})
		err := l.Push(ctx, "  ", PushOptions{Branch: "b"})
		require.ErrorContains(t, err, "empty work directory")
	})

	t.Run("empty_branch", func(t *testing.T) {
		t.Parallel()
		l := NewLocalGitProvider(Credentials{})
		require.ErrorIs(t, l.Push(ctx, "/w", PushOptions{Branch: "  "}), ErrPushBranchRequired)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{}
		rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
			switch args[0] {
			case "remote":
				return "https://github.com/o/r.git", "", nil
			case "push":
				return "", "", nil
			default:
				return "", "", errors.New("unexpected")
			}
		}
		l := newLocalProviderWithRunner(t, Credentials{Token: "ghp_x"}, rr)
		require.NoError(t, l.Push(ctx, "/w", PushOptions{Branch: "b"}))
	})

	t.Run("branch_injection_before_run", func(t *testing.T) {
		t.Parallel()
		l := newLocalProviderWithRunner(t, Credentials{}, &recordingRunner{})
		err := l.Push(ctx, "/w", PushOptions{Branch: "x--receive-pack=pwned"})
		require.ErrorIs(t, err, ErrUnsafePath)
	})

	t.Run("rejected_ErrConflict_wrapped", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{}
		rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
			if args[0] == "remote" {
				return "https://github.com/o/r.git", "", nil
			}
			return "", "failed to push: rejected", errors.New("exit 1")
		}
		l := newLocalProviderWithRunner(t, Credentials{}, rr)
		err := l.Push(ctx, "/w", PushOptions{Branch: "b"})
		require.ErrorIs(t, err, ErrConflict)
		require.Contains(t, err.Error(), "details:")
	})

	t.Run("permission_ErrPermissionDenied_wrapped", func(t *testing.T) {
		t.Parallel()
		rr := &recordingRunner{}
		rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
			if args[0] == "remote" {
				return "https://github.com/o/r.git", "", nil
			}
			return "", "remote: 403", errors.New("exit 1")
		}
		l := newLocalProviderWithRunner(t, Credentials{}, rr)
		err := l.Push(ctx, "/w", PushOptions{Branch: "b"})
		require.ErrorIs(t, err, ErrPermissionDenied)
	})
}

func TestLocalGitProvider_CommitAndPush_pushError_returnsCommitSHA(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		switch args[0] {
		case "remote":
			return "https://github.com/o/r.git", "", nil
		case "push":
			return "", "rejected", errors.New("exit 1")
		case "add":
			return "", "", nil
		case "rev-parse":
			if len(args) >= 2 && args[1] == "--verify" {
				return "", "fatal", errors.New("exit 128")
			}
			return "abc1234", "", nil
		case "status":
			return "M a\n", "", nil
		case "commit":
			return "", "", nil
		default:
			return "", "", errors.New("unexpected")
		}
	}
	l := newLocalProviderWithRunner(t, Credentials{}, rr)
	sha, ch, err := l.CommitAndPush(context.Background(), "/w", CommitOptions{Message: "m"}, PushOptions{Branch: "b"})
	require.ErrorIs(t, err, ErrConflict)
	require.True(t, ch)
	require.Equal(t, "abc1234", sha)
}

func TestLocalGitProvider_Clone_withDepth_argv(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
		require.Equal(t, "clone", args[0])
		require.Equal(t, "--depth", args[1])
		require.Equal(t, "3", args[2])
		i := len(args) - 3
		require.Equal(t, "--", args[i])
		return "", "", nil
	}}
	l := newLocalProviderWithRunner(t, Credentials{}, rr)
	dest := filepath.Join(t.TempDir(), "d")
	require.NoError(t, l.Clone(context.Background(), "https://github.com/o/r.git", CloneOptions{DestPath: dest, Depth: 3}))
}

func TestLocalGitProvider_CommitAndPush_happyPath_mocked(t *testing.T) {
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
	l := newLocalProviderWithRunner(t, Credentials{Token: "ghp_x"}, rr)
	sha, ch, err := l.CommitAndPush(context.Background(), "/w", CommitOptions{Message: "m"}, PushOptions{Branch: "b"})
	require.NoError(t, err)
	require.True(t, ch)
	require.Equal(t, "cafecafe", sha)
}

func TestValidateCloneDestPath_table(t *testing.T) {
	t.Parallel()
	ok := t.TempDir()
	sub := filepath.Join(ok, "nested", "clone")
	require.NoError(t, validateCloneDestPath(sub))

	require.ErrorContains(t, validateCloneDestPath(""), "empty clone destination")
	require.ErrorContains(t, validateCloneDestPath("   "), "empty clone destination")
	require.ErrorIs(t, validateCloneDestPath("/etc/passwd"), ErrUnsafePath)
}

func TestLocalGitProvider_ValidateAccess_token_in_URL_masked_in_error(t *testing.T) {
	t.Parallel()
	tok := "ghp_abc+def"
	rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
		return "", "failed " + tok, errors.New("exit 1")
	}}
	l := newLocalProviderWithRunner(t, Credentials{Token: tok}, rr)
	e := l.ValidateAccess(context.Background(), "https://github.com/o/r.git")
	require.ErrorIs(t, e, ErrRepoNotFound)
	require.NotContains(t, e.Error(), tok)
}
