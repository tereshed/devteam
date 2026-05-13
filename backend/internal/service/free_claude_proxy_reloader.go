package service

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FreeClaudeProxyReloader — Sprint 15.C4/N3: после WriteFile регенерации config.yaml сообщает
// контейнеру прокси, чтобы тот перечитал файл. Без этого CRUD над llm_providers эффективно
// no-op до ручного docker compose restart.
//
// Контракт upstream: free-claude-proxy ДОЛЖЕН экспонировать POST <ReloadPath> (по умолчанию /reload),
// читающий FREE_CLAUDE_PROXY_CONFIG и применяющий конфиг на лету.
//
// Sprint 15.N3 — известная проблема: апстрим Alishahryar1/free-claude-code на момент написания
// этого кода НЕ имеет /reload-endpoint. До патча upstream либо запускайте прокси без флага
// `--profile free-claude-proxy` и рестартуйте контейнер вручную при изменениях, либо оставляйте
// FREE_CLAUDE_PROXY_RELOAD_PATH="" — тогда backend только пишет config.yaml.
//
// Конкуренция (Sprint 15.M2/N3):
//   - Reload() сериализован через mu.
//   - AsyncReloadHook делает coalescing: один reload активен, второй — pending. Это исключает
//     unbounded goroutines при шквале CRUD и сохраняет инвариант «последний WriteFile точно ушёл».
type FreeClaudeProxyReloader struct {
	builder    *FreeClaudeProxyConfigBuilder
	targetPath string
	baseURL    string
	reloadPath string
	token      string
	client     *http.Client
	logger     *slog.Logger

	mu sync.Mutex
	// asyncRunning/asyncPending — coalescing-флаги для AsyncReloadHook (atomic).
	asyncRunning int32
	asyncPending int32
}

// NewFreeClaudeProxyReloader собирает reloader.
// baseURL/reloadPath="" — reload-вызов будет no-op, WriteFile всё равно отработает.
func NewFreeClaudeProxyReloader(
	builder *FreeClaudeProxyConfigBuilder,
	targetPath, baseURL, reloadPath, serviceToken string,
	logger *slog.Logger,
) *FreeClaudeProxyReloader {
	if logger == nil {
		logger = slog.Default()
	}
	return &FreeClaudeProxyReloader{
		builder:    builder,
		targetPath: targetPath,
		baseURL:    strings.TrimRight(baseURL, "/"),
		reloadPath: reloadPath,
		token:      serviceToken,
		client:     &http.Client{Timeout: 5 * time.Second},
		logger:     logger.With("component", "free_claude_proxy_reloader"),
	}
}

// Reload: regenerate config.yaml + POST <reloadPath>. Сериализуется через mu.
func (r *FreeClaudeProxyReloader) Reload(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.builder.WriteFile(ctx, r.targetPath); err != nil {
		r.logger.Error("regenerate config failed", "err", err)
		return
	}
	r.logger.Info("config regenerated", "path", r.targetPath)

	if r.baseURL == "" || r.reloadPath == "" {
		// Reload отключён: оператор должен сам рестартнуть прокси.
		return
	}
	url := r.baseURL + r.reloadPath
	// Sprint 15.minor: http.NoBody — каноничный «пустой body» без аллокации bytes.Reader.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)
	if err != nil {
		r.logger.Error("build reload request", "err", err)
		return
	}
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		// Sprint 15.N3 caveat: upstream может ещё не реализовать /reload.
		r.logger.Warn("proxy unreachable for reload (upstream may not support /reload)",
			"url", url, "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		r.logger.Warn("proxy reload failed (upstream may not support /reload)",
			"status", resp.StatusCode, "url", url)
		return
	}
	r.logger.Info("proxy reload ok", "url", url)
}

// AsyncReloadHook возвращает функцию для LLMProviderHandler.WithOnChange.
// Sprint 15.N3 — coalescing: горутины ограничены 1, дополнительные вызовы во время
// активного reload взводят asyncPending и завершаются мгновенно.
func (r *FreeClaudeProxyReloader) AsyncReloadHook() func(ctx context.Context) {
	return func(ctx context.Context) {
		if atomic.CompareAndSwapInt32(&r.asyncRunning, 0, 1) {
			go r.asyncReloadLoop(context.WithoutCancel(ctx))
			return
		}
		// Reload уже идёт — взводим pending и завершаемся.
		atomic.StoreInt32(&r.asyncPending, 1)
	}
}

func (r *FreeClaudeProxyReloader) asyncReloadLoop(ctx context.Context) {
	for {
		r.Reload(ctx)
		// Sprint 15.Major: используем Swap вместо CAS — сбрасываем pending без window race
		// между «прочитал=1» и «обнулил». Если был pending — повторим reload.
		if atomic.SwapInt32(&r.asyncPending, 0) == 1 {
			continue
		}
		// Освобождаем running. ВАЖНО: после Store(running, 0) ещё раз проверяем pending,
		// который мог быть взведён ПОСЛЕ нашего Swap, но ДО Store. Без этого последний
		// запрос мог потеряться: новый caller увидит running=1 → выйдет → loop тоже выйдет.
		atomic.StoreInt32(&r.asyncRunning, 0)
		if atomic.LoadInt32(&r.asyncPending) == 1 {
			// Пытаемся снова взять running и обработать.
			if atomic.CompareAndSwapInt32(&r.asyncRunning, 0, 1) {
				continue
			}
			// Конкурирующий caller уже взял running — он подхватит наш pending.
		}
		return
	}
}
