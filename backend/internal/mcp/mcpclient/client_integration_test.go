package mcpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestOpen_OverRealHTTP поднимает go-sdk MCP-сервер за httptest.Server (Streamable
// HTTP) и проверяет полный путь коннектора по РЕАЛЬНОМУ HTTP: initialize → ListTools
// → CallTool, плюс что заголовок Authorization доезжает до сервера (header-инъекция
// через headerRoundTripper). Без Docker — httptest in-process.
func TestOpen_OverRealHTTP(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "http-test-server", Version: "1.0.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "Echo the message back"},
		func(_ context.Context, _ *mcp.CallToolRequest, in echoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: in.Message}},
			}, nil, nil
		})

	inner := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)

	var (
		mu      sync.Mutex
		gotAuth string
		seen    bool
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = true
		if a := r.Header.Get("Authorization"); a != "" {
			gotAuth = a
		}
		mu.Unlock()
		inner.ServeHTTP(w, r)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := Open(ctx, ServerConfig{
		Name:      "remote",
		Transport: TransportHTTP,
		URL:       ts.URL,
		Headers:   map[string]string{"Authorization": "Bearer test-token"},
	})
	if err != nil {
		t.Fatalf("Open over http: %v", err)
	}
	defer sess.Close()

	descs, err := sess.ListToolDescriptors(ctx)
	if err != nil {
		t.Fatalf("ListToolDescriptors: %v", err)
	}
	var found bool
	for _, d := range descs {
		if d.Name == "mcp__remote__echo" && d.RawName == "echo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tool mcp__remote__echo, got %+v", descs)
	}

	out, err := sess.Call(ctx, "echo", json.RawMessage(`{"message":"over-the-wire"}`))
	if err != nil {
		t.Fatalf("Call echo: %v", err)
	}
	var res struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, out)
	}
	if res.Text != "over-the-wire" {
		t.Errorf("echo text = %q, want %q", res.Text, "over-the-wire")
	}

	mu.Lock()
	defer mu.Unlock()
	if !seen {
		t.Fatal("server received no HTTP requests")
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("server saw Authorization = %q, want %q (header injection failed)", gotAuth, "Bearer test-token")
	}
}
