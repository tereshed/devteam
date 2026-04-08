package gitprovider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-github/v67/github"
)

func TestValidatePushBranch_flagLike(t *testing.T) {
	t.Parallel()
	if err := validatePushBranch("-h"); err == nil {
		t.Fatal("expected error")
	}
}

func TestInjectTokenInURL_and_isHTTPURL(t *testing.T) {
	t.Parallel()
	tok := "ghp_UNIQUE_TEST_TOKEN_9zz"
	u := injectTokenInURL("https://github.com/o/r.git", tok)
	if !strings.Contains(u, "x-access-token") || !strings.Contains(u, "@github.com") {
		t.Fatalf("unexpected %q", u)
	}
	if injectTokenInURL("ssh://git@github.com/o/r.git", "tok") == "" {
		t.Fatal("expected non-empty")
	}
	if got := injectTokenInURL("https://[::1", "tok"); got != "https://[::1" {
		t.Fatalf("parse fail should return original: %q", got)
	}
	if isHTTPURL("ftp://x") {
		t.Fatal("ftp")
	}
	if !isHTTPURL("HTTPS://github.com/x/y") {
		t.Fatal("https")
	}
}

func TestPushURLForGitHubRemote_table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		remote  string
		token   string
		wantErr bool
	}{
		{"ssh", "git@github.com:o/r.git", "t", false},
		{"https", "https://github.com/o/r.git", "t", false},
		{"empty", "", "t", true},
		{"no token", "https://github.com/o/r.git", "", true},
		{"bad host", "https://gitlab.com/o/r.git", "t", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := pushURLForGitHubRemote(tc.remote, tc.token)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || !strings.Contains(got, "github.com") {
				t.Fatalf("%q %v", got, err)
			}
		})
	}
}

func TestGitHubProvider_Push_httpsRemote(t *testing.T) {
	t.Parallel()
	step := 0
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		step++
		if args[0] == "remote" {
			return "https://github.com/o/r.git", "", nil
		}
		if args[0] == "push" {
			if workDir != "/repo" {
				t.Fatalf("workDir %q", workDir)
			}
			return "", "", nil
		}
		return "", "", errors.New("unexpected")
	}
	p := NewGitHubProviderWithDeps(Credentials{Token: "ghp_X"}, nil, rr)
	err := p.Push(context.Background(), "/repo", PushOptions{Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if step != 2 {
		t.Fatalf("steps %d", step)
	}
}

func TestGitHubProvider_Push_sshRemote(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	n := 0
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		n++
		if args[0] == "remote" {
			return "git@github.com:o/r.git", "", nil
		}
		return "", "", nil
	}
	p := NewGitHubProviderWithDeps(Credentials{Token: "ghp_X"}, nil, rr)
	if err := p.Push(context.Background(), "/repo", PushOptions{Branch: "topic"}); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("calls %d", n)
	}
}

func TestGitHubProvider_Push_rejectedMapsConflict(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		if args[0] == "remote" {
			return "https://github.com/o/r.git", "", nil
		}
		return "", "rejected by hook", errors.New("exit 1")
	}
	p := NewGitHubProviderWithDeps(Credentials{Token: "ghp_longtoken_not_substring_of_git"}, nil, rr)
	err := p.Push(context.Background(), "/repo", PushOptions{Branch: "main"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestExecuteCommit_emptyMessage(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	_, _, err := executeCommit(context.Background(), rr, "", "/w", CommitOptions{Message: "   "})
	if err == nil || !strings.Contains(err.Error(), "empty commit message") {
		t.Fatalf("%v", err)
	}
}

func TestExecuteCommit_noChangesEmptyRepo(t *testing.T) {
	t.Parallel()
	step := 0
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		step++
		switch {
		case args[0] == "add":
			return "", "", nil
		case args[0] == "rev-parse" && args[len(args)-1] == "HEAD":
			return "", "fatal: bad", errors.New("exit 128")
		case args[0] == "status":
			return "", "", nil
		}
		return "", "", errors.New("unexpected")
	}
	sha, changed, err := executeCommit(context.Background(), rr, "", "/w", CommitOptions{Message: "m", Files: nil})
	if err != nil {
		t.Fatal(err)
	}
	if changed || sha != "" {
		t.Fatalf("%q %v", sha, changed)
	}
	if step != 3 {
		t.Fatalf("steps %d", step)
	}
}

func TestRunGit_masksTokenInReturnedError(t *testing.T) {
	t.Parallel()
	tok := "ghp_MASKED_IN_CAUSE"
	rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
		return "", tok + " stderr", errors.New("wrapped " + tok)
	}}
	_, err := runGit(context.Background(), rr, tok, "/w", "status")
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), tok) {
		t.Fatalf("token leaked: %s", err.Error())
	}
}

func TestRunGit_errorMessageUsesQuestionWhenNoArgs(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{runHook: func(ctx context.Context, workDir string, args []string) (string, string, error) {
		if len(args) != 0 {
			t.Fatalf("args %v", args)
		}
		return "", "boom", errors.New("exit")
	}}
	_, err := runGit(context.Background(), rr, "", "/w")
	if err == nil || !strings.Contains(err.Error(), "git ?:") {
		t.Fatalf("%v", err)
	}
}

func TestLocalGitCLI_effectiveRunner_nonNil(t *testing.T) {
	t.Parallel()
	c := LocalGitCLI{creds: Credentials{}}
	if c.effectiveRunner() == nil {
		t.Fatal("nil runner")
	}
}

func TestMapCommitFile_nil(t *testing.T) {
	t.Parallel()
	if m := mapCommitFile(nil); m.Filename != "" {
		t.Fatal(m)
	}
}

func TestMapPullRequest_nil(t *testing.T) {
	t.Parallel()
	if mapPullRequest(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestMapRepository_nil(t *testing.T) {
	t.Parallel()
	if mapRepository(nil) != nil {
		t.Fatal("expected nil")
	}
}

func TestMapFileContentError(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		StatusCode: 404,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"message":"missing"}`)),
	}
	err := mapFileContentError(github.CheckResponse(resp))
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("%v", err)
	}
}
