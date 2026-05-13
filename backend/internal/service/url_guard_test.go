package service

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sprint 15.C5 regression — SSRF guard отвергает private/loopback/metadata hosts
// для non-local kind'ов и принимает их для ollama.

func TestValidateBaseURLForProvider_RejectsLoopbackForOpenRouter(t *testing.T) {
	err := validateBaseURLForProvider(context.Background(),
		"http://localhost:9999/v1", models.LLMProviderKindOpenRouter)
	assert.True(t, errors.Is(err, ErrInsecureBaseURL), "got: %v", err)
}

func TestValidateBaseURLForProvider_RejectsAWSMetadata(t *testing.T) {
	err := validateBaseURLForProvider(context.Background(),
		"http://169.254.169.254/latest/meta-data/", models.LLMProviderKindOpenRouter)
	assert.True(t, errors.Is(err, ErrInsecureBaseURL), "got: %v", err)
}

func TestValidateBaseURLForProvider_RejectsRFC1918(t *testing.T) {
	err := validateBaseURLForProvider(context.Background(),
		"http://10.0.0.5/v1", models.LLMProviderKindDeepSeek)
	assert.True(t, errors.Is(err, ErrInsecureBaseURL), "got: %v", err)
}

func TestValidateBaseURLForProvider_RejectsHTTPSchemeForOpenAI(t *testing.T) {
	err := validateBaseURLForProvider(context.Background(),
		"http://example.com/v1", models.LLMProviderKindOpenAI)
	assert.True(t, errors.Is(err, ErrInsecureBaseURL), "got: %v", err)
}

func TestValidateBaseURLForProvider_AllowsHTTPSPublic(t *testing.T) {
	// example.com резолвится в публичный IP — guard должен пропустить.
	err := validateBaseURLForProvider(context.Background(),
		"https://example.com/v1", models.LLMProviderKindOpenRouter)
	assert.NoError(t, err)
}

func TestValidateBaseURLForProvider_AllowsLocalhostForOllama(t *testing.T) {
	// Ollama легально на localhost.
	err := validateBaseURLForProvider(context.Background(),
		"http://localhost:11434/v1", models.LLMProviderKindOllama)
	assert.NoError(t, err)
}

func TestValidateBaseURLForProvider_EmptyBaseURL_Allowed(t *testing.T) {
	err := validateBaseURLForProvider(context.Background(),
		"", models.LLMProviderKindOpenRouter)
	assert.NoError(t, err)
}

// Sprint 15.minor — IPv4-mapped IPv6 (::ffff:127.0.0.1) тоже должен резолвиться в disallowed.
func TestIsDisallowedIP_IPv4MappedIPv6_Loopback(t *testing.T) {
	ip := net.ParseIP("::ffff:127.0.0.1")
	require.NotNil(t, ip, "must parse IPv4-mapped IPv6")
	assert.True(t, isDisallowedIP(ip), "::ffff:127.0.0.1 must be classified as disallowed")
}

func TestIsDisallowedIP_IPv4MappedIPv6_RFC1918(t *testing.T) {
	ip := net.ParseIP("::ffff:10.0.0.1")
	require.NotNil(t, ip)
	assert.True(t, isDisallowedIP(ip))
}

// Sprint 15.Major1 — regression: 30x редирект на cloud metadata IP должен ловиться
// CheckRedirect ДАЖЕ при allowLoopback=true (metadata всегда блок).
// Без этого теста удаление CheckRedirect из newSSRFSafeHTTPClient прошло бы незаметно.
func TestSSRFSafeHTTPClient_BlocksRedirectToCloudMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "http://169.254.169.254/latest/meta-data/iam/")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	// allowLoopback=true (kind=ollama-like): первый hop (loopback httptest) пройдёт.
	// Но метадата (169.254.169.254) ВСЕГДА блокируется → CheckRedirect ловит.
	client := newSSRFSafeHTTPClient(true, 2*time.Second)
	resp, err := client.Get(srv.URL)
	if resp != nil {
		resp.Body.Close()
	}
	require.Error(t, err, "redirect to cloud metadata must be blocked by CheckRedirect")
	assert.Contains(t, err.Error(), "always-blocked",
		"CheckRedirect must report always-blocked ip; got: %v", err)
}

// Sprint 15.Major1 — redirect на http:// (downgrade-attack) отбрасывается validateRedirectURL.
func TestSSRFSafeHTTPClient_BlocksRedirectToInsecureScheme(t *testing.T) {
	target, _ := url.Parse("http://example.com/legit")
	err := validateRedirectURL(target, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheme")
}
