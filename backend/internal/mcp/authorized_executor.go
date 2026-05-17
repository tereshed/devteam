package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
)

// authorized_executor.go — Sprint 21 §3.3.
//
// AuthorizedExecutor — wrapper над сервис-методами (ProjectService, TaskService,
// ConversationService, AgentService), который собирает фиксированный каталог
// инструментов, видимых глобальному ассистенту, и enforce'ит:
//
//   1. Пробрасывает UserID из agentloop.AuthContext в ctx через
//      CtxKeyUserID / CtxKeyUserRole (те же ключи, что использует MCP HTTP
//      middleware) — handler'ы внутри сервисов уже умеют их читать
//      (см. tools_project.go, tools_task.go).
//
//   2. Жёсткий каталог: ассистент видит ТОЛЬКО те tools, что объявлены здесь.
//      Никаких runtime-расширений. Это закрывает кейс «случайно
//      зарегистрировали destructive admin-tool и LLM его вызвал».
//
//   3. Для list-инструментов навязываются limit/cursor (план §3.4 п.2): если
//      LLM передал limit > MaxToolListLimit или вовсе не передал — мы клампим
//      и подкладываем дефолт. Это защита от мегабайтных tool_result.
//
//   4. Маппинг доменных ошибок в `{status:"forbidden"|"not_found"|"error", message}`
//      payload-ы (Sprint 21 §3.3 п.2). Это идёт в LLM, чтобы он мог
//      отреагировать и не считать каждое forbidden фатальным.
//
// AuthorizedExecutor намеренно ДУБЛИРУЕТ контракт MCP tool handler'ов
// (project_list, task_list, agent_list и т.д.) вместо переиспользования
// JSON-RPC слоя. Причины:
//   - Tool-loop ассистента работает в одном Go-процессе, не нужен HTTP-hop;
//   - MCP SDK обёртки возвращают *mcp.CallToolResult — несовместимо с нашим
//     ToolHandler signature (нам нужен `json.RawMessage`);
//   - Дублирование удержано в минимуме: только thin call → marshal.
//
// КОНТРАКТ: ассистент работает ТОЛЬКО от имени RoleUser. Admin-обход (видеть
// чужие проекты) намеренно недоступен — даже если userID соответствует
// admin-аккаунту, мы передаём RoleUser. Это защита: пользователь-админ,
// просящий «удали все проекты», не должен случайно зацепить чужие.

// Лимиты каталога. Magic numbers запрещены — все в одном месте.
const (
	// MaxToolListLimit — жёсткий cap на limit в *_list инструментах. Любой
	// больший lim в args молча клампится. Это второй уровень защиты от
	// переполнения контекста (первый — agentloop.Config.MaxToolResultBytes).
	MaxToolListLimit = 50

	// DefaultToolListLimit — если LLM не указал limit, берём этот.
	DefaultToolListLimit = 20
)

// AuthorizedExecutor строит каталог инструментов для AssistantService.
//
// Зависимости опциональны: nil-сервис → соответствующая группа tools
// просто не попадает в каталог (это позволяет постепенно расширять
// поверхность без блокировки всего сервиса).
type AuthorizedExecutor struct {
	projectSvc service.ProjectService
	taskSvc    service.TaskService
	convSvc    service.ConversationService
	agentSvc   *service.AgentService
}

// AuthorizedExecutorDeps — DI-структура (для удобства main.go).
type AuthorizedExecutorDeps struct {
	ProjectService      service.ProjectService
	TaskService         service.TaskService
	ConversationService service.ConversationService
	AgentService        *service.AgentService
}

// NewAuthorizedExecutor — конструктор.
func NewAuthorizedExecutor(deps AuthorizedExecutorDeps) *AuthorizedExecutor {
	return &AuthorizedExecutor{
		projectSvc: deps.ProjectService,
		taskSvc:    deps.TaskService,
		convSvc:    deps.ConversationService,
		agentSvc:   deps.AgentService,
	}
}

