package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/metrics"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/async"
	"github.com/google/uuid"
)

var (
	ErrConversationNotFound     = errors.New("conversation not found")
	ErrConversationForbidden    = errors.New("access to conversation denied")
	ErrInvalidConversationTitle = errors.New("conversation title is required and must be less than 255 characters")
	ErrInvalidMessageContent    = errors.New("message content is required and must be less than 4096 characters")
	ErrMessageRateLimit         = errors.New("message rate limit exceeded, please wait")
	ErrDuplicateMessage         = errors.New("duplicate message")
)

// ConversationService интерфейс для работы с чатами
type ConversationService interface {
	// CreateConversation создает новый чат для указанного проекта.
	// Обязательно проверяет права доступа userID к projectID.
	CreateConversation(ctx context.Context, userID, projectID uuid.UUID, title string) (*models.Conversation, error)

	// GetConversation возвращает чат по ID.
	// Обязательно проверяет права доступа userID к чату.
	GetConversation(ctx context.Context, userID, id uuid.UUID) (*models.Conversation, error)

	// ListConversations возвращает список чатов проекта с пагинацией.
	// Обязательно проверяет права доступа userID к проекту.
	ListConversations(ctx context.Context, userID, projectID uuid.UUID, limit, offset int) ([]*models.Conversation, int64, error)

	// SendMessage отправляет сообщение пользователя в чат и запускает процесс оркестрации.
	// Обязательно проверяет права доступа userID к чату.
	// clientMsgID используется для идемпотентности.
	SendMessage(ctx context.Context, userID, conversationID uuid.UUID, content string, clientMsgID uuid.UUID) (*models.ConversationMessage, error)

	// GetHistory возвращает историю сообщений чата.
	// Обязательно проверяет права доступа userID к чату.
	GetHistory(ctx context.Context, userID, conversationID uuid.UUID, limit, offset int) ([]*models.ConversationMessage, int64, error)

	// DeleteConversation удаляет чат и все его сообщения.
	// Обязательно проверяет права доступа userID к чату.
	DeleteConversation(ctx context.Context, userID, id uuid.UUID) error

	// DeleteMessage удаляет сообщение из чата.
	// Обязательно проверяет права доступа userID к чату.
	DeleteMessage(ctx context.Context, userID, conversationID, messageID uuid.UUID) error

	// Shutdown дожидается завершения всех активных процессов оркестрации.
	Shutdown(ctx context.Context) error
}

// TaskOrchestrator — минимальный интерфейс оркестратора задач для conversation_service.
// Sprint 17: заменяет legacy OrchestratorService.ProcessTask на v2 enqueue-flow.
// Реализуется *service.Orchestrator (см. orchestrator_v2.go) через EnqueueInitialStep.
type TaskOrchestrator interface {
	EnqueueInitialStep(ctx context.Context, taskID uuid.UUID) error
}

type conversationService struct {
	convRepo        repository.ConversationRepository
	msgRepo         repository.ConversationMessageRepository
	projectSvc      ProjectService
	taskSvc         TaskService
	orchestratorSvc TaskOrchestrator
	indexer         indexer.ConversationIndexer
	txManager       repository.TransactionManager
	eventBus        events.EventBus

	// Для идемпотентности по UUID
	processedMessages   map[uuid.UUID]*models.ConversationMessage
	processedMessagesMu sync.RWMutex

	wg       sync.WaitGroup
	stopChan chan struct{}
	once     sync.Once
}

type lastMessageInfo struct {
	contentHash string
	timestamp   time.Time
}

// NewConversationService создает новый экземпляр ConversationService
func NewConversationService(
	convRepo repository.ConversationRepository,
	msgRepo repository.ConversationMessageRepository,
	projectSvc ProjectService,
	taskSvc TaskService,
	orchestratorSvc TaskOrchestrator,
	indexer indexer.ConversationIndexer,
	txManager repository.TransactionManager,
	eventBus events.EventBus,
) ConversationService {
	s := &conversationService{
		convRepo:          convRepo,
		msgRepo:           msgRepo,
		projectSvc:        projectSvc,
		taskSvc:           taskSvc,
		orchestratorSvc:   orchestratorSvc,
		indexer:           indexer,
		txManager:         txManager,
		eventBus:          eventBus,
		processedMessages: make(map[uuid.UUID]*models.ConversationMessage),
		stopChan:          make(chan struct{}),
	}

	// Запуск фоновой очистки processedMessages для предотвращения утечки памяти
	go s.cleanupProcessedMessagesLoop()

	return s
}

