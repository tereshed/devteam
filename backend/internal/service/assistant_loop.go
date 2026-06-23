package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/llm"
)

// assistant_loop.go — Sprint 21 §3.1+§3.2: запуск агент-петли и её резюм
// после destructive-confirm. Файл выделен из assistant_service.go ради
// фокуса: здесь только runtime-логика, никакого CRUD/handler-маппинга.
//
// КОНТРАКТ defer: оба runner'а используют один и тот же `defer release()`,
// и обнуляют `released` если возвращают Status=Parked (плана §3.1
// «destructive-confirm: флаг остаётся TRUE»).

// runAgentLoop — стартует петлю на свежей user-message. Сессия уже
// захвачена busy=TRUE в SendMessage (CAS). Здесь только: загрузить
// контекст → собрать RunRequest → Executor.Run → обработать Result.
func (s *assistantService) runAgentLoop(parent context.Context, sessionID, userID uuid.UUID) {
	s.runWithRecovery(parent, sessionID, userID, "fresh")
}

// runAgentLoopResume — стартует петлю ПОСЛЕ confirm. Сессия осталась
// busy=TRUE с момента park'а; история теперь содержит закрытый tool_result
// (writer'ом был ConfirmAndClosePending). LLM получит его как часть
// истории и продолжит с новой итерации.
func (s *assistantService) runAgentLoopResume(parent context.Context, sessionID, userID uuid.UUID) {
	s.runWithRecovery(parent, sessionID, userID, "resume")
}

