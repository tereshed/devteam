package mcpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoIn struct {
	Message string `json:"message"`
}

// newTestServerSession поднимает in-memory MCP-сервер с двумя инструментами (echo и
// boom) и возвращает клиентский транспорт для connect().
func newTestServerSession(t *testing.T, ctx context.Context) mcp.Transport {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)

	mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "Echo the message back"},
		func(_ context.Context, _ *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: in.Message}},
			}, nil, nil
		})

	mcp.AddTool(srv, &mcp.Tool{Name: "boom", Description: "Always returns an error result"},
		func(_ context.Context, _ *mcp.CallToolRequest, _ echoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "kaboom"}},
			}, nil, nil
		})

	serverT, clientT := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, serverT, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	return clientT
}

func TestSession_ListAndCall(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientT := newTestServerSession(t, ctx)

	sess, err := connect(ctx, ServerConfig{Name: "My Server", Transport: TransportHTTP}, clientT)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer sess.Close()

	// ListTools → namespaced-имена + сохранённый RawName.
	descs, err := sess.ListToolDescriptors(ctx)
	if err != nil {
		t.Fatalf("ListToolDescriptors: %v", err)
	}
	byName := map[string]ToolDescriptor{}
	for _, d := range descs {
		byName[d.Name] = d
	}
	echo, ok := byName["mcp__my_server__echo"]
	if !ok {
		t.Fatalf("expected tool mcp__my_server__echo, got %v", keys(byName))
	}
	if echo.RawName != "echo" {
		t.Errorf("RawName = %q, want echo", echo.RawName)
	}
	if echo.Description == "" {
		t.Error("expected non-empty description")
	}
	if len(echo.InputSchema) == 0 {
		t.Error("expected non-empty input schema")
	}

	// Call echo → текст из TextContent попадает в payload.
	out, err := sess.Call(ctx, "echo", json.RawMessage(`{"message":"hello mcp"}`))
	if err != nil {
		t.Fatalf("Call echo: %v", err)
	}
	var res struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("unmarshal echo result: %v (%s)", err, out)
	}
	if res.Text != "hello mcp" {
		t.Errorf("echo text = %q, want %q", res.Text, "hello mcp")
	}

	// Call boom → IsError превращается в Go-ошибку, но payload с текстом возвращается.
	payload, err := sess.Call(ctx, "boom", json.RawMessage(`{"message":"x"}`))
	if err == nil {
		t.Fatal("expected error from IsError result")
	}
	if !strings.Contains(string(payload), "kaboom") {
		t.Errorf("expected error payload to carry server text, got %s", payload)
	}
}

func TestBuildTransport_RemoteOnly(t *testing.T) {
	cases := []struct {
		name    string
		cfg     ServerConfig
		wantErr bool
	}{
		{"http ok", ServerConfig{Name: "s", Transport: TransportHTTP, URL: "https://x/mcp"}, false},
		{"sse ok", ServerConfig{Name: "s", Transport: TransportSSE, URL: "https://x/sse"}, false},
		{"empty url", ServerConfig{Name: "s", Transport: TransportHTTP, URL: ""}, true},
		{"stdio rejected", ServerConfig{Name: "s", Transport: Transport("stdio"), URL: "echo"}, true},
		{"unknown rejected", ServerConfig{Name: "s", Transport: Transport("ws"), URL: "wss://x"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := buildTransport(c.cfg)
			if (err != nil) != c.wantErr {
				t.Errorf("buildTransport err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestNamespacedName(t *testing.T) {
	cases := map[[2]string]string{
		{"My Server", "echo"}:        "mcp__my_server__echo",
		{"GitHub-MCP", "create_pr"}:  "mcp__github_mcp__create_pr",
		{"  spaced  ", "Tool Name"}:  "mcp__spaced__tool_name",
		{"", ""}:                     "mcp__server__server",
	}
	for in, want := range cases {
		if got := NamespacedName(in[0], in[1]); got != want {
			t.Errorf("NamespacedName(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestHeaderRoundTripper_InjectsHeaders(t *testing.T) {
	captured := http_HeaderCapture{}
	rt := &headerRoundTripper{base: &captured, headers: map[string]string{"Authorization": "Bearer tok", "X-Extra": "1"}}
	req := mustNewRequest(t)
	_, _ = rt.RoundTrip(req)
	if captured.req.Header.Get("Authorization") != "Bearer tok" {
		t.Errorf("Authorization not injected: %q", captured.req.Header.Get("Authorization"))
	}
	if captured.req.Header.Get("X-Extra") != "1" {
		t.Errorf("X-Extra not injected: %q", captured.req.Header.Get("X-Extra"))
	}
	// Исходный запрос не должен мутироваться (RoundTripper клонирует).
	if req.Header.Get("Authorization") != "" {
		t.Error("original request was mutated")
	}
}

func keys(m map[string]ToolDescriptor) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// http_HeaderCapture — RoundTripper-заглушка, сохраняющая последний запрос.
type http_HeaderCapture struct{ req *http.Request }

func (c *http_HeaderCapture) RoundTrip(req *http.Request) (*http.Response, error) {
	c.req = req
	// Возвращаем ошибку — тело ответа не нужно, проверяем только заголовки запроса.
	return nil, context.Canceled
}

func mustNewRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://example.com/mcp", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return req
}
