// Package logging — обёртки над slog для безопасного логирования в Orchestration v2.
//
// Главная задача — НЕ дать сырому вводу/выводу LLM (промптам, response'ам, content
// артефактов) утечь в stdout/stderr/file-логи. По плану Sprint 17, такие данные
// хранятся ТОЛЬКО в зашифрованных колонках БД (router_decisions.encrypted_raw_response).
//
// Использование:
//
//	handler := redact.NewHandler(slog.NewJSONHandler(os.Stdout, nil))
//	logger := slog.New(handler)
//	// ... теперь во всех вызовах logger.Info(...) ключи из SensitiveFieldNames
//	// автоматически маскируются как "<redacted len=N>".
//
// CI lint-правило (добавляется в Sprint 5): запрет slog.Default() в файлах
// internal/service/router_*, orchestrator_*, agent_dispatcher.go — все должны
// получать redact-обёрнутый логгер через DI.
package logging

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
)

// SensitiveFieldNames — case-insensitive имена полей, значения которых ВСЕГДА маскируются.
// Если нужно добавить новое имя — добавляй сюда, не размазывай по коду.
var SensitiveFieldNames = map[string]struct{}{
	"raw_response":           {},
	"encrypted_raw_response": {},
	"prompt":                 {},
	"system_prompt":          {},
	"user_prompt":            {},
	"messages":               {},
	"content":                {},
	"output":                 {},
	"response":               {},
	"decision_raw":           {},
	"encrypted_value":        {},
	"ciphertext":             {},
	"secret":                 {},
	"token":                  {},
	"api_key":                {},
	"password":               {},
}

// Handler — slog.Handler, который оборачивает другой Handler и маскирует значения
// атрибутов с именами из SensitiveFieldNames.
type Handler struct {
	inner slog.Handler
}

// NewHandler создаёт обёртку. inner — любой slog.Handler (JSON/Text/Custom).
func NewHandler(inner slog.Handler) *Handler {
	return &Handler{inner: inner}
}

// Enabled делегирует.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// Handle обходит атрибуты Record'а и заменяет sensitive-значения на маркеры.
func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	newRecord := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, newRecord)
}

// WithAttrs возвращает Handler с теми же sensitive-правилами, но добавленными
// (уже отмаскированными) атрибутами.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactAttr(a)
	}
	return &Handler{inner: h.inner.WithAttrs(redacted)}
}

// WithGroup делегирует.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{inner: h.inner.WithGroup(name)}
}

// redactAttr возвращает Attr с замаскированным значением, если ключ — sensitive.
// Иначе возвращает Attr как есть.
func redactAttr(a slog.Attr) slog.Attr {
	key := strings.ToLower(a.Key)
	if _, ok := SensitiveFieldNames[key]; !ok {
		// Не sensitive по верхнему ключу. Группы внутри не разворачиваем —
		// если разработчик кладёт чувствительные данные внутрь group, он должен
		// явно использовать SafeRawAttr / SafeStringAttr.
		return a
	}
	return slog.Attr{Key: a.Key, Value: slog.StringValue(redactedValueString(a.Value))}
}

// redactedValueString возвращает маркер для значения, в зависимости от его типа.
func redactedValueString(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return "<redacted len=" + strconv.Itoa(len(v.String())) + ">"
	case slog.KindGroup:
		return "<redacted group>"
	default:
		return "<redacted>"
	}
}

// SafeRawAttr — единственный санкционированный способ упомянуть сырой LLM-ввод/вывод
// в логах: длина + хэш первых 64 байт (для дедупликации инцидентов). Само содержимое
// НЕ попадает в log-stream.
//
// Используй в parse-error путях:
//
//	if err := json.Unmarshal(raw, &d); err != nil {
//	    logger.Error("router decision parse failed",
//	        "error", err,
//	        "raw", redact.SafeRawAttr(raw))   // <-- безопасно
//	    return err
//	}
func SafeRawAttr(raw []byte) slog.Attr {
	head := raw
	if len(head) > 64 {
		head = head[:64]
	}
	h := sha256.Sum256(head)
	return slog.Group("raw",
		slog.Int("len", len(raw)),
		slog.String("head_sha256_8", hex.EncodeToString(h[:8])),
	)
}
