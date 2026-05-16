package service

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

type fakeResolver struct {
	calls int
	// responses — последовательность ответов; calls=N → responses[N-1]
	responses [][]net.IP
	errs      []error
}

func (f *fakeResolver) LookupIP(_ context.Context, _ string, _ string) ([]net.IP, error) {
	idx := f.calls
	f.calls++
	if idx >= len(f.responses) && idx >= len(f.errs) {
		return nil, errors.New("no more fake responses configured")
	}
	if idx < len(f.errs) && f.errs[idx] != nil {
		return nil, f.errs[idx]
	}
	if idx < len(f.responses) {
		return f.responses[idx], nil
	}
	return nil, errors.New("no more fake responses")
}

func TestValidateGitProviderHost_SchemeAndForm(t *testing.T) {
	v := NewGitProviderHostValidator(&fakeResolver{responses: [][]net.IP{{net.ParseIP("8.8.8.8")}}}, true)

	// Базовый https с публичным IP проходит (canonical отсекает trailing slash).
	canon, _, err := v.ValidateGitProviderHost(context.Background(), "https://gitlab.example.com/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if canon != "https://gitlab.example.com" {
		t.Fatalf("canonical mismatch: %q", canon)
	}
}

func TestValidateGitProviderHost_RejectsHTTPInProd(t *testing.T) {
	v := NewGitProviderHostValidator(&fakeResolver{}, true)
	_, _, err := v.ValidateGitProviderHost(context.Background(), "http://gitlab.example.com")
	if err == nil || !errors.Is(err, ErrInvalidGitProviderHost) {
		t.Fatalf("expected ErrInvalidGitProviderHost, got %v", err)
	}
}

func TestValidateGitProviderHost_AllowsHTTPInDev(t *testing.T) {
	v := NewGitProviderHostValidator(&fakeResolver{responses: [][]net.IP{{net.ParseIP("127.0.0.1")}}}, false)
	if _, _, err := v.ValidateGitProviderHost(context.Background(), "http://gitlab.local"); err != nil {
		t.Fatalf("expected ok in dev: %v", err)
	}
}

func TestValidateGitProviderHost_RejectsUserinfo(t *testing.T) {
	v := NewGitProviderHostValidator(&fakeResolver{}, true)
	_, _, err := v.ValidateGitProviderHost(context.Background(), "https://u:p@gitlab.example.com")
	if err == nil || !errors.Is(err, ErrInvalidGitProviderHost) {
		t.Fatalf("expected userinfo rejection, got %v", err)
	}
}

func TestValidateGitProviderHost_RejectsEmpty(t *testing.T) {
	v := NewGitProviderHostValidator(&fakeResolver{}, true)
	if _, _, err := v.ValidateGitProviderHost(context.Background(), ""); err == nil {
		t.Fatal("expected error on empty input")
	}
	if _, _, err := v.ValidateGitProviderHost(context.Background(), "   "); err == nil {
		t.Fatal("expected error on whitespace input")
	}
}

func TestValidateGitProviderHost_RejectsPrivateIPInProd(t *testing.T) {
	cases := []string{
		"https://10.0.0.1",
		"https://192.168.1.1",
		"https://172.16.5.5",
		"https://127.0.0.1",
		"https://169.254.1.1", // link-local
		"https://[fe80::1]",   // ipv6 link-local
		"https://[fd00::1]",   // ipv6 ULA
		"https://[::1]",       // ipv6 loopback
		"https://100.64.0.1",  // CGNAT
	}
	v := NewGitProviderHostValidator(&fakeResolver{}, true)
	for _, raw := range cases {
		_, _, err := v.ValidateGitProviderHost(context.Background(), raw)
		if err == nil || !errors.Is(err, ErrPrivateGitProviderHost) {
			t.Errorf("%s: expected private IP rejection, got %v", raw, err)
		}
	}
}

func TestValidateGitProviderHost_AllowsLocalhostInDev(t *testing.T) {
	v := NewGitProviderHostValidator(&fakeResolver{}, false)
	if _, _, err := v.ValidateGitProviderHost(context.Background(), "http://127.0.0.1:8080"); err != nil {
		t.Fatalf("expected ok in dev: %v", err)
	}
}

func TestValidateGitProviderHost_RejectsDNSResolveToPrivate(t *testing.T) {
	resolver := &fakeResolver{responses: [][]net.IP{{net.ParseIP("10.0.0.1")}}}
	v := NewGitProviderHostValidator(resolver, true)
	_, _, err := v.ValidateGitProviderHost(context.Background(), "https://evil.example.com")
	if err == nil || !errors.Is(err, ErrPrivateGitProviderHost) {
		t.Fatalf("expected private after DNS, got %v", err)
	}
}

func TestValidateGitProviderHost_DNSLookupFailure(t *testing.T) {
	resolver := &fakeResolver{errs: []error{errors.New("nx")}}
	v := NewGitProviderHostValidator(resolver, true)
	_, _, err := v.ValidateGitProviderHost(context.Background(), "https://nx.example.com")
	if err == nil || !errors.Is(err, ErrGitProviderResolveFailed) {
		t.Fatalf("expected resolve fail, got %v", err)
	}
}