// runWithRecovery — общая обёртка с timeout/panic-recover/release-defer.
// Разделение fresh vs resume только в логах — runtime-логика одинакова.
func (s *assistantService) runWithRecovery(parent context.Context, sessionID, userID uuid.UUID, kind string) {
	ctx, cancel := context.WithTimeout(parent, AssistantLoopTimeout)
	defer cancel()

	// Регистрируем cancel, чтобы StopRun мог мгновенно прервать петлю.
	s.trackRun(sessionID, cancel)
	defer s.untrackRun(sessionID)

	// released flag — управляет тем, снимать ли busy в defer'е. По умолчанию
	// true (снимаем); если Run вернёт Parked — выставляем в false ДО возврата.
	releaseBusy := true
	defer func() {
		if r := recover(); r != nil {
			// Panic в горутине — никогда не должен оставлять сессию навсегда busy.
			s.deps.Logger.ErrorContext(ctx, "assistant: panic in agent loop",
				slog.String("session_id", sessionID.String()),
				slog.String("kind", kind),
				slog.String("panic", fmt.Sprintf("%v", r)),
				slog.String("stack", string(debug.Stack())),
			)
			s.appendErrorMessage(context.Background(), sessionID, userID, "внутренняя ошибка ассистента, попробуйте ещё раз")
			releaseBusy = true
		}
		if releaseBusy {
			if err := s.deps.Repo.ReleaseBusy(context.Background(), sessionID); err != nil {
				s.deps.Logger.WarnContext(context.Background(), "assistant: release busy failed",
					slog.String("session_id", sessionID.String()),
					slog.String("error", err.Error()),
				)
			}
			// Сообщаем фронту, что сессия свободна (input снова активный).
			if sess, err := s.deps.Repo.GetSession(context.Background(), sessionID, userID); err == nil {
				s.broadcastSessionUpdated(userID, sess)
			}
		}
	}()

	// 0) Загружаем сессию, чтобы получить project_id.
	sess, err := s.deps.Repo.GetSession(ctx, sessionID, userID)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "assistant: load session failed",
			slog.String("session_id", sessionID.String()),
			slog.String("error", err.Error()),
		)
		s.appendErrorMessage(ctx, sessionID, userID, "не удалось загрузить сессию")
		return
	}
	projectIDStr := ""
	var project *models.Project
	var teams []models.Team
	if sess.ProjectID != nil {
		projectIDStr = sess.ProjectID.String()
		if p, err := s.deps.ProjectRepo.GetByID(ctx, *sess.ProjectID); err == nil {
			project = p
		} else {
			s.deps.Logger.WarnContext(ctx, "assistant: load project failed for session",
				slog.String("project_id", projectIDStr),
				slog.String("error", err.Error()),
			)
		}
		if list, err := s.deps.TeamRepo.ListByProjectID(ctx, *sess.ProjectID); err == nil {
			teams = list
		} else {
			s.deps.Logger.WarnContext(ctx, "assistant: list teams failed for project",
				slog.String("project_id", projectIDStr),
				slog.String("error", err.Error()),
			)
		}
	}

	// 1) Загружаем agent (system prompt + model + provider).
	agent, err := s.getOrProvisionAssistantAgent(ctx, userID)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "assistant: load agent failed",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
		s.appendErrorMessage(ctx, sessionID, userID, "ассистент не настроен (нет agent role='assistant')")
		return
	}
	if agent == nil || !agent.IsActive {
		s.appendErrorMessage(ctx, sessionID, userID, "ассистент отключён администратором")
		return
	}

	// 2) Резолвим LLM-клиента.
	client, err := s.deps.LLMResolver.ResolveAssistantClient(ctx, agent, userID)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "assistant: resolve llm client failed",
			slog.String("error", err.Error()),
		)
		s.appendErrorMessage(ctx, sessionID, userID, "LLM-провайдер недоступен")
		return
	}

	// 3) Загружаем историю (последние N в хронологическом порядке).
	history, err := s.loadHistory(ctx, sessionID)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "assistant: load history failed",
			slog.String("error", err.Error()),
		)
		s.appendErrorMessage(ctx, sessionID, userID, "не удалось загрузить историю")
		return
	}

	// 4) Собираем хуки.
	hooks := s.buildHooks(sessionID, userID)

	// 5) Run.
	model := ""
	if agent.Model != nil {
		model = *agent.Model
	}
	var promptParts []string
	if agent.Prompt != nil && strings.TrimSpace(agent.Prompt.Template) != "" {
		promptParts = append(promptParts, agent.Prompt.Template)
	}
	if base := resolveAssistantBasePrompt(agent.SystemPrompt, project); base != "" {
		promptParts = append(promptParts, base)
	}
	sysPrompt := strings.Join(promptParts, "\n\n")
	if project != nil {
		var pb strings.Builder
		pb.WriteString("\n\n=== PROJECT CONTEXT ===\n")
		pb.WriteString(fmt.Sprintf("You are operating as a Project Orchestrator/Assistant inside the project %q.\n", project.Name))
		if project.Description != "" {
			pb.WriteString(fmt.Sprintf("Project Description: %s\n", project.Description))
		}
		pb.WriteString(fmt.Sprintf("Project ID: %s\n", project.ID.String()))
		if project.GitURL != "" {
			pb.WriteString(fmt.Sprintf("Git URL: %s\n", project.GitURL))
			if project.GitDefaultBranch != "" {
				pb.WriteString(fmt.Sprintf("Default Branch: %s\n", project.GitDefaultBranch))
			}
		}

		// Авто-директива: если шаблон имён веток проекта требует {ticket}, ассистент
		// обязан получить ключ у пользователя и НЕ выдумывать его. Жёсткий гейт всё
		// равно в task_service.Create — это лишь чтобы агент спросил заранее.
		if project.BranchNameTemplate != nil && TemplateRequiresTicket(strings.TrimSpace(*project.BranchNameTemplate)) {
			pb.WriteString("\n=== TICKET KEY REQUIRED ===\n")
			pb.WriteString("This project REQUIRES a ticket key (e.g. DEV-123) for EVERY task. ")
			pb.WriteString("You MUST pass the key via the task_create `external_key` parameter. ")
			pb.WriteString("Do NOT put it only in the title, description or acceptance_criteria — it MUST be set in external_key. ")
			pb.WriteString("Extract the key from the user's request and set external_key to it. ")
			pb.WriteString("NEVER invent or guess a key — if the user did not provide one, ask before creating the task. ")
			pb.WriteString("Calling task_create without external_key will fail with external_key_required.\n")
		}

		if len(teams) > 0 {
			pb.WriteString("\n=== PROJECT TEAMS & AGENTS ===\n")
			for _, t := range teams {
				pb.WriteString(fmt.Sprintf("Team: %s (Type: %s, ID: %s)\n", t.Name, t.Type, t.ID.String()))
				if len(t.Agents) > 0 {
					pb.WriteString("  Agents in this team:\n")
					for _, a := range t.Agents {
						isActiveStr := "inactive"
						if a.IsActive {
							isActiveStr = "active"
						}
						// system_prompt агентов сюда НЕ инлайнится: после миграции 082 промпты
						// ролей выросли до 1.5-3.3КБ, и полный их текст раздувал контекст
						// ассистента на ~15-25КБ КАЖДОЕ сообщение чужими инструкциями
						// (severity-калибровка ревьюера и т.п. ассистенту бесполезны).
						pb.WriteString(fmt.Sprintf("  - Agent %q (Role: %s, Status: %s, ID: %s)\n", a.Name, a.Role, isActiveStr, a.ID.String()))
					}
				} else {
					pb.WriteString("  No agents configured in this team.\n")
				}
			}
		}

		pb.WriteString("\n=== INSTRUCTIONS FOR PROJECT MODE ===\n")
		pb.WriteString("1. You are strictly isolated to this project. Never attempt to list other projects, access tasks/conversations of other projects, or create new projects.\n")
		pb.WriteString("2. You can help the user configure the teams and agents in this project (using team_list, team_create, team_delete, team_get, team_update, team_agent_patch, team_agent_create, team_agent_delete, team_type_list, team_type_create, team_type_delete tools).\n")
		pb.WriteString("3. You can formulate tasks for this project and start/delegate them to the pipeline (using conversation_create, conversation_send_message, task_create tools).\n")
		pb.WriteString("4. Your tone should be collaborative, professional, and focus on coordinating engineering work in this project.\n")

		sysPrompt += pb.String()
	}
	// Phase 5: пробрасываем agent.ProviderKind в RunRequest.Provider. Без
	// этого llmService.Generate уходил в defaultProvider (openai) независимо
	// от того, что у seed assistant'а написано provider_kind=anthropic/
	// openrouter. agentloop сам переименует тип в llm.ProviderType при
	// сборке запроса (см. executor.go).
	providerKind := ""
	if agent.ProviderKind != nil {
		providerKind = string(*agent.ProviderKind)
	}

	// Подключаем внешние MCP-серверы проекта на время Run: их инструменты идут в
	// каталог как обычные function-tools. Сессии живут до closeMCP (CallTool идёт
	// во время петли). Падение сервера не валит ассистента — см. openProjectMCPTools.
	mcpTools, closeMCP := s.openProjectMCPTools(ctx, project)
	defer closeMCP()

	result, runErr := s.deps.Executor.Run(ctx, agentloop.RunRequest{
		Client:       client,
		Provider:     providerKind,
		Model:        model,
		SystemPrompt: sysPrompt,
		Temperature:  agent.Temperature,
		MaxTokens:    agent.MaxTokens,
		History:      history,
		Tools:        append(s.assistantTools(ctx, sess), mcpTools...),
		ServerTools:  assistantServerTools(providerKind),
		Auth: agentloop.AuthContext{
			UserID:    userID.String(),
			ProjectID: projectIDStr,
			Scope:     "assistant",
		},
		Hooks: hooks,
	})
	if runErr != nil {
		// Сюда попадаем только при программных ошибках конфигурации
		// (см. Executor.Run doc) — это критика, но сессию отпускаем.
		s.deps.Logger.ErrorContext(ctx, "assistant: executor config error",
			slog.String("error", runErr.Error()),
		)
		s.appendErrorMessage(ctx, sessionID, userID, "внутренняя ошибка конфигурации ассистента")
		return
	}

	// 6) Финализация по Result.Status.
	switch result.Status {
	case agentloop.StatusCompleted:
		// Финальный assistant-текст уже записан в OnFinalText-хуке.
		// Тут только метрики/лог.
		s.deps.Logger.DebugContext(ctx, "assistant: loop completed",
			slog.String("session_id", sessionID.String()),
			slog.Int("iterations", result.Iterations),
		)
		// Запуск автогенерации названия сессии в фоне
		go s.autoGenerateSessionTitleIfNeeded(context.Background(), sessionID, userID, client, agent)

	case agentloop.StatusParked:
		if result.ParkedCall == nil {
			s.appendErrorMessage(ctx, sessionID, userID, "внутренняя ошибка ассистента: parked без tool_call")
			return
		}
		// Парк: записываем pending tool-row + ставим pending_tool_call_id,
		// НЕ снимаем busy.
		if err := s.parkPendingConfirmation(ctx, sessionID, userID, *result.ParkedCall); err != nil {
			s.deps.Logger.ErrorContext(ctx, "assistant: park pending failed",
				slog.String("error", err.Error()),
			)
			// Если парк не удался — отпускаем сессию через release-defer,
			// чтобы пользователь не залип.
			s.appendErrorMessage(ctx, sessionID, userID, "не удалось зафиксировать запрос подтверждения")
			return
		}
		releaseBusy = false // ВАЖНО: §3.1 «destructive-confirm: флаг остаётся TRUE»
		// scout_dispatch паркуется не для подтверждения пользователем, а в ожидании
		// прогона разведчика: запускаем его, а закроет tool_call ResumeFromScout.
		if result.ParkedCall.Name == scoutDispatchToolName {
			s.handleScoutPark(ctx, sessionID, userID, sess, *result.ParkedCall)
		} else {
			s.broadcastConfirmRequest(userID, sessionID, *result.ParkedCall)
		}

	case agentloop.StatusLimitExceeded:
		s.appendErrorMessage(ctx, sessionID, userID, fmt.Sprintf("превышен лимит шагов (%d), сформулируйте запрос точнее", AssistantMaxIterations))

	case agentloop.StatusFailed:
		// Пользователь нажал «Стоп» → StopRun отменил контекст (context.Canceled,
		// в отличие от timeout = DeadlineExceeded). Не показываем ошибку — пишем
		// нейтральную заметку. ctx уже отменён, поэтому пишем на context.Background().
		if errors.Is(result.Cause, context.Canceled) && ctx.Err() == context.Canceled {
			s.deps.Logger.InfoContext(context.Background(), "assistant: loop stopped by user",
				slog.String("session_id", sessionID.String()),
			)
			s.appendAssistantNote(context.Background(), sessionID, userID, "⏹️ Выполнение остановлено.")
			return
		}
		// Cause может быть ctx-timeout, LLM-error, hook-error.
		// В историю — нейтральный текст; детали — только в лог.
		causeStr := ""
		if result.Cause != nil {
			causeStr = result.Cause.Error()
		}
		s.deps.Logger.WarnContext(ctx, "assistant: loop failed",
			slog.String("session_id", sessionID.String()),
			slog.String("cause", causeStr),
		)
		userMsg := "запрос к модели не завершился вовремя, попробуйте ещё раз"
		if isCtxTimeoutErr(result.Cause) {
			userMsg = "запрос к модели не завершился вовремя, попробуйте ещё раз"
		}
		s.appendErrorMessage(ctx, sessionID, userID, userMsg)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Загрузка истории.
// ─────────────────────────────────────────────────────────────────────────────

// loadHistory тащит последние N сообщений сессии в хронологическом порядке
// (старые → новые), как ожидает Executor.
func (s *assistantService) loadHistory(ctx context.Context, sessionID uuid.UUID) ([]agentloop.Message, error) {
	rows, err := s.deps.Repo.ListMessages(ctx, sessionID, AssistantHistoryFetchLimit, time.Time{}, uuid.Nil)
	if err != nil {
		return nil, err
	}
	// rows возвращается DESC; разворачиваем в хронологический порядок.
	out := make([]agentloop.Message, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		out = append(out, toAgentloopMessage(rows[i]))
	}
	return out, nil
}

// toAgentloopMessage конвертирует БД-строку в agentloop.Message с учётом
// разных ролей. tool-row с tool_result=NULL (pending) пропускать НЕ нужно
// — такие строки попадают в историю только после ConfirmAndClosePending
// (фоновая cron этого не делает).
func toAgentloopMessage(m *models.AssistantMessage) agentloop.Message {
	switch m.Role {
	case models.AssistantMessageRoleUser:
		return agentloop.Message{
			Role:    llm.RoleUser,
			Content: derefStringEmpty(m.Content),
		}
	case models.AssistantMessageRoleAssistant:
		out := agentloop.Message{
			Role:    llm.RoleAssistant,
			Content: derefStringEmpty(m.Content),
		}
		if m.ToolCallID != nil && m.ToolName != nil {
			out.ToolCalls = []llm.ToolCall{{
				ID:   *m.ToolCallID,
				Type: "function",
				Function: llm.Function{
					Name:      *m.ToolName,
					Arguments: string(m.ToolArguments),
				},
			}}
		}
		return out
	case models.AssistantMessageRoleTool:
		var name string
		if m.ToolName != nil {
			name = *m.ToolName
		}
		var callID string
		if m.ToolCallID != nil {
			callID = *m.ToolCallID
		}
		// tool_result может быть nil (pending). Тогда подадим
		// синтетический «pending» — модель должна была видеть confirm
		// раньше, но на всякий случай.
		result := json.RawMessage(m.ToolResult)
		if len(result) == 0 {
			result = json.RawMessage(`{"status":"pending","message":"awaiting user confirmation"}`)
		}
		return agentloop.Message{
			Role:          llm.RoleTool,
			ToolCallID:    callID,
			ToolName:      name,
			ToolArguments: json.RawMessage(m.ToolArguments),
			ToolResult:    result,
		}
	case models.AssistantMessageRoleSystem:
		return agentloop.Message{
			Role:    llm.RoleSystem,
			Content: derefStringEmpty(m.Content),
		}
	}
	return agentloop.Message{Role: llm.RoleSystem, Content: derefStringEmpty(m.Content)}
}

// ─────────────────────────────────────────────────────────────────────────────
// Hooks builder — связывает Executor с persistence и WS.
// ─────────────────────────────────────────────────────────────────────────────

func (s *assistantService) buildHooks(sessionID, userID uuid.UUID) agentloop.Hooks {
	return agentloop.Hooks{
		// OnAssistantMessage — промежуточные ответы (с tool_calls). Сохраняем
		// в БД (тип assistant), эмитим WS. Финальный текст идёт отдельно
		// через OnFinalText, чтобы не дублировать запись.
		OnAssistantMessage: func(ctx context.Context, msg agentloop.AssistantMsg) error {
			if len(msg.ToolCalls) == 0 {
				// Финальный текст — будет записан OnFinalText'ом.
				return nil
			}
			// Каждый ToolCall сохраняется отдельной assistant-row, чтобы
			// уникальный partial-индекс по tool_call_id отрабатывал.
			for _, tc := range msg.ToolCalls {
				row := &models.AssistantMessage{
					SessionID:     sessionID,
					Role:          models.AssistantMessageRoleAssistant,
					Content:       ptrString(msg.Content),
					ToolCallID:    ptrString(tc.ID),
					ToolName:      ptrString(tc.Function.Name),
					ToolArguments: datatypes.JSON([]byte(tc.Function.Arguments)),
				}
				if err := s.deps.Repo.AppendMessage(ctx, row); err != nil {
					return fmt.Errorf("persist assistant tool_call: %w", err)
				}
				s.broadcastMessage(userID, sessionID, row)
			}
			return nil
		},

		// OnToolCall — фронт показывает «🔧 tool_name(args)» карточку.
		// Persistence уже сделана в OnAssistantMessage; здесь только WS.
		OnToolCall: func(ctx context.Context, call agentloop.ToolCall) error {
			s.broadcastToolCall(userID, sessionID, call)
			return nil
		},

		// OnToolResult — сохраняем tool-row + эмитим WS. Если это
		// app_navigate — дополнительно эмитим assistant.navigate.
		OnToolResult: func(ctx context.Context, res agentloop.ToolResult) error {
			row := &models.AssistantMessage{
				SessionID:  sessionID,
				Role:       models.AssistantMessageRoleTool,
				ToolCallID: ptrString(res.CallID),
				ToolName:   ptrString(res.Name),
				ToolResult: datatypes.JSON(res.Result),
			}
			if err := s.deps.Repo.AppendMessage(ctx, row); err != nil {
				return fmt.Errorf("persist tool_result: %w", err)
			}
			// WS-эмиссия: result уже урезанным сюда НЕ приходит — Executor
			// отдаёт сырой payload (truncation — только для подачи в LLM).
			// Идём через ws.MarshalAssistantToolResult — обёртку,
			// которая собирает корректный UserEnvelope{type,v,ts,user_id,data}.
			s.broadcastToolResultPayload(userID, sessionID, res.CallID, res.Name, res.Status, json.RawMessage(res.Result))

			// Special-case app_navigate → отдельный WS-event для go_router.
			if res.Name == "app_navigate" && res.Status == "ok" {
				var wrapper struct {
					Data struct {
						Route string `json:"route"`
					} `json:"data"`
				}
				if err := json.Unmarshal(res.Result, &wrapper); err == nil && wrapper.Data.Route != "" {
					s.broadcastNavigate(userID, wrapper.Data.Route)
				}
			}
			return nil
		},

		// OnConfirmRequired — destructive операция → паркуем (это контракт
		// Assistant'а; план §3.1 §3.2). Возвращаем ConfirmPark — Executor
		// сразу выйдет, и runAgentLoop сохранит pending state.
		OnConfirmRequired: func(ctx context.Context, call agentloop.ToolCall) (agentloop.ConfirmDecision, error) {
			return agentloop.ConfirmPark, nil
		},

		// OnFinalText — финальный assistant-ответ. Записываем + эмитим.
		OnFinalText: func(ctx context.Context, text string) error {
			row := &models.AssistantMessage{
				SessionID: sessionID,
				Role:      models.AssistantMessageRoleAssistant,
				Content:   ptrString(text),
			}
			if err := s.deps.Repo.AppendMessage(ctx, row); err != nil {
				return fmt.Errorf("persist final assistant message: %w", err)
			}
			s.broadcastMessage(userID, sessionID, row)
			return nil
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Парковка confirm-state в БД.
// ─────────────────────────────────────────────────────────────────────────────

// parkPendingConfirmation атомарно записывает pending tool-row и ставит
// pending_tool_call_id в сессии. tool-row с tool_result=NULL — это и есть
// «pending», ConfirmAndClosePending позже его закроет.
func (s *assistantService) parkPendingConfirmation(ctx context.Context, sessionID, userID uuid.UUID, call agentloop.ToolCall) error {
	// 1) Сначала сохраняем pending tool-row. Уникальный индекс
	//    idx_assistant_messages_tool_call защитит от двойной парковки
	//    на тот же tool_call_id (§4.1).
	row := &models.AssistantMessage{
		SessionID:     sessionID,
		Role:          models.AssistantMessageRoleTool,
		ToolCallID:    ptrString(call.ID),
		ToolName:      ptrString(call.Name),
		ToolArguments: datatypes.JSON(call.Arguments),
		// ToolResult = nil (jsonb null) — «pending», семантика плана §4.1.
	}
	if err := s.deps.Repo.AppendMessage(ctx, row); err != nil {
		return fmt.Errorf("append pending tool row: %w", err)
	}

	// 2) Ставим pending_tool_call_id. ParkOnConfirm проверяет (busy=TRUE,
	//    pending=NULL) — гарантия, что в одной петле нельзя зарегистрировать
	//    двойной park.
	if err := s.deps.Repo.ParkOnConfirm(ctx, sessionID, call.ID); err != nil {
		return fmt.Errorf("park on confirm: %w", err)
	}
	_ = userID // userID нужен для будущей аудит-метрики; пока не используется.
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Разведчик (scout_dispatch): сборка каталога, определение tool'а, park-обработчик.
// ─────────────────────────────────────────────────────────────────────────────

// scoutDispatchToolName — имя assistant-tool'а запуска разведчика.
const scoutDispatchToolName = "scout_dispatch"

// assistantTools — каталог tools для прогона. scout_dispatch добавляется только
// для проектной сессии с включённым разведчиком (требование: доступен только
// ассистенту проекта). Глобальный ассистент его не видит.
func (s *assistantService) assistantTools(ctx context.Context, sess *models.AssistantSession) []agentloop.Tool {
	tools := s.deps.ToolCatalog.Catalog()
	if sess != nil && sess.ProjectID != nil && s.deps.ScoutDispatcher != nil &&
		s.deps.ScoutDispatcher.ScoutEnabled(ctx, *sess.ProjectID) {
		tools = append(tools, scoutDispatchTool())
	}
	return tools
}

// scoutDispatchTool — определение assistant-tool'а. RequiresConfirmation=true,
// чтобы Executor распарковал луп после вызова (ConfirmPark); реальный диспатч и
// закрытие tool_call идут вне Executor (handleScoutPark + ResumeFromScout).
func scoutDispatchTool() agentloop.Tool {
	return agentloop.Tool{
		Name:                 scoutDispatchToolName,
		Description:          "Запустить агента-разведчика: headless-прогон в sandbox, который читает репозитории проекта и собирает досье контекста по проблеме пользователя (релевантные файлы, как устроено, подходы, открытые вопросы, критерии приёмки). Используй, когда пользователь приходит с проблемой/болью, а не с готовой задачей, и нужно собрать контекст перед постановкой. Прогон идёт минуты; досье придёт как результат этого вызова.",
		InputSchema:          json.RawMessage(`{"type":"object","properties":{"problem":{"type":"string","description":"Постановка проблемы пользователя своими словами: что болит и какой желаемый исход."}},"required":["problem"]}`),
		RequiresConfirmation: true,
		Handler:              scoutDispatchHandler,
	}
}

// scoutDispatchHandler — защитная заглушка. scout_dispatch — confirm-park tool:
// его handler не исполняется (диспатч идёт в handleScoutPark, результат — через
// ResumeFromScout). Если всё же вызвался — вернём бизнес-ошибку, а не панику.
func scoutDispatchHandler(ctx context.Context, auth agentloop.AuthContext, args json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{"error", "scout_dispatch обрабатывается асинхронно и не исполняется напрямую"})
}

// handleScoutPark запускает разведчика для распарканного вызова. При любой
// синхронной неудаче немедленно закрывает tool_call ошибкой (ResumeFromScout),
// чтобы сессия не залипла в busy навсегда.
func (s *assistantService) handleScoutPark(ctx context.Context, sessionID, userID uuid.UUID, sess *models.AssistantSession, call agentloop.ToolCall) {
	s.appendAssistantNote(ctx, sessionID, userID, "🔍 Разведчик собирает контекст проекта — вернусь с результатом…")

	if s.deps.ScoutDispatcher == nil || sess == nil || sess.ProjectID == nil {
		s.ResumeFromScout(ctx, sessionID, userID, call.ID, "", fmt.Errorf("разведчик недоступен в этом контексте"))
		return
	}
	var a struct {
		Problem string `json:"problem"`
	}
	_ = json.Unmarshal(call.Arguments, &a)
	problem := strings.TrimSpace(a.Problem)
	if problem == "" {
		s.ResumeFromScout(ctx, sessionID, userID, call.ID, "", fmt.Errorf("не передана постановка проблемы (problem)"))
		return
	}
	if err := s.deps.ScoutDispatcher.DispatchForSession(ctx, userID, *sess.ProjectID, sessionID, call.ID, problem); err != nil {
		s.ResumeFromScout(ctx, sessionID, userID, call.ID, "", err)
		return
	}
	// Успех: прогон пошёл асинхронно; wake-up придёт из ScoutService по завершении.
}

// appendAssistantNote записывает нейтральное assistant-сообщение (статус) и
// рассылает его по WS. В отличие от appendErrorMessage — не про ошибку.
func (s *assistantService) appendAssistantNote(ctx context.Context, sessionID, userID uuid.UUID, text string) {
	row := &models.AssistantMessage{
		SessionID: sessionID,
		Role:      models.AssistantMessageRoleAssistant,
		Content:   ptrString(text),
	}
	if err := s.deps.Repo.AppendMessage(ctx, row); err != nil {
		s.deps.Logger.WarnContext(ctx, "assistant: append scout note failed", slog.String("error", err.Error()))
		return
	}
	s.broadcastMessage(userID, sessionID, row)
}

// ─────────────────────────────────────────────────────────────────────────────
// Error message persistence.
// ─────────────────────────────────────────────────────────────────────────────

// appendErrorMessage записывает короткое assistant-сообщение об ошибке для UI.
// Используется при Failed/LimitExceeded/Park-неудаче. Никаких raw деталей.
func (s *assistantService) appendErrorMessage(ctx context.Context, sessionID, userID uuid.UUID, text string) {
	row := &models.AssistantMessage{
		SessionID: sessionID,
		Role:      models.AssistantMessageRoleAssistant,
		Content:   ptrString(text),
	}
	if err := s.deps.Repo.AppendMessage(ctx, row); err != nil {
		// Логируем, но не возвращаем — мы уже в финализации ошибки.
		s.deps.Logger.WarnContext(ctx, "assistant: append error message failed",
			slog.String("error", err.Error()),
		)
		return
	}
	s.broadcastMessage(userID, sessionID, row)
}

func derefStringEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func isCtxTimeoutErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (s *assistantService) autoGenerateSessionTitleIfNeeded(bgCtx context.Context, sessionID, userID uuid.UUID, client llm.Client, agent *models.Agent) {
	ctx, cancel := context.WithTimeout(bgCtx, 30*time.Second)
	defer cancel()

	// 1. Fetch session to verify it doesn't already have a title
	sess, err := s.deps.Repo.GetSession(ctx, sessionID, userID)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "assistant: auto-title failed to get session",
			slog.String("session_id", sessionID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	// We only generate title if it is currently nil, empty, or default placeholder.
	hasTitle := false
	if sess.Title != nil && *sess.Title != "" {
		t := *sess.Title
		if t != "Без названия" && t != "Untitled chat" {
			hasTitle = true
		}
	}
	if hasTitle {
		return
	}

	// 2. Fetch messages to get the first user message
	messages, err := s.deps.Repo.ListMessages(ctx, sessionID, 100, time.Time{}, uuid.Nil)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "assistant: auto-title failed to list messages",
			slog.String("session_id", sessionID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	var firstUserMsg *models.AssistantMessage
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == models.AssistantMessageRoleUser && messages[i].Content != nil && *messages[i].Content != "" {
			firstUserMsg = messages[i]
			break
		}
	}

	if firstUserMsg == nil {
		s.deps.Logger.DebugContext(ctx, "assistant: auto-title no user messages found",
			slog.String("session_id", sessionID.String()),
		)
		return
	}

	userContent := *firstUserMsg.Content
	var generatedTitle string

	// 3. Call LLM to generate title
	if client != nil && agent != nil {
		providerKind := ""
		if agent.ProviderKind != nil {
			providerKind = string(*agent.ProviderKind)
		}
		model := ""
		if agent.Model != nil {
			model = *agent.Model
		}

		prompt := "Generate a short, concise title (4-5 words maximum) for a chat session based on the following user's first message. Do not use quotes, punctuation, or any introductory text (like 'Title:'). Respond strictly in the same language as the user's message.\n\nUser's message:\n" + userContent

		llmReq := llm.Request{
			Provider:     llm.ProviderType(providerKind),
			Model:        model,
			SystemPrompt: "You are a helpful assistant that generates short chat titles.",
			Messages: []llm.Message{
				{
					Role:    llm.RoleUser,
					Content: prompt,
				},
			},
			Temperature: ptrFloat64(0.5),
			MaxTokens:   ptrInt(30),
		}

		resp, err := client.Chat(ctx, llmReq)
		if err == nil && resp != nil && resp.Content != "" {
			generatedTitle = strings.TrimSpace(resp.Content)
			// Remove surrounding quotes if model generated them
			generatedTitle = strings.Trim(generatedTitle, `"'«»`)
			generatedTitle = strings.TrimSpace(generatedTitle)
		} else {
			s.deps.Logger.WarnContext(ctx, "assistant: auto-title LLM generation failed, falling back to truncation",
				slog.String("session_id", sessionID.String()),
				slog.Any("error", err),
			)
		}
	}

	// 4. Fallback if LLM failed or not configured
	if generatedTitle == "" {
		runes := []rune(userContent)
		if len(runes) > 40 {
			generatedTitle = string(runes[:40]) + "..."
		} else {
			generatedTitle = string(runes)
		}
	}

	// 5. Save and broadcast
	err = s.deps.Repo.UpdateSessionTitle(ctx, sessionID, userID, generatedTitle)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "assistant: auto-title failed to update session",
			slog.String("session_id", sessionID.String()),
			slog.String("error", err.Error()),
		)
		return
	}

	// Fetch updated session and broadcast
	updatedSess, err := s.deps.Repo.GetSession(ctx, sessionID, userID)
	if err == nil {
		s.broadcastSessionUpdated(userID, updatedSess)
	}
}

func ptrFloat64(f float64) *float64 { return &f }
func ptrInt(i int) *int             { return &i }

// assistantServerTools — server-side тулы провайдера для ассистента. OpenRouter
// исполняет web_search сам и встраивает результаты в ответ (аннотации-источники
// дописывает oaicompat-клиент); ~$0.005/поиск, модель ищет только когда сочтёт
// нужным. Другие провайдеры падают на неизвестном type — для них пусто.
func assistantServerTools(providerKind string) []map[string]any {
	if providerKind != string(llm.ProviderOpenRouter) {
		return nil
	}
	return []map[string]any{{"type": "openrouter:web_search"}}
}

// resolveAssistantBasePrompt — выбор базового промпта ассистента: per-project
// промпт (снапшот при создании проекта, правится в настройках проекта) целиком
// ЗАМЕЩАЕТ user-промпт; NULL/пустой — fallback на user-промпт (legacy-проекты
// и сброс). Наследование копией: правка любого уровня не трогает источник.
func resolveAssistantBasePrompt(agentPrompt *string, project *models.Project) string {
	if project != nil && project.AssistantPrompt != nil {
		if p := strings.TrimSpace(*project.AssistantPrompt); p != "" {
			return p
		}
	}
	if agentPrompt != nil {
		return strings.TrimSpace(*agentPrompt)
	}
	return ""
}
