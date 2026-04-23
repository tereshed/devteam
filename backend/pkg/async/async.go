package async

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// TaskOptions опции для запуска асинхронной задачи
type TaskOptions struct {
	Timeout   time.Duration
	Retries   int
	LogTags   map[string]any
	OnSuccess func()
	OnFailure func(err error)
}

// ExecuteWithRetry запускает функцию в горутине с ретраями, таймаутом и защитой от паник.
func ExecuteWithRetry(ctx context.Context, wg *sync.WaitGroup, opts TaskOptions, fn func(ctx context.Context) error) {
	if opts.Timeout == 0 {
		opts.Timeout = 1 * time.Minute
	}
	if opts.Retries <= 0 {
		opts.Retries = 3
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Panic in async task",
					"error", r,
					"stack", string(debug.Stack()),
					"tags", opts.LogTags)
				if opts.OnFailure != nil {
					opts.OnFailure(fmt.Errorf("panic: %v", r))
				}
			}
		}()

		// context.WithoutCancel позволяет задаче завершиться, даже если родительский контекст отменен
		execCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), opts.Timeout)
		defer cancel()

		var lastErr error
		backoff := 500 * time.Millisecond

		for i := 0; i < opts.Retries; i++ {
			if i > 0 {
				select {
				case <-execCtx.Done():
					if opts.OnFailure != nil {
						opts.OnFailure(execCtx.Err())
					}
					return
				case <-time.After(backoff):
					backoff *= 2
				}
			}

			if lastErr = fn(execCtx); lastErr == nil {
				if opts.OnSuccess != nil {
					opts.OnSuccess()
				}
				return
			}

			slog.Warn("async task failed, retrying",
				"attempt", i+1,
				"error", lastErr,
				"tags", opts.LogTags)
		}

		// Если все попытки исчерпаны
		logArgs := []any{"error", lastErr, "tags", opts.LogTags}
		slog.Error("async task failed after retries [INDEX_ORPHAN]", logArgs...)

		if opts.OnFailure != nil {
			opts.OnFailure(lastErr)
		}
	}()
}