// Shutdown дожидается завершения всех активных процессов оркестрации
func (s *conversationService) Shutdown(ctx context.Context) error {
	s.once.Do(func() {
		close(s.stopChan)
	})

	c := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(c)
	}()

	select {
	case <-c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateConversation создает новый чат
func (s *conversationService) CreateConversation(ctx context.Context, userID, projectID uuid.UUID, title string) (*models.Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 255 {
		return nil, ErrInvalidConversationTitle
	}

	if err := s.checkProjectAccess(ctx, userID, projectID); err != nil {
		return nil, err
	}

	conv := &models.Conversation{
		ProjectID: projectID,
		UserID:    userID,
		Title:     title,
		Status:    models.ConversationStatusActive,
	}

	if err := s.convRepo.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conv, nil
}

// GetConversation возвращает чат
func (s *conversationService) GetConversation(ctx context.Context, userID, id uuid.UUID) (*models.Conversation, error) {
	conv, err := s.checkConversationAccess(ctx, userID, id, false)
	if err != nil {
		return nil, err
	}
	return conv, nil
}

// ListConversations возвращает список чатов проекта
func (s *conversationService) ListConversations(ctx context.Context, userID, projectID uuid.UUID, limit, offset int) ([]*models.Conversation, int64, error) {
	if err := s.checkProjectAccess(ctx, userID, projectID); err != nil {
		return nil, 0, err
	}

	limit, offset = normalizePagination(limit, offset)

	filter := repository.ConversationFilter{
		UserID: &userID,
		Limit:  limit,
		Offset: offset,
	}

	return s.convRepo.ListByProjectID(ctx, projectID, filter)
}

// SendMessage отправляет сообщение и запускает оркестрацию
func (s *conversationService) SendMessage(ctx context.Context, userID, conversationID uuid.UUID, content string, clientMsgID uuid.UUID) (*models.ConversationMessage, error) {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 4096 {
		return nil, ErrInvalidMessageContent
	}

	conv, err := s.checkConversationAccess(ctx, userID, conversationID, true)
	if err != nil {
		return nil, err
	}

	// Идемпотентность по clientMsgID
	if clientMsgID != uuid.Nil {
		s.processedMessagesMu.RLock()
		if msg, ok := s.processedMessages[clientMsgID]; ok {
			s.processedMessagesMu.RUnlock()
			return msg, ErrDuplicateMessage
		}
		s.processedMessagesMu.RUnlock()
	}

	msg := &models.ConversationMessage{
		ConversationID: conversationID,
		Role:           models.ConversationRoleUser,
		Content:        content,
	}

	// Убрана лишняя транзакция для одиночного Insert
	if err := s.msgRepo.Create(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	// Публикуем событие создания сообщения
	s.eventBus.Publish(ctx, events.ConversationMessageCreated{
		ProjectID:      conv.ProjectID,
		UserID:         userID,
		ConversationID: conversationID,
		MessageID:      msg.ID,
		Role:           string(msg.Role),
		Content:        msg.Content,
		OccurredAt:     time.Now(),
		TraceID:        getTraceID(ctx),
	})

	// Сохраняем для идемпотентности
	if clientMsgID != uuid.Nil {
		s.processedMessagesMu.Lock()
		s.processedMessages[clientMsgID] = msg
		s.processedMessagesMu.Unlock()
	}

	// Индексируем сообщение пользователя
	s.indexMessageAsync(ctx, conv, msg, "")

	// Запуск оркестрации в защищенной горутине с поддержкой Graceful Shutdown
	s.wg.Add(1)
	go s.runOrchestrator(context.WithoutCancel(ctx), userID, conv.ProjectID, conversationID, content)

	return msg, nil
}

// GetHistory возвращает историю сообщений
func (s *conversationService) GetHistory(ctx context.Context, userID, conversationID uuid.UUID, limit, offset int) ([]*models.ConversationMessage, int64, error) {
	if _, err := s.checkConversationAccess(ctx, userID, conversationID, false); err != nil {
		return nil, 0, err
	}

	limit, offset = normalizePagination(limit, offset)

	filter := repository.MessageFilter{
		Limit:  limit,
		Offset: offset,
	}

	return s.msgRepo.ListByConversationID(ctx, conversationID, filter)
}

// DeleteConversation удаляет чат
func (s *conversationService) DeleteConversation(ctx context.Context, userID, id uuid.UUID) error {
	conv, err := s.checkConversationAccess(ctx, userID, id, true)
	if err != nil {
		return err
	}

	// Sprint 15.N1: возвращаем результат транзакции в переменную, чтобы вызов
	// deleteConversationAsync после успешного commit'а перестал быть unreachable
	// (go vet ловит это; в предыдущей итерации регрессия скрыла очистку Weaviate-индекса).
	if err := s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.convRepo.Delete(txCtx, conv.ProjectID, id); err != nil {
			return err
		}

		// Публикуем событие удаления чата
		s.eventBus.Publish(ctx, events.ConversationDeleted{
			ProjectID:      conv.ProjectID,
			ConversationID: id,
			OccurredAt:     time.Now(),
			TraceID:        getTraceID(ctx),
		})
		return nil
	}); err != nil {
		return err
	}

	// Удаляем из индекса после успешного коммита (вне транзакции, чтобы Weaviate-сбой не откатывал DELETE).
	s.deleteConversationAsync(ctx, conv.ProjectID, id)
	return nil
}

