//go:build featuresmoke

// git_server.go — обёртка над Gitea для featuresmoke-тестов git-провайдера.
//
// Gitea поднимается через docker-compose.test.yml (port 3001). Этот хелпер
// даёт high-level API:
//   - CreateUser(t, login)            — создать нового пользователя через admin-API.
//   - CreateRepo(t, owner, name)      — создать пустой репозиторий.
//   - CreateAccessToken(t, user)      — выпустить PAT для тестов.
//   - URL()                            — базовый URL Gitea для backend env.
//
// Все методы потокобезопасны: каждый вызов — независимый HTTP-запрос.
// Сам сервер общий, изоляция через уникальные UUID-имена пользователей/репо.
package fakes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ensureAdminOnce — пакетный singleton: t.Parallel() допускает несколько
// одновременных NewFakeGit, но Gitea — общий внешний ресурс. Без Once все
// гонщики разом сделают POST /user/sign_up; победит первый, остальные
// получат 409 Conflict → panic в ensureAdmin валит весь suite.
var (
	ensureAdminOnce sync.Once
	ensureAdminErr  any // *string или panic-value; для пробрасывания первой ошибки на все горутины
)

const (
	// DefaultGiteaURL — порт, опубликованный docker-compose.test.yml.
	DefaultGiteaURL = "http://localhost:3001"
	// adminUser / adminPass — Gitea install-lock=true даёт нам право
	// заводить пользователей через /api/v1/admin без предварительной
	// регистрации.  Учётка создаётся первым обращением WaitReady'я.
	adminUser     = "ci-admin"
	adminPassword = "ci-admin-password-1234567890" // ≥16 символов для KnownSecretValues.minLen
	adminEmail    = "ci-admin@example.com"
)

// FakeGit — клиент к локальному Gitea.
type FakeGit struct {
	t       *testing.T
	baseURL string
	hc      *http.Client
}

// NewFakeGit конструирует клиента и проверяет, что Gitea доступна.
// URL берётся из FEATURESMOKE_GITEA_URL (по умолчанию DefaultGiteaURL).
// Если Gitea недоступна — t.Skip (среда без docker / compose не поднят).
func NewFakeGit(t *testing.T) *FakeGit {
	t.Helper()
	base := strings.TrimRight(envOr("FEATURESMOKE_GITEA_URL", DefaultGiteaURL), "/")
	g := &FakeGit{
		t:       t,
		baseURL: base,
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
	if err := g.waitReady(60 * time.Second); err != nil {
		t.Skipf("FakeGit: Gitea не отвечает на %s: %v (запусти `make test-features-up` или подними docker-compose.test.yml)", base, err)
	}
	g.ensureAdmin()
	return g
}

// URL возвращает базовый URL Gitea.
func (g *FakeGit) URL() string { return g.baseURL }

// CreateUser создаёт нового пользователя через admin-API.
// Возвращает {login, password, email}. Login и email уникальны (UUID).
func (g *FakeGit) CreateUser(t *testing.T) GiteaUser {
	t.Helper()
	login := "u-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	user := GiteaUser{
		Login:    login,
		Password: login + "-password-pad-1234567890", // ≥16 символов
		Email:    login + "@example.com",
	}
	payload := map[string]any{
		"username":             user.Login,
		"email":                user.Email,
		"password":             user.Password,
		"must_change_password": false,
	}
	g.doAdmin(t, "POST", "/api/v1/admin/users", payload, http.StatusCreated, nil)
	return user
}

// CreateRepo создаёт пустой репозиторий под пользователем.
// `owner` — login, `name` — имя репо (любая уникальная строка).
func (g *FakeGit) CreateRepo(t *testing.T, owner GiteaUser, name string) string {
	t.Helper()
	payload := map[string]any{
		"name":      name,
		"private":   false,
		"auto_init": true,
		"default_branch": "main",
	}
	var resp struct {
		CloneURL string `json:"clone_url"`
	}
	g.doBasic(t, owner.Login, owner.Password, "POST", "/api/v1/user/repos", payload, http.StatusCreated, &resp)
	return resp.CloneURL
}

// CreateAccessToken выпускает PAT для пользователя (basic-auth).
// Scopes — например ["write:repository", "read:user"].
func (g *FakeGit) CreateAccessToken(t *testing.T, owner GiteaUser, name string, scopes []string) string {
	t.Helper()
	payload := map[string]any{
		"name":   name,
		"scopes": scopes,
	}
	var resp struct {
		Sha1 string `json:"sha1"`
	}
	path := fmt.Sprintf("/api/v1/users/%s/tokens", owner.Login)
	g.doBasic(t, owner.Login, owner.Password, "POST", path, payload, http.StatusCreated, &resp)
	if resp.Sha1 == "" {
		t.Fatalf("FakeGit: пустой PAT в ответе на CreateAccessToken")
	}
	return resp.Sha1
}

// GiteaUser — минимальная структура пользователя для тестов.
type GiteaUser struct {
	Login    string
	Password string
	Email    string
}

