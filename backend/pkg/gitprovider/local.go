package gitprovider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// LocalGitProvider реализует GitProvider для локальных git-репозиториев.
// Операции — через системный git CLI; remote API — ErrNotImplemented.
type LocalGitProvider struct {
	LocalGitCLI
}

// NewLocalGitProvider создаёт экземпляр LocalGitProvider (фабрика 4.5).
func NewLocalGitProvider(creds Credentials) *LocalGitProvider {
	return &LocalGitProvider{LocalGitCLI: LocalGitCLI{creds: creds}}
}

var _ GitProvider = (*LocalGitProvider)(nil)

func (l *LocalGitProvider) ProviderType() string       { return "local" }
func (l *LocalGitProvider) SupportsPullRequests() bool { return false }

func (l *LocalGitProvider) ValidateAccess(ctx context.Context, repoURL string) error {
	if strings.TrimSpace(repoURL) == "" {
		return ErrRepoNotFound
	}
	checkURL := strings.TrimSpace(repoURL)
	if isHTTPURL(checkURL) && l.creds.Token != "" {
		checkURL = injectTokenInURL(checkURL, l.creds.Token)
	}
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--", checkURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return nil
	}
	msg := strings.ToLower(stderr.String())
	if strings.Contains(msg, "authentication failed") || strings.Contains(msg, "could not read username") ||
		strings.Contains(msg, "access denied") || strings.Contains(msg, "invalid username or password") {
		return ErrAuthFailed
	}
	return ErrRepoNotFound
}

func (l *LocalGitProvider) Clone(ctx context.Context, repoURL string, opts CloneOptions) error {
	cloneURL := repoURL
	if l.creds.Token != "" && isHTTPURL(cloneURL) {
		cloneURL = injectTokenInURL(cloneURL, l.creds.Token)
	}
	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	args = append(args, "--", cloneURL, opts.DestPath)

	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := sanitizeToken(strings.TrimSpace(stderr.String()), l.creds.Token)
		return fmt.Errorf("%w: %s", ErrCloneFailed, msg)
	}
	return nil
}

func (l *LocalGitProvider) Push(ctx context.Context, workDir string, opts PushOptions) error {
	if err := validatePushBranch(opts.Branch); err != nil {
		return err
	}
	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}
	remoteURL, err := gitExec(ctx, l.creds.Token, workDir, "remote", "get-url", "--", remote)
	if err != nil {
		return err
	}
	ru := strings.TrimSpace(remoteURL)
	pushTarget := remote
	if l.creds.Token != "" && isHTTPURL(ru) {
		pushTarget = injectTokenInURL(ru, l.creds.Token)
	}
	args := []string{"push"}
	if opts.Force {
		args = append(args, "--force")
	}
	args = append(args, pushTarget, "--", opts.Branch)

	_, err = gitExec(ctx, l.creds.Token, workDir, args...)
	if err != nil {
		le := strings.ToLower(err.Error())
		switch {
		case strings.Contains(le, "rejected"):
			return ErrConflict
		case strings.Contains(le, "permission denied"), strings.Contains(le, "403"):
			return ErrPermissionDenied
		}
		return err
	}
	return nil
}

func (l *LocalGitProvider) CommitAndPush(ctx context.Context, workDir string, commitOpts CommitOptions, pushOpts PushOptions) (string, bool, error) {
	sha, hasChanges, err := l.Commit(ctx, workDir, commitOpts)
	if err != nil {
		return "", false, err
	}
	if err := l.Push(ctx, workDir, pushOpts); err != nil {
		return sha, hasChanges, err
	}
	return sha, hasChanges, nil
}

func (l *LocalGitProvider) ListBranches(ctx context.Context, repoURL string, prefix string) ([]string, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) DeleteBranch(ctx context.Context, repoURL string, branch string) error {
	return ErrNotImplemented
}

func (l *LocalGitProvider) GetDiff(ctx context.Context, repoURL string, base, head string) (io.ReadCloser, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) GetFileContent(ctx context.Context, repoURL string, branch string, path string) (io.ReadCloser, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) GetRepoInfo(ctx context.Context, repoURL string) (*RepoInfo, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) CreatePullRequest(ctx context.Context, repoURL string, opts PRCreateOptions) (*PullRequest, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) UpdatePullRequest(ctx context.Context, repoURL string, number int, opts PRUpdateOptions) (*PullRequest, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) GetPullRequest(ctx context.Context, repoURL string, number int) (*PullRequest, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) ListPullRequests(ctx context.Context, repoURL string, opts PROptions) ([]PullRequest, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) ListPRFiles(ctx context.Context, repoURL string, number int) ([]PRFile, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) ListPRComments(ctx context.Context, repoURL string, number int) ([]PRComment, error) {
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) AddPRComment(ctx context.Context, repoURL string, number int, body string) error {
	return ErrNotImplemented
}

func (l *LocalGitProvider) AddPRReviewComment(ctx context.Context, repoURL string, number int, opts PRReviewCommentOptions) error {
	return ErrNotImplemented
}

func (l *LocalGitProvider) SubmitPRReview(ctx context.Context, repoURL string, number int, opts PRReviewOptions) error {
	return ErrNotImplemented
}

func (l *LocalGitProvider) MergePullRequest(ctx context.Context, repoURL string, number int, opts PRMergeOptions) error {
	return ErrNotImplemented
}
