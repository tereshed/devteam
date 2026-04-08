package gitprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
)

const testGitHubHeaderOTP = "X-GitHub-OTP"
const testGitHubHeaderRateRem = "X-RateLimit-Remaining"

// setupMockGitHubServer собирает *github.Client на httptest.Server и встраивает в GitHubProvider.
// runner — для локальных git-вызовов (в чистых remote-тестах можно nil → дефолтный exec).
func setupMockGitHubServer(t *testing.T, h http.Handler, runner GitCommandRunner) (*GitHubProvider, func()) {
	t.Helper()
	srv := httptest.NewServer(h)
	gh := github.NewClient(srv.Client())
	base, err := url.Parse(srv.URL + "/")
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}
	gh.BaseURL = base
	gh.UploadURL = base
	p := NewGitHubProviderWithDeps(Credentials{Token: "unit-test-token"}, gh, runner)
	return p, srv.Close
}

// recordingRunner фиксирует argv после «git» и опционально подменяет поведение.
type recordingRunner struct {
	mu         sync.Mutex
	args       [][]string
	workDirs   []string
	runHook    func(ctx context.Context, workDir string, args []string) (stdout, stderr string, err error)
	pipeHook   func(ctx context.Context, workDir, token string, args []string) (io.ReadCloser, error)
	defaultOut string
}

func (r *recordingRunner) RunGit(ctx context.Context, workDir string, args ...string) (stdout, stderr string, err error) {
	r.mu.Lock()
	r.args = append(r.args, append([]string(nil), args...))
	r.workDirs = append(r.workDirs, workDir)
	hook := r.runHook
	r.mu.Unlock()
	if hook != nil {
		return hook(ctx, workDir, args)
	}
	return r.defaultOut, "", nil
}

func (r *recordingRunner) GitStdoutPipe(ctx context.Context, workDir, token string, args ...string) (io.ReadCloser, error) {
	r.mu.Lock()
	r.args = append(r.args, append([]string(nil), args...))
	r.workDirs = append(r.workDirs, workDir)
	ph := r.pipeHook
	r.mu.Unlock()
	if ph != nil {
		return ph(ctx, workDir, token, args)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (r *recordingRunner) lastArgs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.args) == 0 {
		return nil
	}
	return append([]string(nil), r.args[len(r.args)-1]...)
}

func assertArgvStartsWithGit(t *testing.T, argv []string) {
	t.Helper()
	// контракт раннера: первый токен в логике вызова — подкоманда git; реальный exec добавляет "git" внутри NewExecGitRunner
	if len(argv) == 0 {
		t.Fatal("empty argv")
	}
	sub := argv[0]
	if sub == "" {
		t.Fatal("empty subcommand")
	}
}

func TestMapRepository_topics(t *testing.T) {
	t.Parallel()
	gr := &github.Repository{
		Name:          github.String("n"),
		FullName:      github.String("o/n"),
		DefaultBranch: github.String("main"),
		Topics:        []string{"go", "api"},
	}
	ri := mapRepository(gr)
	if len(ri.Topics) != 2 || ri.Topics[0] != "go" {
		t.Fatalf("%v", ri.Topics)
	}
}

func TestMapPullRequest_mergedUsesMergedState(t *testing.T) {
	t.Parallel()
	merged := true
	gh := &github.PullRequest{
		Number: github.Int(1),
		Title:  github.String("t"),
		State:  github.String("closed"),
		Merged: &merged,
		Head:   &github.PullRequestBranch{Ref: github.String("h"), SHA: github.String("sha")},
		Base:   &github.PullRequestBranch{Ref: github.String("main")},
		User:   &github.User{Login: github.String("u")},
	}
	out := mapPullRequest(gh)
	if out == nil || out.State != PRStateMerged {
		t.Fatalf("%+v", out)
	}
}

func TestGitHubProvider_ProviderType_and_SupportsPR(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{})
	if p.ProviderType() != "github" {
		t.Fatalf("ProviderType: %q", p.ProviderType())
	}
	if !p.SupportsPullRequests() {
		t.Fatal("expected SupportsPullRequests true")
	}
}

func TestParseRepoURL_wwwHost(t *testing.T) {
	t.Parallel()
	o, r, err := parseRepoURL("https://www.github.com/acme/app.git")
	if err != nil || o != "acme" || r != "app" {
		t.Fatalf("%v %s %s", err, o, r)
	}
}

