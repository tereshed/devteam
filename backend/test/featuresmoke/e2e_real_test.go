//go:build featuresmoke && e2ereal

// Package featuresmoke — TestE2EReal_MixedAgentsPipeline.
//
// Go-замена для scripts/e2e_smoke.sh (см. docs/integration-tests-plan.md Task 5.2).
// Гоняется в `feature-e2e-real.yml` (nightly + on-demand) под реальные ключи
// LLM и GitHub PAT. Цель — поймать регрессии в нашей интеграции с внешними API,
// которые мок-режим (PR-gate) не ловит.
//
// Что покрывается за один прогон:
//   1) Полный pipeline orchestrator → planner → developer → reviewer → tester,
//      где каждый агент сконфигурирован по-разному (Sprint 14.7 + 15.e2e + 16):
//        orchestrator → LLM (anthropic, claude-haiku)
//        planner      → LLM (anthropic, claude-haiku)
//        developer    → sandbox claude-code, provider_kind=anthropic_oauth
//        reviewer     → sandbox claude-code, provider_kind=deepseek
//        tester       → sandbox hermes,      provider_kind=openrouter
//   2) Открытие реального PR на `tereshed/kt-test-repo`.
//   3) Защита от утечки секретов — grep по stdout/stderr backend'а.
//
// Build tag — `featuresmoke && e2ereal`. Тест НЕ подхватывается обычным
// `make test-features-real` (только tag `featuresmoke`); запускается через
// отдельный таргет `make test-features-e2e-real`.
//
// Все вызовы exec.Command в этом файле, которые передают пользовательский ввод
// (BRANCH, EMAIL), идут через явные []string-аргументы — никакого shell-escape
// и никаких git-команд без `--` разделителя (см. docs/integration-tests-plan.md
// «Security»). Сетевые запросы к GitHub API идут через http.Client + url.Parse,
// не curl + строка, поэтому injection невозможен в принципе.

package featuresmoke

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// e2eRealEnvKeys — секреты, которые нужны e2e_real (в дополнение к
// ENCRYPTION_KEY/JWT_SECRET_KEY, которые проверяет harness через cmd/api).
// Список умышленно дублирует e2e_smoke.sh require_env'ы — если меняется
// контракт CI, меняется и здесь.
var e2eRealEnvKeys = []string{
	"GITHUB_PAT",
	"CLAUDE_CODE_OAUTH_ACCESS_TOKEN",
	"DEEPSEEK_API_KEY",
	"OPENROUTER_API_KEY",
	"ENCRYPTION_KEY",
}

// gateE2EReal — единый набор предусловий. Если что-то не выставлено, тест
// делает t.Skip с понятным сообщением (а не падает в полупути). На локалке без
// ключей этот тест всегда тихо пропускается; в nightly CI обязан запуститься.
func gateE2EReal(t *testing.T) {
	t.Helper()
	if CurrentMode() != ModeReal {
		t.Skip("e2e_real: требует FEATURESMOKE_MODE=real (запускается через `make test-features-e2e-real`)")
	}
	for _, k := range e2eRealEnvKeys {
		if strings.TrimSpace(os.Getenv(k)) == "" {
			t.Skipf("e2e_real: env %s не выставлен — пропуск (запускай через nightly CI с реальными ключами)", k)
		}
	}
}

// runSeed запускает один из `cmd/seed_*` бинарей с заданным env. Бинари —
// authoritative источник того, как форматируются AAD и шифрование секретов; мы
// НЕ дублируем их логику здесь, иначе расхождения с production-кодом приведут
// к маскировке багов.
//
// Stdout/stderr стримим в t.Log (через CombinedOutput → форматируем). Без
// этого «seed упал, не понятно почему» — стандартный CI-кошмар.
func runSeed(t *testing.T, cmdSubdir string, env map[string]string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/"+cmdSubdir)
	cmd.Dir = backendRoot()
	// Базовый env родителя + overrides. envOr внутри cmd/seed_* читает
	// конкретные ключи; «лишний» env не вредит.
	merged := append([]string(nil), os.Environ()...)
	for k, v := range env {
		merged = append(merged, k+"="+v)
	}
	cmd.Env = merged

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("seed %s failed: %v\noutput:\n%s", cmdSubdir, err, string(out))
	}
	t.Logf("seed %s OK", cmdSubdir)
}

