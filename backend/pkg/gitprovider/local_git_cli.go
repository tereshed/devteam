package gitprovider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// LocalGitCLI — общие локальные операции через git CLI (используются LocalGitProvider и GitHubProvider).
type LocalGitCLI struct {
	creds Credentials
}

// CreateBranch создаёт ветку в workDir.
func (c *LocalGitCLI) CreateBranch(ctx context.Context, workDir string, opts BranchOptions) error {
	tok := c.creds.Token
	if opts.BaseBranch != "" {
		if _, err := gitExec(ctx, tok, workDir, "checkout", "--", opts.BaseBranch); err != nil {
			return err
		}
	}
	_, err := gitExec(ctx, tok, workDir, "checkout", "-b", "--", opts.BranchName)
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
	out, err := gitExec(ctx, c.creds.Token, workDir, "branch", "--list", "--format=%(refname:short)")
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
	_, err := gitExec(ctx, c.creds.Token, workDir, "branch", "-D", "--", branch)
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
	return executeCommit(ctx, c.creds.Token, workDir, opts)
}

// GetLocalDiff возвращает unified diff base..head (streaming).
func (c *LocalGitCLI) GetLocalDiff(ctx context.Context, workDir string, base, head string) (io.ReadCloser, error) {
	baseSHA, err := gitExec(ctx, c.creds.Token, workDir, "rev-parse", "--verify", base)
	if err != nil {
		return nil, fmt.Errorf("invalid base ref %q: %w", base, err)
	}
	headSHA, err := gitExec(ctx, c.creds.Token, workDir, "rev-parse", "--verify", head)
	if err != nil {
		return nil, fmt.Errorf("invalid head ref %q: %w", head, err)
	}
	rangeSpec := strings.TrimSpace(baseSHA) + ".." + strings.TrimSpace(headSHA)
	cmd := exec.CommandContext(ctx, "git", "diff", rangeSpec)
	cmd.Dir = workDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &readCloserWithWait{ReadCloser: stdout, cmd: cmd, stderr: &stderr, token: c.creds.Token}, nil
}

// GetLocalFileContent читает blob ref:path через plumbing cat-file.
func (c *LocalGitCLI) GetLocalFileContent(ctx context.Context, workDir string, ref string, path string) (io.ReadCloser, error) {
	spec := ref + ":" + path
	out, err := gitExec(ctx, c.creds.Token, workDir, "cat-file", "blob", "--", spec)
	if err != nil {
		if isGitBlobOrPathMissing(err.Error()) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	return io.NopCloser(bytes.NewReader([]byte(out))), nil
}
