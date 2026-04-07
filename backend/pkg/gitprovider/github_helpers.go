package gitprovider

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-github/v67/github"
)

const maxBranchPages = 10

const githubDiffAccept = "application/vnd.github.v3.diff"

// parseRepoURL извлекает owner и repo из HTTPS, SSH или короткого формата github.com/...
func parseRepoURL(repoURL string) (owner, repo string, err error) {
	u := strings.TrimSpace(repoURL)
	if u == "" {
		return "", "", fmt.Errorf("gitprovider: empty repository URL")
	}

	if strings.HasPrefix(u, "git@github.com:") {
		path := strings.TrimPrefix(u, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		path = strings.Trim(path, "/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", fmt.Errorf("gitprovider: invalid github SSH URL")
		}
		return parts[0], parts[1], nil
	}

	if !strings.Contains(u, "://") {
		if strings.HasPrefix(strings.ToLower(u), "github.com/") {
			u = "https://" + u
		} else {
			return "", "", fmt.Errorf("gitprovider: unrecognized repository URL")
		}
	}

	parsed, perr := url.Parse(u)
	if perr != nil {
		return "", "", fmt.Errorf("gitprovider: parse repository URL: %w", perr)
	}
	if parsed.Host == "" {
		return "", "", fmt.Errorf("gitprovider: repository URL has no host")
	}

	host := strings.ToLower(strings.TrimPrefix(parsed.Host, "www."))
	if host != "github.com" {
		return "", "", fmt.Errorf("gitprovider: not a github.com URL")
	}

	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	if idx := strings.Index(path, "/tree/"); idx >= 0 {
		path = path[:idx]
	}
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("gitprovider: invalid github path")
	}
	return parts[0], parts[1], nil
}

func mapPullRequest(ghPR *github.PullRequest) *PullRequest {
	if ghPR == nil {
		return nil
	}
	state := strings.ToLower(ghPR.GetState())
	if ghPR.GetMerged() {
		state = PRStateMerged
	}
	var labels []string
	for _, lb := range ghPR.Labels {
		if lb != nil && lb.GetName() != "" {
			labels = append(labels, lb.GetName())
		}
	}
	pr := &PullRequest{
		Number:      ghPR.GetNumber(),
		Title:       ghPR.GetTitle(),
		Body:        ghPR.GetBody(),
		State:       state,
		HeadBranch:  ghPR.Head.GetRef(),
		BaseBranch:  ghPR.Base.GetRef(),
		HeadSHA:     ghPR.Head.GetSHA(),
		HTMLURL:     ghPR.GetHTMLURL(),
		Draft:       ghPR.GetDraft(),
		Mergeable:   ghPR.Mergeable,
		AuthorLogin: ghPR.User.GetLogin(),
		Labels:      labels,
		ChangedFiles: ghPR.GetChangedFiles(),
		Additions:    ghPR.GetAdditions(),
		Deletions:    ghPR.GetDeletions(),
	}
	if ghPR.CreatedAt != nil {
		pr.CreatedAt = ghPR.CreatedAt.Time
	}
	if ghPR.UpdatedAt != nil {
		pr.UpdatedAt = ghPR.UpdatedAt.Time
	}
	if ghPR.MergedAt != nil {
		pr.MergedAt = ghPR.MergedAt.Time
	}
	if ghPR.ClosedAt != nil {
		pr.ClosedAt = ghPR.ClosedAt.Time
	}
	return pr
}

func mapCommitFile(f *github.CommitFile) PRFile {
	if f == nil {
		return PRFile{}
	}
	add := f.GetAdditions()
	del := f.GetDeletions()
	return PRFile{
		Filename:         f.GetFilename(),
		Status:           f.GetStatus(),
		Additions:        add,
		Deletions:        del,
		Changes:          add + del,
		PreviousFilename: f.GetPreviousFilename(),
		Patch:            f.GetPatch(),
	}
}

