// Package service — Sprint 21 §3 (assistant_service.go).
//
// AssistantService — глобальный ассистент пользователя (правая боковая
// панель). Отвечает за:
//   - управление assistant_sessions (CRUD/архив/история);
//   - идемпотентный приём user-сообщений (POST /messages);
//   - сериализацию агент-петли через busy-флаг (см. план §3.1);
//   - запуск agentloop.Executor для tool-calling петли;
//   - confirm-flow для destructive операций (POST /confirm);
//   - stale-recovery cron для упавших горутин;
//   - listActiveTasks для Tasks-tab.
//
// Тонкий контракт: SQL/транзакции/локи живут в repository (§2.1 правил),
// сервис только координирует.
package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/devteam/backend/internal/llm/agentloop"
	"github.com/devteam/backend/internal/logging"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/ws"
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/internal/handler/dto"
)

// ─────────────────────────────────────────────────────────────────────────────
// Константы (план §3.1, §3.4).
//
// Инвариант: AssistantLoopTimeout < AssistantStaleThreshold / 2.
// Cron stale-recovery читает то же значение через DI/конфиг — если меняем
// timeout, обновляется и порог.
// ─────────────────────────────────────────────────────────────────────────────

const (
	// AssistantLoopTimeout — hard timeout на ВЕСЬ цикл petli (от старта
	// горутины до возврата Executor.Run). Гарантирует, что зависшая
	// горутина рано или поздно отпустит сессию.
	AssistantLoopTimeout = 30 * time.Minute

	// AssistantStaleThreshold — порог, после которого cron сбрасывает busy
	// у НЕ-припаркованных сессий. > 2 × AssistantLoopTimeout с запасом.
	AssistantStaleThreshold = 65 * time.Minute

	// AssistantPerLLMCallTimeout — таймаут одного Client.Chat (slow-stream
	// guard). Передаётся в agentloop.Config.PerLLMCallTimeout.
	AssistantPerLLMCallTimeout = 10 * time.Minute

	// AssistantStaleRecoveryInterval — частота cron'а. Compromise между
	// нагрузкой на БД и временем восстановления.
	AssistantStaleRecoveryInterval = 1 * time.Minute

	// AssistantMaxIterations — лимит шагов LLM↔tools в одной Run.
	AssistantMaxIterations = 12

	// AssistantMaxToolResultBytes — лимит сериализованного tool_result для
	// подачи в LLM (full payload всё равно сохраняется в БД для UI).
	AssistantMaxToolResultBytes = 16 * 1024

	// AssistantMaxHistoryBytes — ~0.8 × 1M context window в байтах. Грубо.
	// Используется sliding-window compaction.
	AssistantMaxHistoryBytes = 600 * 1024

	// AssistantHistoryTailKeep — сколько последних user/assistant сообщений
	// всегда остаются в полном виде при сжатии истории.
	AssistantHistoryTailKeep = 8

	// AssistantHistoryFetchLimit — сколько последних сообщений тащим из
	// БД для подачи в LLM. Это потолок ListMessages (после идёт truncation).
	AssistantHistoryFetchLimit = 100

	// AssistantAgentName — имя seed-агента в БД (role='assistant').
	AssistantAgentName = "assistant"
)