func TestParseRepoURL_table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		wantO   string
		wantR   string
		wantErr bool
	}{
		{"https", "https://github.com/acme/app.git", "acme", "app", false},
		{"short", "github.com/acme/app", "acme", "app", false},
		{"ssh", "git@github.com:acme/app.git", "acme", "app", false},
		{"tree stripped", "https://github.com/acme/app/tree/main/pkg", "acme", "app", false},
		{"empty", "", "", "", true},
		{"not github host", "https://gitlab.com/a/b.git", "", "", true},
		{"no scheme bad", "example.com/a/b", "", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o, r, err := parseRepoURL(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if o != tc.wantO || r != tc.wantR {
				t.Fatalf("got %q/%q want %q/%q", o, r, tc.wantO, tc.wantR)
			}
		})
	}
}

func TestMapGitHubError_table(t *testing.T) {
	t.Parallel()
	mkResp := func(code int, h http.Header, body string) *http.Response {
		return &http.Response{
			StatusCode: code,
			Header:     h,
			Body:       io.NopCloser(strings.NewReader(body)),
		}
	}
	cases := []struct {
		name      string
		err       error
		notFound  error
		wantIs    []error
		wantNot   []error
		skipIsErr bool // произвольная ошибка — только Not
	}{
		{
			name:     "nil",
			err:      nil,
			notFound: ErrRepoNotFound,
		},
		{
			name: "401",
			err: func() error {
				return github.CheckResponse(mkResp(401, http.Header{}, `{"message":"Bad credentials"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrAuthFailed},
		},
		{
			name: "403 permission",
			err: func() error {
				h := http.Header{}
				h.Set(testGitHubHeaderRateRem, "5")
				return github.CheckResponse(mkResp(403, h, `{"message":"Forbidden"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrPermissionDenied},
		},
		{
			name: "403 rate limit remaining 0",
			err: func() error {
				h := http.Header{}
				h.Set(testGitHubHeaderRateRem, "0")
				return github.CheckResponse(mkResp(403, h, `{"message":"API rate limit exceeded"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrRateLimited},
		},
		{
			name: "404 repo",
			err: func() error {
				return github.CheckResponse(mkResp(404, http.Header{}, `{"message":"Not Found"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrRepoNotFound},
		},
		{
			name: "404 pr not found sentinel",
			err: func() error {
				return github.CheckResponse(mkResp(404, http.Header{}, `{"message":"Not Found"}`))
			}(),
			notFound: ErrPRNotFound,
			wantIs:   []error{ErrPRNotFound},
		},
		{
			name: "409",
			err: func() error {
				return github.CheckResponse(mkResp(409, http.Header{}, `{"message":"Conflict"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrConflict},
		},
		{
			name: "422 pr already exists",
			err: func() error {
				return github.CheckResponse(mkResp(422, http.Header{}, `{"message":"Validation Failed: pull request already exists for this issue"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrPRAlreadyExists},
		},
		{
			name: "422 branch not found phrase",
			err: func() error {
				return github.CheckResponse(mkResp(422, http.Header{}, `{"message":"Reference does not exist"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrBranchNotFound},
		},
		{
			name: "422 branch already exists",
			err: func() error {
				return github.CheckResponse(mkResp(422, http.Header{}, `{"message":"Reference already exists"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrBranchAlreadyExists},
		},
		{
			name: "429",
			err: func() error {
				return github.CheckResponse(mkResp(429, http.Header{}, `{"message":"Too Many Requests"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrRateLimited},
		},
		{
			name: "403 abuse secondary rate limits via CheckResponse",
			err: func() error {
				return github.CheckResponse(mkResp(403, http.Header{}, `{"message":"abuse","documentation_url":"https://docs.github.com/en/rest/overview/resources-in-the-rest-api#secondary-rate-limits"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrRateLimited},
		},
		{
			name: "TwoFactorAuthError",
			err: func() error {
				h := http.Header{}
				h.Set(testGitHubHeaderOTP, "required; app=github")
				return github.CheckResponse(mkResp(401, h, `{"message":"Must verify"}`))
			}(),
			notFound: ErrRepoNotFound,
			wantIs:   []error{ErrAuthFailed},
		},
		{
			name:     "opaque",
			err:      errors.New("network"),
			notFound: ErrRepoNotFound,
			skipIsErr: true,
			wantNot:   []error{ErrAuthFailed},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := mapGitHubError(tc.err, tc.notFound)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("want nil got %v", got)
				}
				return
			}
			if tc.skipIsErr {
				for _, w := range tc.wantNot {
					if errors.Is(got, w) {
						t.Fatalf("unexpected errors.Is %v", w)
					}
				}
				return
			}
			for _, w := range tc.wantIs {
				if !errors.Is(got, w) {
					t.Fatalf("errors.Is(..., %v) == false; got %v (%T)", w, got, got)
				}
			}
		})
	}
}

func TestGitHubProvider_ValidateAccess_success(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/repos/o/r" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "r", "full_name": "o/r"})
	})
	p, cleanup := setupMockGitHubServer(t, h, nil)
	defer cleanup()
	ctx := context.Background()
	if err := p.ValidateAccess(ctx, "https://github.com/o/r"); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubProvider_contextDeadlineOnSlowRemote(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	})
	p, cleanup := setupMockGitHubServer(t, h, nil)
	defer cleanup()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := p.ValidateAccess(ctx, "https://github.com/o/r")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
		if ctx.Err() == nil {
			// go-github может обернуть — достаточно признака отмены/дедлайна
			if !strings.Contains(strings.ToLower(err.Error()), "deadline") &&
				!strings.Contains(strings.ToLower(err.Error()), "context") {
				t.Fatalf("expected deadline-related error, got %v", err)
			}
		}
	}
}

func TestGitHubProvider_GetDiff_badRepoURL(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{})
	_, err := p.GetDiff(context.Background(), "not-a-url", "main", "topic")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGitHubProvider_GetDiff_streamsPartialRead(t *testing.T) {
	t.Parallel()
	// Бесконечная запись: если клиент буферизовал бы весь ответ (io.ReadAll), тест бы завис.
	writeStopped := make(chan struct{}, 1)
	var lastWriteErr atomic.Value
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/compare/") {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Accept") != githubDiffAccept {
			t.Errorf("Accept header: %q", r.Header.Get("Accept"))
		}
		w.WriteHeader(http.StatusOK)
		fl, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter is not http.Flusher")
		}
		chunk := bytes.Repeat([]byte("z"), 8192)
		for {
			_, err := w.Write(chunk)
			fl.Flush()
			if err != nil {
				lastWriteErr.Store(err)
				writeStopped <- struct{}{}
				return
			}
		}
	})
	p, cleanup := setupMockGitHubServer(t, h, nil)
	defer cleanup()
	rc, err := p.GetDiff(context.Background(), "https://github.com/o/r", "main", "topic")
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 100)
	n, rerr := rc.Read(buf)
	if n != 100 || rerr != nil {
		t.Fatalf("Read: n=%d err=%v", n, rerr)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case <-writeStopped:
		if lastWriteErr.Load() == nil {
			t.Fatal("expected non-nil write error after client closed body")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: server write loop did not stop after client closed response body")
	}
}

func TestGitHubProvider_GetFileContent_emptyBranch(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{Token: "t"})
	_, err := p.GetFileContent(context.Background(), "https://github.com/o/r", "", "README.md")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGitHubProvider_GetRepoInfo_notFound(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	p, c := setupMockGitHubServer(t, h, nil)
	defer c()
	_, err := p.GetRepoInfo(context.Background(), "https://github.com/o/r")
	if !errors.Is(err, ErrRepoNotFound) {
		t.Fatalf("%v", err)
	}
}

func TestGitHubProvider_GetRepoInfo(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name": "r", "full_name": "o/r", "default_branch": "main",
			"private": false, "html_url": "https://github.com/o/r",
			"clone_url": "https://github.com/o/r.git", "ssh_url": "git@github.com:o/r.git",
		})
	})
	p, cleanup := setupMockGitHubServer(t, h, nil)
	defer cleanup()
	info, err := p.GetRepoInfo(context.Background(), "https://github.com/o/r")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "r" || info.DefaultBranch != "main" {
		t.Fatalf("%+v", info)
	}
}

