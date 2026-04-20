package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// trySendTerminalLogEntry пытается доставить терминальную ошибку в канал без долгой блокировки (5.6).
func trySendTerminalLogEntry(ch chan<- LogEntry, e LogEntry) {
	select {
	case ch <- e:
	default:
		slog.Warn("sandbox: terminal log entry not delivered (channel full)", "sandbox_id", e.SandboxID)
	}
}

func terminalLogStreamError(sandboxID string, publicErr error) LogEntry {
	return LogEntry{
		SandboxID: sandboxID,
		Timestamp: time.Now(),
		Error:     publicErr,
	}
}

// classifyStdCopyError маппит ошибку чтения/демультиплексирования на «публичную» цепочку без сырого текста движка в UI (5.6).
func classifyStdCopyError(sandboxID string, streamCtx context.Context, err error) LogEntry {
	if err == nil {
		return LogEntry{}
	}
	if errors.Is(err, io.EOF) {
		return LogEntry{}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Отмена streamCtx или родительского ctx — терминальная запись; без голого SDK в сообщении.
		if streamCtx.Err() != nil {
			return terminalLogStreamError(sandboxID, fmt.Errorf("log stream: %w", errors.Join(ErrSandboxDocker, streamCtx.Err())))
		}
		return terminalLogStreamError(sandboxID, fmt.Errorf("log stream: %w", errors.Join(ErrSandboxDocker, err)))
	}
	slog.Debug("sandbox: log stream read error", "sandbox_id", sandboxID, "err", err)
	return terminalLogStreamError(sandboxID, fmt.Errorf("log stream: %w", ErrSandboxDocker))
}

func classifyContainerLogsOpenError(sandboxID string, err error) LogEntry {
	slog.Debug("sandbox: container logs open error", "sandbox_id", sandboxID, "err", err)
	return terminalLogStreamError(sandboxID, fmt.Errorf("container logs: %w", ErrSandboxDocker))
}

// trackedInstance возвращает инстанс только из реестра раннера (без adopt через inspect). StreamLogs после Cleanup — ErrSandboxNotFound (5.6).
func (r *DockerSandboxRunner) trackedInstance(sandboxID string) (*instanceState, error) {
	if err := ValidateSandboxID(sandboxID); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.instances[sandboxID]
	if !ok {
		return nil, ErrSandboxNotFound
	}
	return st, nil
}

// StreamLogs реализует SandboxRunner.StreamLogs.
//
// Контракт: вариант А (5.3) — не более одного активного стрима на sandboxID; повторный вызов — ErrStreamAlreadyActive.
// Backpressure (5.6): вариант A — при полном буфере канала входящие LogEntry дропаются, чтение из ContainerLogs не блокируется;
// при первом дропе — одна синтетическая строка logStreamDroppedLine + slog.Warn.
// Since/Until в Docker LogsOptions здесь не поддерживаются (параметры времени — слой service/handler при появлении API).
// Один вызов ContainerLogs на стрим, без reconnect при EOF при живом контейнере (5.6).
// EOF от движка — тихое закрытие канала без обязательной LogEntry.Error (5.6).
func (r *DockerSandboxRunner) StreamLogs(ctx context.Context, sandboxID string) (<-chan LogEntry, error) {
	st, err := r.trackedInstance(sandboxID)
	if err != nil {
		return nil, err
	}
	if r.cli == nil {
		return nil, fmt.Errorf("docker client is nil: %w", ErrSandboxDocker)
	}

	bufCap := StreamLogsDefaultBuffer
	if r.streamLogsEntryBuffer > 0 {
		bufCap = r.streamLogsEntryBuffer
	}
	ch := make(chan LogEntry, bufCap)
	st.streamMu.Lock()
	if st.streamActive {
		// Если стрим уже активен (например, запущен через setupLogPump), 
		// возвращаем сохраненное плечо tee, если оно есть.
		if st.externalCh != nil {
			ch := st.externalCh
			st.externalCh = nil // отдаем только один раз по контракту
			st.streamMu.Unlock()
			return ch, nil
		}
		st.streamMu.Unlock()
		return nil, ErrStreamAlreadyActive
	}
	streamCtx, cancel := context.WithCancel(ctx)
	st.streamCancel = cancel
	st.streamActive = true
	st.streamMu.Unlock()

	go r.runLogStream(streamCtx, cancel, st, sandboxID, ch)
	return ch, nil
}

func (r *DockerSandboxRunner) runLogStream(streamCtx context.Context, cancel context.CancelFunc, st *instanceState, sandboxID string, ch chan LogEntry) {
	defer close(ch)
	defer cancel()
	defer func() {
		st.streamMu.Lock()
		st.streamActive = false
		st.streamCancel = nil
		st.streamCh = nil
		st.externalCh = nil
		st.streamMu.Unlock()
	}()

	// Timestamps в LogsOptions: false (5.6) — время строки задаётся на стороне Go (LogEntry.Timestamp).
	rc, err := r.cli.ContainerLogs(streamCtx, sandboxID, containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: false,
	})
	if err != nil {
		trySendTerminalLogEntry(ch, classifyContainerLogsOpenError(sandboxID, err))
		return
	}
	defer rc.Close()

	outW := newStreamLineWriter(ch, sandboxID, false)
	errW := newStreamLineWriter(ch, sandboxID, true)
	_, cpyErr := stdcopy.StdCopy(outW, errW, rc)
	outW.flush()
	errW.flush()

	if cpyErr != nil {
		if ent := classifyStdCopyError(sandboxID, streamCtx, cpyErr); ent.Error != nil {
			trySendTerminalLogEntry(ch, ent)
		}
		return
	}
	// StdCopy завершился без ошибки (в т.ч. EOF) — канал закрывается без терминальной LogEntry.Error (5.6).
}
