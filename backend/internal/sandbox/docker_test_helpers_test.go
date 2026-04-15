package sandbox

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// testSandboxID — валидный 64-символьный hex sandboxID для тестов.
const testSandboxID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// testFullContainerHex — 64 hex символа (как полный ID от Engine) для RunTask/ValidateSandboxID.
const testFullContainerHex = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func newTestDockerClient(t *testing.T, h http.Handler) *client.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(srv.URL, "http://")
	cli, err := client.NewClientWithOpts(
		client.WithHost("http://"+host),
		client.WithVersion("1.43"),
		client.WithHTTPClient(srv.Client()),
	)
	require.NoError(t, err)
	return cli
}