func (g *FakeGit) waitReady(timeout time.Duration) error {
	// Валидируем URL ОДИН РАЗ до цикла — если он битый, http.NewRequest
	// вернёт nil, и без проверки g.hc.Do(nil) уронит весь suite паникой
	// с невнятным runtime error. Лучше вернуть нормальную ошибку наверх.
	if _, err := http.NewRequest("GET", g.baseURL+"/api/v1/version", nil); err != nil {
		return fmt.Errorf("invalid gitea url %q: %w", g.baseURL, err)
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequest("GET", g.baseURL+"/api/v1/version", nil)
		if err != nil {
			return fmt.Errorf("invalid gitea url %q: %w", g.baseURL, err)
		}
		resp, err := g.hc.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status=%d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return lastErr
}

// ensureAdmin idempotent'но создаёт первого пользователя (он же админ при
// INSTALL_LOCK=true). Гонка между параллельными NewFakeGit гасится sync.Once
// на уровне пакета; первый рег побеждает, все последующие просто видят 200
// на GET /users/<admin>.
//
// Fast-fail (panic): любая ошибка делает остальные тесты бессмысленными
// (CreateUser/CreateRepo → 401/404). panic — потому что NewFakeGit ещё не
// держит *testing.T здесь (он привязывается ниже после waitReady).
//
// Ошибка первого победителя кешируется в `ensureAdminErr`, чтобы все
// последующие вызывающие из других тестов получили тот же panic с
// диагностикой, а не упали тише на 401.
func (g *FakeGit) ensureAdmin() {
	ensureAdminOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				ensureAdminErr = r
				panic(r)
			}
		}()
		g.doEnsureAdmin()
	})
	if ensureAdminErr != nil {
		panic(ensureAdminErr)
	}
}

func (g *FakeGit) doEnsureAdmin() {
	// Пробуем войти — если 200, админ уже есть (мог остаться от прошлого
	// прогона на том же volume).
	req, err := http.NewRequest("GET", g.baseURL+"/api/v1/users/"+adminUser, nil)
	if err != nil {
		panic(fmt.Sprintf("FakeGit: ensureAdmin: build GET request: %v", err))
	}
	req.SetBasicAuth(adminUser, adminPassword)
	if resp, err := g.hc.Do(req); err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return
		}
	}
	// Регистрируем нового пользователя через публичный /user/sign_up.
	body := strings.NewReader(fmt.Sprintf("user_name=%s&email=%s&password=%s&retype=%s",
		adminUser, adminEmail, adminPassword, adminPassword))
	req2, err := http.NewRequest("POST", g.baseURL+"/user/sign_up", body)
	if err != nil {
		panic(fmt.Sprintf("FakeGit: ensureAdmin: build POST request: %v", err))
	}
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp2, err := g.hc.Do(req2)
	if err != nil {
		panic(fmt.Sprintf("FakeGit: ensureAdmin: POST /user/sign_up: %v", err))
	}
	defer resp2.Body.Close()
	// Gitea отдаёт 302 redirect при успешной регистрации, 200 при показе формы
	// с ошибкой. Любой 4xx/5xx — это явный фейл setup'а.
	if resp2.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp2.Body)
		panic(fmt.Sprintf("FakeGit: ensureAdmin: sign_up вернул %d: %s", resp2.StatusCode, string(raw)))
	}
	// Подтверждаем, что админ реально появился: повторный GET /users/<admin>.
	confirm, err := http.NewRequest("GET", g.baseURL+"/api/v1/users/"+adminUser, nil)
	if err != nil {
		panic(fmt.Sprintf("FakeGit: ensureAdmin: build confirm GET: %v", err))
	}
	confirm.SetBasicAuth(adminUser, adminPassword)
	confirmResp, err := g.hc.Do(confirm)
	if err != nil {
		panic(fmt.Sprintf("FakeGit: ensureAdmin: confirm GET: %v", err))
	}
	defer confirmResp.Body.Close()
	if confirmResp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(confirmResp.Body)
		panic(fmt.Sprintf("FakeGit: ensureAdmin: после sign_up админ недоступен (status=%d): %s", confirmResp.StatusCode, string(raw)))
	}
}

func (g *FakeGit) doAdmin(t *testing.T, method, path string, payload any, wantStatus int, out any) {
	t.Helper()
	g.doBasic(t, adminUser, adminPassword, method, path, payload, wantStatus, out)
}

func (g *FakeGit) doBasic(t *testing.T, user, pass, method, path string, payload any, wantStatus int, out any) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("FakeGit: marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, g.baseURL+path, body)
	if err != nil {
		t.Fatalf("FakeGit: new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(user, pass)
	resp, err := g.hc.Do(req)
	if err != nil {
		t.Fatalf("FakeGit: %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("FakeGit: %s %s — статус %d (ожидался %d): %s", method, path, resp.StatusCode, wantStatus, string(raw))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("FakeGit: decode response: %v", err)
		}
	}
}

func envOr(name, def string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return def
}
