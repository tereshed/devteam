package service

import (
	"net/url"
	"strings"
	"sync"
	"testing"
)

// TestKnownSecretValues_RaceSafe — guard на DCL-паттерн:
// параллельные читатели + редкие Reset'ы не должны вызывать data race
// под `-race`. Если кто-то снова заведёт sync.Once и попробует его
// "пересоздать" в Reset — этот тест упадёт.
func TestKnownSecretValues_RaceSafe(t *testing.T) {
	// Сбрасываем кеш на старте, чтобы первый вызов реально пошёл по slow-path.
	ResetKnownSecretValuesCache()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = KnownSecretValues()
		}()
	}
	// Несколько Reset'ов посередине — имитируем тестовый сценарий с
	// перестановкой env.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ResetKnownSecretValuesCache()
		}()
	}
	wg.Wait()
}

// TestScrubKnownSecrets_PlainAndURLEncoded — главный guard URL-encoding'а.
// До этой доработки ScrubSecrets ловил только raw-форму, и попавший в URL
// токен (например, ?token=ghp_… → %2F-варианты при path-escape) утекал в логи.
func TestScrubKnownSecrets_PlainAndURLEncoded(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		text   string
	}{
		{
			name:   "plain occurrence",
			secret: "ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 36 символов, длина ≥ minSecretLen
			text:   "url=https://github.com/foo?token=ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		{
			name:   "queryescape preserves alnum (no-op)",
			secret: "abcdef1234567890abcdef",
			text:   "Authorization: Bearer abcdef1234567890abcdef",
		},
		{
			name:   "queryescape changes (slash)",
			// '/' → '%2F', значит URL-encoded форма обязательно появится в логах.
			secret: "abcd/efgh/ijkl/mnopqrst",
			text:   "callback=" + url.QueryEscape("abcd/efgh/ijkl/mnopqrst"),
		},
		{
			name:   "secret embedded twice (plain + encoded)",
			secret: "tok_abc+def/ghi.jkl=mn+xyz==",
			text: "raw=tok_abc+def/ghi.jkl=mn+xyz== encoded=" +
				url.QueryEscape("tok_abc+def/ghi.jkl=mn+xyz=="),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := ScrubKnownSecrets(tc.text, PrepareSecretValues([]string{tc.secret}))
			if strings.Contains(out, tc.secret) {
				t.Fatalf("plain secret leaked: %q", out)
			}
			if encoded := url.QueryEscape(tc.secret); encoded != tc.secret && strings.Contains(out, encoded) {
				t.Fatalf("url-encoded secret leaked: %q", out)
			}
		})
	}
}

// TestPrepareSecretValues_ShortValuesIgnored — фильтрация короче minSecretLen
// делается на этапе подготовки, не в hot-path scrub'а.
func TestPrepareSecretValues_ShortValuesIgnored(t *testing.T) {
	prepared := PrepareSecretValues([]string{"test"})
	if len(prepared) != 0 {
		t.Fatalf("короткие секреты должны быть отфильтрованы PrepareSecretValues: %v", prepared)
	}
	out := ScrubKnownSecrets("password=test", prepared)
	if !strings.Contains(out, "test") {
		t.Fatalf("после фильтрации scrub не должен трогать строку: %q", out)
	}
}

// TestPrepareSecretValues_LongerFirst — сортировка обязательна и делается
// один раз на этапе подготовки. Тест проверяет именно подготовительный шаг.
func TestPrepareSecretValues_LongerFirst(t *testing.T) {
	short := "0123456789abcdef"                // 16 символов
	long := short + "EXTRASUFFIXEXTRA"         // 32 символа, начинается с `short`
	prepared := PrepareSecretValues([]string{short, long})
	if len(prepared) != 2 || prepared[0] != long {
		t.Fatalf("PrepareSecretValues должен поставить длинные первыми: %v", prepared)
	}
	out := ScrubKnownSecrets("key="+long, prepared)
	if strings.Contains(out, short) {
		t.Fatalf("длинный секрет должен заменяться раньше короткого: %q", out)
	}
}

// TestPrepareSecretValues_Dedup — дубликаты в ENV (один и тот же ключ
// дублируется как LLM_API_KEY и OPENAI_API_KEY) не должны попадать в output.
func TestPrepareSecretValues_Dedup(t *testing.T) {
	v := "duplicated-secret-1234567890abcd"
	prepared := PrepareSecretValues([]string{v, v, v})
	if len(prepared) != 1 {
		t.Fatalf("PrepareSecretValues должен дедупить: %v", prepared)
	}
}

// TestScrubKnownSecrets_JSONEscaped — JSON-логгеры экранируют спецсимволы.
// Секрет с кавычкой или \n обязан маскироваться и в JSON-формате.
func TestScrubKnownSecrets_JSONEscaped(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		text   string
	}{
		{
			name:   "newline escape",
			secret: "line1\nline2-padding-1234567890",
			text:   `{"raw":"line1\nline2-padding-1234567890"}`,
		},
		{
			name:   "quote escape",
			secret: `key="abc"-1234567890abcdef`,
			text:   `{"value":"key=\"abc\"-1234567890abcdef"}`,
		},
		{
			name:   "tab and backslash",
			secret: "tok\twith\\back-1234567890",
			text:   `{"x":"tok\twith\\back-1234567890"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := ScrubKnownSecrets(tc.text, PrepareSecretValues([]string{tc.secret}))
			if strings.Contains(out, tc.secret) {
				t.Fatalf("plain secret leaked: %q", out)
			}
			esc := jsonEscapeString(tc.secret)
			if esc != tc.secret && strings.Contains(out, esc) {
				t.Fatalf("json-escaped secret leaked: in=%q out=%q escaped=%q", tc.text, out, esc)
			}
		})
	}
}

// TestScrubKnownSecrets_Idempotent — повторный вызов на уже зачищенной строке
// не должен что-то трогать (особенно `***` не должен лишний раз заменяться).
func TestScrubKnownSecrets_Idempotent(t *testing.T) {
	secret := "supersecret_value_1234567890abc"
	prepared := PrepareSecretValues([]string{secret})
	once := ScrubKnownSecrets("token="+secret, prepared)
	twice := ScrubKnownSecrets(once, prepared)
	if once != twice {
		t.Fatalf("scrub не идемпотентен: %q vs %q", once, twice)
	}
}

// TestScrubSecrets_PatternBased — sanity на pattern-base ScrubSecrets,
// чтобы убедиться, что мы не сломали старое поведение при доработке.
func TestScrubSecrets_PatternBased(t *testing.T) {
	cases := []string{
		"api_key=verylongsecret123",
		"Authorization: Bearer eyJABCDEF1234567890abcdef",
		"password=verylongpassword",
	}
	for _, in := range cases {
		out := ScrubSecrets(in)
		if strings.Contains(out, "verylongsecret") || strings.Contains(out, "eyJABCDEF") || strings.Contains(out, "verylongpassword") {
			t.Fatalf("ScrubSecrets не сработал: in=%q out=%q", in, out)
		}
		if !strings.Contains(out, "REDACTED") {
			t.Fatalf("ScrubSecrets без маркера REDACTED: in=%q out=%q", in, out)
		}
	}
}