// Catalog возвращает полный список agentloop.Tool, безопасных для
// глобального ассистента. Имена и destructive-флаги — единственный источник
// правды (синхронизирован с docs/tasks/21-assistant-sidebar.md §5.1).
//
// Если какой-то сервис не задан — соответствующие tools пропускаются.
func (e *AuthorizedExecutor) Catalog() []agentloop.Tool {
	tools := make([]agentloop.Tool, 0, 16)

	if e.projectSvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:        "project_list",
				Description: "Список проектов текущего пользователя. Поддерживает фильтры status/git_provider/search и пагинацию (limit/offset).",
				InputSchema: schemaProjectList,
				Handler:     e.projectList,
			},
			agentloop.Tool{
				Name:        "project_get",
				Description: "Получить проект по UUID.",
				InputSchema: schemaProjectGet,
				Handler:     e.projectGet,
			},
			agentloop.Tool{
				Name:                 "project_create",
				Description:          "Создать новый проект. Требует подтверждения пользователя.",
				InputSchema:          schemaProjectCreate,
				RequiresConfirmation: true,
				Handler:              e.projectCreate,
			},
		)
	}

	if e.taskSvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:        "task_list",
				Description: "Список задач проекта. limit/offset обязательны (клампятся к разумным значениям).",
				InputSchema: schemaTaskList,
				Handler:     e.taskList,
			},
			agentloop.Tool{
				Name:        "task_get",
				Description: "Получить задачу по UUID.",
				InputSchema: schemaTaskGet,
				Handler:     e.taskGet,
			},
			agentloop.Tool{
				Name:                 "task_cancel",
				Description:          "Отменить задачу. DESTRUCTIVE — требует подтверждения.",
				InputSchema:          schemaTaskGet, // тот же {task_id}
				RequiresConfirmation: true,
				Handler:              e.taskCancel,
			},
		)
	}

	if e.convSvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:        "conversation_list",
				Description: "Список чатов проекта (проектные чаты, не assistant-сессии).",
				InputSchema: schemaConvList,
				Handler:     e.conversationList,
			},
			agentloop.Tool{
				Name:        "conversation_get",
				Description: "Получить чат по UUID.",
				InputSchema: schemaConvGet,
				Handler:     e.conversationGet,
			},
			agentloop.Tool{
				Name:                 "conversation_create",
				Description:          "Создать новый чат в проекте. Требует подтверждения.",
				InputSchema:          schemaConvCreate,
				RequiresConfirmation: true,
				Handler:              e.conversationCreate,
			},
		)
	}

	if e.agentSvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:        "agent_list",
				Description: "Список агентов реестра. Поддерживает фильтр role и пагинацию.",
				InputSchema: schemaAgentList,
				Handler:     e.agentList,
			},
			agentloop.Tool{
				Name:        "agent_get",
				Description: "Получить агента по UUID.",
				InputSchema: schemaAgentGet,
				Handler:     e.agentGet,
			},
		)
	}

	// Tools без зависимостей — всегда в каталоге.
	tools = append(tools,
		agentloop.Tool{
			Name:        "whoami",
			Description: "Возвращает информацию о текущем пользователе (id, scope).",
			InputSchema: schemaEmpty,
			Handler:     e.whoami,
		},
		agentloop.Tool{
			Name:        "app_navigate",
			Description: "Просит фронт перейти на указанный маршрут (например, '/projects/<uuid>'). Side-effect: WS-событие assistant.navigate.",
			InputSchema: schemaAppNavigate,
			Handler:     e.appNavigate,
		},
	)
	return tools
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers — auth context + error mapping
// ─────────────────────────────────────────────────────────────────────────────

// injectAuth кладёт UserID/Role в ctx по тем же ключам, что использует
// MCP HTTP middleware. Сервисы умеют их читать.
//
// Ассистент работает ТОЛЬКО как RoleUser (см. модуль-doc). Hard-coded.
func injectAuth(ctx context.Context, auth agentloop.AuthContext) (context.Context, uuid.UUID, error) {
	uid, err := uuid.Parse(auth.UserID)
	if err != nil {
		return ctx, uuid.Nil, fmt.Errorf("authorized executor: invalid user id %q: %w", auth.UserID, err)
	}
	ctx = context.WithValue(ctx, CtxKeyUserID, uid)
	ctx = context.WithValue(ctx, CtxKeyUserRole, models.RoleUser)
	return ctx, uid, nil
}