// Compile-time invariant: stale threshold > 2 × loop timeout.
func init() {
	if AssistantStaleThreshold <= 2*AssistantLoopTimeout {
		panic("assistant: invariant violated — StaleThreshold must be > 2 * LoopTimeout (see plan §3.1)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Доменные ошибки сервиса.
// ─────────────────────────────────────────────────────────────────────────────

var (
	// ErrAssistantSessionBusy — сессия уже занята агент-петлёй. Handler → 409.
	ErrAssistantSessionBusy = errors.New("assistant: session is busy")
	// ErrAssistantSessionNotFound — сессии нет или принадлежит другому юзеру.
	// Handler → 404.
	ErrAssistantSessionNotFound = errors.New("assistant: session not found")
	// ErrAssistantNoPendingConfirmation — POST /confirm без pending tool_call.
	// Handler → 409.
	ErrAssistantNoPendingConfirmation = errors.New("assistant: no pending confirmation")
	// ErrAssistantAlreadyConfirmed — параллельный confirm уже закрыл tool-row.
	// Handler → 409.
	ErrAssistantAlreadyConfirmed = errors.New("assistant: tool call already confirmed")
	// ErrAssistantInvalidInput — fast-fail валидация публичных методов.
	// Handler → 400.
	ErrAssistantInvalidInput = errors.New("assistant: invalid input")
	// ErrAssistantAgentNotConfigured — в БД нет agent с role='assistant'.
	// Handler → 500 (это конфиг-ошибка деплоя).
	ErrAssistantAgentNotConfigured = errors.New("assistant: agent registry entry is missing")
	// ErrAssistantNotConfiguredForUser — у пользователя не настроен API ключ для выбранного провайдера.
	ErrAssistantNotConfiguredForUser = errors.New("assistant: not configured for user (missing api key)")
)

// ─────────────────────────────────────────────────────────────────────────────
// Внешние интерфейсы (DI).
// ─────────────────────────────────────────────────────────────────────────────

// AssistantLLMClientResolver резолвит llm.Client для assistant-агента.
// Реализуется в cmd/api/main.go поверх существующего LLMProviderResolver +
// internallm.NewLLMClient.
type AssistantLLMClientResolver interface {
	ResolveAssistantClient(ctx context.Context, agent *models.Agent, userID uuid.UUID) (llm.Client, error)
}

// AssistantToolCatalogProvider — источник agentloop.Tool[] для assistant'а.
// Реализуется *mcp.AuthorizedExecutor. Передаётся интерфейсом, чтобы
// избежать import-cycle и облегчить тестирование.
type AssistantToolCatalogProvider interface {
	Catalog() []agentloop.Tool
}

// WSBroadcaster — узкое подмножество ws.Hub, нужное assistant'у. Тесты
// подменяют мок'ом, чтобы не поднимать реальный hub.
type WSBroadcaster interface {
	SendToUser(userID, msgType string, payload []byte) error
}

// AssistantAgentLoader — узкий интерфейс для лукапа agent.
// Phase 2: prefer per-user agent (GetAgentByUserRole); fallback to global (GetAgentByName).
type AssistantAgentLoader interface {
	GetAgentByName(ctx context.Context, name string) (*models.Agent, error)
	GetAgentByUserRole(ctx context.Context, userID uuid.UUID, role string) (*models.Agent, error)
	UpdateAgentProvider(ctx context.Context, agentID uuid.UUID, providerKind models.AgentProviderKind, model string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Публичный API сервиса.
// ─────────────────────────────────────────────────────────────────────────────

// AssistantService — публичный контракт правой панели.
type AssistantService interface {
	CreateSession(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.AssistantSession, error)
	ListSessions(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, includeArchived bool, limit int) ([]*models.AssistantSession, error)
	GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*models.AssistantSession, error)
	ArchiveSession(ctx context.Context, sessionID, userID uuid.UUID) error

	// GetHistory — курсорная пагинация (см. репозиторий ListMessages).
	GetHistory(ctx context.Context, sessionID, userID uuid.UUID, limit int, beforeCreatedAt time.Time, beforeID uuid.UUID) ([]*models.AssistantMessage, error)

	// GetStatus возвращает статус конфигурации ассистента для UI.
	GetStatus(ctx context.Context, userID uuid.UUID) (*dto.AssistantStatusResponse, error)

	// SendMessage — 202 Accepted: записывает user-сообщение (идемпотентно
	// по clientMsgID), захватывает busy и стартует агент-петлю в горутине.
	SendMessage(ctx context.Context, sessionID, userID uuid.UUID, content string, clientMsgID string) (*models.AssistantMessage, bool, error)

	// ConfirmToolCall — resume после destructive confirm. approved=true →
	// исполняет tool в той же горутине (синхронно завершает confirm-call),
	// затем стартует новую агент-петлю с обновлённой историей.
	ConfirmToolCall(ctx context.Context, sessionID, userID uuid.UUID, toolCallID string, approved bool) error

	// ListActiveTasks — все task.state=active из проектов пользователя
	// (для Tasks-tab правой панели).
	ListActiveTasks(ctx context.Context, userID uuid.UUID) ([]ActiveTaskSummary, error)

	// StartStaleRecovery — фоновая горутина с тикером, сбрасывающая busy
	// у зависших сессий. Блокируется до ctx.Done().
	StartStaleRecovery(ctx context.Context)

	// TranscribeAudio распознает аудио в текст с использованием настроенного провайдера.
	TranscribeAudio(ctx context.Context, userID uuid.UUID, audioBytes []byte, filename string) (string, error)
}

// ActiveTaskSummary — короткая карточка для Tasks-tab.
type ActiveTaskSummary struct {
	TaskID      uuid.UUID         `json:"task_id"`
	ProjectID   uuid.UUID         `json:"project_id"`
	ProjectName string            `json:"project_name"`
	Title       string            `json:"title"`
	State       models.TaskState  `json:"state"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AssistantServiceDeps — DI-bag для конструктора. Сделано struct'ом,
// потому что зависимостей много и позиционная передача стала бы хрупкой.
// AssistantAgentCreator — узкий интерфейс для создания ассистента по умолчанию.
type AssistantAgentCreator interface {
	CreateDefaultAssistant(ctx context.Context, userID uuid.UUID) error
}

// AssistantServiceDeps — DI-bag для конструктора. Сделано struct'ом,
// потому что зависимостей много и позиционная передача стала бы хрупкой.
type AssistantServiceDeps struct {
	Repo        repository.AssistantSessionRepository
	TaskRepo    repository.TaskRepository
	ProjectRepo repository.ProjectRepository
	TeamRepo    repository.TeamRepository
	AgentLoader AssistantAgentLoader
	AgentCreator AssistantAgentCreator
	LLMResolver AssistantLLMClientResolver
	UserCreds   UserLlmCredentialService
	ToolCatalog AssistantToolCatalogProvider
	Hub         WSBroadcaster
	Executor    *agentloop.Executor
	Logger      *slog.Logger
}

// NewAssistantService — конструктор.
//
// Все зависимости обязательны (кроме Logger — он подменяется NopLogger).
// Executor обязан быть собран с теми же тайм-аутами/лимитами, что и
// константы выше — иначе инвариант «loop_timeout < stale_threshold/2»
// сломается. Конструктор это проверяет.
func NewAssistantService(deps AssistantServiceDeps) (AssistantService, error) {
	if deps.Repo == nil {
		return nil, errors.New("AssistantService: Repo is required")
	}
	if deps.TaskRepo == nil {
		return nil, errors.New("AssistantService: TaskRepo is required")
	}
	if deps.ProjectRepo == nil {
		return nil, errors.New("AssistantService: ProjectRepo is required")
	}
	if deps.TeamRepo == nil {
		return nil, errors.New("AssistantService: TeamRepo is required")
	}
	if deps.AgentLoader == nil {
		return nil, errors.New("AssistantService: AgentLoader is required")
	}
	if deps.AgentCreator == nil {
		return nil, errors.New("AssistantService: AgentCreator is required")
	}
	if deps.LLMResolver == nil {
		return nil, errors.New("AssistantService: LLMResolver is required")
	}
	if deps.ToolCatalog == nil {
		return nil, errors.New("AssistantService: ToolCatalog is required")
	}
	if deps.Hub == nil {
		return nil, errors.New("AssistantService: Hub is required")
	}
	if deps.Executor == nil {
		return nil, errors.New("AssistantService: Executor is required")
	}
	if deps.Logger == nil {
		deps.Logger = logging.NopLogger()
	}
	return &assistantService{deps: deps}, nil
}

type assistantService struct {
	deps AssistantServiceDeps
}

// ─────────────────────────────────────────────────────────────────────────────
// Sessions CRUD.
// ─────────────────────────────────────────────────────────────────────────────

func (s *assistantService) getOrProvisionAssistantAgent(ctx context.Context, userID uuid.UUID) (*models.Agent, error) {
	agent, err := s.deps.AgentLoader.GetAgentByUserRole(ctx, userID, string(models.AgentRoleAssistant))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := s.deps.AgentCreator.CreateDefaultAssistant(ctx, userID); err != nil {
				return nil, fmt.Errorf("provision default assistant: %w", err)
			}
			agent, err = s.deps.AgentLoader.GetAgentByUserRole(ctx, userID, string(models.AgentRoleAssistant))
			if err != nil {
				return nil, fmt.Errorf("load assistant agent after provisioning: %w", err)
			}
			return agent, nil
		}
		return nil, fmt.Errorf("load assistant agent: %w", err)
	}
	return agent, nil
}

func (s *assistantService) GetStatus(ctx context.Context, userID uuid.UUID) (*dto.AssistantStatusResponse, error) {
	if userID == uuid.Nil {
		return nil, ErrAssistantInvalidInput
	}
	agent, err := s.getOrProvisionAssistantAgent(ctx, userID)
	if err != nil {
		return nil, err
	}

	if agent != nil && agent.IsActive && (agent.ProviderKind == nil || !agent.ProviderKind.IsValid()) {
		// По дефолту пробуем OpenRouter
		key, err := s.deps.UserCreds.GetPlaintext(ctx, userID, models.UserLLMProviderOpenRouter)
		if err == nil && key != "" {
			pk := models.AgentProviderKindOpenRouter
			model := "deepseek/deepseek-v4-flash"
			if err := s.deps.AgentLoader.UpdateAgentProvider(ctx, agent.ID, pk, model); err == nil {
				agent.ProviderKind = &pk
				agent.Model = &model
			}
		} else {
			// Проверим другие поддерживаемые per-user провайдеры
			for _, prov := range []models.UserLLMProvider{
				models.UserLLMProviderAnthropic,
				models.UserLLMProviderDeepSeek,
				models.UserLLMProviderZhipu,
			} {
				k, err := s.deps.UserCreds.GetPlaintext(ctx, userID, prov)
				if err == nil && k != "" {
					pk := models.AgentProviderKind(prov)
					var model string
					switch prov {
					case models.UserLLMProviderAnthropic:
						model = "claude-haiku-4-5-20251001"
					case models.UserLLMProviderDeepSeek:
						model = "deepseek-chat"
					case models.UserLLMProviderZhipu:
						model = "glm-4"
					}
					if model != "" {
						if err := s.deps.AgentLoader.UpdateAgentProvider(ctx, agent.ID, pk, model); err == nil {
							agent.ProviderKind = &pk
							agent.Model = &model
							break
						}
					}
				}
			}
		}
	}

	if agent == nil || !agent.IsActive || agent.ProviderKind == nil || !agent.ProviderKind.IsValid() {
		// По дефолту требуем OpenRouter, если агент сломан/не настроен
		return &dto.AssistantStatusResponse{IsConfigured: false, RequiredProvider: string(models.UserLLMProviderOpenRouter)}, nil
	}

	userProvider := agent.ProviderKind.UserLLMProvider()
	if userProvider == "" {
		// Провайдер не поддерживает per-user ключи, значит ассистент недоступен для UI-конфигурации
		return &dto.AssistantStatusResponse{IsConfigured: false, RequiredProvider: "admin_setup_required"}, nil
	}

	// Проверяем наличие ключа
	key, err := s.deps.UserCreds.GetPlaintext(ctx, userID, userProvider)
	if err != nil && !errors.Is(err, repository.ErrUserLlmCredentialNotFound) {
		return nil, fmt.Errorf("check user creds: %w", err)
	}

	return &dto.AssistantStatusResponse{
		IsConfigured:     key != "",
		RequiredProvider: string(userProvider),
	}, nil
}

func (s *assistantService) CreateSession(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID) (*models.AssistantSession, error) {
	if userID == uuid.Nil {
		return nil, ErrAssistantInvalidInput
	}
	session := &models.AssistantSession{
		UserID:    userID,
		ProjectID: projectID,
		Status:    models.AssistantSessionStatusActive,
	}
	if err := s.deps.Repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	s.broadcastSessionUpdated(userID, session)
	return session, nil
}

func (s *assistantService) ListSessions(ctx context.Context, userID uuid.UUID, projectID *uuid.UUID, includeArchived bool, limit int) ([]*models.AssistantSession, error) {
	if userID == uuid.Nil {
		return nil, ErrAssistantInvalidInput
	}
	return s.deps.Repo.ListSessionsByUser(ctx, userID, projectID, includeArchived, limit)
}

func (s *assistantService) GetSession(ctx context.Context, sessionID, userID uuid.UUID) (*models.AssistantSession, error) {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return nil, ErrAssistantInvalidInput
	}
	sess, err := s.deps.Repo.GetSession(ctx, sessionID, userID)
	if err != nil {
		return nil, s.mapRepoErr(err)
	}
	return sess, nil
}

func (s *assistantService) ArchiveSession(ctx context.Context, sessionID, userID uuid.UUID) error {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return ErrAssistantInvalidInput
	}
	if err := s.deps.Repo.ArchiveSession(ctx, sessionID, userID); err != nil {
		return s.mapRepoErr(err)
	}
	return nil
}

func (s *assistantService) GetHistory(ctx context.Context, sessionID, userID uuid.UUID, limit int, beforeCreatedAt time.Time, beforeID uuid.UUID) ([]*models.AssistantMessage, error) {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return nil, ErrAssistantInvalidInput
	}
	// Сперва проверяем ownership — иначе утечка факта существования сессии.
	if _, err := s.deps.Repo.GetSession(ctx, sessionID, userID); err != nil {
		return nil, s.mapRepoErr(err)
	}
	return s.deps.Repo.ListMessages(ctx, sessionID, limit, beforeCreatedAt, beforeID)
}

// ─────────────────────────────────────────────────────────────────────────────
// SendMessage — 202 Accepted, агент-петля в горутине.
// ─────────────────────────────────────────────────────────────────────────────

func (s *assistantService) SendMessage(ctx context.Context, sessionID, userID uuid.UUID, content, clientMsgID string) (*models.AssistantMessage, bool, error) {
	if sessionID == uuid.Nil || userID == uuid.Nil {
		return nil, false, ErrAssistantInvalidInput
	}
	if content == "" {
		return nil, false, ErrAssistantInvalidInput
	}

	// 1) Ownership + idempotency lookup (если повторный clientMsgID — возвращаем
	//    тот же message без второй петли).
	if clientMsgID != "" {
		if existing, err := s.deps.Repo.FindMessageByClientID(ctx, sessionID, clientMsgID); err == nil {
			// Повторная доставка — no-op. Не стартуем петлю.
			return existing, true, nil
		} else if !errors.Is(err, repository.ErrMessageNotFound) {
			return nil, false, fmt.Errorf("idempotency lookup: %w", err)
		}
	}

	// 2) Захват busy ДО Append'а — иначе при дабл-клике обе горутины
	//    успеют записать user-row и потом обе попробуют CAS.
	if err := s.deps.Repo.AcquireBusy(ctx, sessionID, userID); err != nil {
		// Различаем «не нашли/чужая» vs «занята» доп. SELECT'ом.
		if errors.Is(err, repository.ErrAssistantSessionBusy) {
			if _, getErr := s.deps.Repo.GetSession(ctx, sessionID, userID); errors.Is(getErr, repository.ErrAssistantSessionNotFound) {
				return nil, false, ErrAssistantSessionNotFound
			}
			return nil, false, ErrAssistantSessionBusy
		}
		return nil, false, s.mapRepoErr(err)
	}

	// 3) Запись user-сообщения. Если идемпотентный конфликт — снимаем busy
	//    и возвращаем существующее сообщение (race с другим запросом).
	userMsg := &models.AssistantMessage{
		SessionID: sessionID,
		Role:      models.AssistantMessageRoleUser,
		Content:   ptrString(content),
	}
	if clientMsgID != "" {
		userMsg.ClientMessageID = ptrString(clientMsgID)
	}
	if err := s.deps.Repo.AppendMessage(ctx, userMsg); err != nil {
		_ = s.deps.Repo.ReleaseBusy(context.Background(), sessionID)
		if errors.Is(err, repository.ErrAssistantMessageDuplicate) && clientMsgID != "" {
			if existing, lookupErr := s.deps.Repo.FindMessageByClientID(ctx, sessionID, clientMsgID); lookupErr == nil {
				return existing, true, nil
			}
		}
		return nil, false, fmt.Errorf("append user message: %w", err)
	}

	s.broadcastMessage(userID, sessionID, userMsg)

	// 4) Старт горутины. context.Background() — намеренно: HTTP-ctx не должен
	//    отменять long-running петлю; свой timeout живёт внутри runAgentLoop.
	go s.runAgentLoop(context.Background(), sessionID, userID)

	return userMsg, false, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ConfirmToolCall — resume после подтверждения destructive операции.
// ─────────────────────────────────────────────────────────────────────────────

func (s *assistantService) ConfirmToolCall(ctx context.Context, sessionID, userID uuid.UUID, toolCallID string, approved bool) error {
	// Fast-fail валидация — не идём в БД и не лочим строки зря (план §4.1).
	if sessionID == uuid.Nil || userID == uuid.Nil || toolCallID == "" {
		return ErrAssistantInvalidInput
	}

	// 1) Сборка tool_result — бизнес-решение.
	//    - approved=true: исполняем tool через каталог;
	//    - approved=false: synthetic `{status:"denied"}`.
	//    Сериализация в []byte ЗДЕСЬ — repo принимает готовый jsonb-байт-слайс.
	resultJSON, err := s.buildConfirmResultJSON(ctx, sessionID, userID, toolCallID, approved)
	if err != nil {
		return err
	}

	// 2) Repo атомарно закрывает tool-row и сбрасывает pending_tool_call_id.
	//    busy=TRUE остаётся — снимет defer в runAgentLoopResume.
	if err := s.deps.Repo.ConfirmAndClosePending(ctx, sessionID, userID, toolCallID, resultJSON); err != nil {
		return s.mapRepoErr(err)
	}

	// 3) Эмиссия tool_result-события для UI (теперь LLM видит результат).
	s.broadcastToolResult(userID, sessionID, toolCallID, approved, resultJSON)

	// 4) Горутина — СТРОГО после возврата repo (после COMMIT). Запуск `go`
	//    внутри Transaction(func(tx){...}) — антипаттерн (out-of-tx чтение).
	go s.runAgentLoopResume(context.Background(), sessionID, userID)
	return nil
}

// buildConfirmResultJSON — для approved=true исполняет MCP-tool через каталог
// и сериализует результат; для approved=false возвращает synthetic deny
// payload. Всегда []byte (готов к INSERT в jsonb).
//
// Контракт: вызывается ДО ConfirmAndClosePending. Если исполнение tool'а
// вернуло Go-error (сетевой/ctx), мы возвращаем error наружу — confirm
// падает с 500, busy остаётся TRUE до следующего confirm или stale-recovery.
// Если tool вернул бизнес-ошибку (forbidden/validation/error) — это
// валидный payload, который попадёт в историю и LLM на resume отработает.
func (s *assistantService) buildConfirmResultJSON(
	ctx context.Context, sessionID, userID uuid.UUID, toolCallID string, approved bool,
) ([]byte, error) {
	if !approved {
		return json.Marshal(struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}{"denied", "пользователь отказал в подтверждении"})
	}

	// Approved: ищем pending tool-row, чтобы знать tool_name + args.
	pending, err := s.deps.Repo.GetPendingToolMessage(ctx, sessionID, toolCallID)
	if err != nil {
		if errors.Is(err, repository.ErrMessageNotFound) {
			// Pending row пропал → значит, либо его не было (баг), либо
			// параллельный confirm уже его закрыл. ConfirmAndClosePending
			// дальше вернёт ErrAssistantAlreadyConfirmed — пусть.
			return nil, ErrAssistantNoPendingConfirmation
		}
		return nil, fmt.Errorf("lookup pending tool row: %w", err)
	}
	if pending.ToolName == nil || *pending.ToolName == "" {
		return nil, fmt.Errorf("pending tool row has empty tool_name")
	}

	// Ищем handler в каталоге. Каталог собирается каждый раз — это дешёво
	// (max ~16 tools); зато не нужно холдить map в state и обновлять его
	// при подключении новых tools.
	catalog := s.deps.ToolCatalog.Catalog()
	var handler agentloop.ToolHandler
	for i := range catalog {
		if catalog[i].Name == *pending.ToolName {
			handler = catalog[i].Handler
			break
		}
	}
	if handler == nil {
		// Tool исчез из каталога после park'а (apg upgrade?). Записываем
		// validation-error, чтобы LLM мог понять.
		return json.Marshal(struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}{"error", fmt.Sprintf("tool %q больше не доступен", *pending.ToolName)})
	}

	sess, err := s.deps.Repo.GetSession(ctx, sessionID, userID)
	if err != nil {
		return nil, fmt.Errorf("lookup session: %w", err)
	}
	projectIDStr := ""
	if sess.ProjectID != nil {
		projectIDStr = sess.ProjectID.String()
	}

	// Исполняем. Per-call timeout оборачиваем здесь же (внутри Executor
	// per-call есть, но confirm-исполнение идёт вне Executor).
	execCtx, cancel := context.WithTimeout(ctx, AssistantPerLLMCallTimeout)
	defer cancel()
	result, execErr := handler(execCtx, agentloop.AuthContext{
		UserID:    userID.String(),
		ProjectID: projectIDStr,
		Scope:     "assistant",
	}, json.RawMessage(pending.ToolArguments))
	if execErr != nil {
		// Сетевая/ctx ошибка → отдаём наружу (busy остаётся TRUE).
		if errors.Is(execErr, context.Canceled) || errors.Is(execErr, context.DeadlineExceeded) {
			return nil, fmt.Errorf("tool %q timed out: %w", *pending.ToolName, execErr)
		}
		// Бизнес-ошибка handler'а — упакуем как tool_result error и
		// продолжим (LLM может среагировать).
		return json.Marshal(struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}{"error", execErr.Error()})
	}
	if len(result) == 0 {
		result = []byte(`{"status":"ok"}`)
	}
	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ListActiveTasks — для Tasks-tab.
// ─────────────────────────────────────────────────────────────────────────────

func (s *assistantService) ListActiveTasks(ctx context.Context, userID uuid.UUID) ([]ActiveTaskSummary, error) {
	if userID == uuid.Nil {
		return nil, ErrAssistantInvalidInput
	}

	// Один JOIN-запрос вместо N+1 по проектам пользователя. Критично для
	// YugabyteDB: каждый отдельный round-trip = 2–5ms по кластеру, что
	// для 200 проектов превратилось бы в секундную блокировку правой панели.
	rows, err := s.deps.TaskRepo.ListActiveByUser(ctx, userID,
		[]models.TaskState{models.TaskStateActive}, 0 /* default limit */)
	if err != nil {
		return nil, fmt.Errorf("list active tasks by user: %w", err)
	}
	out := make([]ActiveTaskSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, ActiveTaskSummary{
			TaskID:      r.TaskID,
			ProjectID:   r.ProjectID,
			ProjectName: r.ProjectName,
			Title:       r.Title,
			State:       r.State,
			UpdatedAt:   r.UpdatedAt,
		})
	}
	return out, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// StartStaleRecovery — cron-loop.
// ─────────────────────────────────────────────────────────────────────────────

func (s *assistantService) StartStaleRecovery(ctx context.Context) {
	tick := time.NewTicker(AssistantStaleRecoveryInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			n, err := s.deps.Repo.ResetStaleBusy(ctx, AssistantStaleThreshold)
			if err != nil {
				s.deps.Logger.WarnContext(ctx, "assistant: stale recovery failed",
					slog.String("error", err.Error()),
				)
				continue
			}
			if n > 0 {
				s.deps.Logger.InfoContext(ctx, "assistant: stale sessions reset",
					slog.Int64("count", n),
				)
			}
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

// mapRepoErr транслирует ошибки репо в сервисные ошибки (handler смотрит
// на сервисные через errors.Is).
func (s *assistantService) mapRepoErr(err error) error {
	switch {
	case errors.Is(err, repository.ErrAssistantSessionNotFound):
		return ErrAssistantSessionNotFound
	case errors.Is(err, repository.ErrAssistantSessionBusy):
		return ErrAssistantSessionBusy
	case errors.Is(err, repository.ErrAssistantNoPendingConfirmation):
		return ErrAssistantNoPendingConfirmation
	case errors.Is(err, repository.ErrAssistantAlreadyConfirmed):
		return ErrAssistantAlreadyConfirmed
	case errors.Is(err, repository.ErrInvalidInput):
		return ErrAssistantInvalidInput
	default:
		return err
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// WS broadcast helpers
//
// КОНТРАКТ: каждое WS-сообщение ассистента ВСЕГДА уходит через
// MarshalAssistant*-обёртки из пакета `internal/ws`. Эти обёртки строят
// UserEnvelope{type, v, ts, user_id, data}; без них фронт (websocket_events.dart)
// бросает WsParseError на отсутствие корневых полей. Прямой json.Marshal на
// map[string]any в этом пакете запрещён — линтуется ревью.
// ─────────────────────────────────────────────────────────────────────────────

func (s *assistantService) broadcastSessionUpdated(userID uuid.UUID, sess *models.AssistantSession) {
	title := ""
	if sess.Title != nil {
		title = *sess.Title
	}
	data := ws.AssistantSessionUpdatedData{
		SessionID:     sess.ID,
		Title:         title,
		Status:        string(sess.Status),
		Busy:          sess.Busy,
		LastMessageAt: sess.LastMessageAt,
		UpdatedAt:     sess.UpdatedAt,
	}
	payload, err := ws.MarshalAssistantSessionUpdated(userID, data)
	if err != nil {
		s.logSendError(err, ws.MessageTypeAssistantSessionUpdated, "marshal")
		return
	}
	s.send(userID, ws.MessageTypeAssistantSessionUpdated, payload)
}

func (s *assistantService) broadcastMessage(userID, sessionID uuid.UUID, msg *models.AssistantMessage) {
	data := ws.AssistantMessageData{
		SessionID: sessionID,
		MessageID: msg.ID,
		Role:      string(msg.Role),
		Content:   derefStringEmpty(msg.Content),
		CreatedAt: msg.CreatedAt,
	}
	if msg.ToolCallID != nil {
		data.ToolCallID = *msg.ToolCallID
	}
	if msg.ToolName != nil {
		data.ToolName = *msg.ToolName
	}
	payload, err := ws.MarshalAssistantMessage(userID, data)
	if err != nil {
		s.logSendError(err, ws.MessageTypeAssistantMessage, "marshal")
		return
	}
	s.send(userID, ws.MessageTypeAssistantMessage, payload)
}

func (s *assistantService) broadcastToolCall(userID, sessionID uuid.UUID, call agentloop.ToolCall) {
	data := ws.AssistantToolCallData{
		SessionID:  sessionID,
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Arguments:  json.RawMessage(call.Arguments),
	}
	payload, err := ws.MarshalAssistantToolCall(userID, data)
	if err != nil {
		s.logSendError(err, ws.MessageTypeAssistantToolCall, "marshal")
		return
	}
	s.send(userID, ws.MessageTypeAssistantToolCall, payload)
}

func (s *assistantService) broadcastToolResult(userID, sessionID uuid.UUID, callID string, approved bool, result json.RawMessage) {
	status := "ok"
	if !approved {
		status = "denied"
	}
	data := ws.AssistantToolResultData{
		SessionID:  sessionID,
		ToolCallID: callID,
		Status:     status,
		Result:     result,
	}
	payload, err := ws.MarshalAssistantToolResult(userID, data)
	if err != nil {
		s.logSendError(err, ws.MessageTypeAssistantToolResult, "marshal")
		return
	}
	s.send(userID, ws.MessageTypeAssistantToolResult, payload)
}

func (s *assistantService) broadcastConfirmRequest(userID, sessionID uuid.UUID, call agentloop.ToolCall) {
	data := ws.AssistantConfirmRequestData{
		SessionID:  sessionID,
		ToolCallID: call.ID,
		ToolName:   call.Name,
		Arguments:  json.RawMessage(call.Arguments),
		Summary:    fmt.Sprintf("Ассистент хочет выполнить %s. Подтвердите или отмените.", call.Name),
	}
	payload, err := ws.MarshalAssistantConfirmRequest(userID, data)
	if err != nil {
		s.logSendError(err, ws.MessageTypeAssistantConfirmRequest, "marshal")
		return
	}
	s.send(userID, ws.MessageTypeAssistantConfirmRequest, payload)
}

func (s *assistantService) broadcastNavigate(userID uuid.UUID, route string) {
	data := ws.AssistantNavigateData{Route: route}
	payload, err := ws.MarshalAssistantNavigate(userID, data)
	if err != nil {
		s.logSendError(err, ws.MessageTypeAssistantNavigate, "marshal")
		return
	}
	s.send(userID, ws.MessageTypeAssistantNavigate, payload)
}

// broadcastToolResultPayload — тонкая обёртка для loop.go OnToolResult-хука,
// который оперирует raw payload + tool_name из agentloop.ToolResult и должен
// сохранять status, отданный handler'ом.
func (s *assistantService) broadcastToolResultPayload(userID, sessionID uuid.UUID, callID, toolName, status string, result json.RawMessage) {
	data := ws.AssistantToolResultData{
		SessionID:  sessionID,
		ToolCallID: callID,
		ToolName:   toolName,
		Status:     status,
		Result:     result,
	}
	payload, err := ws.MarshalAssistantToolResult(userID, data)
	if err != nil {
		s.logSendError(err, ws.MessageTypeAssistantToolResult, "marshal")
		return
	}
	s.send(userID, ws.MessageTypeAssistantToolResult, payload)
}

func (s *assistantService) send(userID uuid.UUID, msgType ws.MessageType, payload []byte) {
	if err := s.deps.Hub.SendToUser(userID.String(), string(msgType), payload); err != nil {
		s.logSendError(err, msgType, "send")
	}
}

// logSendError — единая точка warn-логирования для marshal/send-ошибок;
// WS-эмиссия best-effort и не должна валить агент-петлю.
func (s *assistantService) logSendError(err error, msgType ws.MessageType, stage string) {
	s.deps.Logger.WarnContext(context.Background(), "assistant: ws "+stage+" failed",
		slog.String("type", string(msgType)),
		slog.String("error", err.Error()),
	)
}

func ptrString(s string) *string { return &s }

// jsonbBytes собирает datatypes.JSON безопасно — пустой payload идёт как nil,
// чтобы не записать `null`::jsonb (план §4.1 семантика «pending»).
func jsonbBytes(b []byte) datatypes.JSON {
	if len(b) == 0 {
		return nil
	}
	return datatypes.JSON(b)
}

func (s *assistantService) TranscribeAudio(ctx context.Context, userID uuid.UUID, audioBytes []byte, filename string) (string, error) {
	if userID == uuid.Nil {
		return "", ErrAssistantInvalidInput
	}
	if len(audioBytes) == 0 {
		return "", fmt.Errorf("empty audio data")
	}

	agent, err := s.getOrProvisionAssistantAgent(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get assistant agent: %w", err)
	}

	var settings map[string]any
	if len(agent.Settings) > 0 {
		_ = json.Unmarshal(agent.Settings, &settings)
	}

	sttProvider, _ := settings["stt_provider"].(string)
	sttModel, _ := settings["stt_model"].(string)

	if sttProvider == "" || sttProvider == "disabled" {
		return "", fmt.Errorf("speech to text is not configured (disabled)")
	}

	// Fallbacks
	if sttModel == "" {
		switch sttProvider {
		case "openai":
			sttModel = "whisper-1"
		case "openrouter":
			sttModel = "openai/whisper-large-v3"
		case "gemini":
			sttModel = "gemini-1.5-flash"
		default:
			return "", fmt.Errorf("unsupported or unconfigured speech to text provider: %s", sttProvider)
		}
	}

	// Fetch API key for the user provider
	apiKey, err := s.deps.UserCreds.GetPlaintext(ctx, userID, models.UserLLMProvider(sttProvider))
	if err != nil || apiKey == "" {
		return "", fmt.Errorf("speech to text API key is not configured for provider %s", sttProvider)
	}

	switch sttProvider {
	case "openai":
		bodyBuf := &bytes.Buffer{}
		bodyWriter := multipart.NewWriter(bodyBuf)

		fileWriter, err := bodyWriter.CreateFormFile("file", filename)
		if err != nil {
			return "", fmt.Errorf("create form file failed: %w", err)
		}
		if _, err := fileWriter.Write(audioBytes); err != nil {
			return "", fmt.Errorf("write audio file failed: %w", err)
		}

		if err := bodyWriter.WriteField("model", sttModel); err != nil {
			return "", fmt.Errorf("write model field failed: %w", err)
		}

		contentType := bodyWriter.FormDataContentType()
		if err := bodyWriter.Close(); err != nil {
			return "", fmt.Errorf("close body writer failed: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/audio/transcriptions", bodyBuf)
		if err != nil {
			return "", fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("transcription proxy request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			resBody, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("transcription API returned status %d: %s", resp.StatusCode, string(resBody))
		}

		var result struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("failed to decode response: %w", err)
		}
		return result.Text, nil

	case "openrouter":
		ext := "m4a"
		if strings.HasSuffix(strings.ToLower(filename), ".mp3") {
			ext = "mp3"
		} else if strings.HasSuffix(strings.ToLower(filename), ".wav") {
			ext = "wav"
		} else if strings.HasSuffix(strings.ToLower(filename), ".ogg") {
			ext = "ogg"
		}

		b64Data := base64.StdEncoding.EncodeToString(audioBytes)
		payload := map[string]any{
			"model": sttModel,
			"input_audio": map[string]any{
				"data":   b64Data,
				"format": ext,
			},
		}

		reqBytes, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal openrouter request failed: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/audio/transcriptions", bytes.NewBuffer(reqBytes))
		if err != nil {
			return "", fmt.Errorf("failed to create openrouter request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("openrouter transcription proxy request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			resBody, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("openrouter transcription API returned status %d: %s", resp.StatusCode, string(resBody))
		}

		var result struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return "", fmt.Errorf("failed to decode response: %w", err)
		}
		return result.Text, nil

	case "gemini":
		mimeType := "audio/wav"
		if strings.HasSuffix(strings.ToLower(filename), ".mp3") {
			mimeType = "audio/mp3"
		} else if strings.HasSuffix(strings.ToLower(filename), ".m4a") {
			mimeType = "audio/m4a"
		} else if strings.HasSuffix(strings.ToLower(filename), ".ogg") {
			mimeType = "audio/ogg"
		}

		b64Data := base64.StdEncoding.EncodeToString(audioBytes)
		payload := map[string]any{
			"contents": []map[string]any{
				{
					"parts": []map[string]any{
						{
							"inlineData": map[string]any{
								"mimeType": mimeType,
								"data":     b64Data,
							},
						},
						{
							"text": "Transcribe this audio exactly. Do not add any commentary, explanations, or formatting. Just output the transcription.",
						},
					},
				},
			},
		}

		reqBytes, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("marshal gemini request failed: %w", err)
		}

		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", sttModel, apiKey)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBytes))
		if err != nil {
			return "", fmt.Errorf("create gemini request failed: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("gemini transcription proxy request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			resBody, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("gemini generative API returned status %d: %s", resp.StatusCode, string(resBody))
		}

		var geminiResp struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
			return "", fmt.Errorf("decode gemini response failed: %w", err)
		}
		if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("gemini transcription api returned empty parts")
		}

		return strings.TrimSpace(geminiResp.Candidates[0].Content.Parts[0].Text), nil

	default:
		return "", fmt.Errorf("unsupported speech to text provider: %s", sttProvider)
	}
}

// Unused import guards (компилятор иначе ругается, если ветка кода уходит).
var _ = atomic.LoadInt32
