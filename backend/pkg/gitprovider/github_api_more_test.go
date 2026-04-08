package gitprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v67/github"
)

func TestGitHubProvider_DeleteBranch(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/git/refs/heads/topic") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.NotFound(w, r)
	})
	p, c := setupMockGitHubServer(t, h, nil)
	defer c()
	if err := p.DeleteBranch(context.Background(), "https://github.com/o/r", "topic"); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubProvider_GetPullRequest_Update_ListPRs(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls/7":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 7, "title": "x", "state": "open",
				"head": map[string]any{"ref": "h", "sha": "s"},
				"base": map[string]any{"ref": "main"},
				"user": map[string]any{"login": "a"},
			})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/o/r/pulls/7":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 7, "title": "y", "state": "open",
				"head": map[string]any{"ref": "h"},
				"base": map[string]any{"ref": "main"},
				"user": map[string]any{"login": "a"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/pulls":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"number": 7, "title": "x", "state": "open",
				"head":   map[string]any{"ref": "h"},
				"base":   map[string]any{"ref": "main"},
				"user":   map[string]any{"login": "a"},
			}})
		default:
			http.NotFound(w, r)
		}
	})
	p, c := setupMockGitHubServer(t, h, nil)
	defer c()
	ctx := context.Background()
	pr, err := p.GetPullRequest(ctx, "https://github.com/o/r", 7)
	if err != nil || pr.Number != 7 {
		t.Fatalf("%v %+v", err, pr)
	}
	title := "y"
	up, err := p.UpdatePullRequest(ctx, "https://github.com/o/r", 7, PRUpdateOptions{Title: &title})
	if err != nil || up.Title != "y" {
		t.Fatal(err, up)
	}
	list, err := p.ListPullRequests(ctx, "https://github.com/o/r", PROptions{HeadBranch: "h"})
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", err, list)
	}
}

func TestGitHubProvider_ListPRFiles_and_Comments(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/pulls/3/files"):
			q := r.URL.Query().Get("page")
			if q == "" || q == "1" {
				nu := *r.URL
				nq := nu.Query()
				nq.Set("page", "2")
				nu.RawQuery = nq.Encode()
				w.Header().Add("Link", fmt.Sprintf(`<%s>; rel="next"`, nu.String()))
				_ = json.NewEncoder(w).Encode([]map[string]any{{"filename": "a.go", "status": "added", "additions": 1, "deletions": 0}})
				return
			}
			_ = json.NewEncoder(w).Encode([]map[string]any{{"filename": "b.go", "status": "modified", "additions": 0, "deletions": 2}})
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/issues/3/comments"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": 1, "body": "issue", "created_at": time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC).Format(time.RFC3339),
				"updated_at": time.Date(2020, 1, 2, 3, 4, 6, 0, time.UTC).Format(time.RFC3339),
				"user":         map[string]any{"login": "u1"},
			}})
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/pulls/3/comments"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": 2, "body": "review", "path": "a.go", "line": 10, "commit_id": "abc", "side": "RIGHT",
				"created_at": time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
				"updated_at": time.Date(2020, 1, 1, 0, 0, 1, 0, time.UTC).Format(time.RFC3339),
				"user":       map[string]any{"login": "u2"},
			}})
		default:
			http.NotFound(w, r)
		}
	})
	p, c := setupMockGitHubServer(t, h, nil)
	defer c()
	files, err := p.ListPRFiles(context.Background(), "https://github.com/o/r", 3)
	if err != nil || len(files) != 2 {
		t.Fatalf("%v %+v", err, files)
	}
	comments, err := p.ListPRComments(context.Background(), "https://github.com/o/r", 3)
	if err != nil || len(comments) != 2 {
		t.Fatalf("%v %+v", err, comments)
	}
	if comments[0].Type != CommentTypeReview || comments[1].Type != CommentTypeIssue {
		t.Fatalf("sort order / types: %+v", comments)
	}
}

func TestGitHubProvider_PRCommentsActions_and_SubmitReview(t *testing.T) {
	t.Parallel()
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/issues/5/comments":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/5/comments":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/pulls/5/reviews":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	})
	p, c := setupMockGitHubServer(t, h, nil)
	defer c()
	ctx := context.Background()
	if err := p.AddPRComment(ctx, "https://github.com/o/r", 5, "hi"); err != nil {
		t.Fatal(err)
	}
	if err := p.AddPRReviewComment(ctx, "https://github.com/o/r", 5, PRReviewCommentOptions{
		Body: "c", Path: "f.go", Line: 2, CommitSHA: "sha",
	}); err != nil {
		t.Fatal(err)
	}
	if err := p.SubmitPRReview(ctx, "https://github.com/o/r", 5, PRReviewOptions{Event: ReviewEventApprove}); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubProvider_MergePullRequest_conflictAndSuccess(t *testing.T) {
	t.Parallel()
	t.Run("409", func(t *testing.T) {
		t.Parallel()
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge") {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"message":"merge conflict"}`))
				return
			}
			http.NotFound(w, r)
		})
		p, c := setupMockGitHubServer(t, h, nil)
		defer c()
		err := p.MergePullRequest(context.Background(), "https://github.com/o/r", 9, PRMergeOptions{SHA: "abc"})
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/merge") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"sha":"merged"}`))
				return
			}
			http.NotFound(w, r)
		})
		p, c := setupMockGitHubServer(t, h, nil)
		defer c()
		err := p.MergePullRequest(context.Background(), "https://github.com/o/r", 9, PRMergeOptions{SHA: "abc"})
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestGitHubProvider_GetFileContent_downloadFlow(t *testing.T) {
	t.Parallel()
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/contents/"):
			dl := base + "/download/blob"
			arr := []map[string]any{{
				"name": "README.md", "type": "file",
				"download_url": dl,
			}}
			_ = json.NewEncoder(w).Encode(arr)
		case r.URL.Path == "/download/blob":
			_, _ = w.Write([]byte("hello-content"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	base = srv.URL
	gh := githubClientForTestServer(t, srv)
	p := NewGitHubProviderWithDeps(Credentials{Token: "t"}, gh, nil)
	rc, err := p.GetFileContent(context.Background(), "https://github.com/o/r", "main", "README.md")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "hello-content" {
		t.Fatalf("%q", b)
	}
}

func githubClientForTestServer(t *testing.T, srv *httptest.Server) *github.Client {
	t.Helper()
	c := github.NewClient(srv.Client())
	base, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = base
	c.UploadURL = base
	return c
}
