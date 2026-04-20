package sandbox

import (
	"bytes"
	"log/slog"
	"sync/atomic"
	"time"
)

// logStreamDroppedLine — синтетическая строка при первом дропе по backpressure (5.6, вариант A: не блокировать ридер).
const logStreamDroppedLine = "logs dropped: slow consumer"

// streamLineWriter принимает байты из stdcopy.StdCopy и эмитит LogEntry с политикой backpressure 5.6 (вариант A).
//
// Timestamp (5.6): time.Now() только у первого чанка логической строки; продолжения после разбиения
// по LogEntryMaxLineBytes — с Timestamp.IsZero().
type streamLineWriter struct {
	ch        chan<- LogEntry
	sandboxID string
	stderr    bool
	buf       []byte

	lineStart   bool // начало новой логической строки (после \n или старт стрима)
	droppedOnce atomic.Bool
}

func newStreamLineWriter(ch chan<- LogEntry, sandboxID string, stderr bool) *streamLineWriter {
	return &streamLineWriter{
		ch:        ch,
		sandboxID: sandboxID,
		stderr:    stderr,
		lineStart: true,
	}
}

func (w *streamLineWriter) trySend(entry LogEntry) {
	select {
	case w.ch <- entry:
	default:
		// Входящая порция теряется (вариант A, 5.6); ридер Docker не блокируется.
		if !w.droppedOnce.CompareAndSwap(false, true) {
			return
		}
		drop := LogEntry{
			SandboxID: w.sandboxID,
			Timestamp: time.Now(),
			Line:      logStreamDroppedLine,
			Stderr:    w.stderr,
			Truncated: true,
		}
		select {
		case w.ch <- drop:
		default:
			// Однократно за жизнь writer (droppedOnce): одна аллокация таймера — допустимо (5.6).
			t := time.NewTimer(5 * time.Millisecond)
			select {
			case w.ch <- drop:
			case <-t.C:
			}
			t.Stop()
		}
		slog.Warn("sandbox: log stream dropping entries (slow consumer)", "sandbox_id", w.sandboxID, "stderr", w.stderr)
	}
}

// emitFragment эмитит data; completesLogicalLine=true если закончен фрагмент до символа '\n' (или пустая строка между \n\n).
func (w *streamLineWriter) emitFragment(data []byte, completesLogicalLine bool) {
	for len(data) > 0 {
		n := len(data)
		if n > LogEntryMaxLineBytes {
			n = LogEntryMaxLineBytes
		}
		var ts time.Time
		if w.lineStart {
			ts = time.Now()
		}
		w.trySend(LogEntry{
			SandboxID: w.sandboxID,
			Timestamp: ts,
			Line:      string(data[:n]),
			Stderr:    w.stderr,
			Truncated: len(data) > n || !completesLogicalLine,
		})
		data = data[n:]
		w.lineStart = false
	}
	if completesLogicalLine {
		w.lineStart = true
	}
}

func (w *streamLineWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	origLen := len(p)

	merged := len(w.buf) > 0
	var data []byte
	if merged {
		w.buf = append(w.buf, p...)
		data = w.buf
	} else {
		data = p
	}

	for len(data) > 0 {
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			break
		}
		w.emitFragment(data[:i], true)
		data = data[i+1:]
	}

	if len(data) == 0 {
		if merged {
			w.buf = w.buf[:0]
		}
		return origLen, nil
	}

	const maxHold = LogEntryMaxLineBytes * 8
	if len(data) > maxHold {
		w.emitFragment(data, false)
		w.buf = w.buf[:0]
		return origLen, nil
	}

	if merged {
		n := copy(w.buf, data)
		w.buf = w.buf[:n]
	} else {
		w.buf = append(w.buf[:0], data...)
	}
	return origLen, nil
}

func (w *streamLineWriter) flush() {
	if len(w.buf) == 0 {
		return
	}
	w.emitFragment(w.buf, false)
	w.buf = w.buf[:0]
}
