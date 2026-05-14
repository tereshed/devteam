package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// TestHandler_RedactsSensitiveTopLevelKeys — основной guard: значения по sensitive-ключам
// не должны попадать в JSON-output логгера.
func TestHandler_RedactsSensitiveTopLevelKeys(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logger := slog.New(h)

	logger.Info("router decision parse failed",
		"raw_response", "LEAK_CANARY_PAYLOAD secret content",
		"prompt", "very long system prompt with API keys",
		"task_id", "00000000-0000-0000-0000-000000000001",
	)

	out := buf.String()
	if strings.Contains(out, "LEAK_CANARY_PAYLOAD") {
		t.Fatalf("sensitive raw_response value leaked into log: %s", out)
	}
	if strings.Contains(out, "very long system prompt") {
		t.Fatalf("sensitive prompt value leaked into log: %s", out)
	}
	if !strings.Contains(out, "00000000-0000-0000-0000-000000000001") {
		t.Fatalf("non-sensitive task_id MUST NOT be redacted, but missing: %s", out)
	}
	if !strings.Contains(out, "<redacted len=") {
		t.Fatalf("redact marker missing: %s", out)
	}
}

// TestHandler_CaseInsensitive — sensitive-имена матчатся case-insensitive.
func TestHandler_CaseInsensitive(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&buf, nil)))
	logger.Info("test",
		"Prompt", "secret-from-camelcase",
		"API_KEY", "should-be-redacted-too",
	)
	out := buf.String()
	if strings.Contains(out, "secret-from-camelcase") {
		t.Fatalf("Prompt key (mixed case) not redacted: %s", out)
	}
	if strings.Contains(out, "should-be-redacted-too") {
		t.Fatalf("API_KEY (uppercase) not redacted: %s", out)
	}
}

// TestHandler_WithAttrs — атрибуты, добавленные через With(), также маскируются.
func TestHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(NewHandler(slog.NewJSONHandler(&buf, nil)))
	logger := base.With("system_prompt", "you are an AI agent with secret rules")
	logger.Info("step started")
	out := buf.String()
	if strings.Contains(out, "secret rules") {
		t.Fatalf("system_prompt via With() not redacted: %s", out)
	}
}

// TestSafeRawAttr — длина и хэш сохраняются, само содержимое не утекает.
func TestSafeRawAttr(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&buf, nil)))
	raw := []byte("LEAK_CANARY_PAYLOAD: this is a sensitive LLM response we cannot leak")

	// SafeRawAttr возвращает slog.Attr с зашитым key="raw", передаётся БЕЗ преcedeing key.
	logger.Error("router parse failed",
		"error", "json: invalid character",
		SafeRawAttr(raw),
	)

	out := buf.String()
	if strings.Contains(out, "LEAK_CANARY_PAYLOAD") {
		t.Fatalf("raw payload leaked despite SafeRawAttr: %s", out)
	}
	// Длина должна присутствовать.
	if !strings.Contains(out, `"len":`) {
		t.Fatalf("SafeRawAttr should expose `len`, got: %s", out)
	}
	if !strings.Contains(out, `"head_sha256_8":`) {
		t.Fatalf("SafeRawAttr should expose hash, got: %s", out)
	}
}

// TestHandler_NonSensitivePassThrough — обычные ключи логируются как есть.
func TestHandler_NonSensitivePassThrough(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&buf, nil)))
	logger.Info("step",
		"task_id", "abc-123",
		"step_no", 5,
		"agent", "planner",
	)
	out := buf.String()
	for _, expected := range []string{"abc-123", `"step_no":5`, "planner"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected %q in output, got: %s", expected, out)
		}
	}
}

// TestHandler_Enabled — делегирует уровню inner handler'а.
func TestHandler_Enabled(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	h := NewHandler(inner)
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("Info should be disabled when inner level is Warn")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("Error should be enabled when inner level is Warn")
	}
}
