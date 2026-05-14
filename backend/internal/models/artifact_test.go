package models

import (
	"strings"
	"testing"
)

// TestValidateArtifactSummary_Ascii — базовые границы для ASCII.
func TestValidateArtifactSummary_Ascii(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty", "", false},
		{"whitespace only", "   \t\n  ", false},
		{"single char", "x", true},
		{"max length ASCII", strings.Repeat("a", 500), true},
		{"over limit ASCII", strings.Repeat("a", 501), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ValidateArtifactSummary(c.input); got != c.want {
				t.Errorf("ValidateArtifactSummary(%q) = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

// TestValidateArtifactSummary_Cyrillic — кириллица занимает 2 байта на символ
// в UTF-8, но Postgres VARCHAR(500) считает символы. Go-валидатор обязан тоже
// считать символы (utf8.RuneCountInString), иначе кириллический valid-summary
// в 500 символов (1000 байт) ошибочно отвергался бы.
func TestValidateArtifactSummary_Cyrillic(t *testing.T) {
	// 500 кириллических символов — должно пройти (БД примет VARCHAR(500)).
	s500 := strings.Repeat("а", 500)
	if len(s500) <= 500 {
		t.Fatalf("test prerequisite: 500 cyrillic chars should be > 500 bytes, got len=%d", len(s500))
	}
	if !ValidateArtifactSummary(s500) {
		t.Errorf("500 cyrillic chars should pass, but rejected (len=%d bytes)", len(s500))
	}

	// 501 кириллический — должно быть отвергнуто.
	s501 := strings.Repeat("а", 501)
	if ValidateArtifactSummary(s501) {
		t.Errorf("501 cyrillic chars should be rejected, but passed")
	}
}

// TestValidateArtifactSummary_Emoji — эмодзи могут занимать 4 байта; rune-счёт
// должен корректно считать одну руну (а не байты или кластеры).
func TestValidateArtifactSummary_Emoji(t *testing.T) {
	// 500 эмодзи "🔥" (4 байта UTF-8 каждый, 1 руна) — должно пройти.
	s500 := strings.Repeat("🔥", 500)
	if !ValidateArtifactSummary(s500) {
		t.Errorf("500 emoji should pass (byte len=%d), but rejected", len(s500))
	}

	s501 := strings.Repeat("🔥", 501)
	if ValidateArtifactSummary(s501) {
		t.Errorf("501 emoji should be rejected, but passed")
	}
}
