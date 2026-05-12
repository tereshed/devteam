package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// FreeClaudeProxyHealthCheck — реализация FreeClaudeProxyHealthChecker.
// Дёргает GET <baseURL>/healthz; считает 2xx успехом. Используется в orchestrator.Start (Sprint 15.19).
type FreeClaudeProxyHealthCheck struct {
	baseURL string
	client  *http.Client
}

// NewFreeClaudeProxyHealthCheck собирает чекер. Пустой baseURL — отключает фичу (Check вернёт ошибку).
func NewFreeClaudeProxyHealthCheck(baseURL string) *FreeClaudeProxyHealthCheck {
	return &FreeClaudeProxyHealthCheck{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

// Check бьёт GET /healthz и валидирует 2xx.
func (h *FreeClaudeProxyHealthCheck) Check(ctx context.Context) error {
	if h.baseURL == "" {
		return fmt.Errorf("free-claude-proxy: base URL is not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("free-claude-proxy: unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("free-claude-proxy: status %d on /healthz", resp.StatusCode)
	}
	return nil
}
