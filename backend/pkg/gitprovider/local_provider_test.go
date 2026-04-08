package gitprovider

import (
	"context"
	"errors"
	"testing"
)

func TestLocalGitProvider_NotImplementedRemoteMethods(t *testing.T) {
	t.Parallel()
	l := NewLocalGitProvider(Credentials{})
	ctx := context.Background()
	repo := "https://github.com/o/r"
	if _, err := l.ListBranches(ctx, repo, ""); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if err := l.DeleteBranch(ctx, repo, "b"); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.GetDiff(ctx, repo, "a", "b"); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.GetFileContent(ctx, repo, "m", "p"); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.GetRepoInfo(ctx, repo); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.CreatePullRequest(ctx, repo, PRCreateOptions{}); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.UpdatePullRequest(ctx, repo, 1, PRUpdateOptions{}); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.GetPullRequest(ctx, repo, 1); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.ListPullRequests(ctx, repo, PROptions{}); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.ListPRFiles(ctx, repo, 1); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if _, err := l.ListPRComments(ctx, repo, 1); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if err := l.AddPRComment(ctx, repo, 1, "x"); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if err := l.AddPRReviewComment(ctx, repo, 1, PRReviewCommentOptions{}); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if err := l.SubmitPRReview(ctx, repo, 1, PRReviewOptions{Event: ReviewEventApprove}); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
	if err := l.MergePullRequest(ctx, repo, 1, PRMergeOptions{SHA: "x"}); !errors.Is(err, ErrNotImplemented) {
		t.Fatal(err)
	}
}

func TestLocalGitProvider_meta(t *testing.T) {
	t.Parallel()
	l := NewLocalGitProvider(Credentials{})
	if l.ProviderType() != "local" || l.SupportsPullRequests() {
		t.Fatal("meta")
	}
}
