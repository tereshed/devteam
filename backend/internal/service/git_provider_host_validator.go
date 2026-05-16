package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ErrInvalidGitProviderHost — невалидный host (схема, userinfo, формат).
var ErrInvalidGitProviderHost = errors.New("invalid git provider host")

// ErrPrivateGitProviderHost — host резолвится в приватный/локальный IP.
var ErrPrivateGitProviderHost = errors.New("git provider host resolves to private IP (rejected)")

// ErrGitProviderResolveFailed — DNS lookup не вернул ни одного A/AAAA.
var ErrGitProviderResolveFailed = errors.New("git provider host did not resolve to any IP")

// HostResolver — абстракция над DNS-резолвером (для тестов).
type HostResolver interface {
	LookupIP(ctx context.Context, network, host string) ([]net.IP, error)
}

type netDefaultResolver struct{}

func (netDefaultResolver) LookupIP(ctx context.Context, network, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, network, host)
}

// DefaultHostResolver — резолвер на основе net.DefaultResolver.
func DefaultHostResolver() HostResolver { return netDefaultResolver{} }

// GitProviderHostValidator — валидация и DNS-резолв host'ов для BYO git-провайдеров.
//
// Особенности:
//   - schema разрешена https; http — только если allowHTTP=true (dev/test).
//   - userinfo (user:pass@) в URL запрещён.
//   - trailing slash в pathе — отрезаем при канонизации.
//   - IP-литералы (включая IPv6 в [скобках]) допускаются как host; в этом случае
//     validateGitProviderHost возвращает [IP] как allowedIPs без DNS-резолва.
//   - DNS-резолв host'а делается ОДИН раз; список возвращённых публичных IP запоминается
//     и используется safeGitHTTPClient.DialContext (см. git_provider_safe_http.go) —
//     defence against DNS Rebinding (TOCTOU между validate и outbound HTTP).
type GitProviderHostValidator struct {
	resolver       HostResolver
	allowPrivateIP bool // dev/test only; в prod должен быть false
	allowHTTP      bool // dev/test only; в prod — только https
}

// NewGitProviderHostValidator — конструктор. prod=true (production) выставляет жёсткие правила.
func NewGitProviderHostValidator(resolver HostResolver, prod bool) *GitProviderHostValidator {
	if resolver == nil {
		resolver = netDefaultResolver{}
	}
	return &GitProviderHostValidator{
		resolver:       resolver,
		allowPrivateIP: !prod,
		allowHTTP:      !prod,
	}
}

// ValidateGitProviderHost — каноникализирует host и резолвит DNS в список разрешённых IP.
// Возвращает:
//   - canonical: scheme://host[:port] (без trailing slash и path);
//   - allowedIPs: список IP, на которые можно открывать TCP (защита от DNS rebinding);
//   - err: ErrInvalidGitProviderHost / ErrPrivateGitProviderHost / ErrGitProviderResolveFailed.
func (v *GitProviderHostValidator) ValidateGitProviderHost(ctx context.Context, raw string) (canonical string, allowedIPs []net.IP, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, fmt.Errorf("%w: empty", ErrInvalidGitProviderHost)
	}

	u, parseErr := url.Parse(raw)
	if parseErr != nil {
		return "", nil, fmt.Errorf("%w: parse: %v", ErrInvalidGitProviderHost, parseErr)
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && v.allowHTTP) {
		return "", nil, fmt.Errorf("%w: scheme %q not allowed", ErrInvalidGitProviderHost, u.Scheme)
	}
	if u.User != nil {
		return "", nil, fmt.Errorf("%w: userinfo not allowed", ErrInvalidGitProviderHost)
	}
	if u.Host == "" {
		return "", nil, fmt.Errorf("%w: empty host", ErrInvalidGitProviderHost)
	}
	// path/fragment/query — игнорируем; canonical = scheme://host[:port]
	hostname := u.Hostname()
	if hostname == "" {
		return "", nil, fmt.Errorf("%w: empty hostname", ErrInvalidGitProviderHost)
	}

	// canonical — scheme + "://" + u.Host. u.Host уже хранит IPv6 в скобках и порт через ":",
	// поэтому ручная сборка из hostname+port ломала бы IPv6-литералы (https://[::1]:8080).
	canon := u.Scheme + "://" + u.Host

	// Если host — IP-литерал, не делаем DNS.
	if ip := net.ParseIP(hostname); ip != nil {
		if !v.allowPrivateIP && isPrivateOrLocalIP(ip) {
			return "", nil, fmt.Errorf("%w: %s", ErrPrivateGitProviderHost, ip.String())
		}
		return canon, []net.IP{ip}, nil
	}

	ips, lookupErr := v.resolver.LookupIP(ctx, "ip", hostname)
	if lookupErr != nil {
		return "", nil, fmt.Errorf("%w: lookup %s: %v", ErrGitProviderResolveFailed, hostname, lookupErr)
	}
	if len(ips) == 0 {
		return "", nil, fmt.Errorf("%w: %s", ErrGitProviderResolveFailed, hostname)
	}
	for _, ip := range ips {
		if !v.allowPrivateIP && isPrivateOrLocalIP(ip) {
			return "", nil, fmt.Errorf("%w: %s -> %s", ErrPrivateGitProviderHost, hostname, ip.String())
		}
	}
	return canon, ips, nil
}

// isPrivateOrLocalIP — true для loopback / unspecified / link-local /
// private (RFC1918 + RFC4193) / CGNAT / multicast.
func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip.IsPrivate() { // RFC1918 + RFC4193 (fc00::/7)
		return true
	}
	// CGNAT (RFC 6598): 100.64.0.0/10
	if v4 := ip.To4(); v4 != nil {
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}

// ipAllowed — true если ip присутствует в allow-list (без DNS lookup).
func ipAllowed(allowed []net.IP, ip net.IP) bool {
	for _, a := range allowed {
		if a.Equal(ip) {
			return true
		}
	}
	return false
}