func mapRepository(r *github.Repository) *RepoInfo {
	if r == nil {
		return nil
	}
	topics := append([]string(nil), r.Topics...)
	return &RepoInfo{
		Name:          r.GetName(),
		FullName:      r.GetFullName(),
		Description:   r.GetDescription(),
		DefaultBranch: r.GetDefaultBranch(),
		HTMLURL:       r.GetHTMLURL(),
		CloneURL:      r.GetCloneURL(),
		SSHURL:        r.GetSSHURL(),
		Private:       r.GetPrivate(),
		Archived:      r.GetArchived(),
		Language:      r.GetLanguage(),
		Topics:        topics,
	}
}

// mapGitHubError маппит ошибки go-github на sentinel-ошибки пакета.
// notFoundErr задаёт значение для HTTP 404 (репозиторий, PR, файл и т.д.).
func mapGitHubError(err error, notFoundErr error) error {
	if err == nil {
		return nil
	}

	var rl *github.RateLimitError
	if errors.As(err, &rl) {
		return ErrRateLimited
	}
	var abuse *github.AbuseRateLimitError
	if errors.As(err, &abuse) {
		return ErrRateLimited
	}
	var tf *github.TwoFactorAuthError
	if errors.As(err, &tf) {
		return ErrAuthFailed
	}

	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) || ghErr.Response == nil {
		return err
	}

	switch ghErr.Response.StatusCode {
	case 401:
		return ErrAuthFailed
	case 403:
		if ghErr.Response.Header.Get("X-RateLimit-Remaining") == "0" {
			return ErrRateLimited
		}
		return ErrPermissionDenied
	case 404:
		return notFoundErr
	case 409:
		return ErrConflict
	case 422:
		if mapped := mapUnprocessableEntity(ghErr); mapped != nil {
			return mapped
		}
		return err
	case 429:
		return ErrRateLimited
	default:
		return err
	}
}

func mapUnprocessableEntity(ghErr *github.ErrorResponse) error {
	msg := strings.ToLower(ghErr.Message)
	if strings.Contains(msg, "pull request already exists") {
		return ErrPRAlreadyExists
	}
	if strings.Contains(msg, "reference does not exist") {
		return ErrBranchNotFound
	}
	if strings.Contains(msg, "already exists") {
		return ErrBranchAlreadyExists
	}
	return nil
}

func authenticatedGitHubCloneURL(owner, repo, token string) string {
	u := &url.URL{
		Scheme: "https",
		User:   url.UserPassword(gitHTTPAccessTokenUser, token),
		Host:   "github.com",
		Path:   fmt.Sprintf("/%s/%s.git", owner, repo),
	}
	return u.String()
}

// pushURLForGitHubRemote строит URL для `git push <url> <branch>` без изменения .git/config.
func pushURLForGitHubRemote(remoteURL, token string) (string, error) {
	ru := strings.TrimSpace(remoteURL)
	if ru == "" {
		return "", fmt.Errorf("gitprovider: empty remote URL")
	}
	if token != "" && strings.Contains(ru, gitHTTPAccessTokenUser+":") {
		return ru, nil
	}
	if token == "" {
		return "", fmt.Errorf("gitprovider: missing token for push")
	}

	if strings.HasPrefix(ru, "git@github.com:") {
		path := strings.TrimPrefix(ru, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		path = strings.Trim(path, "/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("gitprovider: invalid SSH remote")
		}
		return authenticatedGitHubCloneURL(parts[0], parts[1], token), nil
	}

	parsed, err := url.Parse(ru)
	if err != nil {
		return "", err
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Host, "www."))
	if host != "github.com" {
		return "", fmt.Errorf("gitprovider: unsupported remote host %q", parsed.Host)
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("gitprovider: invalid remote path")
	}
	return authenticatedGitHubCloneURL(parts[0], parts[1], token), nil
}

func mapFileContentError(err error) error {
	return mapGitHubError(err, ErrFileNotFound)
}
