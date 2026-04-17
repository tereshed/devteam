package service

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Лимит текста коррекции: защита от OOM и чрезмерных промптов (задача 6.7).
const UserCorrectionMaxBytes = 256 * 1024 // 256 KiB

// correctionCloseTagPattern — закрывающий тег в любом регистре/с пробелами (prompt injection в обёртку).
var correctionCloseTagPattern = regexp.MustCompile(`(?i)</\s*user_correction\s*>`)

var (
	ErrUserCorrectionTooLarge    = errors.New("correction text exceeds maximum allowed size")
	ErrUserCorrectionInvalidUTF8 = errors.New("correction text is not valid UTF-8")
	ErrUserCorrectionEmpty       = errors.New("correction text is empty")
)

func stripCorrectionCloseTags(s string) string {
	return correctionCloseTagPattern.ReplaceAllString(s, "")
}

// ValidateAndSanitizeUserCorrection проверяет размер и UTF-8, убирает опасные для логов символы.
func ValidateAndSanitizeUserCorrection(raw string) (string, error) {
	if len(raw) > UserCorrectionMaxBytes {
		return "", ErrUserCorrectionTooLarge
	}
	if !utf8.ValidString(raw) {
		return "", ErrUserCorrectionInvalidUTF8
	}
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ErrUserCorrectionEmpty
	}
	s = stripCorrectionCloseTags(s)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrUserCorrectionEmpty
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == 0:
			continue
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(r)
		case r < 32:
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = stripCorrectionCloseTags(out)
	out = strings.TrimSpace(out)
	if out == "" {
		return "", ErrUserCorrectionEmpty
	}
	return out, nil
}

// RedactCorrectionForLog однострочная форма для структурных логов.
func RedactCorrectionForLog(s string) string {
	const max = 200
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// FormatCorrectionForPrompt оборачивает текст для подстановки в промпт.
// text уже должен пройти ValidateAndSanitizeUserCorrection; дополнительно вырезаются закрывающие теги.
func FormatCorrectionForPrompt(text string) string {
	safe := stripCorrectionCloseTags(text)
	var buf bytes.Buffer
	buf.WriteString("<user_correction>\n")
	buf.WriteString(safe)
	buf.WriteString("\n</user_correction>")
	return buf.String()
}
