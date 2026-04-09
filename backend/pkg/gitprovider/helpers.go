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
	"syscall"
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

func gitSubcommandOrUnknown(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return "?"
}

// runGitError — при непустом токене Error() маскирует весь текст (stderr + cause), чтобы токен не утекал через err.Error() в цепочке %w.
type runGitError struct {
	prefix string
	cause  error
	token  string
	stderr string // сырой stderr для классификации (Push и др.), не для прямого вывода в лог
}

func (e *runGitError) Error() string {
	s := fmt.Sprintf("%s: %v", e.prefix, e.cause)
	return sanitizeToken(s, e.token)
}

func (e *runGitError) Unwrap() error { return e.cause }

// isGitStdoutClosedSigPIPE — дочерний процесс завершился из‑за SIGPIPE после того, как читатель
// закрыл stdout (штатное раннее закрытие стрима). На Unix часто exit 141 или WaitStatus(Signal=SIGPIPE);
// на macOS встречается текст «signal: broken pipe» без кода 141.
func isGitStdoutClosedSigPIPE(waitErr error) bool {
	if waitErr == nil {
		return false
	}
	var ee *exec.ExitError
	if errors.As(waitErr, &ee) {
		if ee.ExitCode() == 141 {
			return true
		}
		if ws, ok := ee.Sys().(syscall.WaitStatus); ok {
			if ws.Signaled() && ws.Signal() == syscall.SIGPIPE {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(waitErr.Error()), "broken pipe")
}

func (r *readCloserWithWait) Close() error {
	var pipeErr error
	if r.ReadCloser != nil {
		pipeErr = r.ReadCloser.Close()
	}
	waitErr := r.cmd.Wait()
	if waitErr != nil && isGitStdoutClosedSigPIPE(waitErr) {
		return pipeErr
	}
	if waitErr != nil {
		if r.stderr != nil {
			stdStr := r.stderr.String()
			if isGitBlobOrPathMissing(stdStr) {
				return ErrFileNotFound
			}
			if strings.TrimSpace(stdStr) != "" {
				msg := sanitizeToken(strings.TrimSpace(stdStr), r.token)
				if r.token != "" {
					wm := sanitizeToken(waitErr.Error(), r.token)
					return fmt.Errorf("git command failed: %s, stderr: %s", wm, msg)
				}
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

// runGit выполняет git через runner; при ошибке в текст включается stderr (с маскировкой токена).
func runGit(ctx context.Context, runner GitCommandRunner, token, workDir string, args ...string) (string, error) {
	stdout, stderr, err := runner.RunGit(ctx, workDir, args...)
	if err != nil {
		sub := gitSubcommandOrUnknown(args)
		se := strings.TrimSpace(stderr)
		prefix := fmt.Sprintf("git %s: %s", sub, se)
		return "", &runGitError{prefix: prefix, cause: err, token: token, stderr: stderr}
	}
	return stdout, nil
}

// gitRunFailure — результат прямого runner.RunGit(...) с маскировкой токена (как у runGit).
func gitRunFailure(token, subcommand, stderr string, cause error) error {
	se := strings.TrimSpace(stderr)
	prefix := fmt.Sprintf("git %s: %s", subcommand, se)
	return &runGitError{prefix: prefix, cause: cause, token: token, stderr: stderr}
}

// requireContext отклоняет nil context до вызова exec / runner (без panic).
func requireContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("gitprovider: nil context")
	}
	return nil
}

// mapGitCLIError сопоставляет stderr неуспешной git-команды с доменной sentinel-ошибкой (ls-remote / общие случаи).
func mapGitCLIError(stderr string) error {
	msg := strings.ToLower(stderr)
	if strings.Contains(msg, "authentication failed") || strings.Contains(msg, "could not read username") ||
		strings.Contains(msg, "access denied") || strings.Contains(msg, "invalid username or password") {
		return ErrAuthFailed
	}
	return ErrRepoNotFound
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
	return validateNonFlagGitString(branch)
}

// executeCommit — общая логика локального commit (LocalGitProvider и GitHubProvider).
// Между проверкой индекса (git diff --cached) и commit кратковременное окно; в sandbox один процесс — приемлемо.
func executeCommit(ctx context.Context, runner GitCommandRunner, token, workDir string, opts CommitOptions) (string, bool, error) {
	if err := requireContext(ctx); err != nil {
		return "", false, err
	}
	if strings.TrimSpace(opts.Message) == "" {
		return "", false, fmt.Errorf("gitprovider: empty commit message")
	}
	if err := opts.Author.Validate(); err != nil {
		return "", false, err
	}
	var addArgs []string
	addArgs = append(addArgs, "add")
	if len(opts.Files) == 0 {
		addArgs = append(addArgs, "-A")
	} else {
		for _, f := range opts.Files {
			if err := validateGitPathForCommit(f); err != nil {
				return "", false, err
			}
		}
		addArgs = append(addArgs, "--")
		addArgs = append(addArgs, opts.Files...)
	}
	if _, err := runGit(ctx, runner, token, workDir, addArgs...); err != nil {
		return "", false, err
	}

	_, headErr := runGit(ctx, runner, token, workDir, "rev-parse", "--verify", "--end-of-options", "HEAD")
	if headErr != nil {
		statusOut, err := runGit(ctx, runner, token, workDir, "status", "--porcelain")
		if err != nil {
			return "", false, err
		}
		if strings.TrimSpace(statusOut) == "" {
			return "", false, nil
		}
	} else {
		_, stderr, diffErr := runner.RunGit(ctx, workDir, "diff", "--cached", "--quiet")
		if diffErr == nil {
			return "", false, nil
		}
		var exitErr *exec.ExitError
		if !errors.As(diffErr, &exitErr) || exitErr.ExitCode() != 1 {
			return "", false, gitRunFailure(token, "diff", stderr, diffErr)
		}
	}

	commitArgs := []string{"commit", "-m", opts.Message}
	name := strings.TrimSpace(opts.Author.Name)
	email := strings.TrimSpace(opts.Author.Email)
	if name != "" && email != "" {
		commitArgs = append(commitArgs, "--author", fmt.Sprintf("%s <%s>", name, email))
	}
	if _, err := runGit(ctx, runner, token, workDir, commitArgs...); err != nil {
		return "", false, err
	}
	shaOut, err := runGit(ctx, runner, token, workDir, "rev-parse", "HEAD")
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
