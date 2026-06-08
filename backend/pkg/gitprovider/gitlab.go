package gitprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitLabProvider — GitProvider для GitLab (gitlab.com и self-hosted).
//
// Локальные git-операции (Clone с токеном, Commit, Push, ls-remote ValidateAccess/
// GetLatestCommitSHA) наследуются от LocalGitProvider; для OAuth2-токенов username
// в URL — "oauth2" (GitLab требует именно его). Через GitLab REST API v4 реализованы
// GetRepoInfo (default branch при создании проекта) и CreatePullRequest (merge request,
// done-гейт оркестратора). Остальные API-методы наследуют ErrNotImplemented.
type GitLabProvider struct {
	LocalGitProvider
	httpClient *http.Client
}

// NewGitLabProvider создаёт провайдер GitLab. tokenUser="oauth2" — GitLab принимает
// OAuth2-токен только с этим username в HTTPS-URL.
func NewGitLabProvider(creds Credentials) *GitLabProvider {
	return &GitLabProvider{
		LocalGitProvider: LocalGitProvider{
			LocalGitCLI: LocalGitCLI{creds: creds, tokenUser: "oauth2"},
		},
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

var _ GitProvider = (*GitLabProvider)(nil)

func (g *GitLabProvider) ProviderType() string       { return "gitlab" }
func (g *GitLabProvider) SupportsPullRequests() bool { return true }

// gitlabBaseAndPath парсит repoURL → base ("https://host") и project path ("group/repo").
func gitlabBaseAndPath(repoURL string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", "", fmt.Errorf("%w: invalid gitlab url", ErrRepoNotFound)
	}
	base := u.Scheme + "://" + u.Host
	path := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	if path == "" {
		return "", "", fmt.Errorf("%w: empty gitlab project path", ErrRepoNotFound)
	}
	return base, path, nil
}

func (g *GitLabProvider) apiRequest(ctx context.Context, method, endpoint string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if g.creds.Token != "" {
		req.Header.Set("Authorization", "Bearer "+g.creds.Token)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return g.httpClient.Do(req)
}

// GetRepoInfo — метаданные проекта (default_branch и т.д.) через GitLab API.
func (g *GitLabProvider) GetRepoInfo(ctx context.Context, repoURL string) (*RepoInfo, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	base, path, err := gitlabBaseAndPath(repoURL)
	if err != nil {
		return nil, err
	}
	endpoint := base + "/api/v4/projects/" + url.PathEscape(path)
	resp, err := g.apiRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab get project: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrAuthFailed
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrRepoNotFound
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("gitlab get project HTTP %d", resp.StatusCode)
	}
	var p struct {
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		Description       string `json:"description"`
		DefaultBranch     string `json:"default_branch"`
		WebURL            string `json:"web_url"`
		HTTPURLToRepo     string `json:"http_url_to_repo"`
		SSHURLToRepo      string `json:"ssh_url_to_repo"`
		Visibility        string `json:"visibility"`
		Archived          bool   `json:"archived"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, fmt.Errorf("gitlab decode project: %w", err)
	}
	return &RepoInfo{
		Name:          p.Name,
		FullName:      p.PathWithNamespace,
		Description:   p.Description,
		DefaultBranch: p.DefaultBranch,
		HTMLURL:       p.WebURL,
		CloneURL:      p.HTTPURLToRepo,
		SSHURL:        p.SSHURLToRepo,
		Private:       p.Visibility != "public",
		Archived:      p.Archived,
	}, nil
}

// CreatePullRequest создаёт Merge Request через GitLab API.
func (g *GitLabProvider) CreatePullRequest(ctx context.Context, repoURL string, opts PRCreateOptions) (*PullRequest, error) {
	if err := requireContext(ctx); err != nil {
		return nil, err
	}
	base, path, err := gitlabBaseAndPath(repoURL)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"source_branch": opts.HeadBranch,
		"target_branch": opts.BaseBranch,
		"title":         opts.Title,
		"description":   opts.Body,
	}
	if len(opts.Labels) > 0 {
		payload["labels"] = strings.Join(opts.Labels, ",")
	}
	buf, _ := json.Marshal(payload)
	endpoint := base + "/api/v4/projects/" + url.PathEscape(path) + "/merge_requests"
	resp, err := g.apiRequest(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("gitlab create MR: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, ErrAuthFailed
	case resp.StatusCode == http.StatusConflict:
		return nil, fmt.Errorf("%w: gitlab MR already exists", ErrConflict)
	case resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("gitlab create MR HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	var mr struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		State        string `json:"state"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		WebURL       string `json:"web_url"`
		SHA          string `json:"sha"`
	}
	if err := json.Unmarshal(respBody, &mr); err != nil {
		return nil, fmt.Errorf("gitlab decode MR: %w", err)
	}
	return &PullRequest{
		Number:     mr.IID,
		Title:      mr.Title,
		Body:       mr.Description,
		State:      mr.State,
		HeadBranch: mr.SourceBranch,
		BaseBranch: mr.TargetBranch,
		HeadSHA:    mr.SHA,
		HTMLURL:    mr.WebURL,
	}, nil
}