// parseArgs декодирует args, разрешая пустой payload (= zero-value параметры).
// DisallowUnknownFields — защита от LLM-галлюцинаций «лишних» параметров,
// чтобы handler не молча игнорировал намерение модели.
func parseArgs[T any](args json.RawMessage, out *T) error {
	if len(args) == 0 || string(args) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

// marshalResult упаковывает ok-результат `{status:"ok", data}`. Унифицируем
// форму чтобы LLM мог парсить однородно.
func marshalResult(data any) (json.RawMessage, error) {
	return json.Marshal(struct {
		Status string `json:"status"`
		Data   any    `json:"data,omitempty"`
	}{Status: "ok", Data: data})
}

// businessErr возвращает payload `{status, message}` для бизнес-ошибки,
// которая должна попасть в LLM (не прерывать петлю). Сетевые/ctx ошибки —
// возвращаем как Go-error.
func businessErr(status, message string) (json.RawMessage, error) {
	b, _ := json.Marshal(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{status, message})
	return b, nil
}

// mapServiceErr классифицирует ошибки сервисного слоя:
//   - ErrProjectForbidden / ErrTaskForbidden и т.п. → forbidden
//   - ErrProjectNotFound / ErrTaskNotFound и т.п. → not_found
//   - ErrInvalidInput / ErrAgentValidation → validation
//   - всё остальное → error
// Возвращает payload + nil (ошибка идёт в LLM, не в loop-failure).
func mapServiceErr(err error) (json.RawMessage, error) {
	switch {
	case errors.Is(err, service.ErrProjectForbidden):
		return businessErr("forbidden", "доступ к проекту запрещён")
	case errors.Is(err, service.ErrProjectNotFound):
		return businessErr("not_found", "проект не найден")
	case errors.Is(err, service.ErrTaskNotFound):
		return businessErr("not_found", "задача не найдена")
	case errors.Is(err, service.ErrProjectNameExists):
		return businessErr("validation", "проект с таким именем уже существует")
	case errors.Is(err, repository.ErrInvalidInput):
		return businessErr("validation", "некорректные аргументы")
	default:
		// Generic — но НЕ просачиваем raw err.Error() (может содержать SQL/secrets).
		return businessErr("error", "внутренняя ошибка при выполнении инструмента")
	}
}

// clampListLimit нормализует limit с учётом MaxToolListLimit.
func clampListLimit(limit int) int {
	if limit <= 0 {
		return DefaultToolListLimit
	}
	if limit > MaxToolListLimit {
		return MaxToolListLimit
	}
	return limit
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: project_*
// ─────────────────────────────────────────────────────────────────────────────

type projectListArgs struct {
	Status      *string `json:"status,omitempty"`
	GitProvider *string `json:"git_provider,omitempty"`
	Search      *string `json:"search,omitempty"`
	Limit       int     `json:"limit,omitempty"`
	Offset      int     `json:"offset,omitempty"`
}

func (e *AuthorizedExecutor) projectList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a projectListArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	req := dto.ListProjectsRequest{
		Status:      a.Status,
		GitProvider: a.GitProvider,
		Search:      a.Search,
		Limit:       clampListLimit(a.Limit),
		Offset:      maxInt(a.Offset, 0),
	}
	projects, total, err := e.projectSvc.List(ctx, uid, models.RoleUser, req)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{
		"items":  projects,
		"total":  total,
		"limit":  req.Limit,
		"offset": req.Offset,
	})
}

type idArgs struct {
	ID string `json:"id,omitempty"`
	// Альясы для дружелюбия — модель часто пишет project_id вместо id.
	ProjectID string `json:"project_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	AgentID   string `json:"agent_id,omitempty"`
}

func (a idArgs) resolve() (uuid.UUID, error) {
	for _, raw := range []string{a.ID, a.ProjectID, a.TaskID, a.AgentID} {
		if raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return uuid.Nil, fmt.Errorf("invalid uuid %q", raw)
			}
			return id, nil
		}
	}
	return uuid.Nil, errors.New("id/project_id/task_id/agent_id is required")
}

func (e *AuthorizedExecutor) projectGet(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a idArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	pid, err := a.resolve()
	if err != nil {
		return businessErr("validation", err.Error())
	}
	p, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(p)
}

func (e *AuthorizedExecutor) projectCreate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var req dto.CreateProjectRequest
	if err := parseArgs(args, &req); err != nil {
		return businessErr("validation", err.Error())
	}
	if req.Name == "" {
		return businessErr("validation", "name is required")
	}
	p, err := e.projectSvc.Create(ctx, uid, req)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(p)
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: task_*
// ─────────────────────────────────────────────────────────────────────────────

type taskListArgs struct {
	ProjectID string `json:"project_id"`
	State     string `json:"state,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

func (e *AuthorizedExecutor) taskList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a taskListArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	req := dto.ListTasksRequest{
		Limit:  clampListLimit(a.Limit),
		Offset: maxInt(a.Offset, 0),
	}
	if a.State != "" {
		req.Status = &a.State
	}
	tasks, total, err := e.taskSvc.List(ctx, uid, models.RoleUser, pid, req)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{
		"items":  tasks,
		"total":  total,
		"limit":  req.Limit,
		"offset": req.Offset,
	})
}

