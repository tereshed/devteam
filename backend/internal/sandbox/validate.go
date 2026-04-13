package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
	"unicode"
)

// repoHostLookupTimeout — верхняя граница DNS при ValidateRepoURL (не блокировать RunTask навсегда).
const repoHostLookupTimeout = 5 * time.Second

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
// Строка должна быть уже без ведущих/хвостовых пробелов (см. SandboxOptions.Validate).
// Правила ориентированы на git check-ref-format / безопасность: без ведущего '-', без пробелов
// и без символов, опасных для оболочки и ref-парсинга.
func ValidateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch: empty: %w", ErrInvalidBranchName)
	}
	if len(branch) > 255 {
		return fmt.Errorf("branch: too long: %w", ErrInvalidBranchName)
	}
	if strings.Contains(branch, "//") {
		return fmt.Errorf("branch: must not contain consecutive slashes: %w", ErrInvalidBranchName)
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
// Допустимы только схемы http, https, git, ssh; запрещены file:// и хосты loopback / link-local / unspecified (0.0.0.0, ::),
// в том числе после DNS (десятичный/иный формат IP, который netip.ParseAddr не понимает, но резолвер ОС — да).
// Поддерживается SCP-форма git@host:org/repo.git (эквивалент ssh).
func ValidateRepoURL(ctx context.Context, raw string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(raw) != raw {
		return fmt.Errorf("repo_url: leading or trailing whitespace: %w", ErrInvalidRepoURL)
	}
	if raw == "" {
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
	if err := validateRepoHostNoSSHOptionInjection(host); err != nil {
		return err
	}
	lookupCtx, cancel := context.WithTimeout(ctx, repoHostLookupTimeout)
	defer cancel()
	blocked, lookupErr := isBlockedRepoHost(lookupCtx, host)
	if lookupErr != nil {
		return fmt.Errorf("repo_url: host %q: %w", host, errors.Join(ErrInvalidRepoURL, lookupErr))
	}
	if blocked {
		return fmt.Errorf("repo_url: host %q not allowed: %w", host, ErrInvalidRepoURL)
	}
	return nil
}

// validateRepoHostNoSSHOptionInjection — git вызывает ssh с «хостом» из URL; хост не должен начинаться с '-' и т.п.
// (ssh://-oProxyCommand=…). Допускается литерал IP, который парсится netip (в т.ч. IPv6 вида ::1).
func validateRepoHostNoSSHOptionInjection(host string) error {
	if host == "" {
		return fmt.Errorf("repo_url: empty host: %w", ErrInvalidRepoURL)
	}
	c := host[0]
	alnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
	if alnum {
		return nil
	}
	if _, perr := netip.ParseAddr(host); perr == nil {
		return nil
	}
	return fmt.Errorf("repo_url: host must start with alphanumeric or be a literal IP (got %q): %w", host, ErrInvalidRepoURL)
}

func parseRepoURLHost(raw string) (scheme, host string, err error) {
	if u, e := url.Parse(raw); e == nil && u.Scheme != "" {
		switch strings.ToLower(u.Scheme) {
		case "http", "https", "git", "ssh":
			h := u.Hostname()
			if h == "" {
				return "", "", fmt.Errorf("repo_url: empty host: %w", ErrInvalidRepoURL)
			}
			return u.Scheme, h, nil
		default:
			// «github.com:org/repo» даёт scheme=github.com без реального хоста — только SCP-ветка.
		}
	}
	// SCP: [user@]host:path или host:path (без схемы) — url.Parse для таких строк в Go не даёт схему.
	at := strings.IndexByte(raw, '@')
	rest := raw
	if at >= 0 {
		rest = raw[at+1:]
	}
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

// isBlockedRepoHost возвращает (true, nil), если хост запрещён; (false, nil) если можно продолжать;
// (true, err) при ошибке DNS или пустом наборе адресов — консервативно для SSRF.
func isBlockedRepoHost(ctx context.Context, host string) (blocked bool, err error) {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" || h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true, nil
	}
	// IPv6 может приходить в квадратных скобках
	hostForParse := strings.TrimPrefix(strings.TrimSuffix(h, "]"), "[")
	addr, perr := netip.ParseAddr(hostForParse)
	if perr == nil {
		return addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified(), nil
	}

	ips, lerr := net.DefaultResolver.LookupIPAddr(ctx, hostForParse)
	if lerr != nil {
		return true, lerr
	}
	if len(ips) == 0 {
		return true, fmt.Errorf("repo_url: host %q resolved to no IP addresses", host)
	}
	for _, ipa := range ips {
		if len(ipa.IP) == 0 {
			continue
		}
		a, ok := netip.AddrFromSlice(ipa.IP)
		if !ok {
			continue
		}
		if a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsUnspecified() {
			return true, nil
		}
	}
	return false, nil
}

// ValidateAllowedImage отклоняет образы вне allowlist (строгое совпадение строки ref).
// allowed должен быть непустым (раннер подставляет безопасные дефолты в конструкторе).
func ValidateAllowedImage(image string, allowed []string) error {
	if len(allowed) == 0 {
		return fmt.Errorf("image: empty allowlist: %w", ErrInvalidOptions)
	}
	for _, a := range allowed {
		if image == a {
			return nil
		}
	}
	return fmt.Errorf("image %q not in allowlist: %w", image, ErrInvalidOptions)
}
