package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

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
	projectSvc        service.ProjectService
	taskSvc           service.TaskService
	convSvc           service.ConversationService
	teamSvc           service.TeamService
	agentSvc          *service.AgentService
	querySvc          *service.OrchestrationQueryService
	gitIntegrationSvc service.GitIntegrationService
	orchestratorSvc   service.TaskOrchestrator
}

// AuthorizedExecutorDeps — DI-структура (для удобства main.go).
type AuthorizedExecutorDeps struct {
	ProjectService        service.ProjectService
	TaskService           service.TaskService
	ConversationService   service.ConversationService
	TeamService           service.TeamService
	AgentService          *service.AgentService
	QueryService          *service.OrchestrationQueryService
	GitIntegrationService service.GitIntegrationService
	OrchestratorService   service.TaskOrchestrator
}

// NewAuthorizedExecutor — конструктор.
func NewAuthorizedExecutor(deps AuthorizedExecutorDeps) *AuthorizedExecutor {
	return &AuthorizedExecutor{
		projectSvc:        deps.ProjectService,
		taskSvc:           deps.TaskService,
		convSvc:           deps.ConversationService,
		teamSvc:           deps.TeamService,
		agentSvc:          deps.AgentService,
		querySvc:          deps.QueryService,
		gitIntegrationSvc: deps.GitIntegrationService,
		orchestratorSvc:   deps.OrchestratorService,
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
			agentloop.Tool{
				Name:                 "project_update",
				Description:          "Обновить настройки проекта. Требует подтверждения.",
				InputSchema:          schemaProjectUpdate,
				RequiresConfirmation: true,
				Handler:              e.projectUpdate,
			},
			agentloop.Tool{
				Name:                 "project_delete",
				Description:          "Удалить проект. DESTRUCTIVE — требует подтверждения.",
				InputSchema:          schemaProjectGet, // тот же {id/project_id}
				RequiresConfirmation: true,
				Handler:              e.projectDelete,
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
			agentloop.Tool{
				Name:                 "task_create",
				Description:          "Создать задачу в проекте. Требует подтверждения.",
				InputSchema:          schemaTaskCreate,
				RequiresConfirmation: true,
				Handler:              e.taskCreate,
			},
			agentloop.Tool{
				Name:                 "task_update",
				Description:          "Обновить задачу. Требует подтверждения.",
				InputSchema:          schemaTaskUpdate,
				RequiresConfirmation: true,
				Handler:              e.taskUpdate,
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
			agentloop.Tool{
				Name:                 "conversation_send_message",
				Description:          "Отправить сообщение в чат проекта. Требует подтверждения.",
				InputSchema:          schemaConvSendMessage,
				RequiresConfirmation: true,
				Handler:              e.conversationSendMessage,
			},
		)
	}

	if e.agentSvc != nil {
		// Sprint 21 §5.1: agent-реестр — глобальный (см. models.Agent: нет user_id /
		// ownership), а AgentService мутации НЕ гейтит ни по user'у, ни по роли.
		// Executor же hard-coded подаёт RoleUser (см. модуль-doc). Поэтому
		// agent_create/update/delete/set_secret/delete_secret не проходят AuthZ-
		// чек-лист и в каталог ассистента НЕ попадают. Управление шаблонами агентов —
		// через REST/UI (admin-only). Здесь оставляем только read.
		tools = append(tools,
			agentloop.Tool{
				Name:        "agent_list",
				Description: "Список агентов реестра (только чтение). Поддерживает фильтр role и пагинацию.",
				InputSchema: schemaAgentList,
				Handler:     e.agentList,
			},
			agentloop.Tool{
				Name:        "agent_get",
				Description: "Получить агента по UUID (только чтение).",
				InputSchema: schemaAgentGet,
				Handler:     e.agentGet,
			},
		)
	}

	if e.querySvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:        "artifact_list",
				Description: "Список артефактов задачи (без содержимого).",
				InputSchema: schemaArtifactList,
				Handler:     e.artifactList,
			},
			agentloop.Tool{
				Name:        "artifact_get",
				Description: "Получить полный артефакт (с содержимым).",
				InputSchema: schemaArtifactGet,
				Handler:     e.artifactGet,
			},
			agentloop.Tool{
				Name:        "router_decision_list",
				Description: "Лог Router-решений по задаче.",
				InputSchema: schemaArtifactList, // тот же task_id
				Handler:     e.routerDecisionList,
			},
			agentloop.Tool{
				Name:        "worktree_list",
				Description: "Список git worktree задачи (debug-view).",
				InputSchema: schemaArtifactList, // тот же task_id
				Handler:     e.worktreeList,
			},
		)
	}

	if e.gitIntegrationSvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:        "list_git_integrations",
				Description: "Возвращает список подключённых git-провайдеров (GitHub / GitLab) для текущего пользователя. Read-only.",
				InputSchema: schemaGitIntegrationList,
				Handler:     e.listGitIntegrations,
			},
			agentloop.Tool{
				Name:        "list_git_repositories",
				Description: "Возвращает список репозиториев подключённого git-провайдера (GitHub или GitLab) для текущего пользователя.",
				InputSchema: schemaGitRepositoryList,
				Handler:     e.listGitRepositories,
			},
			agentloop.Tool{
				Name:                 "create_git_repository",
				Description:          "Создаёт новый репозиторий у подключённого git-провайдера (GitHub или GitLab) для текущего пользователя. Требует подтверждения.",
				InputSchema:          schemaGitRepositoryCreate,
				RequiresConfirmation: true,
				Handler:              e.createGitRepository,
			},
		)
	}

	if e.teamSvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:        "team_get",
				Description: "Получить команду проекта с агентами.",
				InputSchema: schemaTeamGet,
				Handler:     e.teamGet,
			},
			agentloop.Tool{
				Name:                 "team_update",
				Description:          "Обновить команду проекта (название). Требует подтверждения.",
				InputSchema:          schemaTeamUpdate,
				RequiresConfirmation: true,
				Handler:              e.teamUpdate,
			},
			agentloop.Tool{
				Name:                 "team_agent_patch",
				Description:          "Частично обновить настройки агента в команде проекта. Требует подтверждения. Поля: model/clear_model, prompt_id/clear_prompt_id, code_backend/clear_code_backend, is_active, tool_definition_ids.",
				InputSchema:          schemaTeamAgentPatch,
				RequiresConfirmation: true,
				Handler:              e.teamAgentPatch,
			},
			agentloop.Tool{
				Name:        "team_list",
				Description: "Получить список всех команд проекта (как GET /projects/:id/teams).",
				InputSchema: schemaTeamList,
				Handler:     e.teamList,
			},
			agentloop.Tool{
				Name:                 "team_create",
				Description:          "Создать новую команду в проекте. Требует подтверждения. Поля: project_id, name, type.",
				InputSchema:          schemaTeamCreate,
				RequiresConfirmation: true,
				Handler:              e.teamCreate,
			},
			agentloop.Tool{
				Name:                 "team_delete",
				Description:          "Удалить команду из проекта. Требует подтверждения. Поля: project_id, team_id.",
				InputSchema:          schemaTeamDelete,
				RequiresConfirmation: true,
				Handler:              e.teamDelete,
			},
			agentloop.Tool{
				Name:        "team_type_list",
				Description: "Получить список всех доступных типов команд (как GET /team-types).",
				InputSchema: schemaTeamTypeList,
				Handler:     e.teamTypeList,
			},
			agentloop.Tool{
				Name:                 "team_type_create",
				Description:          "Создать новый тип команды. Требует подтверждения. Поля: code, name. Доступно только администраторам.",
				InputSchema:          schemaTeamTypeCreate,
				RequiresConfirmation: true,
				Handler:              e.teamTypeCreate,
			},
			agentloop.Tool{
				Name:                 "team_type_delete",
				Description:          "Удалить тип команды. Требует подтверждения. Поля: code. Доступно только администраторам.",
				InputSchema:          schemaTeamTypeDelete,
				RequiresConfirmation: true,
				Handler:              e.teamTypeDelete,
			},
		)
	}

	if e.teamSvc != nil && e.agentSvc != nil {
		tools = append(tools,
			agentloop.Tool{
				Name:                 "team_agent_create",
				Description:          "Создать нового агента и добавить его в команду проекта. Требует подтверждения.",
				InputSchema:          schemaTeamAgentCreate,
				RequiresConfirmation: true,
				Handler:              e.teamAgentCreate,
			},
			agentloop.Tool{
				Name:                 "team_agent_delete",
				Description:          "Удалить агента из команды проекта. Требует подтверждения.",
				InputSchema:          schemaTeamAgentDelete,
				RequiresConfirmation: true,
				Handler:              e.teamAgentDelete,
			},
		)
	}

	// Tools без зависимостей — всегда в каталоге.
	tools = append(tools,
		agentloop.Tool{
			Name:        "assistant_active_tasks_count",
			Description: "Возвращает количество активных задач пользователя во всех проектах.",
			InputSchema: schemaEmpty,
			Handler:     e.assistantActiveTasksCount,
		},
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
	case errors.Is(err, repository.ErrGitIntegrationNotFound):
		return businessErr("validation", "git-интеграция не найдена (сначала подключите её в настройках)")
	case errors.Is(err, repository.ErrInvalidInput):
		return businessErr("validation", "некорректные аргументы")
	case errors.Is(err, service.ErrTeamNotFound):
		return businessErr("not_found", "команда проекта не найдена")
	case errors.Is(err, service.ErrTeamInvalidName):
		return businessErr("validation", "некорректное имя команды")
	case errors.Is(err, service.ErrTeamAgentNotFound):
		return businessErr("not_found", "агент команды не найден")
	case errors.Is(err, service.ErrTeamAgentInvalidModel),
		errors.Is(err, service.ErrTeamAgentInvalidCodeBackend),
		errors.Is(err, service.ErrTeamAgentInvalidToolBindings):
		return businessErr("validation", err.Error())
	case errors.Is(err, service.ErrTeamAgentConflict):
		return businessErr("error", "конфликт при обновлении агента")
	case errors.Is(err, service.ErrAgentValidation):
		return businessErr("validation", err.Error())
	case errors.Is(err, service.ErrAgentNameAlreadyTaken):
		return businessErr("validation", "агент с таким именем уже существует")
	case errors.Is(err, service.ErrAgentNotInRegistry):
		return businessErr("not_found", "агент не найден")
	case errors.Is(err, service.ErrAgentConcurrentUpdate):
		return businessErr("error", "конфликт параллельного обновления агента")
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
	if auth.ProjectID != "" {
		pid, err := uuid.Parse(auth.ProjectID)
		if err != nil {
			return businessErr("validation", "invalid session project id")
		}
		p, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid)
		if err != nil {
			return mapServiceErr(err)
		}
		return marshalResult(map[string]any{
			"items":  []any{p},
			"total":  1,
			"limit":  1,
			"offset": 0,
		})
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
	ProjectID  string `json:"project_id,omitempty"`
	TaskID     string `json:"task_id,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	ArtifactID string `json:"artifact_id,omitempty"`
}

func (a idArgs) resolve() (uuid.UUID, error) {
	for _, raw := range []string{a.ID, a.ProjectID, a.TaskID, a.AgentID, a.ArtifactID} {
		if raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil {
				return uuid.Nil, fmt.Errorf("invalid uuid %q", raw)
			}
			return id, nil
		}
	}
	return uuid.Nil, errors.New("id/project_id/task_id/agent_id/artifact_id is required")
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
	if auth.ProjectID != "" && auth.ProjectID != pid.String() {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	p, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(p)
}

func (e *AuthorizedExecutor) projectCreate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	if auth.ProjectID != "" {
		return businessErr("forbidden", "создание проектов запрещено в контексте проекта")
	}
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

func (e *AuthorizedExecutor) projectUpdate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a struct {
		ID          string  `json:"id,omitempty"`
		ProjectID   string  `json:"project_id,omitempty"`
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
	}
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	idArg := idArgs{ID: a.ID, ProjectID: a.ProjectID}
	pid, err := idArg.resolve()
	if err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" && auth.ProjectID != pid.String() {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	req := dto.UpdateProjectRequest{
		Name:        a.Name,
		Description: a.Description,
	}
	p, err := e.projectSvc.Update(ctx, uid, models.RoleUser, pid, req)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(p)
}

func (e *AuthorizedExecutor) projectDelete(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
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
	if auth.ProjectID != "" && auth.ProjectID != pid.String() {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	if err := e.projectSvc.Delete(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]string{"status": "deleted"})
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
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
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
	if auth.ProjectID != "" && t.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
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
	t, err := e.taskSvc.GetByID(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	if auth.ProjectID != "" && t.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	t, err = e.taskSvc.Cancel(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(t)
}

type taskCreateArgs struct {
	ProjectID       string  `json:"project_id"`
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Priority        *string `json:"priority,omitempty"`
	AssignedAgentID *string `json:"assigned_agent_id,omitempty"`
	ParentTaskID    *string `json:"parent_task_id,omitempty"`
}

func (e *AuthorizedExecutor) taskCreate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a taskCreateArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}

	createReq := dto.CreateTaskRequest{
		Title: a.Title,
	}
	if a.Description != nil {
		createReq.Description = *a.Description
	}
	if a.Priority != nil {
		createReq.Priority = *a.Priority
	}
	if a.AssignedAgentID != nil && *a.AssignedAgentID != "" {
		agentID, err := uuid.Parse(*a.AssignedAgentID)
		if err != nil {
			return businessErr("validation", fmt.Sprintf("invalid assigned_agent_id: %q", *a.AssignedAgentID))
		}
		createReq.AssignedAgentID = &agentID
	}
	if a.ParentTaskID != nil && *a.ParentTaskID != "" {
		parentID, err := uuid.Parse(*a.ParentTaskID)
		if err != nil {
			return businessErr("validation", fmt.Sprintf("invalid parent_task_id: %q", *a.ParentTaskID))
		}
		createReq.ParentTaskID = &parentID
	}

	task, err := e.taskSvc.Create(ctx, uid, models.RoleUser, pid, createReq)
	if err != nil {
		return mapServiceErr(err)
	}

	if e.orchestratorSvc != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic in background task orchestration (assistant create)", "error", r, "task_id", task.ID)
				}
			}()
			if err := e.orchestratorSvc.EnqueueInitialStep(context.Background(), task.ID); err != nil {
				slog.Error("Background task orchestration failed (assistant create)", "error", err, "task_id", task.ID)
			}
		}()
	}

	return marshalResult(task)
}

type taskUpdateArgs struct {
	TaskID             string  `json:"task_id"`
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	Priority           *string `json:"priority,omitempty"`
	Status             *string `json:"status,omitempty"`
	AssignedAgentID    *string `json:"assigned_agent_id,omitempty"`
	ClearAssignedAgent bool    `json:"clear_assigned_agent,omitempty"`
	BranchName         *string `json:"branch_name,omitempty"`
}

func (e *AuthorizedExecutor) taskUpdate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a taskUpdateArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	tid, err := uuid.Parse(a.TaskID)
	if err != nil {
		return businessErr("validation", "task_id is required (UUID)")
	}

	t, err := e.taskSvc.GetByID(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	if auth.ProjectID != "" && t.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}

	updateReq := dto.UpdateTaskRequest{
		Title:              a.Title,
		Description:        a.Description,
		Priority:           a.Priority,
		Status:             a.Status,
		ClearAssignedAgent: a.ClearAssignedAgent,
		BranchName:         a.BranchName,
	}
	if a.AssignedAgentID != nil && *a.AssignedAgentID != "" {
		agentID, err := uuid.Parse(*a.AssignedAgentID)
		if err != nil {
			return businessErr("validation", fmt.Sprintf("invalid assigned_agent_id: %q", *a.AssignedAgentID))
		}
		updateReq.AssignedAgentID = &agentID
	}

	updatedTask, err := e.taskSvc.Update(ctx, uid, models.RoleUser, tid, updateReq)
	if err != nil {
		return mapServiceErr(err)
	}

	if a.Status != nil && *a.Status == string(models.TaskStateActive) && e.orchestratorSvc != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic in background task orchestration (assistant update)", "error", r, "task_id", updatedTask.ID)
				}
			}()
			if err := e.orchestratorSvc.EnqueueInitialStep(context.Background(), updatedTask.ID); err != nil {
				slog.Error("Background task orchestration failed (assistant update)", "error", err, "task_id", updatedTask.ID)
			}
		}()
	}

	return marshalResult(updatedTask)
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
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
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
	if auth.ProjectID != "" && c.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
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
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
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

type convSendMessageArgs struct {
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
}

func (e *AuthorizedExecutor) conversationSendMessage(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a convSendMessageArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	cid, err := uuid.Parse(a.ConversationID)
	if err != nil {
		return businessErr("validation", "conversation_id is required (UUID)")
	}
	if a.Content == "" {
		return businessErr("validation", "content is required")
	}
	c, err := e.convSvc.GetConversation(ctx, uid, cid)
	if err != nil {
		return mapServiceErr(err)
	}
	if auth.ProjectID != "" && c.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	// Мы используем рандомный client_msg_id, т.к. это разовый вызов от ассистента.
	clientMsgID := uuid.New()
	msg, err := e.convSvc.SendMessage(ctx, uid, cid, a.Content, clientMsgID)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(msg)
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: agent_* (read-only — §5.1)
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

// Sprint 21 §5.1 — agent_create/update/delete/set_secret/delete_secret НЕ попадают
// в catalog глобального ассистента: AgentService мутации не гейтит по user/role,
// а Executor подаёт hard-coded RoleUser. До появления per-agent ownership/admin-
// проверки в AgentService — мутации остаются исключительно за REST/UI.

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: query_* (artifacts, router_decisions, worktrees)
// ─────────────────────────────────────────────────────────────────────────────

type taskIdArgs struct {
	TaskID string `json:"task_id"`
}

func (e *AuthorizedExecutor) artifactList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a taskIdArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	tid, err := uuid.Parse(a.TaskID)
	if err != nil {
		return businessErr("validation", "task_id must be a valid UUID")
	}
	// Проверка доступа к задаче (неявно через taskSvc)
	task, err := e.taskSvc.GetByID(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	if auth.ProjectID != "" && task.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	artifacts, err := e.querySvc.ListArtifacts(ctx, tid, false)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{"items": artifacts})
}

func (e *AuthorizedExecutor) artifactGet(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a idArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	artID, err := a.resolve()
	if err != nil {
		return businessErr("validation", err.Error())
	}
	artifact, err := e.querySvc.GetArtifact(ctx, artID)
	if err != nil {
		return mapServiceErr(err)
	}
	// Проверка доступа к задаче артефакта
	task, err := e.taskSvc.GetByID(ctx, uid, models.RoleUser, artifact.TaskID)
	if err != nil {
		return mapServiceErr(err)
	}
	if auth.ProjectID != "" && task.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	return marshalResult(artifact)
}

func (e *AuthorizedExecutor) routerDecisionList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a taskIdArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	tid, err := uuid.Parse(a.TaskID)
	if err != nil {
		return businessErr("validation", "task_id must be a valid UUID")
	}
	task, err := e.taskSvc.GetByID(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	if auth.ProjectID != "" && task.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	decisions, err := e.querySvc.ListRouterDecisions(ctx, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{"items": decisions})
}

func (e *AuthorizedExecutor) worktreeList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a taskIdArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	tid, err := uuid.Parse(a.TaskID)
	if err != nil {
		return businessErr("validation", "task_id must be a valid UUID")
	}
	task, err := e.taskSvc.GetByID(ctx, uid, models.RoleUser, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	if auth.ProjectID != "" && task.ProjectID.String() != auth.ProjectID {
		return businessErr("forbidden", "доступ к другим проектам запрещён")
	}
	trees, err := e.querySvc.ListWorktrees(ctx, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{"items": trees})
}

// ─────────────────────────────────────────────────────────────────────────────
// Handlers: meta tools
// ─────────────────────────────────────────────────────────────────────────────

func (e *AuthorizedExecutor) assistantActiveTasksCount(ctx context.Context, auth agentloop.AuthContext, _ json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	tasks, err := e.taskSvc.ListActiveByUser(ctx, uid, []models.TaskState{models.TaskStateActive}, activeTasksCountLimit)
	if err != nil {
		return mapServiceErr(err)
	}
	count := 0
	if auth.ProjectID != "" {
		for _, t := range tasks {
			if t.ProjectID.String() == auth.ProjectID {
				count++
			}
		}
	} else {
		count = len(tasks)
	}
	return marshalResult(map[string]any{"count": count})
}

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
// Handlers: git_*
// ─────────────────────────────────────────────────────────────────────────────

func (e *AuthorizedExecutor) listGitIntegrations(ctx context.Context, auth agentloop.AuthContext, _ json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	items, err := e.gitIntegrationSvc.ListStatuses(ctx, uid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{"integrations": items})
}

type listGitRepositoriesArgs struct {
	Provider string `json:"provider"`
}

func (e *AuthorizedExecutor) listGitRepositories(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a listGitRepositoriesArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if a.Provider == "" {
		return businessErr("validation", "provider is required")
	}
	provider := models.GitIntegrationProvider(a.Provider)
	if provider != models.GitIntegrationProviderGitHub && provider != models.GitIntegrationProviderGitLab {
		return businessErr("validation", "invalid provider, must be 'github' or 'gitlab'")
	}
	repos, err := e.gitIntegrationSvc.ListRepositories(ctx, uid, provider)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{"repositories": repos})
}

type createGitRepositoryArgs struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Private     bool   `json:"private"`
	Description string `json:"description,omitempty"`
}

func (e *AuthorizedExecutor) createGitRepository(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a createGitRepositoryArgs
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if a.Provider == "" || a.Name == "" {
		return businessErr("validation", "provider and name are required")
	}
	provider := models.GitIntegrationProvider(a.Provider)
	if provider != models.GitIntegrationProviderGitHub && provider != models.GitIntegrationProviderGitLab {
		return businessErr("validation", "invalid provider, must be 'github' or 'gitlab'")
	}
	repo, err := e.gitIntegrationSvc.CreateRepository(ctx, uid, provider, a.Name, a.Private, a.Description)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(map[string]any{"repository": repo})
}

// ─────────────────────────────────────────────────────────────────────────────
// JSON-schemas (inline; короткие — magic JSON допустим)
// ─────────────────────────────────────────────────────────────────────────────

var (
	schemaEmpty             = json.RawMessage(`{"type":"object","properties":{}}`)
	schemaProjectList       = json.RawMessage(`{"type":"object","properties":{"status":{"type":"string"},"git_provider":{"type":"string"},"search":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	// Anthropic tool-schemas НЕ принимают oneOf/anyOf/allOf на верхнем уровне input_schema
	// (status 400 invalid_request_error). Альтернативность id|project_id валидируется в
	// runtime через idArgs.resolve() — этого достаточно.
	schemaProjectGet        = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"project_id":{"type":"string","format":"uuid"}},"description":"Передайте либо id, либо project_id (UUID проекта)."}`)
	schemaProjectCreate     = json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"},"description":{"type":"string"},"git_provider":{"type":"string"},"git_url":{"type":"string"},"git_default_branch":{"type":"string"}}}`)
	schemaProjectUpdate     = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"project_id":{"type":"string","format":"uuid"},"name":{"type":"string"},"description":{"type":"string"}},"description":"Передайте либо id, либо project_id (UUID проекта)."}`)
	schemaTaskList          = json.RawMessage(`{"type":"object","required":["project_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"state":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	schemaTaskGet           = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"task_id":{"type":"string","format":"uuid"}}}`)
	schemaTaskCreate        = json.RawMessage(`{"type":"object","required":["project_id","title"],"properties":{"project_id":{"type":"string","format":"uuid"},"title":{"type":"string"},"description":{"type":"string"},"priority":{"type":"string","enum":["critical","high","medium","low"]},"assigned_agent_id":{"type":"string","format":"uuid"},"parent_task_id":{"type":"string","format":"uuid"}}}`)
	schemaTaskUpdate        = json.RawMessage(`{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string","format":"uuid"},"title":{"type":"string"},"description":{"type":"string"},"priority":{"type":"string","enum":["critical","high","medium","low"]},"status":{"type":"string"},"assigned_agent_id":{"type":"string","format":"uuid"},"clear_assigned_agent":{"type":"boolean"},"branch_name":{"type":"string"}}}`)
	schemaConvList          = json.RawMessage(`{"type":"object","required":["project_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	schemaConvGet           = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"conversation_id":{"type":"string","format":"uuid"}}}`)
	schemaConvCreate        = json.RawMessage(`{"type":"object","required":["project_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"title":{"type":"string"}}}`)
	schemaConvSendMessage   = json.RawMessage(`{"type":"object","required":["conversation_id","content"],"properties":{"conversation_id":{"type":"string","format":"uuid"},"content":{"type":"string"}}}`)
	schemaAgentList    = json.RawMessage(`{"type":"object","properties":{"role":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50},"offset":{"type":"integer","minimum":0}}}`)
	schemaAgentGet     = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"agent_id":{"type":"string","format":"uuid"}}}`)
	schemaArtifactList = json.RawMessage(`{"type":"object","required":["task_id"],"properties":{"task_id":{"type":"string","format":"uuid"}}}`)
	schemaArtifactGet       = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","format":"uuid"},"artifact_id":{"type":"string","format":"uuid"}},"description":"Передайте либо id, либо artifact_id (UUID артефакта)."}`)
	schemaAppNavigate       = json.RawMessage(`{"type":"object","required":["route"],"properties":{"route":{"type":"string","description":"go_router path, например '/projects/<uuid>'"}}}`)
	schemaGitIntegrationList = json.RawMessage(`{"type":"object","properties":{}}`)
	schemaGitRepositoryList  = json.RawMessage(`{"type":"object","required":["provider"],"properties":{"provider":{"type":"string","description":"Провайдер git-интеграции (github или gitlab)"}}}`)
	schemaGitRepositoryCreate = json.RawMessage(`{"type":"object","required":["provider","name"],"properties":{"provider":{"type":"string","description":"Провайдер git-интеграции (github или gitlab)"},"name":{"type":"string","description":"Имя нового репозитория"},"private":{"type":"boolean","description":"Сделать ли репозиторий приватным"},"description":{"type":"string","description":"Описание нового репозитория"}}}`)
	schemaTeamGet        = json.RawMessage(`{"type":"object","properties":{"project_id":{"type":"string","format":"uuid"}},"description":"UUID проекта (опционально, если сессия привязана к проекту)."}`)
	schemaTeamUpdate     = json.RawMessage(`{"type":"object","required":["name"],"properties":{"project_id":{"type":"string","format":"uuid"},"name":{"type":"string"}},"description":"UUID проекта и новое название."}`)
	schemaTeamAgentPatch = json.RawMessage(`{"type":"object","required":["agent_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"agent_id":{"type":"string","format":"uuid"},"clear_model":{"type":"boolean"},"model":{"type":"string"},"clear_prompt_id":{"type":"boolean"},"prompt_id":{"type":"string","format":"uuid"},"clear_system_prompt":{"type":"boolean"},"system_prompt":{"type":"string"},"clear_code_backend":{"type":"boolean"},"code_backend":{"type":"string"},"is_active":{"type":"boolean"},"tool_definition_ids":{"type":"array","items":{"type":"string","format":"uuid"}}}}`)
	schemaTeamAgentCreate = json.RawMessage(`{"type":"object","required":["name","role","execution_kind"],"properties":{"project_id":{"type":"string","format":"uuid"},"team_id":{"type":"string","format":"uuid"},"name":{"type":"string"},"role":{"type":"string","enum":["orchestrator","router","developer","reviewer","tester","planner","coder","researcher","writer"]},"execution_kind":{"type":"string","enum":["llm","sandbox"]},"role_description":{"type":"string"},"system_prompt":{"type":"string"},"prompt_id":{"type":"string","format":"uuid"},"model":{"type":"string"},"provider_kind":{"type":"string","enum":["openai","anthropic","deepseek","zhipu","openrouter","anthropic_oauth","antigravity","antigravity_oauth"]},"code_backend":{"type":"string","enum":["claude-code","aider","hermes","custom"]},"temperature":{"type":"number"},"max_tokens":{"type":"integer"}}}`)
	schemaTeamAgentDelete = json.RawMessage(`{"type":"object","required":["agent_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"agent_id":{"type":"string","format":"uuid"}}}`)
	schemaTeamList       = json.RawMessage(`{"type":"object","properties":{"project_id":{"type":"string","format":"uuid"}},"description":"UUID проекта (опционально, если сессия привязана к проекту)."}`)
	schemaTeamCreate     = json.RawMessage(`{"type":"object","required":["name","type"],"properties":{"project_id":{"type":"string","format":"uuid"},"name":{"type":"string"},"type":{"type":"string"}},"description":"Создать новую команду в проекте."}`)
	schemaTeamDelete     = json.RawMessage(`{"type":"object","required":["team_id"],"properties":{"project_id":{"type":"string","format":"uuid"},"team_id":{"type":"string","format":"uuid"}},"description":"Удалить команду из проекта."}`)
	schemaTeamTypeList   = json.RawMessage(`{"type":"object","properties":{}}`)
	schemaTeamTypeCreate = json.RawMessage(`{"type":"object","required":["code","name"],"properties":{"code":{"type":"string"},"name":{"type":"string"}}}`)
	schemaTeamTypeDelete = json.RawMessage(`{"type":"object","required":["code"],"properties":{"code":{"type":"string"}}}`)
)