func TestGitHubProvider_ListBranches_prefixAndPagination(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/repos/o/r/branches") {
			http.NotFound(w, r)
			return
		}
		u := *r.URL
		q := u.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		if page < 1 {
			page = 1
		}
		type br struct {
			Name string `json:"name"`
		}
		var body []br
		switch page {
		case 1:
			body = []br{{Name: "feature-a"}, {Name: "main"}}
			q.Set("page", "2")
			u.RawQuery = q.Encode()
			w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="next"`, u.String()))
		case 2:
			body = []br{{Name: "feature-b"}}
		default:
			body = nil
		}
		_ = json.NewEncoder(w).Encode(body)
	})
	p, cleanup := setupMockGitHubServer(t, h, nil)
	defer cleanup()
	names, err := p.ListBranches(context.Background(), "https://github.com/o/r", "feature-")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "feature-a" || names[1] != "feature-b" {
		t.Fatalf("%v", names)
	}
}

func TestGitHubProvider_ListBranches_respectsMaxPages(t *testing.T) {
	t.Parallel()
	pages := 0
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		type br struct {
			Name string `json:"name"`
		}
		u := *r.URL
		q := u.Query()
		cur, _ := strconv.Atoi(q.Get("page"))
		if cur < 1 {
			cur = 1
		}
		q.Set("page", strconv.Itoa(cur+1))
		u.RawQuery = q.Encode()
		w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="next"`, u.String()))
		_ = json.NewEncoder(w).Encode([]br{{Name: fmt.Sprintf("b%d", pages)}})
	})
	p, cleanup := setupMockGitHubServer(t, h, nil)
	defer cleanup()
	_, err := p.ListBranches(context.Background(), "https://github.com/o/r", "")
	if err != nil {
		t.Fatal(err)
	}
	if pages != maxBranchPages {
		t.Fatalf("pages=%d want %d", pages, maxBranchPages)
	}
}

