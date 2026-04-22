package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/pkoukk/tiktoken-go"
	"github.com/sony/gobreaker"
)

const (
	ConversationMaxTokensPerChunk   = 512
	ConversationChunkOverlap        = 50
	ConversationMaxUserPromptTokens = 200 // Лимит на контекст вопроса пользователя в чанке ответа
)

type contextKey string

const traceIDKey contextKey = "trace_id"

type conversationIndexer struct {
	convRepo   repository.ConversationRepository
	msgRepo    repository.ConversationMessageRepository
	vectorRepo repository.VectorRepository
	eventBus   events.EventBus
	logger     *slog.Logger
	tokenizer  *tiktoken.Tiktoken
	semaphore  chan struct{}
	wg         sync.WaitGroup
	cb         *gobreaker.CircuitBreaker
	stopChan   chan struct{}
}

// NewConversationIndexer создает новый экземпляр ConversationIndexer
func NewConversationIndexer(
	convRepo repository.ConversationRepository,
	msgRepo repository.ConversationMessageRepository,
	vectorRepo repository.VectorRepository,
	eventBus events.EventBus,
	logger *slog.Logger,
) (ConversationIndexer, error) {
	tkm, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding: %w", err)
	}

	// Настройка Circuit Breaker для Weaviate
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name: "weaviate-indexer",
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("Circuit Breaker state changed", "name", name, "from", from.String(), "to", to.String())
		},
	})

	return &conversationIndexer{
		convRepo:   convRepo,
		msgRepo:    msgRepo,
		vectorRepo: vectorRepo,
		eventBus:   eventBus,
		logger:     logger,
		tokenizer:  tkm,
		semaphore:  make(chan struct{}, 50), // Лимит одновременных запросов к Weaviate
		cb:         cb,
		stopChan:   make(chan struct{}),
	}, nil
}

func (i *conversationIndexer) Start(ctx context.Context) error {
	// 1. Healthcheck Weaviate при старте
	if _, err := i.vectorRepo.CountByContentType(ctx, "00000000-0000-0000-0000-000000000000", "healthcheck", ""); err != nil && !strings.Contains(err.Error(), "not found") {
		// Мы не падаем, если Weaviate недоступен, но логируем ошибку.
		// CountByContentType для несуществующего проекта может вернуть ошибку, это нормально.
		i.logger.Warn("Weaviate healthcheck failed or collection not ready", "error", err)
	}

	ch, unsubscribe := i.eventBus.Subscribe("conversation_indexer", 100)
	
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		defer unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return
			case <-i.stopChan:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				// Асинхронная обработка событий для предотвращения блокировки Event Loop
				i.wg.Add(1)
				go func(event events.DomainEvent) {
					defer i.wg.Done()
					i.handleEvent(ctx, event)
				}(ev)
			}
		}
	}()

	return nil
}

func (i *conversationIndexer) Stop() {
	close(i.stopChan)
	i.wg.Wait()
}

func (i *conversationIndexer) handleEvent(ctx context.Context, ev events.DomainEvent) {
	// Используем фоновый контекст для обработки событий, чтобы не прерывать при отмене исходного ctx
	// Но с таймаутом, как требует ТЗ. TraceID прокидываем из события.
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	if ev.GetTraceID() != "" {
		bgCtx = context.WithValue(bgCtx, traceIDKey, ev.GetTraceID())
	}

	var err error
	switch e := ev.(type) {
	case events.ConversationMessageCreated:
		err = i.IndexMessage(bgCtx, e.ProjectID, e.ConversationID, e.MessageID)
	case events.ConversationMessageDeleted:
		err = i.DeleteMessage(bgCtx, e.ProjectID, e.MessageID)
	case events.ConversationDeleted:
		err = i.DeleteConversation(bgCtx, e.ProjectID, e.ConversationID)
	case events.ProjectDeleted:
		err = i.vectorRepo.DeleteByContentType(bgCtx, e.ProjectID.String(), "conversation_message", "conversation")
	case events.UserDeleted:
		i.logger.Info("user deleted event received, manual cleanup might be required for vectors", "user_id", e.UserID)
	}

	if err != nil {
		// Унифицированная обработка ошибок через vectordb хелперы (предположим они есть в pkg/vectordb)
		// Если их нет, используем обычное логирование.
		i.logger.Error("failed to handle event", "event_type", fmt.Sprintf("%T", ev), "error", err)
	}
}