// pollTaskStatus ждёт, пока task перейдёт в терминальное состояние или
// истечёт timeout. Возвращает последний статус.
func pollTaskStatus(t *testing.T, h *Harness, token, taskID string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var status string
	for time.Now().Before(deadline) {
		resp := h.Do(t, "GET", "/api/v1/tasks/"+taskID, nil, token)
		if resp.Status != http.StatusOK {
			t.Fatalf("poll task: GET status=%d body=%s", resp.Status, truncBody(resp.Body))
		}
		var task struct {
			Status string `json:"status"`
		}
		resp.JSON(t, &task)
		status = task.Status
		switch status {
		case "completed", "failed", "cancelled":
			return status
		}
		// 5 секунд — тот же интервал, что в bash-скрипте. Реальный pipeline
		// идёт минутами, чаще пулить нет смысла.
		time.Sleep(5 * time.Second)
	}
	return status
}

// readTeamID достаёт team_id для проекта напрямую из БД (auto-create при создании
// проекта). Публичной /teams?project_id=... ручки нет, а нам нужен team_id, чтобы
// привязать seed-агентов.
func readTeamID(t *testing.T, projectID string) string {
	t.Helper()
	db := directDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var teamID string
	if err := db.QueryRowContext(ctx,
		`SELECT id::text FROM teams WHERE project_id = $1::uuid LIMIT 1`,
		projectID,
	).Scan(&teamID); err != nil {
		t.Fatalf("readTeamID: %v", err)
	}
	if teamID == "" {
		t.Fatalf("readTeamID: пустой team_id для проекта %s", projectID)
	}
	return teamID
}

// readUserID — то же для users.id по email. Эндпоинт /auth/me даёт id в payload,
// но мы уже сохранили его в User.ID; используем его. Тем не менее этот хелпер
// нужен как fallback для случаев, когда регистрация выдала id асинхронно.
func readUserID(t *testing.T, h *Harness, token string) string {
	t.Helper()
	resp := h.Do(t, "GET", "/api/v1/auth/me", nil, token)
	if resp.Status != http.StatusOK {
		t.Fatalf("readUserID: status=%d body=%s", resp.Status, truncBody(resp.Body))
	}
	var me struct {
		ID string `json:"id"`
	}
	resp.JSON(t, &me)
	if me.ID == "" {
		t.Fatalf("readUserID: пустой id в /auth/me: %s", truncBody(resp.Body))
	}
	return me.ID
}

// seedAgentsForTeam вставляет 5 агентов в team напрямую через SQL. Публичный
// REST API для команды агентов «через POST с code_backend/provider_kind» в
// один заход отсутствует (это исторически делалось seed-миграциями); поэтому
// дублируем точную SQL-логику e2e_smoke.sh.
//
// Возвращает orchestrator.id — он нужен как assigned_agent_id для POST /tasks.
func seedAgentsForTeam(t *testing.T, teamID string) string {
	t.Helper()
	db := directDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hermesModel := envOr("HERMES_MODEL", "anthropic/claude-3.5-haiku")

	// Параметризованный INSERT — никакого SQL-injection даже теоретически.
	// Идентификаторы команды и моделей подставляем как $-параметры.
	// `is_active=true, requires_code_context=false, skills='[]', settings='{}'`
	//
	// Phase 5 review: orchestrator + planner переехали на OpenRouter+v4-flash
	// для скорости pipeline (см. orchestrator.yaml / planner.yaml). YAML model
	// перетирает DB.Model для LLM-executor агентов (см. resolveInputModel
	// orchestrator_context_builder.go:472), поэтому здесь главное — выставить
	// provider_kind="openrouter", чтобы LLM-диспатчер пошёл в правильный backend.
	// Sandbox-агенты (developer/reviewer/tester) — без изменений, покрытие
	// auth-resolver матрицы сохранено.
	rows := [][]any{
		{"orchestrator", "orchestrator", TestModelOpenRouter, nil, "openrouter"},
		{"planner", "planner", TestModelOpenRouter, nil, "openrouter"},
		{"developer", "developer", TestModelAnthropic, "claude-code", "anthropic_oauth"},
		{"reviewer", "reviewer", TestModelDeepSeek, "claude-code", "deepseek"},
		{"tester", "tester", hermesModel, "hermes", "openrouter"},
	}
	for _, r := range rows {
		_, err := db.ExecContext(ctx, `
			INSERT INTO agents
			    (id, name, role, team_id, model, code_backend, provider_kind,
			     is_active, requires_code_context, skills, settings)
			VALUES
			    (gen_random_uuid(), $1, $2, $3::uuid, $4, $5, $6,
			     true, false, '[]'::jsonb, '{}'::jsonb)`,
			r[0], r[1], teamID, r[2], r[3], r[4],
		)
		if err != nil {
			t.Fatalf("seedAgentsForTeam: INSERT %s: %v", r[0], err)
		}
	}

	// Достаём orchestrator.id — он нужен для POST /tasks.assigned_agent_id.
	var orchestratorID string
	if err := db.QueryRowContext(ctx,
		`SELECT id::text FROM agents WHERE team_id = $1::uuid AND role = 'orchestrator'`,
		teamID,
	).Scan(&orchestratorID); err != nil {
		t.Fatalf("seedAgentsForTeam: select orchestrator: %v", err)
	}
	return orchestratorID
}