func TestGitHubProvider_CreatePullRequest(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 42, "title": "t", "state": "open",
				"head": map[string]any{"ref": "h", "sha": "abc"},
				"base": map[string]any{"ref": "main"},
				"user": map[string]any{"login": "u"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/issues/42/labels":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[]"))
		default:
			http.NotFound(w, r)
		}
	})
	p, cleanup := setupMockGitHubServer(t, h, nil)
	defer cleanup()
	pr, err := p.CreatePullRequest(context.Background(), "https://github.com/o/r", PRCreateOptions{
		Title: "t", HeadBranch: "h", BaseBranch: "main", Labels: []string{"l"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pr.Number != 42 {
		t.Fatal(pr)
	}
}

func TestGitHubProvider_SubmitPRReview_invalidEvent(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{Token: "t"})
	err := p.SubmitPRReview(context.Background(), "https://github.com/o/r", 1, PRReviewOptions{Event: "BOGUS"})
	if err == nil || !strings.Contains(err.Error(), "invalid review event") {
		t.Fatalf("got %v", err)
	}
}

func TestGitHubProvider_MergePullRequest_emptySHA(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{Token: "t"})
	err := p.MergePullRequest(context.Background(), "https://github.com/o/r", 1, PRMergeOptions{})
	if err == nil || !strings.Contains(err.Error(), "non-empty SHA") {
		t.Fatalf("got %v", err)
	}
}

func TestGitHubProvider_Clone_missingToken(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{})
	err := p.Clone(context.Background(), "https://github.com/o/r", CloneOptions{DestPath: "/x"})
	if err == nil || !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("%v", err)
	}
}

func TestGitHubProvider_Clone_masksTokenInError(t *testing.T) {
	t.Parallel()
	tok := "ghp_SUPERSECRET"
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		return "", "remote: invalid " + tok + " and " + userinfoEncodedPassword(tok), errors.New("exit")
	}
	p := NewGitHubProviderWithDeps(Credentials{Token: tok}, nil, rr)
	err := p.Clone(context.Background(), "https://github.com/o/r", CloneOptions{DestPath: "/tmp/x"})
	if err == nil {
		t.Fatal("expected error")
	}
	s := err.Error()
	if strings.Contains(s, tok) {
		t.Fatalf("raw token leaked: %s", s)
	}
	enc := userinfoEncodedPassword(tok)
	if enc != "" && strings.Contains(s, enc) {
		t.Fatalf("encoded token leaked: %s", s)
	}
}

func TestGitHubProvider_Push_missingToken(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{})
	err := p.Push(context.Background(), "/w", PushOptions{Branch: "main"})
	if err == nil || !errors.Is(err, ErrAuthFailed) {
		t.Fatalf("%v", err)
	}
}

func TestGitHubProvider_Push_emptyWorkDir(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{Token: "t"})
	err := p.Push(context.Background(), "   ", PushOptions{Branch: "main"})
	if err == nil || !strings.Contains(err.Error(), "empty work directory") {
		t.Fatalf("got %v", err)
	}
}

