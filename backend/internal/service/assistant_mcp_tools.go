package service

import (
	"context"
	"encoding/json"
	"errors"
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
	// assistantMCPConnectAttempts — сколько раз пробуем connect+tools/list одного сервера.
	// Remote SSE через облачные балансировщики разово рвёт long-lived стрим (EOF между
	// initialize и tools/list) — один такой обрыв иначе выкидывал бы сервер на весь прогон.
	assistantMCPConnectAttempts = 3
	// assistantMCPConnectBackoff — базовая пауза между попытками (линейно растёт).
	assistantMCPConnectBackoff = 400 * time.Millisecond
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
		sess, descs, ok := s.connectAndListWithRetry(ctx, rs)
		if !ok {
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

// connectAndListWithRetry подключается к серверу и читает tools/list как ЕДИНУЮ
// операцию с несколькими попытками: после обрыва SSE-стрима сессия мертва, поэтому
// при ошибке tools/list мы закрываем её и переоткрываем заново. При успехе возвращает
// ЖИВУЮ сессию (вызывающий закроет через closeFn) + дескрипторы.
//
// Ретраятся только транзиентные ошибки (EOF/reset/refused — обычно мгновенные).
// На context.DeadlineExceeded (сервер завис на потолке assistantMCPConnectTimeout) или
// отмену внешнего ctx ретрай прекращается — нет смысла множить 15s-таймауты и тормозить
// ответ ассистента на реально недоступном сервере.
func (s *assistantService) connectAndListWithRetry(ctx context.Context, rs ResolvedMCPServer) (*mcpclient.Session, []mcpclient.ToolDescriptor, bool) {
	var lastErr error
	for attempt := 1; attempt <= assistantMCPConnectAttempts; attempt++ {
		sess, descs, err := s.connectAndListOnce(ctx, rs)
		if err == nil {
			if attempt > 1 {
				s.deps.Logger.InfoContext(ctx, "assistant: mcp connect recovered after retry",
					slog.String("server", rs.Config.Name), slog.Int("attempt", attempt))
			}
			return sess, descs, true
		}
		lastErr = err
		// Зависший сервер (таймаут) или отменённый прогон — ретрай бесполезен.
		if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
			break
		}
		if attempt < assistantMCPConnectAttempts {
			select {
			case <-ctx.Done():
				lastErr = ctx.Err()
			case <-time.After(time.Duration(attempt) * assistantMCPConnectBackoff):
				continue
			}
			break
		}
	}
	s.deps.Logger.WarnContext(ctx, "assistant: mcp connect/list failed (skipping server)",
		slog.String("server", rs.Config.Name),
		slog.Int("attempts", assistantMCPConnectAttempts),
		slog.String("error", errString(lastErr)),
	)
	return nil, nil, false
}

// connectAndListOnce — одна попытка connect+tools/list под общим bounded-таймаутом.
// При ошибке tools/list сессия закрывается (она уже непригодна).
func (s *assistantService) connectAndListOnce(ctx context.Context, rs ResolvedMCPServer) (*mcpclient.Session, []mcpclient.ToolDescriptor, error) {
	connectCtx, cancel := context.WithTimeout(ctx, assistantMCPConnectTimeout)
	defer cancel()
	sess, err := mcpclient.Open(connectCtx, rs.Config)
	if err != nil {
		return nil, nil, err
	}
	descs, err := sess.ListToolDescriptors(connectCtx)
	if err != nil {
		_ = sess.Close()
		return nil, nil, err
	}
	return sess, descs, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