func (e *AuthorizedExecutor) taskGet(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a idArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	tid, err := a.resolve()
	if err != nil {
		return businessErr("validation", err.Error())
	}
	t, err := e.taskSvc.GetByID(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(t)
}

func (e *AuthorizedExecutor) taskCancel(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a idArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	tid, err := a.resolve()
	if err != nil {
		return businessErr("validation", err.Error())
	}
	t, err := e.taskSvc.Cancel(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: conversation_*
// ─────────────────────────────────────────────────────────────────────────────

type convListArgs struct {
	ProjectID string `json:"project_id"`
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

func (e *AuthorizedExecutor) conversationList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a convListArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	limit := clampListLimit(a.Limit)
	offset := maxInt(a.Offset, 0)
	convs, total, err := e.convSvc.ListConversations(ctx, uid, pid, limit, offset)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{
		"items":  convs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (e *AuthorizedExecutor) conversationGet(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a idArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	cid, err := a.resolve()
	if err != nil {
		return businessErr("validation", err.Error())
	}
	c, err := e.convSvc.GetConversation(ctx, uid, cid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(c)
}

type convCreateArgs struct {
	ProjectID string `json:"project_id"`
	Title     string `json:"title,omitempty"`
}

func (e *AuthorizedExecutor) conversationCreate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a convCreateArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	c, err := e.convSvc.CreateConversation(ctx, uid, pid, a.Title)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(c)
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: agent_* (read-only catalog — никаких mutations)
// ─────────────────────────────────────────────────────────────────────────────

type agentListArgs struct {
	Role   string `json:"role,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

func (e *AuthorizedExecutor) agentList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	// Agent registry — глобальный, не фильтруется по userID (это шаблоны
	// агентов для всех команд). Но мы оставляем UserID в ctx для аудита.
	ctx, _, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a agentListArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	filter := repository.AgentFilter{
		Limit:  clampListLimit(a.Limit),
		Offset: maxInt(a.Offset, 0),
	}
	if a.Role != "" {
		r := models.AgentRole(a.Role)
		filter.Role = &r
	}
	agents, total, err := e.agentSvc.List(ctx, filter)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{
		"items":  agents,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

func (e *AuthorizedExecutor) agentGet(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, _, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a idArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	id, err := a.resolve()
	if err != nil {
		return businessErr("validation", err.Error())
	}
	ag, err := e.agentSvc.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, service.ErrAgentNotInRegistry) {
			return businessErr("not_found", "агент не найден")
		}
		return mapServiceErr(err)
	}
	return marshalResult(ag)
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: meta tools
// ─────────────────────────────────────────────────────────────────────────────

func (e *AuthorizedExecutor) whoami(_ context.Context, auth agentloop.AuthContext, _ json.RawMessage) (json.RawMessage, error) {
	return marshalResult(map[string]any{
		"user_id": auth.UserID,
		"scope":   auth.Scope,
	})
}

type navigateArgs struct {
	Route string `json:"route"`
}

// AppNavigatePayload — публичная структура, AssistantService использует её для
// эмиссии assistant.navigate WS-события после успешного вызова tool'а.
// Возвращается через tool_result data, чтобы Assistant мог достать route
// из payload и отправить юзеру.
type AppNavigatePayload struct {
	Route string `json:"route"`
}

func (e *AuthorizedExecutor) appNavigate(_ context.Context, _ agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	var a navigateArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if a.Route == "" {
		return businessErr("validation", "route is required")
	}
	// Сама эмиссия WS — забота AssistantService (он держит Hub). Tool лишь
	// возвращает payload, который Assistant прочитает в OnToolResult-хуке.
	return marshalResult(AppNavigatePayload{Route: a.Route})
}

// ─────────────────────────────────────────────────────────────────────────────
// JSON-schemas (inline; короткие — magic JSON допустим)
// ─────────────────────────────────────────────────────────────────────────────

var (
	schemaEmpty         = json.RawMessage(`{"type":"object","properties":{}}`)
	schemaProjectList   = json.RawMessage(`{"type":"object","properties":{"status":{"type":"string"},"git_provider":{"type":"string"},"search":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	schemaProjectGet    = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"project_id":{"type":"string","format":"uuid"}},"oneOf":[{"required":["id"]},{"required":["project_id"]}]}`)
	schemaProjectCreate = json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"description":{"type":"string"},"git_provider":{"type":"string"},"git_url":{"type":"string"},"git_default_branch":{"type":"string"}}}`)
	schemaTaskList      = json.RawMessage(`{"type":"object","required":["project_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"state":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	schemaTaskGet       = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"task_id":{"type":"string","format":"uuid"}}}`)
	schemaConvList      = json.RawMessage(`{"type":"object","required":["project_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	schemaConvGet       = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"conversation_id":{"type":"string","format":"uuid"}}}`)
	schemaConvCreate    = json.RawMessage(`{"type":"object","required":["project_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"title":{"type":"string"}}}`)
	schemaAgentList     = json.RawMessage(`{"type":"object","properties":{"role":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	schemaAgentGet      = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"agent_id":{"type":"string","format":"uuid"}}}`)
	schemaAppNavigate   = json.RawMessage(`{"type":"object","required":["route"],"properties":{"route":{"type":"string","description":"go_router path, например '/projects/<uuid>'"}}}`)
)

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