func (i *conversationIndexer) IndexMessage(ctx context.Context, projectID, conversationID, messageID uuid.UUID) error {
	// Чтение актуальных данных строго из Master-узла БД (Replication Lag)
	conv, err := i.convRepo.GetByID(ctx, projectID, conversationID, true)
	if err != nil {
		return fmt.Errorf("failed to get conversation: %w", err)
	}

	msg, err := i.msgRepo.GetByID(ctx, conversationID, messageID, true)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Если это ответ ассистента, нам нужен контекст вопроса пользователя
	var userPrompt string
	if msg.Role == models.ConversationRoleAssistant {
		// Ищем последнее сообщение пользователя перед этим
		messages, _, err := i.msgRepo.ListByConversationID(ctx, conversationID, repository.MessageFilter{
			Limit:    10,
			OrderBy:  "created_at",
			OrderDir: "desc",
		})
		if err == nil {
			for _, m := range messages {
				if m.CreatedAt.Before(msg.CreatedAt) && m.Role == models.ConversationRoleUser {
					userPrompt = m.Content
					break
				}
			}
		}
	}

	return i.indexMessageWithData(ctx, conv, msg, userPrompt)
}

func (i *conversationIndexer) indexMessageWithData(ctx context.Context, conv *models.Conversation, msg *models.ConversationMessage, userPrompt string) error {
	// Санитаризация
	content := i.sanitizeText(msg.Content)
	if strings.TrimSpace(content) == "" {
		return nil
	}

	// Чанкирование
	chunks := i.buildChunks(content, msg.Role, userPrompt)

	for idx, chunk := range chunks {
		// Детерминированный UUID для Weaviate (Upsert)
		// uuid.NewMD5(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s_%d", msg.ID, idx)))
		chunkID := uuid.NewMD5(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s_%d", msg.ID, idx))).String()

		doc := models.NewVectorDocument(msg.ID.String(), chunk, "conversation_message")
		doc.ID = chunkID // Устанавливаем детерминированный ID для Upsert
		doc.Category = "conversation"
		doc.SetMetadata("message_id", msg.ID.String())
		doc.SetMetadata("conversation_id", conv.ID.String())
		doc.SetMetadata("project_id", conv.ProjectID.String())
		doc.SetMetadata("user_id", conv.UserID.String())
		doc.SetMetadata("role", string(msg.Role))
		doc.SetMetadata("created_at", msg.CreatedAt.Format(time.RFC3339))
		
		if len(chunks) > 1 {
			doc.SetMetadata("chunk_index", idx)
			doc.SetMetadata("total_chunks", len(chunks))
		}

		// Защита Weaviate через Semaphore и Circuit Breaker
		_, err := i.cb.Execute(func() (interface{}, error) {
			i.semaphore <- struct{}{}
			defer func() { <-i.semaphore }()
			
			return i.vectorRepo.Create(ctx, conv.ProjectID.String(), doc)
		})
		
		if err != nil {
			return fmt.Errorf("failed to create vector document for chunk %d: %w", idx, err)
		}
	}

	return nil
}

func (i *conversationIndexer) DeleteMessage(ctx context.Context, projectID, messageID uuid.UUID) error {
	return i.vectorRepo.DeleteByContentID(ctx, projectID.String(), messageID.String())
}

func (i *conversationIndexer) DeleteConversation(ctx context.Context, projectID, conversationID uuid.UUID) error {
	// Исправлено: Удаление только ОДНОГО чата, а не всего проекта.
	// Weaviate не поддерживает удаление по метаданным напрямую через DeleteByContentID.
	// Нам нужно найти все message_id этого чата и удалить их.
	
	// GDPR: Удаление всех сообщений чата с пагинацией
	offset := 0
	limit := 1000
	for {
		messages, _, err := i.msgRepo.ListByConversationID(ctx, conversationID, repository.MessageFilter{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return err
		}
		
		if len(messages) == 0 {
			break
		}
		
		for _, m := range messages {
			if err := i.vectorRepo.DeleteByContentID(ctx, projectID.String(), m.ID.String()); err != nil {
				i.logger.Warn("failed to delete message vectors", "message_id", m.ID, "error", err)
			}
		}

		if len(messages) < limit {
			break
		}
		offset += limit
	}
	
	return nil
}

func (i *conversationIndexer) IndexProjectConversations(ctx context.Context, projectID uuid.UUID) error {
	// Keyset pagination (без OFFSET) по сообщениям
	var lastID *uuid.UUID
	batchSize := 100

	// Кэш последнего сообщения пользователя для каждого чата (для борьбы с N+1)
	lastUserPromptCache := make(map[uuid.UUID]string)

	for {
		// Eager Loading (Preload) чатов внутри ListByProjectID
		messages, err := i.msgRepo.ListByProjectID(ctx, projectID, lastID, batchSize, true)
		if err != nil {
			return fmt.Errorf("failed to list messages for batch indexing: %w", err)
		}

		if len(messages) == 0 {
			break
		}

		for _, msg := range messages {
			if msg.Conversation == nil {
				i.logger.Error("message without conversation in batch", "message_id", msg.ID)
				continue
			}

			// Если это сообщение пользователя, обновляем кэш
			if msg.Role == models.ConversationRoleUser {
				lastUserPromptCache[msg.ConversationID] = msg.Content
			}

			// Для ответов ассистента ищем контекст вопроса
			var userPrompt string
			if msg.Role == models.ConversationRoleAssistant {
				var ok bool
				userPrompt, ok = lastUserPromptCache[msg.ConversationID]
				if !ok {
					// Если в кэше нет (например, сообщение ассистента первое в батче),
					// делаем точечный запрос (это допустимый fallback)
					prevMsgs, _, err := i.msgRepo.ListByConversationID(ctx, msg.ConversationID, repository.MessageFilter{
						Limit:    5,
						OrderBy:  "created_at",
						OrderDir: "desc",
					})
					if err == nil {
						for _, pm := range prevMsgs {
							if pm.CreatedAt.Before(msg.CreatedAt) && pm.Role == models.ConversationRoleUser {
								userPrompt = pm.Content
								break
							}
						}
						// Сохраняем результат в кэш (даже если он пустой - Negative Cache Miss)
						lastUserPromptCache[msg.ConversationID] = userPrompt
					}
				}
			}

			if err := i.indexMessageWithData(ctx, msg.Conversation, msg, userPrompt); err != nil {
				i.logger.Error("failed to index message in batch", "message_id", msg.ID, "error", err)
			}
			
			lastID = &msg.ID
		}

		if len(messages) < batchSize {
			break
		}
	}

	return nil
}

func (i *conversationIndexer) buildChunks(content string, role models.ConversationRole, userPrompt string) []string {
	tokens := i.tokenizer.Encode(content, nil, nil)
	
	if len(tokens) <= ConversationMaxTokensPerChunk {
		return []string{i.formatChunk(content, role, userPrompt)}
	}

	var chunks []string
	for j := 0; j < len(tokens); j += (ConversationMaxTokensPerChunk - ConversationChunkOverlap) {
		end := j + ConversationMaxTokensPerChunk
		if end > len(tokens) {
			end = len(tokens)
		}

		chunkContent := i.tokenizer.Decode(tokens[j:end])
		chunks = append(chunks, i.formatChunk(chunkContent, role, userPrompt))

		if end == len(tokens) {
			break
		}
	}

	return chunks
}

func (i *conversationIndexer) formatChunk(content string, role models.ConversationRole, userPrompt string) string {
	var sb strings.Builder
	sb.Grow(len(content) + len(userPrompt) + 100)

	if role == models.ConversationRoleAssistant && userPrompt != "" {
		// Обрезаем промпт пользователя если он слишком длинный
		promptTokens := i.tokenizer.Encode(userPrompt, nil, nil)
		if len(promptTokens) > ConversationMaxUserPromptTokens {
			userPrompt = i.tokenizer.Decode(promptTokens[:ConversationMaxUserPromptTokens]) + "... [TRUNCATED]"
		}
		sb.WriteString("Question: ")
		sb.WriteString(userPrompt)
		sb.WriteString("\n\nAnswer: ")
	}

	sb.WriteString(content)
	return sb.String()
}

var (
	convSecretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[-_]?key|secret|password|token|auth|credential)(["']?\s*[:=]\s*["']?)([a-zA-Z0-9\-_.~]{4,})`),
		regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_.~]{15,})`),
	}
)

func (i *conversationIndexer) sanitizeText(text string) string {
	decoded, err := url.PathUnescape(text)
	if err == nil {
		text = decoded
	}
	result := text
	for _, pattern := range convSecretPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			groups := pattern.FindStringSubmatch(match)
			if len(groups) >= 3 {
				if len(groups) >= 4 {
					return groups[1] + groups[2] + "********"
				}
				return groups[1] + "********"
			}
			return "********"
		})
	}
	return strings.TrimSpace(result)
}