func (e *AuthorizedExecutor) teamGet(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamGetParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	team, err := e.teamSvc.GetByProjectID(ctx, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(dto.ToTeamResponse(team))
}

func (e *AuthorizedExecutor) teamUpdate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamUpdateParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	upd := dto.UpdateTeamRequest{Name: a.Name}
	team, err := e.teamSvc.Update(ctx, pid, upd)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(dto.ToTeamResponse(team))
}

func (e *AuthorizedExecutor) teamAgentPatch(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamAgentPatchParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	aid, err := uuid.Parse(a.AgentID)
	if err != nil {
		return businessErr("validation", "agent_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	raw, err := teamAgentPatchWireJSON(&a)
	if err != nil {
		return businessErr("validation", err.Error())
	}
	var patch dto.PatchAgentRequest
	if err := json.Unmarshal(raw, &patch); err != nil {
		return businessErr("validation", fmt.Sprintf("invalid patch fields: %v", err))
	}
	team, err := e.teamSvc.PatchAgent(ctx, pid, aid, patch)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(dto.ToTeamResponse(team))
}

type TeamAgentCreateParams struct {
	ProjectID       string   `json:"project_id"`
	TeamID          string   `json:"team_id,omitempty"`
	Name            string   `json:"name"`
	Role            string   `json:"role"`
	ExecutionKind   string   `json:"execution_kind"`
	RoleDescription *string  `json:"role_description,omitempty"`
	SystemPrompt    *string  `json:"system_prompt,omitempty"`
	PromptID        *string  `json:"prompt_id,omitempty"`
	Model           *string  `json:"model,omitempty"`
	ProviderKind    *string  `json:"provider_kind,omitempty"`
	CodeBackend     *string  `json:"code_backend,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxTokens       *int     `json:"max_tokens,omitempty"`
}

func (e *AuthorizedExecutor) teamAgentCreate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamAgentCreateParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	// Get teams
	teams, err := e.teamSvc.ListByProjectID(ctx, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	var team *models.Team
	if a.TeamID != "" {
		tid, err := uuid.Parse(a.TeamID)
		if err != nil {
			return businessErr("validation", "team_id must be a valid UUID")
		}
		for i := range teams {
			if teams[i].ID == tid {
				team = &teams[i]
				break
			}
		}
		if team == nil {
			return businessErr("not_found", "команда не найдена в текущем проекте")
		}
	} else {
		if len(teams) == 0 {
			return businessErr("not_found", "команда проекта не найдена")
		}
		team = &teams[0]
	}

	var promptUUID *uuid.UUID
	if a.PromptID != nil && *a.PromptID != "" {
		parsed, err := uuid.Parse(*a.PromptID)
		if err != nil {
			return businessErr("validation", "prompt_id must be a valid UUID")
		}
		promptUUID = &parsed
	}

	// Prepare CreateAgentInput
	in := service.CreateAgentInput{
		Name:            a.Name,
		Role:            models.AgentRole(a.Role),
		ExecutionKind:   models.AgentExecutionKind(a.ExecutionKind),
		RoleDescription: a.RoleDescription,
		SystemPrompt:    a.SystemPrompt,
		PromptID:        promptUUID,
		Model:           a.Model,
		Temperature:     a.Temperature,
		MaxTokens:       a.MaxTokens,
		TeamID:          &team.ID,
		UserID:          nil, // For team-level agent, UserID must be NULL due to chk_agents_ownership constraint
	}
	if a.ProviderKind != nil {
		pk := models.AgentProviderKind(*a.ProviderKind)
		in.ProviderKind = &pk
	}
	if a.CodeBackend != nil {
		cb := models.CodeBackend(*a.CodeBackend)
		in.CodeBackend = &cb
	}

	_, err = e.agentSvc.Create(ctx, in)
	if err != nil {
		return mapServiceErr(err)
	}

	// Reload team to return updated list of agents
	updatedTeams, err := e.teamSvc.ListByProjectID(ctx, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	var updatedTeam *models.Team
	for i := range updatedTeams {
		if updatedTeams[i].ID == team.ID {
			updatedTeam = &updatedTeams[i]
			break
		}
	}
	if updatedTeam == nil {
		return marshalResult(nil)
	}
	return marshalResult(dto.ToTeamResponse(updatedTeam))
}

type TeamAgentDeleteParams struct {
	ProjectID string `json:"project_id"`
	AgentID   string `json:"agent_id"`
}

func (e *AuthorizedExecutor) teamAgentDelete(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamAgentDeleteParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	aid, err := uuid.Parse(a.AgentID)
	if err != nil {
		return businessErr("validation", "agent_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}

	// Get agent to verify ownership
	agent, err := e.agentSvc.GetByID(ctx, aid)
	if err != nil {
		return mapServiceErr(err)
	}
	if agent.TeamID == nil {
		return businessErr("forbidden", "агент не принадлежит ни одной команде")
	}

	// Verify that the agent's team belongs to the current project
	teams, err := e.teamSvc.ListByProjectID(ctx, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	var targetTeam *models.Team
	for i := range teams {
		if teams[i].ID == *agent.TeamID {
			targetTeam = &teams[i]
			break
		}
	}
	if targetTeam == nil {
		return businessErr("forbidden", "агент не принадлежит команде текущего проекта")
	}

	// Delete agent
	if err := e.agentSvc.Delete(ctx, aid); err != nil {
		return mapServiceErr(err)
	}

	// Reload the team to return its updated state
	updatedTeams, err := e.teamSvc.ListByProjectID(ctx, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	var updatedTeam *models.Team
	for i := range updatedTeams {
		if updatedTeams[i].ID == *agent.TeamID {
			updatedTeam = &updatedTeams[i]
			break
		}
	}
	if updatedTeam == nil {
		return marshalResult(nil)
	}
	return marshalResult(dto.ToTeamResponse(updatedTeam))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (e *AuthorizedExecutor) teamList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamListParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	teams, err := e.teamSvc.ListByProjectID(ctx, pid)
	if err != nil {
		return mapServiceErr(err)
	}
	resp := make([]dto.TeamResponse, 0, len(teams))
	for i := range teams {
		resp = append(resp, dto.ToTeamResponse(&teams[i]))
	}
	return marshalResult(resp)
}

func (e *AuthorizedExecutor) teamCreate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamCreateParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	req := dto.CreateTeamRequest{
		Name: a.Name,
		Type: a.Type,
	}
	team, err := e.teamSvc.Create(ctx, pid, req)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(dto.ToTeamResponse(team))
}

func (e *AuthorizedExecutor) teamDelete(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	ctx, uid, err := injectAuth(ctx, auth)
	if err != nil {
		return nil, err
	}
	var a TeamDeleteParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	if auth.ProjectID != "" {
		if a.ProjectID != "" && a.ProjectID != auth.ProjectID {
			return businessErr("forbidden", "доступ к другим проектам запрещён")
		}
		a.ProjectID = auth.ProjectID
	}
	pid, err := uuid.Parse(a.ProjectID)
	if err != nil {
		return businessErr("validation", "project_id is required (UUID)")
	}
	tid, err := uuid.Parse(a.TeamID)
	if err != nil {
		return businessErr("validation", "team_id is required (UUID)")
	}
	// Check project access
	if _, err := e.projectSvc.GetByID(ctx, uid, models.RoleUser, pid); err != nil {
		return mapServiceErr(err)
	}
	err = e.teamSvc.Delete(ctx, pid, tid)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(nil)
}

func (e *AuthorizedExecutor) teamTypeList(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	list, err := e.teamSvc.ListTeamTypes(ctx)
	if err != nil {
		return mapServiceErr(err)
	}
	resp := make([]dto.TeamTypeResponse, 0, len(list))
	for i := range list {
		resp = append(resp, dto.ToTeamTypeResponse(&list[i]))
	}
	return marshalResult(resp)
}

func (e *AuthorizedExecutor) teamTypeCreate(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	var a TeamTypeCreateParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	req := dto.CreateTeamTypeRequest{
		Code: a.Code,
		Name: a.Name,
	}
	tt, err := e.teamSvc.CreateTeamType(ctx, req)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(dto.ToTeamTypeResponse(tt))
}

func (e *AuthorizedExecutor) teamTypeDelete(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	var a TeamTypeDeleteParams
	if err := parseArgs(args, &a); err != nil {
		return businessErr("validation", err.Error())
	}
	err := e.teamSvc.DeleteTeamType(ctx, a.Code)
	if err != nil {
		return mapServiceErr(err)
	}
	return marshalResult(nil)
}
