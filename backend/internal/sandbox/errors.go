package sandbox

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strings"
	"unicode"
)

// Sentinel-ошибки пакета sandbox (маппинг на HTTP — на границе handler/apierror).
var (
	// ErrInvalidSandboxID — sandboxID не прошёл валидацию до вызова Docker API.
	ErrInvalidSandboxID = errors.New("sandbox: invalid sandbox id")
	// ErrSandboxNotFound — валидный ID, инстанс неизвестен раннеру.
	ErrSandboxNotFound = errors.New("sandbox: sandbox not found")
	// ErrSandboxAlreadyStopped — повторная остановка в недопустимом состоянии.
	ErrSandboxAlreadyStopped = errors.New("sandbox: sandbox already stopped")
	// ErrInvalidOptions — невалидные SandboxOptions в начале RunTask.
	ErrInvalidOptions = errors.New("sandbox: invalid options")
	// ErrStreamAlreadyActive — повторный StreamLogs при уже активном стриме (вариант А, MVP).
	ErrStreamAlreadyActive = errors.New("sandbox: log stream already active")
	// ErrInvalidBranchName — имя ветки не прошло ValidateBranchName (инъекции, пробелы, правила ref).
	ErrInvalidBranchName = errors.New("sandbox: invalid branch name")
	// ErrInvalidEnvKeys — ключи EnvVars не прошли ValidateEnvKeys (инъекция через PATH/LD_* и т.д.).
	ErrInvalidEnvKeys = errors.New("sandbox: invalid env keys")
	// ErrInvalidRepoURL — RepoURL не прошёл ValidateRepoURL (SSRF, file://, недопустимая схема).
	ErrInvalidRepoURL = errors.New("sandbox: invalid repo url")
	// ErrSandboxRunConflict — повторный RunTask при уже существующем контейнере для того же TaskID (политика без adopt).
	ErrSandboxRunConflict = errors.New("sandbox: run conflict for task id")
)

// allowedSandboxEnvKeys — белый список ключей, разрешённых в SandboxOptions.EnvVars для entrypoint/clone.
// Расширение: константы из types.go, префикс APP_, дев-only TASK_* (малый объём); см. ValidateEnvKeys.
var allowedSandboxEnvKeys = map[string]struct{}{
	EnvRepoURL:          {},
	EnvBranchName:       {},
	EnvBaseRef:          {},
	EnvGitDefaultBranch: {},
	EnvBackend:          {},
	EnvAnthropicAPIKey:  {},
	EnvMaxTurns:         {},
	"TASK_INSTRUCTION":  {},
	"TASK_CONTEXT":      {},
	"GITHUB_TOKEN":      {},
	"GITLAB_TOKEN":      {},
	"GIT_TOKEN":         {},
	"BITBUCKET_TOKEN":   {},
}

// ValidateSandboxID проверяет формат идентификатора до обращения к Docker API.
// Политика MVP: полный ID контейнера Docker Engine — 64 символа [0-9a-f]
// (нижний регистр, как отдаёт Engine). Короткие префиксы и произвольный ввод не допускаются.
func ValidateSandboxID(id string) error {
	if len(id) != 64 {
		return ErrInvalidSandboxID
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return ErrInvalidSandboxID
		}
	}
	return nil
}

// ValidateBranchName проверяет Branch до передачи в Docker/entrypoint (defense in depth).
// Правила ориентированы на git check-ref-format / безопасность: без ведущего '-', без пробелов
// и без символов, опасных для оболочки и ref-парсинга.
func ValidateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch: empty: %w", ErrInvalidBranchName)
	}
	if len(branch) > 255 {
		return fmt.Errorf("branch: too long: %w", ErrInvalidBranchName)
	}
	if branch[0] == '-' || branch[0] == '.' || branch[0] == '/' {
		return fmt.Errorf("branch: must not start with '-', '.' or '/': %w", ErrInvalidBranchName)
	}
	if branch[len(branch)-1] == '/' || branch[len(branch)-1] == '.' {
		return fmt.Errorf("branch: must not end with '/' or '.': %w", ErrInvalidBranchName)
	}
	if strings.Contains(branch, "@{") {
		return fmt.Errorf("branch: must not contain '@{': %w", ErrInvalidBranchName)
	}
	if strings.Contains(branch, "..") {
		return fmt.Errorf("branch: must not contain '..': %w", ErrInvalidBranchName)
	}
	if strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("branch: must not end with '.lock': %w", ErrInvalidBranchName)
	}
	for _, r := range branch {
		if r < 32 || r == 127 {
			return fmt.Errorf("branch: control character forbidden: %w", ErrInvalidBranchName)
		}
		if unicode.IsSpace(r) {
			return fmt.Errorf("branch: whitespace forbidden: %w", ErrInvalidBranchName)
		}
		switch r {
		case '~', '^', ':', '\\', '?', '*', '[':
			return fmt.Errorf("branch: forbidden character %q: %w", r, ErrInvalidBranchName)
		}
	}
	return nil
}

