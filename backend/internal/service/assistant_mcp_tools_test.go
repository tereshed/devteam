package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/datatypes"
)

type mcpEchoIn struct {
	Message string `json:"message"`
}

// newHTTPMCPServer поднимает go-sdk MCP-сервер (tool echo) за httptest.Server.
func newHTTPMCPServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "svc-test", Version: "1.0.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "echo back"},
		func(_ context.Context, _ *mcp.CallToolRequest, in mcpEchoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: in.Message}},
			}, nil, nil
		})
	handler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv }, nil)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

func newAssistantSvcWithMCP(repo *fakeAssistantMCPRepo) *assistantService {
	return &assistantService{
		deps: AssistantServiceDeps{
			MCPServers: NewAssistantMCPServerService(repo, nil),
			Logger:     discardLogger(),
		},
	}
}

// TestOpenProjectMCPTools_OverHTTP — капстоун: полный путь склейки петли против
// реального HTTP MCP-сервера. ResolveEnabledConfigs → mcpclient.Open → ListTools →
// agentloop.Tool → Handler → CallTool. Проверяет namespaced-имя, флаг подтверждения
// и что Handler реально вызывает инструмент сервера.
func TestOpenProjectMCPTools_OverHTTP(t *testing.T) {
	ts := newHTTPMCPServer(t)

	repo := &fakeAssistantMCPRepo{enabled: []models.AssistantMCPServer{{
		Name:                "remote",
		Transport:           models.MCPTransportHTTP,
		URL:                 ts.URL,
		Headers:             datatypes.JSON([]byte("{}")),
		RequireConfirmation: true,
		IsEnabled:           true,
	}}}
	svc := newAssistantSvcWithMCP(repo)

	ctx := context.Background()
	tools, closeFn := svc.openProjectMCPTools(ctx, &models.Project{ID: uuid.New()})
	defer closeFn()

	if len(tools) != 1 {
		t.Fatalf("expected 1 mcp tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Name != "mcp__remote__echo" {
		t.Errorf("tool name = %q, want mcp__remote__echo", tool.Name)
	}
	if !tool.RequiresConfirmation {
		t.Error("expected RequiresConfirmation=true from per-server flag")
	}
	if tool.Handler == nil {
		t.Fatal("tool handler is nil")
	}

	out, err := tool.Handler(ctx, agentloop.AuthContext{}, json.RawMessage(`{"message":"loop-wired"}`))
	if err != nil {
		t.Fatalf("handler call: %v", err)
	}
	var res struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("unmarshal handler result: %v (%s)", err, out)
	}
	if res.Text != "loop-wired" {
		t.Errorf("handler result text = %q, want loop-wired", res.Text)
	}
}

// newFlakyHTTPMCPServer оборачивает реальный MCP-сервер так, что первые failFirst
// входящих HTTP-запросов обрываются (соединение закрывается → клиент видит EOF),
// а дальше идёт нормальный MCP. Имитирует разовый дроп SSE/стрима балансировщиком.
func newFlakyHTTPMCPServer(t *testing.T, failFirst int) *httptest.Server {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "svc-flaky", Version: "1.0.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{Name: "echo", Description: "echo back"},
		func(_ context.Context, _ *mcp.CallToolRequest, in mcpEchoIn) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: in.Message}}}, nil, nil
		})
	real := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)

	var mu sync.Mutex
	n := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		cur := n
		mu.Unlock()
		if cur <= failFirst {
			// Рвём соединение на полуслове — клиентский transport получит EOF/reset.
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					_ = conn.Close()
					return
				}
			}
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		real.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

// TestOpenProjectMCPTools_RecoversAfterTransientDrop — ретрай: первый запрос рвётся
// (как разовый EOF на tools/list у облачного балансировщика), вторая попытка проходит
// → сервер НЕ выпадает из каталога, инструмент доступен.
func TestOpenProjectMCPTools_RecoversAfterTransientDrop(t *testing.T) {
	ts := newFlakyHTTPMCPServer(t, 1) // уронить только самый первый HTTP-запрос

	repo := &fakeAssistantMCPRepo{enabled: []models.AssistantMCPServer{{
		Name:      "flaky",
		Transport: models.MCPTransportHTTP,
		URL:       ts.URL,
		Headers:   datatypes.JSON([]byte("{}")),
		IsEnabled: true,
	}}}
	svc := newAssistantSvcWithMCP(repo)

	tools, closeFn := svc.openProjectMCPTools(context.Background(), &models.Project{ID: uuid.New()})
	defer closeFn()

	if len(tools) != 1 {
		t.Fatalf("expected server to recover after transient drop (1 tool), got %d", len(tools))
	}
	if tools[0].Name != "mcp__flaky__echo" {
		t.Errorf("tool name = %q, want mcp__flaky__echo", tools[0].Name)
	}
}

// TestOpenProjectMCPTools_DeadServerSkipped — устойчивость: недоступный сервер не
// валит ассистента, просто выпадает из каталога (ноль инструментов, без паники).
func TestOpenProjectMCPTools_DeadServerSkipped(t *testing.T) {
	repo := &fakeAssistantMCPRepo{enabled: []models.AssistantMCPServer{{
		Name:      "dead",
		Transport: models.MCPTransportHTTP,
		URL:       "http://127.0.0.1:1/mcp", // порт 1 → connection refused
		Headers:   datatypes.JSON([]byte("{}")),
		IsEnabled: true,
	}}}
	svc := newAssistantSvcWithMCP(repo)

	tools, closeFn := svc.openProjectMCPTools(context.Background(), &models.Project{ID: uuid.New()})
	defer closeFn()

	if len(tools) != 0 {
		t.Fatalf("expected dead server skipped (0 tools), got %d", len(tools))
	}
}

// TestOpenProjectMCPTools_NoProject — user-scoped сессия (project == nil) → no-op.
func TestOpenProjectMCPTools_NoProject(t *testing.T) {
	svc := newAssistantSvcWithMCP(&fakeAssistantMCPRepo{})
	tools, closeFn := svc.openProjectMCPTools(context.Background(), nil)
	defer closeFn()
	if tools != nil {
		t.Fatalf("expected nil tools for nil project, got %d", len(tools))
	}
}
