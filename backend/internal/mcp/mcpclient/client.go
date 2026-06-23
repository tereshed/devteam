// Package mcpclient — клиент внешних MCP-серверов для in-process петли ассистента.
//
// Зачем отдельно от internal/mcp (который хостит НАШИ инструменты как MCP-сервер):
// ассистент проекта — это Go-петля agentloop поверх llm.Client (OpenRouter/Gemini),
// а не Claude Code CLI в sandbox. Нативного MCP-коннектора у этих провайдеров нет
// (параметр mcp_servers есть только у прямого Anthropic API), поэтому MCP-host-ом
// выступает само приложение: подключаемся к серверу, читаем tools/list и отдаём
// инструменты в LLM как обычные function-tools, а вызовы роутим обратно в CallTool.
//
// По решению (безопасность мультитенантного бэкенда): ТОЛЬКО удалённые транспорты
// (http/sse). stdio не поддерживается — он запускал бы произвольную команду внутри
// backend-процесса (RCE). Это инвариант пакета, продублированный на слое конфигурации.
package mcpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Transport — транспорт подключения к удалённому MCP-серверу.
type Transport string

const (
	// TransportHTTP — Streamable HTTP (рекомендуемый современный транспорт MCP).
	TransportHTTP Transport = "http"
	// TransportSSE — legacy Server-Sent Events транспорт.
	TransportSSE Transport = "sse"
)

// clientImpl — идентификатор нашего клиента в MCP initialize-хендшейке.
var clientImpl = &mcp.Implementation{Name: "devteam-assistant", Version: "1.0.0"}

// toolNamePrefix — префикс пространства имён MCP-инструментов в каталоге ассистента,
// чтобы они не сталкивались с хардкод-инструментами (project_*, task_* и т.п.).
const toolNamePrefix = "mcp__"

// ServerConfig — РАЗРЕШЁННАЯ конфигурация одного удалённого MCP-сервера: секреты в
// Headers уже подставлены (никаких ${secret:...} плейсхолдеров на этом слое).
type ServerConfig struct {
	// Name — логическое имя сервера; попадает в namespace инструментов
	// (mcp__<name>__<tool>). Санитизируется до [a-z0-9_].
	Name string
	// Transport — http | sse (remote-only по дизайну).
	Transport Transport
	// URL — endpoint удалённого сервера.
	URL string
	// Headers — заголовки запроса (например Authorization). Уже с резолвленными
	// секретами. Прокидываются на каждый HTTP-запрос транспорта.
	Headers map[string]string
}

// ToolDescriptor — инструмент, обнаруженный на сервере, нормализованный для каталога.
type ToolDescriptor struct {
	// Name — namespaced-имя для LLM (mcp__<server>__<tool>).
	Name string
	// RawName — оригинальное имя инструмента на сервере (для CallTool).
	RawName string
	// Description — описание-подсказка для LLM.
	Description string
	// InputSchema — JSON Schema параметров (как отдал сервер).
	InputSchema json.RawMessage
}

// Session — живое подключение к одному MCP-серверу. Не потокобезопасно для
// конкурентных CallTool на одной сессии сверх гарантий go-sdk; используется
// последовательно из петли агента. Обязателен Close().
type Session struct {
	cfg ServerConfig
	cs  *mcp.ClientSession
}

// headerRoundTripper добавляет статические заголовки к каждому исходящему запросу.
// go-sdk транспорты (Streamable/SSE) не имеют поля Headers — единственная точка
// инъекции авторизации — кастомный http.Client.
type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Клонируем запрос перед мутацией заголовков (контракт http.RoundTripper).
	if len(h.headers) > 0 {
		req = req.Clone(req.Context())
		for k, v := range h.headers {
			req.Header.Set(k, v)
		}
	}
	base := h.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func httpClientFor(cfg ServerConfig) *http.Client {
	return &http.Client{
		// Timeout=0: per-операционные дедлайны задаёт ctx вызывающего. Общий таймаут
		// клиента порвал бы потенциальный long-lived SSE.
		Transport: &headerRoundTripper{base: http.DefaultTransport, headers: cfg.Headers},
	}
}

// buildTransport собирает go-sdk транспорт из конфигурации. Remote-only: всё, кроме
// http/sse, отвергается здесь (defense-in-depth поверх валидации конфиг-слоя).
func buildTransport(cfg ServerConfig) (mcp.Transport, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("mcpclient: empty URL for server %q", cfg.Name)
	}
	hc := httpClientFor(cfg)
	switch cfg.Transport {
	case TransportHTTP:
		// DisableStandaloneSSE: ассистенту не нужны server-initiated уведомления —
		// только request/response. Это избегает персистентного GET-соединения.
		return &mcp.StreamableClientTransport{Endpoint: cfg.URL, HTTPClient: hc, DisableStandaloneSSE: true}, nil
	case TransportSSE:
		return &mcp.SSEClientTransport{Endpoint: cfg.URL, HTTPClient: hc}, nil
	default:
		return nil, fmt.Errorf("mcpclient: unsupported transport %q (remote-only: http|sse)", cfg.Transport)
	}
}