// githubPRInfo — минимальный shape для /pulls + /pulls/N/files.
type githubPRItem struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

type githubPRFile struct {
	Filename string `json:"filename"`
}

// fetchOpenPR ищет открытый PR с head=<owner>:<branch>. Возвращает (number, htmlURL).
// owner — GitHub user/org, у которого находится head ветка. Для tereshed/kt-test-repo
// fork-flow ветка может быть в repo владельца — head формируется как `tereshed:<branch>`.
func fetchOpenPR(t *testing.T, githubPAT, ownerRepo, branch string) (int, string) {
	t.Helper()
	owner, _, ok := strings.Cut(ownerRepo, "/")
	if !ok {
		t.Fatalf("fetchOpenPR: ожидали ownerRepo=`owner/repo`, получили %q", ownerRepo)
	}
	q := url.Values{}
	q.Set("state", "open")
	q.Set("head", owner+":"+branch)
	q.Set("per_page", "1")
	u := "https://api.github.com/repos/" + ownerRepo + "/pulls?" + q.Encode()

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		t.Fatalf("fetchOpenPR: build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+githubPAT)
	req.Header.Set("Accept", "application/vnd.github+json")
	hc := &http.Client{Timeout: 15 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("fetchOpenPR: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetchOpenPR: GitHub returned %d", resp.StatusCode)
	}
	var items []githubPRItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("fetchOpenPR: decode: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("fetchOpenPR: открытых PR с head=%s:%s не найдено", owner, branch)
	}
	return items[0].Number, items[0].HTMLURL
}

func fetchPRFiles(t *testing.T, githubPAT, ownerRepo string, prNum int) []string {
	t.Helper()
	u := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d/files", ownerRepo, prNum)
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		t.Fatalf("fetchPRFiles: build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+githubPAT)
	req.Header.Set("Accept", "application/vnd.github+json")
	hc := &http.Client{Timeout: 15 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		t.Fatalf("fetchPRFiles: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fetchPRFiles: GitHub returned %d", resp.StatusCode)
	}
	var files []githubPRFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		t.Fatalf("fetchPRFiles: decode: %v", err)
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Filename)
	}
	return out
}

