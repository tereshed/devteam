package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// ErrDisallowedDialTarget — попытка открыть TCP на IP, не входящий в allowedIPs (DNS rebinding / TOCTOU).
var ErrDisallowedDialTarget = errors.New("dial target not in allowed IP list")

// SafeGitHTTPClient собирает *http.Client с кастомным DialContext, который:
//  1. парсит host:port из addr;
//  2. если host — literal IP, проверяет его против allowedIPs;
//  3. если host — DNS-name, использует allowedIPs НАПРЯМУЮ (без повторного резолва) —
//     это защищает от DNS Rebinding между ValidateGitProviderHost и outbound HTTP.
//
// TLS-handshake выполняется по hostname (не подменяем SNI/cert verification — иначе
// клиент перестанет валидировать сертификат провайдера). Подмена адресов происходит
// исключительно на уровне TCP-dial.
func SafeGitHTTPClient(allowedIPs []net.IP, timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 nil, // не наследуем env-proxy: проксирование могло бы обойти allow-list
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          5,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext:           safeDialContextFactory(dialer, allowedIPs),
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// safeDialContextFactory создаёт DialContext с фиксированным списком allowedIPs.
// Семантика:
//   - host = literal IP → должен быть в allowedIPs;
//   - host = name → пробуем каждый allowedIP по порядку, без повторного DNS-lookup.
func safeDialContextFactory(dialer *net.Dialer, allowedIPs []net.IP) func(ctx context.Context, network, addr string) (net.Conn, error) {
	allowed := make([]net.IP, len(allowedIPs))
	copy(allowed, allowedIPs)

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("split host port: %w", err)
		}
		// Literal IP: проверяем напрямую (без DNS).
		if ip := net.ParseIP(host); ip != nil {
			if !ipAllowed(allowed, ip) {
				return nil, fmt.Errorf("%w: %s", ErrDisallowedDialTarget, ip.String())
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		// DNS name: НЕ резолвим — используем allowedIPs напрямую (anti DNS rebinding).
		var lastErr error
		for _, ip := range allowed {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		if lastErr == nil {
			return nil, fmt.Errorf("%w: empty allow-list", ErrDisallowedDialTarget)
		}
		return nil, fmt.Errorf("dial all allowed IPs failed: %w", lastErr)
	}
}