func TestValidateGitProviderHost_PublicResolves(t *testing.T) {
	resolver := &fakeResolver{responses: [][]net.IP{{net.ParseIP("8.8.8.8")}}}
	v := NewGitProviderHostValidator(resolver, true)
	canon, ips, err := v.ValidateGitProviderHost(context.Background(), "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if canon != "https://gitlab.example.com" {
		t.Fatalf("canonical mismatch: %q", canon)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("8.8.8.8")) {
		t.Fatalf("ips mismatch: %v", ips)
	}
}

func TestValidateGitProviderHost_DNSRebindingDefence(t *testing.T) {
	// Сначала резолв даёт 8.8.8.8 (валидно), затем — 127.0.0.1 (rebind).
	resolver := &fakeResolver{responses: [][]net.IP{
		{net.ParseIP("8.8.8.8")},
		{net.ParseIP("127.0.0.1")},
	}}
	v := NewGitProviderHostValidator(resolver, true)

	canon, allowed, err := v.ValidateGitProviderHost(context.Background(), "https://gitlab.example.com")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !strings.HasPrefix(canon, "https://") {
		t.Fatalf("canonical mismatch: %q", canon)
	}
	if len(allowed) != 1 || !allowed[0].Equal(net.ParseIP("8.8.8.8")) {
		t.Fatalf("expected only 8.8.8.8 in allow-list, got %v", allowed)
	}

	// Симулируем второй резолв — он бы вернул 127.0.0.1, но в safeDialContext
	// мы используем allowed напрямую и не вызываем LookupIP повторно.
	dialFn := safeDialContextFactory(&net.Dialer{}, allowed)
	// Дайл против literal 127.0.0.1 должен быть отклонён.
	_, dialErr := dialFn(context.Background(), "tcp", "127.0.0.1:443")
	if !errors.Is(dialErr, ErrDisallowedDialTarget) {
		t.Fatalf("expected ErrDisallowedDialTarget for 127.0.0.1 literal, got %v", dialErr)
	}
}

func TestSafeDialContext_AllowedLiteralAllowsConnect(t *testing.T) {
	// Поднимаем локальный TCP listener и помещаем его IP в allow-list.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	host, port, _ := net.SplitHostPort(ln.Addr().String())
	ip := net.ParseIP(host)

	dialFn := safeDialContextFactory(&net.Dialer{}, []net.IP{ip})
	conn, err := dialFn(context.Background(), "tcp", net.JoinHostPort(host, port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	_ = conn.Close()
}

func TestSafeDialContext_NameUsesAllowedIPsNoResolve(t *testing.T) {
	// host=name → DialContext должен использовать allowedIPs без дополнительного резолва.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	ip := net.ParseIP("127.0.0.1")

	dialFn := safeDialContextFactory(&net.Dialer{}, []net.IP{ip})
	conn, err := dialFn(context.Background(), "tcp", net.JoinHostPort("gitlab.example.com", port))
	if err != nil {
		t.Fatalf("dial via name: %v", err)
	}
	_ = conn.Close()
}

func TestSafeDialContext_EmptyAllowedFails(t *testing.T) {
	dialFn := safeDialContextFactory(&net.Dialer{}, nil)
	_, err := dialFn(context.Background(), "tcp", "gitlab.example.com:443")
	if !errors.Is(err, ErrDisallowedDialTarget) {
		t.Fatalf("expected ErrDisallowedDialTarget, got %v", err)
	}
}

func TestSafeDialContext_RejectsBadAddr(t *testing.T) {
	dialFn := safeDialContextFactory(&net.Dialer{}, []net.IP{net.ParseIP("1.1.1.1")})
	_, err := dialFn(context.Background(), "tcp", "no-port")
	if err == nil {
		t.Fatal("expected error on malformed addr")
	}
}

func TestValidateGitProviderHost_IPv6Canonical(t *testing.T) {
	// Публичный IPv6-литерал с портом — canonical должен сохранить квадратные скобки.
	v := NewGitProviderHostValidator(&fakeResolver{}, true)
	canon, ips, err := v.ValidateGitProviderHost(context.Background(), "https://[2001:4860:4860::8888]:8443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if canon != "https://[2001:4860:4860::8888]:8443" {
		t.Fatalf("canonical mismatch (IPv6 brackets/port lost): %q", canon)
	}
	if len(ips) != 1 {
		t.Fatalf("ips: %v", ips)
	}
}

func TestValidateGitProviderHost_PreservesPort(t *testing.T) {
	v := NewGitProviderHostValidator(&fakeResolver{responses: [][]net.IP{{net.ParseIP("8.8.8.8")}}}, true)
	canon, _, err := v.ValidateGitProviderHost(context.Background(), "https://gitlab.example.com:8443/foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if canon != "https://gitlab.example.com:8443" {
		t.Fatalf("canonical mismatch: %q", canon)
	}
}

func TestIsPrivateOrLocalIP_Cases(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":         false,
		"1.1.1.1":         false,
		"127.0.0.1":       true,
		"10.0.0.1":        true,
		"172.16.0.1":      true,
		"192.168.0.1":     true,
		"169.254.0.1":     true,
		"100.64.0.1":      true,
		"100.128.0.1":     false, // вне CGNAT
		"::1":             true,
		"fe80::1":         true,
		"fc00::1":         true,
		"2001:4860:4860::8888": false,
	}
	for ipStr, want := range cases {
		got := isPrivateOrLocalIP(net.ParseIP(ipStr))
		if got != want {
			t.Errorf("isPrivateOrLocalIP(%s)=%v, want %v", ipStr, got, want)
		}
	}
}
