package service

import (
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// secret_scrub.go — переносится из удалённого result_processor.go (Sprint 17 cleanup).
// Используется:
//   - orchestrator_context_builder.go для маскирования секретов в промптах.
//   - Sprint 4: AgentWorker.saveArtifact для scrubbing TestResult.RawOutputTruncated
//     перед записью в artifact.content (jsonb, незашифрован).
//
// Это НЕ заменяет основную защиту через internal/logging/redact.go и
// pkg/crypto-шифрование колонок БД — это дополнительный слой scrubbing'а в текстах.

// secretPatterns — скомпилированы один раз при загрузке пакета (MustCompile).
// Каждое выражение покрывает типичный формат секрета в логах: ENV-style, GitHub PAT,
// Bearer-токены, базовые auth-форматы.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|auth[_-]?token|secret|password|passwd|bearer|token)[\s:=]+[^\s,]{8,}`),
	regexp.MustCompile(`(?i)ghp_[a-zA-Z0-9]{36}`),
	regexp.MustCompile(`(?i)(bearer\s+)[a-zA-Z0-9\-._~+/]+=*`),
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)[a-zA-Z0-9\-_]{8,}`),
	regexp.MustCompile(`(?i)(token\s*[:=]\s*)[a-zA-Z0-9\-_]{8,}`),
	regexp.MustCompile(`(?i)(password\s*[:=]\s*)[^\s]+`),
}

// splitRe — пакетный singleton: используется внутри ReplaceAllStringFunc.
// Раньше компилировался на КАЖДЫЙ найденный матч → горячий путь жёг CPU/память
// (особенно на длинных логах с десятками секретов).
var splitRe = regexp.MustCompile(`[\s:=]+`)

// KnownSecretEnvNames — имена переменных окружения, значения которых считаются
// «известными секретами». При prepare'е тестов / запуске сервера эти значения
// собираются в KnownSecretValues и применяются ScrubKnownSecrets для замены
// сырых значений (plain и URL-encoded) на маркер `***`.
//
// Если добавляешь новую переменную — добавь сюда и в CI mask-secrets шаг.
var KnownSecretEnvNames = []string{
	"ANTHROPIC_API_KEY",
	"OPENAI_API_KEY",
	"OPENROUTER_API_KEY",
	"OPENROUTER_KEY",
	"DEEPSEEK_API_KEY",
	"GEMINI_API_KEY",
	"QWEN_API_KEY",
	"LLM_API_KEY",
	"GITHUB_PAT",
	"GITHUB_OAUTH_CLIENT_SECRET",
	"GITLAB_OAUTH_CLIENT_SECRET",
	"CLAUDE_CODE_OAUTH_ACCESS_TOKEN",
	"CLAUDE_CODE_OAUTH_REFRESH_TOKEN",
	"CLAUDE_CODE_OAUTH_CLIENT_SECRET",
	"ENCRYPTION_KEY",
	"JWT_SECRET_KEY",
	"DB_PASSWORD",
	"ADMIN_PASSWORD",
}

// minSecretLen — секреты короче этой длины игнорируются, иначе замена
// поломает тривиальные подстроки в логах (например, dev-значения "test").
// Реальные API-ключи всегда длиннее 16 символов.
const minSecretLen = 16

// ScrubSecrets применяет secretPatterns к строке, заменяя совпадения на маркер
// "[REDACTED]" с сохранением ключа (если паттерн содержит группу-ключ).
// Public-функция чтобы можно было использовать из других пакетов (например, scrub
// raw_output перед записью в artifact.content).
func ScrubSecrets(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllStringFunc(s, func(match string) string {
			// Сохраняем имя ключа до разделителя; маскируем значение.
			parts := splitRe.Split(match, 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
				return parts[0] + ": [REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return s
}

// KnownSecretValues возвращает уникальный список значений секретов из ENV,
// готовый к подаче в ScrubKnownSecrets (отфильтрован по длине, отсортирован
// по убыванию длины). Результат закеширован.
//
// Безопасно вызывать параллельно. Hot-path использует RLock и стоит несколько
// наносекунд. Если в тестах нужно переинициализировать кеш после mutate'а
// env — вызвать ResetKnownSecretValuesCache().
//
// Почему не sync.Once: sync.Once невозможно безопасно "сбросить" в тестах
// без копирования внутреннего мьютекса (go vet: "assignment copies lock
// value"). Double-checked locking с RWMutex даёт ту же гарантию happens-before
// и при этом честно работает с Reset.
func KnownSecretValues() []string {
	// Fast path — чтение под RLock. nil-check на slice безопасен после RLock.
	knownSecretsMu.RLock()
	cache := knownSecretsCache
	knownSecretsMu.RUnlock()
	if cache != nil {
		return cache
	}
	// Slow path — инициализация под Lock с double-check.
	knownSecretsMu.Lock()
	defer knownSecretsMu.Unlock()
	if knownSecretsCache != nil {
		return knownSecretsCache
	}
	raw := make([]string, 0, len(KnownSecretEnvNames))
	for _, name := range KnownSecretEnvNames {
		raw = append(raw, strings.TrimSpace(os.Getenv(name)))
	}
	knownSecretsCache = PrepareSecretValues(raw)
	return knownSecretsCache
}

// ResetKnownSecretValuesCache сбрасывает кеш. Использовать ТОЛЬКО в тестах —
// в проде ENV не меняется на лету.
func ResetKnownSecretValuesCache() {
	knownSecretsMu.Lock()
	defer knownSecretsMu.Unlock()
	knownSecretsCache = nil
}

var (
	knownSecretsMu    sync.RWMutex
	knownSecretsCache []string
)

// ScrubKnownSecrets заменяет в строке все вхождения секретов из `secrets`
// (а также их URL-encoded / JSON-escaped версии) на маркер `***`.
//
// КРИТИЧНО (hot-path): функция вызывается на КАЖДЫЙ scrub лог-записи
// или артефакта. Никаких аллокаций / сортировок здесь быть не должно.
//
// Контракт ВЫЗЫВАЮЩЕГО:
//   - `secrets` ОБЯЗАН быть pre-sorted по длине (длинные сначала), иначе
//     короткий префикс затрёт более длинный соседний секрет (см.
//     TestScrubKnownSecrets_LongerFirst).
//   - `secrets` ОБЯЗАН быть pre-filtered: значения короче `minSecretLen`
//     должны быть отброшены (иначе тривиальные "test"/"dev-key" поломают
//     невинные подстроки в логах).
//
// Стандартный путь — пользоваться `KnownSecretValues()`, которая делает и
// фильтрацию, и сортировку, и кеширование (см. PreparedSecretValues). Если
// зовёшь напрямую — пропусти свой slice через `PrepareSecretValues`.
func ScrubKnownSecrets(s string, secrets []string) string {
	if s == "" || len(secrets) == 0 {
		return s
	}
	for _, v := range secrets {
		s = strings.ReplaceAll(s, v, "***")
		if encoded := url.QueryEscape(v); encoded != v {
			s = strings.ReplaceAll(s, encoded, "***")
		}
		// PathEscape — иногда отличается от QueryEscape (например, для '+').
		if pathEncoded := url.PathEscape(v); pathEncoded != v {
			s = strings.ReplaceAll(s, pathEncoded, "***")
		}
		// JSON-escape: структурированные логгеры (slog.JSONHandler, zap.JSON)
		// заворачивают строки в JSON, где `"`, `\n`, `\t`, управляющие символы
		// и не-ASCII могут быть экранированы. Без этой замены секрет с такими
		// символами просочится в JSON-лог.
		if jsonEscaped := jsonEscapeString(v); jsonEscaped != v {
			s = strings.ReplaceAll(s, jsonEscaped, "***")
		}
	}
	return s
}

// PrepareSecretValues фильтрует значения короче minSecretLen и сортирует
// оставшиеся по длине (длинные сначала). Возвращает НОВЫЙ slice — вход не
// мутируется. Используется вызывающим кодом для подготовки `secrets` перед
// многократными вызовами ScrubKnownSecrets.
//
// Контракт: вызывать редко (на старте сервиса / в KnownSecretValues), не
// в hot-path scrub'а.
func PrepareSecretValues(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, v := range in {
		if len(v) < minSecretLen {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// jsonEscapeString возвращает JSON-экранированное представление строки
// без обрамляющих кавычек. Для строки без спецсимволов вернёт её саму
// (вызывающий код пропустит no-op через `if escaped != v`).
//
// json.Marshal не паникует на string, ошибка тут невозможна.
func jsonEscapeString(s string) string {
	raw, err := json.Marshal(s)
	if err != nil || len(raw) < 2 {
		return s
	}
	// json.Marshal обрамляет кавычками — отрезаем.
	return string(raw[1 : len(raw)-1])
}