// DeleteMessage удаляет сообщение
func (s *conversationService) DeleteMessage(ctx context.Context, userID, conversationID, messageID uuid.UUID) error {
	conv, err := s.checkConversationAccess(ctx, userID, conversationID, true)
	if err != nil {
		return err
	}

	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.msgRepo.Delete(txCtx, conversationID, messageID); err != nil {
			return err
		}

		// Публикуем событие удаления сообщения
		s.eventBus.Publish(ctx, events.ConversationMessageDeleted{
			ProjectID:      conv.ProjectID,
			ConversationID: conversationID,
			MessageID:      messageID,
			OccurredAt:     time.Now(),
			TraceID:        getTraceID(ctx),
		})
		return nil
	})

	if err != nil {
		return err
	}

	// Удаляем из индекса после успешного коммита
	s.deleteMessageAsync(ctx, conv.ProjectID, messageID)

	return nil
}

// Приватные хелперы

func getTraceID(ctx context.Context) string {
	// Предположим, TraceID лежит в контексте под определенным ключом.
	// В реальном проекте здесь будет обращение к библиотеке трассировки (например, OpenTelemetry).
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

type contextKey string

const traceIDKey contextKey = "trace_id"

func (s *conversationService) checkProjectAccess(ctx context.Context, userID, projectID uuid.UUID) error {
	err := s.projectSvc.HasAccess(ctx, userID, models.RoleUser, projectID)
	if err != nil {
		if errors.Is(err, ErrProjectForbidden) {
			return ErrConversationForbidden
		}
		if errors.Is(err, ErrProjectNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	return nil
}

func (s *conversationService) checkConversationAccess(ctx context.Context, userID, conversationID uuid.UUID, master bool) (*models.Conversation, error) {
	conv, err := s.convRepo.GetOnlyByID(ctx, conversationID, master)
	if err != nil {
		if errors.Is(err, repository.ErrConversationNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}

	if conv.UserID != userID {
		return nil, ErrConversationForbidden
	}

	return conv, nil
}

func (s *conversationService) cleanupProcessedMessagesLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.cleanupOldMessages()
		}
	}
}

func (s *conversationService) cleanupOldMessages() {
	s.processedMessagesMu.Lock()
	defer s.processedMessagesMu.Unlock()
	now := time.Now()
	for id, msg := range s.processedMessages {
		// Очищаем через 10 минут
		if now.Sub(msg.CreatedAt) > 10*time.Minute {
			delete(s.processedMessages, id)
		}
	}
}

func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *conversationService) runOrchestrator(ctx context.Context, userID, projectID, conversationID uuid.UUID, content string) {
	defer s.wg.Done()

	var success bool
	var assistantMsg *models.ConversationMessage

	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in orchestration flow",
				"userID", userID,
				"projectID", projectID,
				"conversationID", conversationID,
				"error", r,
				"stack", string(debug.Stack()))
			success = false
		}

		if !success {
			// TODO: Обновить статус сообщения/разговора на Failed в БД,
			// чтобы UI не оставался в состоянии вечного ожидания.
			// s.markMessageFailed(ctx, conversationID, ...)
		} else if assistantMsg != nil {
			// Индексируем ответ ассистента после успешного завершения оркестрации
			// Загружаем Conversation для передачи в индексатор (N+1 protection)
			conv, err := s.convRepo.GetOnlyByID(ctx, conversationID, true)
			if err == nil {
				s.indexMessageAsync(ctx, conv, assistantMsg, content)
			}
		}
	}()

	// 1. Создание задачи для оркестратора
	taskReq := dto.CreateTaskRequest{
		Title:       fmt.Sprintf("Chat Request: %s", truncateRunes(content, 50)),
		Description: content,
		Priority:    string(models.TaskPriorityMedium),
	}

	task, err := s.taskSvc.Create(ctx, userID, models.RoleUser, projectID, taskReq)
	if err != nil {
		slog.Error("Failed to create task for orchestration",
			"userID", userID,
			"projectID", projectID,
			"conversationID", conversationID,
			"error", err)
		return
	}

	// 2. Sprint 17 / Orchestration v2: enqueue первого step_req в durable очередь.
	// StepWorker подберёт его, вызовет Orchestrator.Step → Router → fan-out агентов.
	// В отличие от legacy ProcessTask, эта операция мгновенная (только INSERT в task_events).
	if err := s.orchestratorSvc.EnqueueInitialStep(ctx, task.ID); err != nil {
		slog.Error("Orchestrator failed to enqueue initial step",
			"userID", userID,
			"projectID", projectID,
			"taskID", task.ID,
			"conversationID", conversationID,
			"error", err)
		return
	}

	// 3. Получаем созданный ассистентом ответ
	messages, _, err := s.msgRepo.ListByConversationID(ctx, conversationID, repository.MessageFilter{
		Limit:    1,
		OrderBy:  "created_at",
		OrderDir: "desc",
	})
	if err == nil && len(messages) > 0 && messages[0].Role == models.ConversationRoleAssistant {
		assistantMsg = messages[0]
	}

	success = true
}

