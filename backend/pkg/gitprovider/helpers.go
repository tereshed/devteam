package gitprovider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"
)

// gitHTTPAccessTokenUser — имя пользователя в HTTPS URL при token injection (как в injectTokenInURL).
const gitHTTPAccessTokenUser = "x-access-token"

// readCloserWithWait оборачивает stdout git-процесса: Close() сначала закрывает pipe, затем cmd.Wait()
// (порядок важен — иначе возможен deadlock при переполнении pipe).
type readCloserWithWait struct {
	io.ReadCloser
	cmd    *exec.Cmd
	stderr *bytes.Buffer
	token  string
}

func (r *readCloserWithWait) Close() error {
	var pipeErr error
	if r.ReadCloser != nil {
		pipeErr = r.ReadCloser.Close()
	}
	waitErr := r.cmd.Wait()
	if waitErr != nil {
		if r.stderr != nil {
			stdStr := r.stderr.String()
			if isGitBlobOrPathMissing(stdStr) {
				return ErrFileNotFound
			}
			if strings.TrimSpace(stdStr) != "" {
				msg := sanitizeToken(strings.TrimSpace(stdStr), r.token)
				return fmt.Errorf("git command failed: %w, stderr: %s", waitErr, msg)
			}
		}
		if r.token != "" {
			return errors.New(sanitizeToken(waitErr.Error(), r.token))
		}
		return waitErr
	}
	return pipeErr
}

// gitExec выполняет git в workDir; при ошибке в текст включается stderr (с маскировкой токена).
func gitExec(ctx context.Context, token, workDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := fmt.Sprintf("git %s: %s", args[0], strings.TrimSpace(stderr.String()))
		if token != "" {
			msg = sanitizeToken(msg, token)
		}
		return "", fmt.Errorf("%s: %w", msg, err)
	}
	return stdout.String(), nil
}

// userinfoEncodedPassword возвращает пароль в том же виде, в каком net/url кодирует userinfo
// (совпадает с тем, что часто попадает в stderr git для HTTPS с токеном). Не использовать url.QueryEscape:
// для пробела QueryEscape даёт «+», а UserPassword — «%20».
func userinfoEncodedPassword(password string) string {
	full := url.UserPassword(gitHTTPAccessTokenUser, password).String()
	prefix := gitHTTPAccessTokenUser + ":"
	if strings.HasPrefix(full, prefix) {
		return strings.TrimPrefix(full, prefix)
	}
	return ""
}

// sanitizeToken заменяет токен на *** (сырой и форма из userinfo), чтобы не утекал в логи/ошибки.
func sanitizeToken(s, token string) string {
	if token == "" {
		return s
	}
	if enc := userinfoEncodedPassword(token); enc != "" {
		s = strings.ReplaceAll(s, enc, "***")
	}
	s = strings.ReplaceAll(s, token, "***")
	return s
}

// isGitBlobOrPathMissing — типичные сообщения git cat-file / show / diff о несуществующем blob или пути.
func isGitBlobOrPathMissing(stderr string) bool {
	low := strings.ToLower(stderr)
	return strings.Contains(low, "does not exist") || strings.Contains(low, "not a valid object") ||
		strings.Contains(low, "not in ") || strings.Contains(low, "fatal: path")
}

func validatePushBranch(branch string) error {
	if strings.TrimSpace(branch) == "" {
		return ErrPushBranchRequired
	}
	return nil
}

// executeCommit — общая логика локального commit (LocalGitProvider и GitHubProvider).
// Между проверкой индекса (git diff --cached) и commit кратковременное окно; в sandbox один процесс — приемлемо.
func executeCommit(ctx context.Context, token, workDir string, opts CommitOptions) (string, bool, error) {
	if err := opts.Author.Validate(); err != nil {
		return "", false, err
	}
	var addArgs []string
	addArgs = append(addArgs, "add")
	if len(opts.Files) == 0 {
		addArgs = append(addArgs, "-A")
	} else {
		addArgs = append(addArgs, "--")
		addArgs = append(addArgs, opts.Files...)
	}
	if _, err := gitExec(ctx, token, workDir, addArgs...); err != nil {
		return "", false, err
	}

	_, headErr := gitExec(ctx, token, workDir, "rev-parse", "--verify", "HEAD")
	if headErr != nil {
		statusOut, err := gitExec(ctx, token, workDir, "status", "--porcelain")
		if err != nil {
			return "", false, err
		}
		if strings.TrimSpace(statusOut) == "" {
			return "", false, nil
		}
	} else {
		dcmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--quiet")
		dcmd.Dir = workDir
		diffErr := dcmd.Run()
		if diffErr == nil {
			return "", false, nil
		}
		var exitErr *exec.ExitError
		if !errors.As(diffErr, &exitErr) || exitErr.ExitCode() != 1 {
			return "", false, diffErr
		}
	}

	commitArgs := []string{"commit", "-m", opts.Message}
	name := strings.TrimSpace(opts.Author.Name)
	email := strings.TrimSpace(opts.Author.Email)
	if name != "" && email != "" {
		commitArgs = append(commitArgs, "--author", fmt.Sprintf("%s <%s>", name, email))
	}
	if _, err := gitExec(ctx, token, workDir, commitArgs...); err != nil {
		return "", false, err
	}
	shaOut, err := gitExec(ctx, token, workDir, "rev-parse", "HEAD")
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(shaOut), true, nil
}

// injectTokenInURL подставляет x-access-token в HTTP/HTTPS URL через net/url (без strings.Replace).
func injectTokenInURL(repoURL, token string) string {
	if token == "" || !isHTTPURL(repoURL) {
		return repoURL
	}
	u, err := url.Parse(repoURL)
	if err != nil {
		return repoURL
	}
	u.User = url.UserPassword(gitHTTPAccessTokenUser, token)
	return u.String()
}

// isHTTPURL возвращает true для схем http/https.
func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	default:
		return false
	}
}