// closePRBestEffort закрывает PR (state=closed) через PATCH /pulls/:n.
// best-effort: ошибки логируются, не валят тест. 404 — нормально, PR мог
// быть уже закрыт ручкой или предыдущим cleanup'ом.
func closePRBestEffort(t *testing.T, githubPAT, ownerRepo string, prNum int) {
	t.Helper()
	u := fmt.Sprintf("https://api.github.com/repos/%s/pulls/%d", ownerRepo, prNum)
	body := strings.NewReader(`{"state":"closed"}`)
	req, err := http.NewRequest("PATCH", u, body)
	if err != nil {
		t.Logf("cleanup: closePR #%d: build request: %v", prNum, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+githubPAT)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	hc := &http.Client{Timeout: 15 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		t.Logf("cleanup: closePR #%d: %v", prNum, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Logf("cleanup: closePR #%d returned %d", prNum, resp.StatusCode)
		return
	}
	t.Logf("cleanup: closed PR #%d", prNum)
}

// deleteBranchBestEffort удаляет git-ref ветки через DELETE /git/refs/heads/...
// 422 — ветка не существует или уже удалена, это OK. branch уже находится в
// контролируемом нами имени (UUID), command-injection невозможен в принципе —
// мы шлём это в JSON-API, а не в exec.Command.
func deleteBranchBestEffort(t *testing.T, githubPAT, ownerRepo, branch string) {
	t.Helper()
	// branch может содержать `/` (`feature/smoke-mixed-…`) — это валидный git-ref.
	// PathEscape сегмент за сегментом, чтобы не превратить slash в %2F (GitHub
	// API на /git/refs/heads/<segment>/<segment> ждёт неэкранированные `/`).
	parts := strings.Split(branch, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	encodedBranch := strings.Join(parts, "/")
	u := fmt.Sprintf("https://api.github.com/repos/%s/git/refs/heads/%s", ownerRepo, encodedBranch)
	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		t.Logf("cleanup: deleteBranch %s: build request: %v", branch, err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+githubPAT)
	req.Header.Set("Accept", "application/vnd.github+json")
	hc := &http.Client{Timeout: 15 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		t.Logf("cleanup: deleteBranch %s: %v", branch, err)
		return
	}
	defer resp.Body.Close()
	// 204 = удалено, 422 = «not found / already deleted» (GitHub так отвечает
	// для несуществующего ref).
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusUnprocessableEntity {
		t.Logf("cleanup: deleteBranch %s returned %d", branch, resp.StatusCode)
		return
	}
	t.Logf("cleanup: deleted branch %s", branch)
}

// assertNoSecretInBackendLog грепает stdout/stderr backend'а на наличие
// конкретных значений секретов. Если в логе нашли хоть один символ
// длинной ≥16 — fail. Дубликат логики из bash-скрипта (assert_no_leak),
// но запускается из Go и поэтому видит ВЕСЬ лог (а не только `since`).
//
// КРИТИЧНО: проверяем ТРИ формы значения:
//  1. raw (как в env);
//  2. url.QueryEscape (пробел → `+`) — это Go-style query-encoding, его
//     эмитит наш собственный backend, когда токен прилетает в URL;
//  3. url.PathEscape (пробел → `%20`) — это «strict» %-encoding, его эмитит
//     RFC3986-compliant клиент при попадании секрета в path-сегмент.
//
// Маскировка в `scripts/mask-secrets.sh` покрывает обе encoding-формы через
// `urllib.parse.quote_plus` + `urllib.parse.quote`; здесь — то же зеркало,
// чтобы тест не дал ложный pass при leak'е через одну из форм.
func assertNoSecretInBackendLog(t *testing.T, value, name string) {
	t.Helper()
	if value == "" {
		return // нечего проверять
	}
	if len(value) < 16 {
		// Слишком короткое значение → много ложных срабатываний на легитимных
		// подстроках в логах. mask-secrets.sh применяет ту же гарантию.
		return
	}
	logPath := BackendLogPath()
	if logPath == "" {
		t.Fatalf("assertNoSecretInBackendLog: BackendLogPath() пуст — StartServer не был вызван?")
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("assertNoSecretInBackendLog: read %s: %v", logPath, err)
	}
	content := string(raw)

	// Дедуплицируем варианты: для секрета без пробелов QueryEscape == PathEscape;
	// нет смысла два раза грепать ту же строку.
	variants := map[string]string{"raw": value}
	if enc := url.QueryEscape(value); enc != value {
		variants["QueryEscape"] = enc
	}
	if enc := url.PathEscape(value); enc != value {
		variants["PathEscape"] = enc
	}
	for form, v := range variants {
		if strings.Contains(content, v) {
			t.Fatalf("LEAK: секрет %s (%s) найден в backend log (%s). "+
				"Это критичный регресс secret-scrub.",
				name, form, logPath)
		}
	}
}

// TestE2EReal_MixedAgentsPipeline — главный «full pipeline» smoke.
//
// Это НЕ t.Parallel — мы создаём реальный PR на shared-репозитории, и параллельный
// прогон одной и той же ветки приведёт к 422 от GitHub («already exists»).
// Уникальность достигается через uuid в branch_name, но всё равно держим в один поток.
func TestE2EReal_MixedAgentsPipeline(t *testing.T) {
	gateE2EReal(t)

	h := StartServer(t)
	user := h.NewUser(t)
	userID := user.ID
	if userID == "" {
		userID = readUserID(t, h, user.AccessToken)
	}

	// 1) Проект, указывающий на kt-test-repo. git_provider="local" совпадает с
	//    bash-скриптом: реальная аутентификация добавляется через seed_git_credential.
	repoURL := envOr("E2E_REPO_URL", "https://github.com/tereshed/kt-test-repo")
	ownerRepo := strings.TrimPrefix(strings.TrimSuffix(repoURL, ".git"), "https://github.com/")

	pName := "smoke-mixed-" + uuid.NewString()
	createResp := h.Do(t, "POST", "/api/v1/projects", map[string]any{
		"name":         pName,
		"description":  "smoke (mixed agents) — e2e_real",
		"git_provider": "local",
		"git_url":      repoURL,
	}, user.AccessToken)
	if createResp.Status != http.StatusCreated {
		t.Fatalf("create project: status=%d body=%s", createResp.Status, truncBody(createResp.Body))
	}
	var project struct {
		ID string `json:"id"`
	}
	createResp.JSON(t, &project)
	t.Logf("project id: %s", project.ID)

	teamID := readTeamID(t, project.ID)
	t.Logf("team id: %s", teamID)

	// 2) Per-user secrets seed. ENCRYPTION_KEY читаем из родительского env —
	//    backend поднимался с ним же (см. composeEnv в harness.go).
	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	commonEnv := map[string]string{
		"USER_ID":        userID,
		"ENCRYPTION_KEY": encryptionKey,
	}

	// Claude Code OAuth subscription → developer.
	runSeed(t, "seed_claude_code_subscription", mergeEnv(commonEnv, map[string]string{
		"CLAUDE_CODE_OAUTH_ACCESS_TOKEN":  os.Getenv("CLAUDE_CODE_OAUTH_ACCESS_TOKEN"),
		"CLAUDE_CODE_OAUTH_REFRESH_TOKEN": os.Getenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN"),
		"CLAUDE_CODE_OAUTH_EXPIRES_AT":    os.Getenv("CLAUDE_CODE_OAUTH_EXPIRES_AT"),
	}))

	// DeepSeek key → reviewer.
	runSeed(t, "seed_user_llm_credential", mergeEnv(commonEnv, map[string]string{
		"PROVIDER": "deepseek",
		"API_KEY":  os.Getenv("DEEPSEEK_API_KEY"),
	}))

	// OpenRouter key → tester (Hermes).
	runSeed(t, "seed_user_llm_credential", mergeEnv(commonEnv, map[string]string{
		"PROVIDER": "openrouter",
		"API_KEY":  os.Getenv("OPENROUTER_API_KEY"),
	}))

	// 3) Seed 5 agents через прямой SQL (нет публичной ручки для team-bulk).
	orchestratorID := seedAgentsForTeam(t, teamID)
	t.Logf("orchestrator agent id: %s", orchestratorID)

	// 4) Git credential — привязываем PAT к проекту.
	runSeed(t, "seed_git_credential", mergeEnv(commonEnv, map[string]string{
		"PAT":        os.Getenv("GITHUB_PAT"),
		"PROJECT_ID": project.ID,
	}))

	// 5) POST задачи.
	githubPAT := os.Getenv("GITHUB_PAT")
	branch := "feature/smoke-mixed-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]

	// Регистрируем cleanup СРАЗУ — ещё до того, как pipeline начнёт работать
	// с этим branch'ем. Если тест упадёт в любой точке между этой строкой и
	// концом, t.Cleanup всё равно удалит ветку из GitHub. Без этого через
	// месяц nightly tereshed/kt-test-repo завален сотнями осиротевших веток
	// (см. review Phase 5 #2).
	//
	// Cleanup делает best-effort, чтобы не маскировать падение основного теста
	// другой ошибкой (например, GitHub 5xx во время удаления — мы хотим
	// видеть исходный фейл, а не cleanup-noise).
	t.Cleanup(func() {
		deleteBranchBestEffort(t, githubPAT, ownerRepo, branch)
	})

	title := "Smoke[mixed e2e_real]: add " + branch + ".md"
	desc := fmt.Sprintf(`Create file %s.md at the repository root with three lines:
'# Smoke (mixed agents, e2e_real)'
'%s'
'developer=claude-code/oauth | reviewer=claude-code/deepseek | tester=hermes/openrouter'`,
		lastSegment(branch), branch)

	taskResp := h.Do(t, "POST", "/api/v1/projects/"+project.ID+"/tasks", map[string]any{
		"title":             title,
		"description":       desc,
		"assigned_agent_id": orchestratorID,
		"branch_name":       branch,
	}, user.AccessToken)
	if taskResp.Status != http.StatusCreated && taskResp.Status != http.StatusOK {
		t.Fatalf("create task: status=%d body=%s", taskResp.Status, truncBody(taskResp.Body))
	}
	var task struct {
		ID string `json:"id"`
	}
	taskResp.JSON(t, &task)
	t.Logf("task id: %s, branch: %s", task.ID, branch)

	// 6) Polling. Pipeline orchestrator → planner → developer → reviewer → tester
	//    с реальным sandbox-build занимает ~5-10 мин на kt-test-repo; держим 15
	//    минут запаса (та же дефолтная 900s из e2e_smoke.sh).
	timeout := 15 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("E2E_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			timeout = d
		}
	}
	status := pollTaskStatus(t, h, user.AccessToken, task.ID, timeout)
	if status != "completed" {
		t.Fatalf("task didn't reach completed: last=%q (timeout=%s) — pipeline regression: смотри backend logs (%s) и llm_logs",
			status, timeout, BackendLogPath())
	}
	t.Logf("task completed in <= %s", timeout)

	// 7) Verify PR on GitHub.
	prNum, prURL := fetchOpenPR(t, githubPAT, ownerRepo, branch)
	t.Logf("PR opened: #%d %s", prNum, prURL)

	// Регистрируем cleanup PR'а ровно тут, когда мы УЖЕ знаем prNum. Идёт ДО
	// удаления ветки (LIFO: t.Cleanup'ы вызываются в обратном порядке —
	// сначала close PR, потом delete branch). Это правильный порядок для
	// GitHub: при удалении ветки с открытым PR auto-close часто происходит,
	// но явный close оставляет чистый аудит-лог.
	t.Cleanup(func() {
		closePRBestEffort(t, githubPAT, ownerRepo, prNum)
	})

	files := fetchPRFiles(t, githubPAT, ownerRepo, prNum)
	wantFile := lastSegment(branch) + ".md"
	if !containsString(files, wantFile) {
		t.Fatalf("PR #%d does not include %s (files: %v)", prNum, wantFile, files)
	}
	t.Logf("PR includes %s — ok", wantFile)

	// 8) Secret-leak guard. Здесь проверяем РЕАЛЬНЫЕ ключи в логах backend'а.
	//    Это критично — мы только что прогнали полный pipeline с настоящими
	//    DEEPSEEK_API_KEY / OPENROUTER_API_KEY / CLAUDE_CODE_OAUTH_ACCESS_TOKEN
	//    в env, и если SecretScrub где-то пропустит — мы это поймаем здесь.
	for _, name := range []string{
		"DEEPSEEK_API_KEY",
		"OPENROUTER_API_KEY",
		"CLAUDE_CODE_OAUTH_ACCESS_TOKEN",
		"GITHUB_PAT",
		"ANTHROPIC_API_KEY",
		"ENCRYPTION_KEY",
		"JWT_SECRET_KEY",
	} {
		assertNoSecretInBackendLog(t, os.Getenv(name), name)
	}
	t.Logf("secret-scrub guard: clean — no real secrets leaked to backend log")
}

// mergeEnv — мелкий хелпер: возвращает union двух map'ов. Второй map перетирает
// первый, чтобы PROVIDER/API_KEY/PAT не были глобальными для всего теста.
func mergeEnv(base, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// lastSegment — последний компонент пути «a/b/c» → «c». Без зависимости от
// filepath, чтобы под Windows / на CI с разными слешами поведение было
// детерминированным (branch — это всегда «/»-разделённое имя).
func lastSegment(s string) string {
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