// Open подключается к удалённому MCP-серверу и выполняет initialize-хендшейк.
// ctx ограничивает время подключения. При ошибке возвращает её (вызывающий
// решает — дропнуть сервер из каталога и залогировать, не валя ассистента).
func Open(ctx context.Context, cfg ServerConfig) (*Session, error) {
	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, err
	}
	return connect(ctx, cfg, transport)
}

// connect — тестируемый шов: принимает уже готовый транспорт (в тестах — in-memory).
func connect(ctx context.Context, cfg ServerConfig, transport mcp.Transport) (*Session, error) {
	client := mcp.NewClient(clientImpl, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: connect %q: %w", cfg.Name, err)
	}
	return &Session{cfg: cfg, cs: cs}, nil
}

// ListToolDescriptors читает tools/list и нормализует в дескрипторы каталога.
func (s *Session) ListToolDescriptors(ctx context.Context) ([]ToolDescriptor, error) {
	res, err := s.cs.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: list tools %q: %w", s.cfg.Name, err)
	}
	out := make([]ToolDescriptor, 0, len(res.Tools))
	for _, t := range res.Tools {
		if t == nil || strings.TrimSpace(t.Name) == "" {
			continue
		}
		// От клиента InputSchema приходит как map[string]any — ремаршалим в JSON.
		var schema json.RawMessage
		if t.InputSchema != nil {
			if b, mErr := json.Marshal(t.InputSchema); mErr == nil {
				schema = b
			}
		}
		out = append(out, ToolDescriptor{
			Name:        NamespacedName(s.cfg.Name, t.Name),
			RawName:     t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	return out, nil
}

// Call вызывает инструмент сервера по ОРИГИНАЛЬНОМУ имени (rawName) с JSON-аргументами.
// Возвращает JSON-payload результата для подачи обратно в LLM. IsError на стороне
// сервера превращается в Go-ошибку (вместе с уже извлечённым payload'ом для контекста).
func (s *Session) Call(ctx context.Context, rawName string, args json.RawMessage) (json.RawMessage, error) {
	var arguments any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &arguments); err != nil {
			return nil, fmt.Errorf("mcpclient: invalid arguments for %q: %w", rawName, err)
		}
	}
	res, err := s.cs.CallTool(ctx, &mcp.CallToolParams{Name: rawName, Arguments: arguments})
	if err != nil {
		return nil, fmt.Errorf("mcpclient: call %q: %w", rawName, err)
	}
	payload := flattenResult(res)
	if res.IsError {
		return payload, fmt.Errorf("mcpclient: tool %q returned an error result", rawName)
	}
	return payload, nil
}

// Close закрывает сессию (idempotent-безопасно вызывать после ошибок).
func (s *Session) Close() error {
	if s == nil || s.cs == nil {
		return nil
	}
	return s.cs.Close()
}

// Name возвращает санитизированное имя сервера (для логов).
func (s *Session) Name() string { return sanitizeName(s.cfg.Name) }

// flattenResult сводит CallToolResult к JSON-объекту для LLM: собирает текстовые
// блоки и (если есть) структурированный вывод. Не текстовые блоки игнорируются —
// агент-петля оперирует текстом.
func flattenResult(res *mcp.CallToolResult) json.RawMessage {
	if res == nil {
		return json.RawMessage(`{}`)
	}
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(tc.Text)
		}
	}
	obj := map[string]any{}
	if sb.Len() > 0 {
		obj["text"] = sb.String()
	}
	if res.StructuredContent != nil {
		obj["structured"] = res.StructuredContent
	}
	if len(obj) == 0 {
		// Пустой результат — отдаём явный маркер, чтобы LLM не получил "null".
		obj["text"] = ""
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return json.RawMessage(`{"text":""}`)
	}
	return b
}

var nonNameChars = regexp.MustCompile(`[^a-z0-9_]+`)

// sanitizeName приводит имя к [a-z0-9_] (требование к именам tool'ов у LLM-провайдеров).
func sanitizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonNameChars.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "server"
	}
	return s
}

// NamespacedName строит имя инструмента в каталоге ассистента: mcp__<server>__<tool>.
func NamespacedName(server, tool string) string {
	return toolNamePrefix + sanitizeName(server) + "__" + sanitizeName(tool)
}
