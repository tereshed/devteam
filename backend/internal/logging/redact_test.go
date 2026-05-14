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

// TestHandler_RedactsInsideGroup — Sprint 5 review fix #2 (CRITICAL security):
// slog.Group("data", slog.String("raw_response", "leak")) — старый код пропускал
// потому что верхний ключ "data" не sensitive. Новый рекурсивный обход маскирует
// "raw_response" внутри группы.
func TestHandler_RedactsInsideGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&buf, nil)))

	canary := "GROUP_LEAK_CANARY_xyz_must_not_appear"
	logger.Info("dispatch result",
		slog.Group("data",
			slog.String("task_id", "abc-123"),
			slog.String("raw_response", canary),
		),
	)

	out := buf.String()
	if strings.Contains(out, canary) {
		t.Fatalf("CANARY LEAKED via nested slog.Group: %s", out)
	}
	// task_id внутри той же группы должен остаться (не sensitive).
	if !strings.Contains(out, "abc-123") {
		t.Errorf("non-sensitive task_id inside group MUST stay, got: %s", out)
	}
}

// TestHandler_RedactsInsideDeeplyNestedGroup — глубина 3+ уровня тоже маскируется.
func TestHandler_RedactsInsideDeeplyNestedGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&buf, nil)))

	canary := "DEEP_GROUP_CANARY_alpha_beta_gamma"
	logger.Info("nested",
		slog.Group("outer",
			slog.Group("middle",
				slog.Group("inner",
					slog.String("api_key", canary),
				),
			),
		),
	)

	out := buf.String()
	if strings.Contains(out, canary) {
		t.Fatalf("CANARY LEAKED through 3-level nesting: %s", out)
	}
}

// TestHandler_RedactsLogValuer — slog.LogValuer'ы (объекты, которые сами
// решают, что выдать в лог через LogValue()) тоже проходят через redact
// после .Resolve() в redactAttr.
func TestHandler_RedactsLogValuer(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&buf, nil)))

	canary := "LOGVALUER_CANARY_must_be_redacted"
	logger.Info("via valuer",
		"prompt", logValuerString(canary), // ключ sensitive → маскируется после Resolve
	)

	out := buf.String()
	if strings.Contains(out, canary) {
		t.Fatalf("CANARY LEAKED via LogValuer: %s", out)
	}
}

// logValuerString — мини-обёртка для теста LogValuer-резолва.
type logValuerString string

func (s logValuerString) LogValue() slog.Value { return slog.StringValue(string(s)) }

// TestNopLogger_DiscardsButHasRedactWrapper — NopLogger ничего не пишет, но handler
// — наш redact-wrapper, не raw default. Гарантия: даже sensitive ключи проходят
// через маскирование "по дороге к null'у" — это защищает от случайного wrap'а
// NopLogger в другой sink через WithGroup/WithAttrs.
func TestNopLogger_DiscardsButHasRedactWrapper(t *testing.T) {
	logger := NopLogger()
	// Не должно паниковать; ничего не пишется (io.Discard).
	logger.Info("noop", "raw_response", "supposed secret")
	logger.Error("noop", "api_key", "abc")
	// Дополнительная проверка: вытащить inner handler и убедиться что он — наш *Handler.
	if _, ok := logger.Handler().(*Handler); !ok {
		t.Errorf("NopLogger must wrap with redact *Handler, got %T", logger.Handler())
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
