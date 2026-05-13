package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/devteam/backend/internal/models"
)

// Sprint 15.C5 — SSRF guard для исходящих запросов к LLM-провайдерам.
//
// Без него admin (или скомпрометированный admin-токен) через TestConnection / HealthCheck
// может пробить http://169.254.169.254 (AWS metadata), http://localhost:5432 (внутр. сервисы),
// http://10.0.0.1 (RFC1918) — бэкенд выполнит запрос со своими сетевыми правами.
//
// Политика:
//   - схема обязана быть https://, кроме провайдеров явно «локальных»
//     (kind=ollama, kind=anthropic c base_url из defaultResolver).
//   - host НЕ должен разрешаться в loopback / RFC1918 / link-local / CGNAT (кроме «локальных»);
//   - DNS-резолв выполняется с context, чтобы кто-то не подсунул бесконечно тормозящий host.

// ErrInsecureBaseURL — guard отверг URL.
var ErrInsecureBaseURL = errors.New("insecure or disallowed base_url")

// allowsLoopback — kind'ы, которые легально используют loopback/internal addresses.
// ollama по дизайну на localhost.
func allowsLoopback(kind models.LLMProviderKind) bool {
	switch kind {
	case models.LLMProviderKindOllama:
		return true
	default:
		return false
	}
}

// validateBaseURLForProvider — guard на base_url. baseURL=="" — пропускаем (дефолт провайдера).
// Использует DNS-резолв с context (важно при медленных DNS / атаке).
func validateBaseURLForProvider(ctx context.Context, baseURL string, kind models.LLMProviderKind) error {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("%w: parse: %v", ErrInsecureBaseURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" {
		if !(scheme == "http" && allowsLoopback(kind)) {
			return fmt.Errorf("%w: scheme must be https (got %s)", ErrInsecureBaseURL, scheme)
		}
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrInsecureBaseURL)
	}
	// DNS-резолв с контекст-таймаутом. Если ALL IPs запрещены — отказ.
	resolver := &net.Resolver{}
	addrs, err := resolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf("%w: dns lookup: %v", ErrInsecureBaseURL, err)
	}
	for _, a := range addrs {
		ip := net.ParseIP(a)
		if ip == nil {
			continue
		}
		// Sprint 15.Major1: metadata/unspecified — ВСЕГДА запрещены.
		if isAlwaysBlockedIP(ip) {
			return fmt.Errorf("%w: host resolves to always-blocked ip %s", ErrInsecureBaseURL, ip)
		}
		if isLocalLikeIP(ip) && !allowsLoopback(kind) {
			return fmt.Errorf("%w: host resolves to private ip %s", ErrInsecureBaseURL, ip)
		}
	}
	return nil
}

// isDisallowedIP — комбинированная проверка для проверочной валидации (validateBaseURLForProvider).
// Sprint 15.Major: учитываем IPv4-mapped IPv6 (`::ffff:127.0.0.1`).
//
// allowLoopback=false → запрещены loopback/private/link-local/CGNAT/metadata/unspecified.
// allowLoopback=true  → loopback/private/link-local разрешены (для kind=ollama),
// но metadata (169.254.169.254) и unspecified остаются ВСЕГДА запрещёнными.
func isDisallowedIP(ip net.IP) bool {
	return isAlwaysBlockedIP(ip) || isLocalLikeIP(ip)
}

// isAlwaysBlockedIP — ip, который НИКОГДА не должен быть достижим, даже для kind=ollama.
// Это: cloud-metadata, unspecified (0.0.0.0/::), multicast.
func isAlwaysBlockedIP(ip net.IP) bool {
	if ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// AWS / OpenStack / GCP cloud metadata.
		if v4[0] == 169 && v4[1] == 254 && v4[2] == 169 && v4[3] == 254 {
			return true
		}
		// Azure IMDS 169.254.169.254 — то же, что выше; кроме того, fd00:ec2::254 (IPv6) — multicast=false.
	}
	return false
}

// isLocalLikeIP — loopback / RFC1918 / link-local / CGNAT.
// Условно разрешено для kind=ollama (allowLoopback=true).
func isLocalLikeIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// CGNAT 100.64.0.0/10.
		if v4[0] == 100 && (v4[1] >= 64 && v4[1] <= 127) {
			return true
		}
	}
	return false
}

// Sprint 15.Major (SSRF redirect-bypass): http.Client с CheckRedirect отказывается
// следовать редиректам в private/loopback хосты, а DialContext дополнительно проверяет
// resolved-IP при каждом установлении TCP-соединения (на случай DNS rebinding между LookupHost
// и подключением).
//
// Использование:
//
//	client := newSSRFSafeHTTPClient(true /*allowLoopback*/)
//	resp, err := client.Get("https://example.com")
//
// allowLoopback — для kind=ollama.
func newSSRFSafeHTTPClient(allowLoopback bool, timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			// network=tcp4/tcp6, address=host:port. Парсим — net.Dial уже разрешил DNS, так что
			// здесь нам приходит IP:port (или host:port если name resolution отложен).
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return nil // пусть Dial сам отдаст вменяемую ошибку.
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return nil
			}
			// Sprint 15.Major1: metadata/unspecified — никогда не разрешаем (даже для ollama).
			if isAlwaysBlockedIP(ip) {
				return fmt.Errorf("%w: connect to always-blocked %s", ErrInsecureBaseURL, ip)
			}
			if isLocalLikeIP(ip) && !allowLoopback {
				return fmt.Errorf("%w: connect to private %s blocked", ErrInsecureBaseURL, ip)
			}
			return nil
		},
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Sprint 15.Major redirect-bypass: повторно валидируем target redirect.
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return validateRedirectURL(req.URL, allowLoopback)
		},
	}
}

func validateRedirectURL(u *url.URL, allowLoopback bool) error {
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && !(scheme == "http" && allowLoopback) {
		return fmt.Errorf("%w: redirect to scheme %s", ErrInsecureBaseURL, scheme)
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		// Sprint 15.Major1: metadata всегда блок; private — если не allowLoopback.
		if isAlwaysBlockedIP(ip) {
			return fmt.Errorf("%w: redirect to always-blocked ip %s", ErrInsecureBaseURL, ip)
		}
		if isLocalLikeIP(ip) && !allowLoopback {
			return fmt.Errorf("%w: redirect to disallowed ip %s", ErrInsecureBaseURL, ip)
		}
	}
	return nil
}
