//go:build featuresmoke

package featuresmoke

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// auth_smoke_test.go — P0 happy + negative path для /api/v1/auth/*.
//
// Покрытие:
//   - register → login → /auth/me → refresh → logout
//   - дубликат email = 409
//   - login с неверным паролем = 401 (и пароль не утекает в payload)
//   - запрос защищённой ручки без токена = 401
//   - refresh с битым токеном = 401
//   - после logout старый access-token остаётся валиден до естественного истечения
//     (мы валидируем только то, что logout вернул 200 и refresh-токен инвалидирован).

// TestAuth_RegisterLoginMeRefreshLogout — основной happy path авторизации.
func TestAuth_RegisterLoginMeRefreshLogout(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	email := "smoke-auth-" + uuid.NewString() + "@example.com"
	password := "Pass-" + uuid.NewString()

	// 1. Register — ожидаем 201 + пара токенов.
	regResp := h.Do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	if regResp.Status != http.StatusCreated {
		t.Fatalf("register: status=%d body=%s", regResp.Status, truncBody(regResp.Body))
	}
	var regBody struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	regResp.JSON(t, &regBody)
	if regBody.AccessToken == "" || regBody.RefreshToken == "" {
		t.Fatalf("register: пустые токены: %+v", regBody)
	}
	if regBody.TokenType != "Bearer" {
		t.Fatalf("register: token_type=%q, ожидали Bearer", regBody.TokenType)
	}
	if regBody.ExpiresIn <= 0 {
		t.Fatalf("register: expires_in=%d, ожидали > 0", regBody.ExpiresIn)
	}

	// 2. Login — другой набор валидных токенов на том же пользователе.
	loginResp := h.Do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "")
	if loginResp.Status != http.StatusOK {
		t.Fatalf("login: status=%d body=%s", loginResp.Status, truncBody(loginResp.Body))
	}
	var loginBody struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	loginResp.JSON(t, &loginBody)
	if loginBody.AccessToken == "" || loginBody.RefreshToken == "" {
		t.Fatalf("login: пустые токены")
	}

	// 3. /auth/me — статус 200 + email совпадает.
	meResp := h.Do(t, "GET", "/api/v1/auth/me", nil, loginBody.AccessToken)
	if meResp.Status != http.StatusOK {
		t.Fatalf("me: status=%d body=%s", meResp.Status, truncBody(meResp.Body))
	}
	var meBody struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Role          string `json:"role"`
		EmailVerified bool   `json:"email_verified"`
	}
	meResp.JSON(t, &meBody)
	if meBody.Email != email {
		t.Fatalf("me: email=%q ожидали %q", meBody.Email, email)
	}
	if _, err := uuid.Parse(meBody.ID); err != nil {
		t.Fatalf("me: id=%q не валидный UUID: %v", meBody.ID, err)
	}

	// 4. Refresh — возвращает новую пару токенов.
	refreshResp := h.Do(t, "POST", "/api/v1/auth/refresh", map[string]string{
		"refresh_token": loginBody.RefreshToken,
	}, "")
	if refreshResp.Status != http.StatusOK {
		t.Fatalf("refresh: status=%d body=%s", refreshResp.Status, truncBody(refreshResp.Body))
	}
	var refreshBody struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	refreshResp.JSON(t, &refreshBody)
	if refreshBody.AccessToken == "" || refreshBody.RefreshToken == "" {
		t.Fatalf("refresh: пустые токены")
	}

	// 5. Logout — 200.
	logoutResp := h.Do(t, "POST", "/api/v1/auth/logout", nil, refreshBody.AccessToken)
	if logoutResp.Status != http.StatusOK {
		t.Fatalf("logout: status=%d body=%s", logoutResp.Status, truncBody(logoutResp.Body))
	}

	// 6. После logout повторный refresh должен отвалиться 401.
	rrAfter := h.Do(t, "POST", "/api/v1/auth/refresh", map[string]string{
		"refresh_token": refreshBody.RefreshToken,
	}, "")
	if rrAfter.Status != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: status=%d (ожидали 401) body=%s",
			rrAfter.Status, truncBody(rrAfter.Body))
	}
}

// TestAuth_DuplicateEmailReturnsConflict — повторная регистрация = 409.
func TestAuth_DuplicateEmailReturnsConflict(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	dupResp := h.Do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email":    user.Email,
		"password": user.Password,
	}, "")
	if dupResp.Status != http.StatusConflict {
		t.Fatalf("duplicate register: status=%d (ожидали 409) body=%s",
			dupResp.Status, truncBody(dupResp.Body))
	}
}

// TestAuth_LoginBadPasswordReturns401AndScrubs — bad password = 401, и пароль не утекает.
func TestAuth_LoginBadPasswordReturns401AndScrubs(t *testing.T) {
	t.Parallel()
	h := StartServer(t)
	user := h.NewUser(t)

	const wrongPwd = "this-is-CANARY-bad-password-1234567890"
	resp := h.Do(t, "POST", "/api/v1/auth/login", map[string]string{
		"email":    user.Email,
		"password": wrongPwd,
	}, "")
	if resp.Status != http.StatusUnauthorized {
		t.Fatalf("login bad pwd: status=%d (ожидали 401) body=%s",
			resp.Status, truncBody(resp.Body))
	}
	if strings.Contains(string(resp.Body), wrongPwd) {
		t.Fatalf("login bad pwd: пароль из payload утёк в ответ: %s", truncBody(resp.Body))
	}
}

// TestAuth_ProtectedEndpointWithoutTokenReturns401 — без Bearer = 401.
func TestAuth_ProtectedEndpointWithoutTokenReturns401(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	resp := h.Do(t, "GET", "/api/v1/auth/me", nil, "")
	if resp.Status != http.StatusUnauthorized {
		t.Fatalf("me without token: status=%d (ожидали 401)", resp.Status)
	}
}

// TestAuth_RefreshWithGarbageTokenReturns401.
func TestAuth_RefreshWithGarbageTokenReturns401(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	resp := h.Do(t, "POST", "/api/v1/auth/refresh", map[string]string{
		"refresh_token": "not.a.real.jwt",
	}, "")
	if resp.Status != http.StatusUnauthorized {
		t.Fatalf("refresh garbage: status=%d (ожидали 401) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}

// TestAuth_BadEmailFormatReturns400 — валидация email на уровне DTO.
func TestAuth_BadEmailFormatReturns400(t *testing.T) {
	t.Parallel()
	h := StartServer(t)

	resp := h.Do(t, "POST", "/api/v1/auth/register", map[string]string{
		"email":    "not-an-email",
		"password": "Pass-" + uuid.NewString(),
	}, "")
	if resp.Status != http.StatusBadRequest {
		t.Fatalf("register bad email: status=%d (ожидали 400) body=%s",
			resp.Status, truncBody(resp.Body))
	}
}
