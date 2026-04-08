package gitprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v67/github"
	"golang.org/x/oauth2"
)

// GitHubProvider реализует GitProvider для GitHub REST API v3.
// Remote-операции — go-github; локальные — git CLI.
type GitHubProvider struct {
	LocalGitCLI
	client *github.Client
}

// NewGitHubProvider создаёт провайдер (вызывается из фабрики 4.5).
// oauth2.NewClient использует context.Background() только для транспорта (TLS);
// каждый вызов API прокидывает ctx в go-github.
func NewGitHubProvider(creds Credentials) *GitHubProvider {
	return NewGitHubProviderWithDeps(creds, nil, nil)
}

// NewGitHubProviderWithDeps — для тестов: runner и/или github.Client (nil = значения по умолчанию).
func NewGitHubProviderWithDeps(creds Credentials, ghClient *github.Client, runner GitCommandRunner) *GitHubProvider {
	var client *github.Client
	if ghClient != nil {
		client = ghClient
	} else {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: creds.Token})
		httpClient := oauth2.NewClient(context.Background(), ts)
		client = github.NewClient(httpClient)
	}
	return &GitHubProvider{
		LocalGitCLI: LocalGitCLI{creds: creds, runner: runner},
		client:      client,
	}
}

var _ GitProvider = (*GitHubProvider)(nil)

func (g *GitHubProvider) ProviderType() string         { return "github" }
func (g *GitHubProvider) SupportsPullRequests() bool   { return true }

func (g *GitHubProvider) ValidateAccess(ctx context.Context, repoURL string) error {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	_, _, err = g.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return mapGitHubError(err, ErrRepoNotFound)
	}
	return nil
}

