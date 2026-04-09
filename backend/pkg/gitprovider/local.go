package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	return &LocalGitProvider{LocalGitCLI: LocalGitCLI{creds: creds, runner: nil}}
}

var _ GitProvider = (*LocalGitProvider)(nil)

func (l *LocalGitProvider) ProviderType() string       { return "local" }
func (l *LocalGitProvider) SupportsPullRequests() bool { return false }

func (l *LocalGitProvider) ValidateAccess(ctx context.Context, repoURL string) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(repoURL) == "" {
		return ErrRepoNotFound
	}
	checkURL := strings.TrimSpace(repoURL)
	if isHTTPURL(checkURL) && l.creds.Token != "" {
		checkURL = injectTokenInURL(checkURL, l.creds.Token)
	}
	_, stderr, err := l.effectiveRunner().RunGit(ctx, "", "ls-remote", "--", checkURL)
	if err == nil {
		return nil
	}
	tok := l.creds.Token
	details := sanitizeToken(strings.TrimSpace(stderr), tok)
	sent := mapGitCLIError(stderr)
	return fmt.Errorf("git validate access: %w, details: %s", sent, details)
}

func (l *LocalGitProvider) Clone(ctx context.Context, repoURL string, opts CloneOptions) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	if err := validateGitBranchForClone(opts.Branch); err != nil {
		return err
	}
	if err := validateCloneDestPath(opts.DestPath); err != nil {
		return err
	}
	cloneURL := repoURL
	if l.creds.Token != "" && isHTTPURL(cloneURL) {
		cloneURL = injectTokenInURL(cloneURL, l.creds.Token)
	}
	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch="+opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	args = append(args, "--", cloneURL, opts.DestPath)

	_, stderr, err := l.effectiveRunner().RunGit(ctx, "", args...)
	if err != nil {
		msg := sanitizeToken(strings.TrimSpace(stderr), l.creds.Token)
		return fmt.Errorf("git clone: %w, details: %s", ErrCloneFailed, msg)
	}
	return nil
}

func (l *LocalGitProvider) Push(ctx context.Context, workDir string, opts PushOptions) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(workDir) == "" {
		return fmt.Errorf("gitprovider: empty work directory")
	}
	if err := validatePushBranch(opts.Branch); err != nil {
		return err
	}
	remote := opts.Remote
	if remote == "" {
		remote = "origin"
	}
	if err := validateGitRefName(remote); err != nil {
		return err
	}
	r := l.effectiveRunner()
	remoteURL, err := runGit(ctx, r, l.creds.Token, workDir, "remote", "get-url", "--", remote)
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

	_, err = runGit(ctx, r, l.creds.Token, workDir, args...)
	if err != nil {
		details := strings.TrimSpace(err.Error())
		var rge *runGitError
		if errors.As(err, &rge) && strings.TrimSpace(rge.stderr) != "" {
			details = sanitizeToken(strings.TrimSpace(rge.stderr), l.creds.Token)
		}
		le := strings.ToLower(details)
		switch {
		case strings.Contains(le, "rejected"):
			return fmt.Errorf("git push: %w, details: %s", ErrConflict, details)
		case strings.Contains(le, "permission denied"), strings.Contains(le, "403"):
			return fmt.Errorf("git push: %w, details: %s", ErrPermissionDenied, details)
		}
		return err
	}
	return nil
}

func (l *LocalGitProvider) CommitAndPush(ctx context.Context, workDir string, commitOpts CommitOptions, pushOpts PushOptions) (string, bool, error) {
	if err := requireContext(ctx); err != nil {
		return "", false, err
	}
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
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) DeleteBranch(ctx context.Context, repoURL string, branch string) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	return ErrNotImplemented
}

func (l *LocalGitProvider) GetDiff(ctx context.Context, repoURL string, base, head string) (io.ReadCloser, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) GetFileContent(ctx context.Context, repoURL string, branch string, path string) (io.ReadCloser, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) GetRepoInfo(ctx context.Context, repoURL string) (*RepoInfo, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) CreatePullRequest(ctx context.Context, repoURL string, opts PRCreateOptions) (*PullRequest, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) UpdatePullRequest(ctx context.Context, repoURL string, number int, opts PRUpdateOptions) (*PullRequest, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) GetPullRequest(ctx context.Context, repoURL string, number int) (*PullRequest, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) ListPullRequests(ctx context.Context, repoURL string, opts PROptions) ([]PullRequest, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) ListPRFiles(ctx context.Context, repoURL string, number int) ([]PRFile, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) ListPRComments(ctx context.Context, repoURL string, number int) ([]PRComment, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	return nil, ErrNotImplemented
}

func (l *LocalGitProvider) AddPRComment(ctx context.Context, repoURL string, number int, body string) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	return ErrNotImplemented
}

func (l *LocalGitProvider) AddPRReviewComment(ctx context.Context, repoURL string, number int, opts PRReviewCommentOptions) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	return ErrNotImplemented
}

func (l *LocalGitProvider) SubmitPRReview(ctx context.Context, repoURL string, number int, opts PRReviewOptions) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	return ErrNotImplemented
}

func (l *LocalGitProvider) MergePullRequest(ctx context.Context, repoURL string, number int, opts PRMergeOptions) error {
	if err := requireContext(ctx); err != nil {
		return err
	}
	return ErrNotImplemented
}
