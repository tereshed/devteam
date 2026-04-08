package gitprovider

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

// GitCommandRunner запускает подкоманду git без глобального состояния (DI для тестов).
// Args — только аргументы после слова «git» (первый элемент — подкоманда: clone, push, …).
type GitCommandRunner interface {
	RunGit(ctx context.Context, workDir string, args ...string) (stdout, stderr string, err error)
	// GitStdoutPipe запускает git и отдаёт stdout как ReadCloser; Close() обязан дождаться процесса.
	GitStdoutPipe(ctx context.Context, workDir, token string, args ...string) (io.ReadCloser, error)
}

type execGitRunner struct{}

// NewExecGitRunner — реализация через os/exec (прод и integration-тесты).
func NewExecGitRunner() GitCommandRunner {
	return &execGitRunner{}
}

func (e *execGitRunner) RunGit(ctx context.Context, workDir string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func (e *execGitRunner) GitStdoutPipe(ctx context.Context, workDir, token string, args ...string) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
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
	return &readCloserWithWait{ReadCloser: stdout, cmd: cmd, stderr: &stderr, token: token}, nil
}