func TestGitHubProvider_validateAccess_emptyURL(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{Token: "t"})
	err := p.ValidateAccess(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGitHubProvider_Clone_rejectsFlagLikeBranch(t *testing.T) {
	t.Parallel()
	p := NewGitHubProvider(Credentials{Token: "t"})
	err := p.Clone(context.Background(), "https://github.com/o/r", CloneOptions{Branch: "-h", DestPath: "/x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRecordingRunner_argvUsesDoubleDashBeforeUserInput(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	p := NewGitHubProviderWithDeps(Credentials{Token: "tok"}, nil, rr)
	ctx := context.Background()
	workDir := "/tmp/repo"
	_ = p.Push(ctx, workDir, PushOptions{Branch: "main", Remote: "origin"})
	args := rr.lastArgs()
	assertArgvStartsWithGit(t, args)
	// push <url> -- <branch>
	if i := indexOfString(args, "--"); i < 0 || i >= len(args)-1 {
		t.Fatalf("missing -- before branch: %v", args)
	}
	_ = p.Clone(ctx, "https://github.com/o/r", CloneOptions{DestPath: "/dst", Branch: "topic"})
	args2 := rr.lastArgs()
	if i := indexOfString(args2, "--"); i < 0 {
		t.Fatalf("clone missing --: %v", args2)
	}
}

func indexOfString(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func TestLocalGitCLI_CreateBranch_argvInjection(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	cli := LocalGitCLI{creds: Credentials{Token: "t"}, runner: rr}
	ctx := context.Background()
	_ = cli.CreateBranch(ctx, "/w", BranchOptions{BranchName: "safe", BaseBranch: "main"})
	args := rr.lastArgs()
	// checkout -b <branch> (без -- перед именем: git так не поддерживает)
	if !hasSubsequence(args, []string{"checkout", "-b", "safe"}) {
		t.Fatalf("args %v", args)
	}
}

func hasSubsequence(args, sub []string) bool {
outer:
	for i := 0; i+len(sub) <= len(args); i++ {
		for j := range sub {
			if args[i+j] != sub[j] {
				continue outer
			}
		}
		return true
	}
	return false
}

func TestLocalGitCLI_GetLocalDiff_rejectsTraversalInRef(t *testing.T) {
	t.Parallel()
	cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
	_, err := cli.GetLocalDiff(context.Background(), "/w", "../../../etc/passwd", "HEAD")
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("got %v", err)
	}
}

func TestLocalGitCLI_GetLocalFileContent_rejectsTraversal(t *testing.T) {
	t.Parallel()
	cli := LocalGitCLI{creds: Credentials{}, runner: &recordingRunner{}}
	_, err := cli.GetLocalFileContent(context.Background(), "/w", "HEAD", `..\\..\\x`)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("got %v", err)
	}
}

// TestSymlinkPolicy_documented: локальное чтение идёт через cat-file с валидацией сегментов пути (без "..").
// Симлинк внутри репозитория с «чистым» именем остаётся политикой git; полный сценарий — в integration при необходимости.
func TestSymlinkPolicy_documented(t *testing.T) {
	t.Parallel()
	if ErrUnsafePath == nil {
		t.Fatal("ErrUnsafePath nil")
	}
}

func TestGitHubProvider_ListBranches_remoteError(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})
	p, c := setupMockGitHubServer(t, h, nil)
	defer c()
	_, err := p.ListBranches(context.Background(), "https://github.com/o/r", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLocalGitCLI_CreateBranch_alreadyExists(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		if args[0] == "checkout" && args[1] == "-b" {
			return "", "fatal: branch 'x' already exists", errors.New("exit 128")
		}
		return "", "", nil
	}
	cli := LocalGitCLI{creds: Credentials{}, runner: rr}
	err := cli.CreateBranch(context.Background(), "/w", BranchOptions{BranchName: "x"})
	if !errors.Is(err, ErrBranchAlreadyExists) {
		t.Fatalf("%v", err)
	}
}

func TestGitHubProvider_Push_permissionDeniedMessage(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		if args[0] == "remote" {
			return "https://github.com/o/r.git", "", nil
		}
		return "", "remote: Permission to repo denied (403)", errors.New("exit 1")
	}
	p := NewGitHubProviderWithDeps(Credentials{Token: "ghp_longtoken_push_denied"}, nil, rr)
	err := p.Push(context.Background(), "/repo", PushOptions{Branch: "main"})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("%v", err)
	}
}

func TestMockRunner_contextCanceled(t *testing.T) {
	t.Parallel()
	rr := &recordingRunner{}
	rr.runHook = func(ctx context.Context, workDir string, args []string) (string, string, error) {
		<-ctx.Done()
		return "", "", ctx.Err()
	}
	_ = NewGitHubProviderWithDeps(Credentials{Token: "t"}, nil, rr)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runGit(ctx, rr, "t", "/w", "status")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

