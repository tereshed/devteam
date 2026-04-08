package gitprovider

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// LocalGitCLI — общие локальные операции через git CLI (используются LocalGitProvider и GitHubProvider).
type LocalGitCLI struct {
	creds  Credentials
	runner GitCommandRunner // если nil — NewExecGitRunner() при первом использовании через effectiveRunner
}

func (c *LocalGitCLI) effectiveRunner() GitCommandRunner {
	if c.runner != nil {
		return c.runner
	}
	return NewExecGitRunner()
}

// CreateBranch создаёт ветку в workDir.
func (c *LocalGitCLI) CreateBranch(ctx context.Context, workDir string, opts BranchOptions) error {
	if strings.TrimSpace(opts.BranchName) == "" {
		return fmt.Errorf("gitprovider: branch name is required")
	}
	if err := validateNonFlagGitString(opts.BranchName); err != nil {
		return err
	}
	if opts.BaseBranch != "" {
		if err := validateGitRefName(opts.BaseBranch); err != nil {
			return err
		}
	}
	tok := c.creds.Token
	r := c.effectiveRunner()
	if opts.BaseBranch != "" {
		if _, err := runGit(ctx, r, tok, workDir, "checkout", "--", opts.BaseBranch); err != nil {
			return err
		}
	}
	// Имя ветки не идёт после --: git checkout -b не принимает «-b -- name» (ошибка fatal: '--' is not a valid branch name).
	// Инъекция флагов закрыта validateNonFlagGitString.
	_, err := runGit(ctx, r, tok, workDir, "checkout", "-b", opts.BranchName)
	if err != nil {
		le := strings.ToLower(err.Error())
		if strings.Contains(le, "already exists") {
			return ErrBranchAlreadyExists
		}
		return err
	}
	return nil
}

// ListLocalBranches возвращает локальные ветки с опциональным prefix.
func (c *LocalGitCLI) ListLocalBranches(ctx context.Context, workDir string, prefix string) ([]string, error) {
	out, err := runGit(ctx, c.effectiveRunner(), c.creds.Token, workDir, "branch", "--list", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if prefix == "" || strings.HasPrefix(line, prefix) {
			names = append(names, line)
		}
	}
	return names, nil
}

// DeleteLocalBranch удаляет локальную ветку.
func (c *LocalGitCLI) DeleteLocalBranch(ctx context.Context, workDir string, branch string) error {
	if err := validateNonFlagGitString(branch); err != nil {
		return err
	}
	_, err := runGit(ctx, c.effectiveRunner(), c.creds.Token, workDir, "branch", "-D", "--", branch)
	if err != nil {
		le := strings.ToLower(err.Error())
		if strings.Contains(le, "checked out") {
			return ErrConflict
		}
		if strings.Contains(le, "not found") {
			return ErrBranchNotFound
		}
		return err
	}
	return nil
}

// Commit создаёт локальный коммит без push.
func (c *LocalGitCLI) Commit(ctx context.Context, workDir string, opts CommitOptions) (string, bool, error) {
	return executeCommit(ctx, c.effectiveRunner(), c.creds.Token, workDir, opts)
}

// GetLocalDiff возвращает unified diff base..head (streaming).
func (c *LocalGitCLI) GetLocalDiff(ctx context.Context, workDir string, base, head string) (io.ReadCloser, error) {
	if err := validateGitRefName(base); err != nil {
		return nil, err
	}
	if err := validateGitRefName(head); err != nil {
		return nil, err
	}
	tok := c.creds.Token
	r := c.effectiveRunner()
	baseSHA, err := runGit(ctx, r, tok, workDir, "rev-parse", "--verify", "--", base)
	if err != nil {
		return nil, fmt.Errorf("invalid base ref %q: %w", base, err)
	}
	headSHA, err := runGit(ctx, r, tok, workDir, "rev-parse", "--verify", "--", head)
	if err != nil {
		return nil, fmt.Errorf("invalid head ref %q: %w", head, err)
	}
	rangeSpec := strings.TrimSpace(baseSHA) + ".." + strings.TrimSpace(headSHA)
	return r.GitStdoutPipe(ctx, workDir, tok, "diff", rangeSpec)
}

// GetLocalFileContent читает blob ref:path через plumbing cat-file (stdout стримится, без буферизации всего объекта в памяти).
func (c *LocalGitCLI) GetLocalFileContent(ctx context.Context, workDir string, ref string, path string) (io.ReadCloser, error) {
	if err := validateGitRefName(ref); err != nil {
		return nil, err
	}
	if err := validateGitPathForBlob(path); err != nil {
		return nil, err
	}
	spec := ref + ":" + path
	tok := c.creds.Token
	return c.effectiveRunner().GitStdoutPipe(ctx, workDir, tok, "cat-file", "blob", "--", spec)
}