// ValidateEnvKeys проверяет ключи EnvVars до передачи в Docker (защита от LD_PRELOAD, PATH и т.п.).
// Разрешены только ключи из белого списка (известные имена для entrypoint/git) или с префиксом APP_.
func ValidateEnvKeys(env map[string]string) error {
	for k := range env {
		if k == "" {
			return fmt.Errorf("env: empty key: %w", ErrInvalidEnvKeys)
		}
		if !isSafeEnvKeyToken(k) {
			return fmt.Errorf("env: invalid key syntax %q: %w", k, ErrInvalidEnvKeys)
		}
		if _, ok := allowedSandboxEnvKeys[k]; ok {
			continue
		}
		if strings.HasPrefix(k, "APP_") {
			continue
		}
		return fmt.Errorf("env: disallowed key %q (use known keys or APP_*): %w", k, ErrInvalidEnvKeys)
	}
	return nil
}

// isSafeEnvKeyToken — только ASCII [A-Za-z_][A-Za-z0-9_]* (без =, пробелов, unicode).
func isSafeEnvKeyToken(k string) bool {
	for i := 0; i < len(k); i++ {
		c := k[i]
		if i == 0 {
			if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && c != '_' {
				return false
			}
			continue
		}
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

// ValidateRepoURL проверяет URL клона до git clone в контейнере (SSRF, file://).
// Допустимы только схемы http, https, git, ssh; запрещены file:// и хосты loopback / link-local (в т.ч. 169.254.0.0/16).
// Поддерживается SCP-форма git@host:org/repo.git (эквивалент ssh).
func ValidateRepoURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("repo_url: empty: %w", ErrInvalidRepoURL)
	}
	scheme, host, err := parseRepoURLHost(raw)
	if err != nil {
		return err
	}
	switch strings.ToLower(scheme) {
	case "http", "https", "git", "ssh":
	default:
		return fmt.Errorf("repo_url: scheme %q not allowed: %w", scheme, ErrInvalidRepoURL)
	}
	if host == "" {
		return fmt.Errorf("repo_url: empty host: %w", ErrInvalidRepoURL)
	}
	if isBlockedRepoHost(host) {
		return fmt.Errorf("repo_url: host %q not allowed: %w", host, ErrInvalidRepoURL)
	}
	return nil
}

func parseRepoURLHost(raw string) (scheme, host string, err error) {
	if u, e := url.Parse(raw); e == nil && u.Scheme != "" {
		h := u.Hostname()
		if h == "" {
			return "", "", fmt.Errorf("repo_url: empty host: %w", ErrInvalidRepoURL)
		}
		return u.Scheme, h, nil
	}
	// SCP: [user@]host:path (без схемы) — url.Parse для git@host:path в Go возвращает ошибку.
	at := strings.IndexByte(raw, '@')
	if at < 0 {
		return "", "", fmt.Errorf("repo_url: parse: %w", ErrInvalidRepoURL)
	}
	rest := raw[at+1:]
	colon := strings.IndexByte(rest, ':')
	if colon <= 0 || colon == len(rest)-1 {
		return "", "", fmt.Errorf("repo_url: scp form: %w", ErrInvalidRepoURL)
	}
	h := rest[:colon]
	if strings.ContainsAny(h, "/?#") {
		return "", "", fmt.Errorf("repo_url: bad host in scp form: %w", ErrInvalidRepoURL)
	}
	return "ssh", h, nil
}

func isBlockedRepoHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" || h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	// IPv6 может приходить в квадратных скобках
	h = strings.TrimPrefix(strings.TrimSuffix(h, "]"), "[")
	addr, err := netip.ParseAddr(h)
	if err == nil {
		return addr.IsLoopback() || addr.IsLinkLocalUnicast()
	}
	return false
}