func (g *GitHubProvider) ListBranches(ctx context.Context, repoURL string, prefix string) ([]string, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	var names []string
	opt := &github.BranchListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for page := 1; page <= maxBranchPages; page++ {
		opt.Page = page
		branches, resp, err := g.client.Repositories.ListBranches(ctx, owner, repo, opt)
		if err != nil {
			return nil, mapGitHubError(err, ErrRepoNotFound)
		}
		for _, b := range branches {
			n := b.GetName()
			if prefix == "" || strings.HasPrefix(n, prefix) {
				names = append(names, n)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
	}
	return names, nil
}

func (g *GitHubProvider) DeleteBranch(ctx context.Context, repoURL string, branch string) error {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	_, err = g.client.Git.DeleteRef(ctx, owner, repo, "heads/"+branch)
	if err != nil {
		return mapGitHubError(err, ErrBranchNotFound)
	}
	return nil
}

func (g *GitHubProvider) GetDiff(ctx context.Context, repoURL string, base, head string) (io.ReadCloser, error) {
	if err := validateGitRefName(base); err != nil {
		return nil, err
	}
	if err := validateGitRefName(head); err != nil {
		return nil, err
	}
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	eb := url.QueryEscape(base)
	eh := url.QueryEscape(head)
	u := fmt.Sprintf("repos/%v/%v/compare/%v...%v", owner, repo, eb, eh)
	req, err := g.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", githubDiffAccept)
	resp, err := g.client.BareDo(ctx, req)
	if err != nil {
		return nil, mapGitHubError(err, ErrRepoNotFound)
	}
	return resp.Body, nil
}

func (g *GitHubProvider) CreatePullRequest(ctx context.Context, repoURL string, opts PRCreateOptions) (*PullRequest, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	newPR := &github.NewPullRequest{
		Title: github.String(opts.Title),
		Body:  github.String(opts.Body),
		Head:  github.String(opts.HeadBranch),
		Base:  github.String(opts.BaseBranch),
		Draft: github.Bool(opts.Draft),
	}
	pr, _, err := g.client.PullRequests.Create(ctx, owner, repo, newPR)
	if err != nil {
		return nil, mapGitHubError(err, ErrRepoNotFound)
	}
	if len(opts.Labels) > 0 {
		_, _, lerr := g.client.Issues.AddLabelsToIssue(ctx, owner, repo, pr.GetNumber(), opts.Labels)
		if lerr != nil {
			return nil, mapGitHubError(lerr, ErrRepoNotFound)
		}
	}
	return mapPullRequest(pr), nil
}

func (g *GitHubProvider) UpdatePullRequest(ctx context.Context, repoURL string, number int, opts PRUpdateOptions) (*PullRequest, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	edit := &github.PullRequest{}
	if opts.Title != nil {
		edit.Title = opts.Title
	}
	if opts.Body != nil {
		edit.Body = opts.Body
	}
	if opts.State != nil {
		edit.State = opts.State
	}
	pr, _, err := g.client.PullRequests.Edit(ctx, owner, repo, number, edit)
	if err != nil {
		return nil, mapGitHubError(err, ErrPRNotFound)
	}
	return mapPullRequest(pr), nil
}

func (g *GitHubProvider) GetPullRequest(ctx context.Context, repoURL string, number int) (*PullRequest, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, mapGitHubError(err, ErrPRNotFound)
	}
	return mapPullRequest(pr), nil
}

func (g *GitHubProvider) ListPullRequests(ctx context.Context, repoURL string, opts PROptions) ([]PullRequest, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	state := opts.State
	if state == "" {
		state = PRStateOpen
	}
	perPage := opts.Limit
	if perPage <= 0 {
		perPage = 30
	}
	head := opts.HeadBranch
	if head != "" && !strings.Contains(head, ":") {
		head = owner + ":" + head
	}
	listOpts := &github.PullRequestListOptions{
		State: state,
		Head:  head,
		Base:  opts.BaseBranch,
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}
	prs, _, err := g.client.PullRequests.List(ctx, owner, repo, listOpts)
	if err != nil {
		return nil, mapGitHubError(err, ErrRepoNotFound)
	}
	out := make([]PullRequest, 0, len(prs))
	for _, p := range prs {
		if m := mapPullRequest(p); m != nil {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (g *GitHubProvider) ListPRFiles(ctx context.Context, repoURL string, number int) ([]PRFile, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	var all []*github.CommitFile
	opt := &github.ListOptions{PerPage: 100}
	for {
		files, resp, err := g.client.PullRequests.ListFiles(ctx, owner, repo, number, opt)
		if err != nil {
			return nil, mapGitHubError(err, ErrPRNotFound)
		}
		all = append(all, files...)
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	out := make([]PRFile, 0, len(all))
	for _, f := range all {
		out = append(out, mapCommitFile(f))
	}
	return out, nil
}

func (g *GitHubProvider) ListPRComments(ctx context.Context, repoURL string, number int) ([]PRComment, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	var out []PRComment

	issueOpts := &github.IssueListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		comments, resp, err := g.client.Issues.ListComments(ctx, owner, repo, number, issueOpts)
		if err != nil {
			return nil, mapGitHubError(err, ErrPRNotFound)
		}
		for _, c := range comments {
			out = append(out, PRComment{
				ID:          c.GetID(),
				Body:        c.GetBody(),
				AuthorLogin: c.User.GetLogin(),
				CreatedAt:   c.GetCreatedAt().Time,
				UpdatedAt:   c.GetUpdatedAt().Time,
				Type:        CommentTypeIssue,
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		issueOpts.Page = resp.NextPage
	}

	reviewOpts := &github.PullRequestListCommentsOptions{ListOptions: github.ListOptions{PerPage: 100}}
	for {
		comments, resp, err := g.client.PullRequests.ListComments(ctx, owner, repo, number, reviewOpts)
		if err != nil {
			return nil, mapGitHubError(err, ErrPRNotFound)
		}
		for _, c := range comments {
			out = append(out, PRComment{
				ID:          c.GetID(),
				Body:        c.GetBody(),
				AuthorLogin: c.User.GetLogin(),
				CreatedAt:   c.GetCreatedAt().Time,
				UpdatedAt:   c.GetUpdatedAt().Time,
				Type:        CommentTypeReview,
				Path:        c.GetPath(),
				Line:        c.GetLine(),
				CommitSHA:   c.GetCommitID(),
				Side:        c.GetSide(),
			})
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		reviewOpts.Page = resp.NextPage
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (g *GitHubProvider) AddPRComment(ctx context.Context, repoURL string, number int, body string) error {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	_, _, err = g.client.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{Body: github.String(body)})
	if err != nil {
		return mapGitHubError(err, ErrPRNotFound)
	}
	return nil
}

func (g *GitHubProvider) AddPRReviewComment(ctx context.Context, repoURL string, number int, opts PRReviewCommentOptions) error {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	side := opts.Side
	if side == "" {
		side = PRDiffSideRight
	}
	comment := &github.PullRequestComment{
		Body:     github.String(opts.Body),
		Path:     github.String(opts.Path),
		Line:     github.Int(opts.Line),
		CommitID: github.String(opts.CommitSHA),
		Side:     github.String(side),
	}
	_, _, err = g.client.PullRequests.CreateComment(ctx, owner, repo, number, comment)
	if err != nil {
		return mapGitHubError(err, ErrPRNotFound)
	}
	return nil
}

func (g *GitHubProvider) SubmitPRReview(ctx context.Context, repoURL string, number int, opts PRReviewOptions) error {
	switch opts.Event {
	case ReviewEventApprove, ReviewEventRequestChanges, ReviewEventComment:
	default:
		return fmt.Errorf("gitprovider: invalid review event %q", opts.Event)
	}
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	body := opts.Body
	review := &github.PullRequestReviewRequest{
		Event: github.String(opts.Event),
		Body:  github.String(body),
	}
	_, _, err = g.client.PullRequests.CreateReview(ctx, owner, repo, number, review)
	if err != nil {
		return mapGitHubError(err, ErrPRNotFound)
	}
	return nil
}

func (g *GitHubProvider) MergePullRequest(ctx context.Context, repoURL string, number int, opts PRMergeOptions) error {
	if strings.TrimSpace(opts.SHA) == "" {
		return fmt.Errorf("gitprovider: MergePullRequest requires non-empty SHA")
	}
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	method := opts.MergeMethod
	if method == "" {
		method = MergeMethodMerge
	}
	prOpts := &github.PullRequestOptions{
		CommitTitle: opts.CommitTitle,
		MergeMethod: method,
		SHA:         opts.SHA,
	}
	_, _, err = g.client.PullRequests.Merge(ctx, owner, repo, number, opts.CommitMessage, prOpts)
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response != nil {
			switch ghErr.Response.StatusCode {
			case 405, 409:
				return ErrConflict
			}
		}
		return mapGitHubError(err, ErrPRNotFound)
	}
	return nil
}

func (g *GitHubProvider) GetRepoInfo(ctx context.Context, repoURL string) (*RepoInfo, error) {
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	r, _, err := g.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, mapGitHubError(err, ErrRepoNotFound)
	}
	return mapRepository(r), nil
}

func (g *GitHubProvider) GetFileContent(ctx context.Context, repoURL string, branch string, path string) (io.ReadCloser, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, fmt.Errorf("gitprovider: empty branch for GetFileContent")
	}
	if err := validateGitPathForBlob(path); err != nil {
		return nil, err
	}
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	rc, _, err := g.client.Repositories.DownloadContents(ctx, owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return nil, mapFileContentError(err)
	}
	return rc, nil
}

func (g *GitHubProvider) Clone(ctx context.Context, repoURL string, opts CloneOptions) error {
	if g.creds.Token == "" {
		return fmt.Errorf("%w: missing token", ErrAuthFailed)
	}
	if err := validateGitBranchForClone(opts.Branch); err != nil {
		return err
	}
	owner, repo, err := parseRepoURL(repoURL)
	if err != nil {
		return err
	}
	authURL := authenticatedGitHubCloneURL(owner, repo, g.creds.Token)
	args := []string{"clone"}
	if opts.Branch != "" {
		args = append(args, "--branch="+opts.Branch)
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	args = append(args, "--", authURL, opts.DestPath)

	_, stderr, err := g.effectiveRunner().RunGit(ctx, "", args...)
	if err != nil {
		msg := sanitizeToken(strings.TrimSpace(stderr), g.creds.Token)
		return fmt.Errorf("%w: %s", ErrCloneFailed, msg)
	}
	return nil
}

func (g *GitHubProvider) Push(ctx context.Context, workDir string, opts PushOptions) error {
	if g.creds.Token == "" {
		return fmt.Errorf("%w: missing token", ErrAuthFailed)
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
	r := g.effectiveRunner()
	remoteURL, err := runGit(ctx, r, g.creds.Token, workDir, "remote", "get-url", "--", remote)
	if err != nil {
		return err
	}
	ru := strings.TrimSpace(remoteURL)
	var pushTarget string
	if isHTTPURL(ru) {
		pushTarget = injectTokenInURL(ru, g.creds.Token)
	} else {
		pushTarget, err = pushURLForGitHubRemote(ru, g.creds.Token)
		if err != nil {
			return err
		}
	}
	args := []string{"push"}
	if opts.Force {
		args = append(args, "--force")
	}
	args = append(args, pushTarget, "--", opts.Branch)

	_, err = runGit(ctx, r, g.creds.Token, workDir, args...)
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

func (g *GitHubProvider) CommitAndPush(ctx context.Context, workDir string, commitOpts CommitOptions, pushOpts PushOptions) (string, bool, error) {
	sha, hasChanges, err := g.Commit(ctx, workDir, commitOpts)
	if err != nil {
		return "", false, err
	}
	if err := g.Push(ctx, workDir, pushOpts); err != nil {
		return sha, hasChanges, err
	}
	return sha, hasChanges, nil
}
