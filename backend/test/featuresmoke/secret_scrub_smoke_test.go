//go:build featuresmoke

package featuresmoke

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// canaryAPIKey — заведомо «палевный» секрет, который мы кладём в env backend'а
// (через cmd/api) ПЕРЕД bootstrapServer'ом. Если этот canary всплывёт в:
//   - HTTP-ответе от backend на какой-либо ручке,
//   - stdout/stderr backend'а,
//   - HTTP-логах между gateway и stub'ами,
// — значит scrubbing провалился.
//
// Реальный backend env читается из родителя через composeEnv (harness.go), так
// что выставление ANTHROPIC_API_KEY в TestMain'е прилетит и в backend.
const canaryAPIKey = "sk-test-CANARY-DO-NOT-LEAK-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const canaryGithubPAT = "ghp_CANARYaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// TestSecretScrub_NotInAPIErrorResponses — backend не должен возвращать сам
// канареечный ключ даже при ошибках валидации (например, при попытке
// сохранить кривой LLM-credential).
//
// Шаги:
//   1. Создаём пользователя через /auth/register.
//   2. Дергаем `/llm-credentials` (или ближайший эндпоинт, где сервер мог бы
//      отрефлектить переданное значение) с canary-токеном в payload.
//   3. Грепаем ответ — токена не должно быть ни в plain, ни в URL-encoded виде.
//
// На этом этапе harness'а конкретный эндпоинт не имеет значения — мы валидируем
// общий контракт «нигде, никогда». Используем POST /auth/login с canary в
// password: это гарантированно ошибка 401/403, и payload содержит секрет.
func TestSecretScrub_NotInAPIErrorResponses(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)
	t.Logf("provisioned user %s", user.Email)

	// 1) Логин с заведомо неправильным паролем (canary).
	resp := h.Do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email":    user.Email,
		"password": canaryAPIKey,
	}, "")
	if resp.Status >= 200 && resp.Status < 300 {
		t.Fatalf("ожидали ошибку логина с canary-паролем, но получили %d: %s", resp.Status, truncBody(resp.Body))
	}
	assertNoCanary(t, "POST /auth/login (bad password)", string(resp.Body))

	// 2) Передаём canary в Authorization при обращении к защищённой ручке.
	resp2 := h.Do(t, "GET", "/api/v1/users/me", nil, canaryGithubPAT)
	assertNoCanary(t, "GET /users/me (bad token)", string(resp2.Body))

	// 3) В URL — наиболее опасный случай (попадает в access-log с URL-encoded'ом).
	encoded := url.QueryEscape(canaryGithubPAT)
	resp3 := h.Do(t, "GET", "/api/v1/users/me?debug="+encoded, nil, "")
	assertNoCanary(t, "GET /users/me?debug=<canary>", string(resp3.Body))
}

// TestSecretScrub_NotInBackendStdout — после прогонов нескольких сценариев
// читаем лог-файл backend'а (см. harness.bootstrapServer.cmd.Stdout) и
// грепаем на canary. Файл создаётся в os.TempDir, путь логируется при
// падении waitHealth, но здесь мы поднимаем backend всегда успешно, значит
// читаем через зарегистрированный путь.
//
// КРИТИЧНО: тест бежит t.Parallel(), но читает лог одного shared-процесса.
// Это OK: чтение лога — read-only, мы не очищаем файл между прогонами.
func TestSecretScrub_NotInBackendStdout(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	// Прогоняем несколько ручек, передавая canary как Authorization.
	for _, path := range []string{"/api/v1/users/me", "/api/v1/projects", "/api/v1/teams"} {
		_ = h.Do(t, "GET", path, nil, canaryAPIKey)
		_ = h.Do(t, "GET", path, nil, canaryGithubPAT)
	}
	// И один POST с canary в body.
	_ = h.Do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email":    user.Email,
		"password": canaryAPIKey,
	}, "")

	// Лог пишется через harness.bootstrapServer.logFile.
	// Имя файла регистрируется в env BACKEND_LOG_PATH (см. harness, добавлено
	// для этого теста — иначе путь не достать). Если переменная не выставлена,
	// тест дегейтится (graceful skip), чтобы не падать на dev-машинах без full-harness.
	logPath := os.Getenv("FEATURESMOKE_BACKEND_LOG")
	if logPath == "" {
		t.Skip("FEATURESMOKE_BACKEND_LOG не выставлен — пропускаем grep по stdout backend'а")
	}
	// HTTP-ответ возвращается ДО гарантированного flush'а буферов slog/stdout
	// в файл. Без задержки тест может прочитать файл за несколько мс до того,
	// как ОС туда запишет утечку → ложный pass. 100 ms — компромисс между
	// надёжностью и скоростью suite.
	time.Sleep(100 * time.Millisecond)

	// Внимание: грязное чтение — файл активно пишется backend-процессом
	// параллельно. Допустимо для smoke: мы ищем подстроку-канарейку, частично
	// записанная строка не помешает grep'у обнаружить утечку, а если канарейка
	// окажется на границе двух чтений — этот же тест поймает её на следующем
	// прогоне CI (флак в сторону false-negative, не false-positive).
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read backend log %s: %v", logPath, err)
	}
	assertNoCanary(t, "backend stdout/stderr", string(raw))
}

func assertNoCanary(t *testing.T, where, payload string) {
	t.Helper()
	for _, c := range []string{canaryAPIKey, canaryGithubPAT} {
		if strings.Contains(payload, c) {
			t.Fatalf("LEAK в %s: canary %q найден в выводе: %s", where, c, truncBody([]byte(payload)))
		}
		if encoded := url.QueryEscape(c); encoded != c && strings.Contains(payload, encoded) {
			t.Fatalf("LEAK в %s: URL-encoded canary %q найден в выводе: %s", where, encoded, truncBody([]byte(payload)))
		}
	}
}
