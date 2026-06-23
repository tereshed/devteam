package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/mcp/mcpclient"
	"github.com/devteam/backend/internal/models"
)

const (
	// assistantMCPConnectTimeout — потолок на connect (initialize) и tools/list одного сервера.
	assistantMCPConnectTimeout = 15 * time.Second
	// assistantMCPCallTimeout — потолок на один CallTool (защита от зависшего сервера в петле).
	assistantMCPCallTimeout = 60 * time.Second
)

// openProjectMCPTools подключается ко всем ВКЛЮЧЁННЫМ MCP-серверам проекта и
// возвращает их инструменты как agentloop.Tool + closeFn для закрытия сессий.
//
// Контракт устойчивости: ошибка резолва/подключения/листинга НЕ валит ассистента —
// проблемный сервер логируется и пропускается, петля работает с остальными.
// Сессии остаются открытыми до closeFn: вызывающий делает `defer closeFn()` вокруг
// Executor.Run, потому что CallTool идёт во время петли. На MCP-сервер == nil или
// user-scoped сессию (project == nil) возвращает (nil, noop).
func (s *assistantService) openProjectMCPTools(ctx context.Context, project *models.Project) ([]agentloop.Tool, func()) {
	noop := func() {}
	if s.deps.MCPServers == nil || project == nil {
		return nil, noop
	}
	resolved, err := s.deps.MCPServers.ResolveEnabledConfigs(ctx, project)
	if err != nil {
		s.deps.Logger.WarnContext(ctx, "assistant: resolve mcp configs failed (mcp tools disabled this run)",
			slog.String("project_id", project.ID.String()),
			slog.String("error", err.Error()),
		)
		return nil, noop
	}
	if len(resolved) == 0 {
		return nil, noop
	}

	var (
		tools    []agentloop.Tool
		sessions []*mcpclient.Session
		seen     = map[string]bool{}
	)
	for _, rs := range resolved {
		sess, ok := s.openOneMCPServer(ctx, rs)
		if !ok {
			continue
		}
		descs, err := func() ([]mcpclient.ToolDescriptor, error) {
			listCtx, cancel := context.WithTimeout(ctx, assistantMCPConnectTimeout)
			defer cancel()
			return sess.ListToolDescriptors(listCtx)
		}()
		if err != nil {
			s.deps.Logger.WarnContext(ctx, "assistant: mcp list tools failed (skipping server)",
				slog.String("server", rs.Config.Name),
				slog.String("error", err.Error()),
			)
			_ = sess.Close()
			continue
		}
		sessions = append(sessions, sess)
		added := 0
		for _, d := range descs {
			// Дедуп по namespaced-имени: дублирующиеся имена сломали бы валидацию
			// каталога executor'ом (она требует уникальности) → весь луп упал бы.
			if seen[d.Name] {
				s.deps.Logger.WarnContext(ctx, "assistant: duplicate mcp tool name skipped",
					slog.String("tool", d.Name), slog.String("server", rs.Config.Name))
				continue
			}
			seen[d.Name] = true
			tools = append(tools, mcpToolToAgentloop(sess, d, rs.RequireConfirmation))
			added++
		}
		s.deps.Logger.InfoContext(ctx, "assistant: mcp server connected",
			slog.String("server", rs.Config.Name), slog.Int("tools", added))
	}

	closeFn := func() {
		for _, sess := range sessions {
			_ = sess.Close()
		}
	}
	return tools, closeFn
}

// openOneMCPServer открывает одну сессию с bounded-таймаутом; (nil,false) при ошибке.
func (s *assistantService) openOneMCPServer(ctx context.Context, rs ResolvedMCPServer) (*mcpclient.Session, bool) {
	connectCtx, cancel := context.WithTimeout(ctx, assistantMCPConnectTimeout)
	defer cancel()
	sess, err := mcpclient.Open(connectCtx, rs.Config)
	if err != nil {
		s.deps.Logger.WarnContext(ctx, "assistant: mcp connect failed (skipping server)",
			slog.String("server", rs.Config.Name),
			slog.String("error", err.Error()),
		)
		return nil, false
	}
	return sess, true
}

// mcpToolToAgentloop оборачивает дескриптор MCP-инструмента в agentloop.Tool: Handler
// роутит вызов в session.Call по ОРИГИНАЛЬНОМУ имени с per-call таймаутом. auth не
// нужен (MCP-сервер авторизуется собственными заголовками, уже подставленными).
func mcpToolToAgentloop(sess *mcpclient.Session, d mcpclient.ToolDescriptor, requireConfirm bool) agentloop.Tool {
	schema := d.InputSchema
	if len(schema) == 0 {
		// Function-tools у провайдеров ожидают объектную схему; пустую заменяем минимальной.
		schema = json.RawMessage(`{"type":"object"}`)
	}
	rawName := d.RawName
	return agentloop.Tool{
		Name:                 d.Name,
		Description:          d.Description,
		InputSchema:          schema,
		RequiresConfirmation: requireConfirm,
		Handler: func(ctx context.Context, _ agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
			callCtx, cancel := context.WithTimeout(ctx, assistantMCPCallTimeout)
			defer cancel()
			return sess.Call(callCtx, rawName, args)
		},
	}
}
