package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FreeClaudeProxyHealthCheck — реализация FreeClaudeProxyHealthChecker.
// Дёргает GET <baseURL><healthPath>; считает 2xx успехом. Используется в orchestrator.Start (Sprint 15.19).
//
// Sprint 15.M10: путь health-check вынесен в конфиг (FREE_CLAUDE_PROXY_HEALTH_PATH),
// потому что upstream-образ free-claude-code не гарантирует именно /healthz.
type FreeClaudeProxyHealthCheck struct {
	baseURL    string
	healthPath string
	client     *http.Client
}

// NewFreeClaudeProxyHealthCheck собирает чекер. Пустой healthPath → "/healthz".
func NewFreeClaudeProxyHealthCheck(baseURL, healthPath string) *FreeClaudeProxyHealthCheck {
	if strings.TrimSpace(healthPath) == "" {
		healthPath = "/healthz"
	}
	if !strings.HasPrefix(healthPath, "/") {
		healthPath = "/" + healthPath
	}
	return &FreeClaudeProxyHealthCheck{
		baseURL:    strings.TrimRight(baseURL, "/"),
		healthPath: healthPath,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

// Check бьёт GET <healthPath> и валидирует 2xx.
func (h *FreeClaudeProxyHealthCheck) Check(ctx context.Context) error {
	if h.baseURL == "" {
		return fmt.Errorf("free-claude-proxy: base URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+h.healthPath, nil)
	if err != nil {
		return err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("free-claude-proxy: unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("free-claude-proxy: status %d on %s", resp.StatusCode, h.healthPath)
	}
	return nil
}