func (s *conversationService) indexMessageAsync(ctx context.Context, conv *models.Conversation, msg *models.ConversationMessage, userPrompt string) {
	if conv == nil || msg == nil {
		return
	}

	// Глубокое копирование моделей для предотвращения data race
	convCopy := *conv
	msgCopy := *msg

	async.ExecuteWithRetry(ctx, &s.wg, async.TaskOptions{
		Timeout: 1 * time.Minute,
		Retries: 3,
		LogTags: map[string]any{
			"project_id":      convCopy.ProjectID,
			"conversation_id": convCopy.ID,
			"message_id":      msgCopy.ID,
			"action":          "index_message",
		},
		OnSuccess: func() {
			metrics.IncAsyncTask("index_message", "success")
		},
		OnFailure: func(err error) {
			metrics.IncAsyncTask("index_message", "error")
		},
	}, func(idxCtx context.Context) error {
		return s.indexer.IndexMessageFromModel(idxCtx, &convCopy, &msgCopy, userPrompt)
	})
}

func (s *conversationService) deleteMessageAsync(ctx context.Context, projectID, messageID uuid.UUID) {
	async.ExecuteWithRetry(ctx, &s.wg, async.TaskOptions{
		Timeout: 1 * time.Minute,
		Retries: 3,
		LogTags: map[string]any{
			"project_id": projectID,
			"message_id": messageID,
			"action":     "delete_message",
		},
		OnSuccess: func() {
			metrics.IncAsyncTask("delete_message", "success")
		},
		OnFailure: func(err error) {
			metrics.IncAsyncTask("delete_message", "error")
		},
	}, func(idxCtx context.Context) error {
		return s.indexer.DeleteMessage(idxCtx, projectID, messageID)
	})
}

func (s *conversationService) deleteConversationAsync(ctx context.Context, projectID, conversationID uuid.UUID) {
	async.ExecuteWithRetry(ctx, &s.wg, async.TaskOptions{
		Timeout: 2 * time.Minute,
		Retries: 3,
		LogTags: map[string]any{
			"project_id":      projectID,
			"conversation_id": conversationID,
			"action":          "delete_conversation",
		},
		OnSuccess: func() {
			metrics.IncAsyncTask("delete_conversation", "success")
		},
		OnFailure: func(err error) {
			metrics.IncAsyncTask("delete_conversation", "error")
		},
	}, func(idxCtx context.Context) error {
		return s.indexer.DeleteConversation(idxCtx, projectID, conversationID)
	})
}

func truncateRunes(s string, n int) string {
	count := 0
	for i := range s {
		if count == n {
			return s[:i] + "..."
		}
		count++
	}
	return s
}
