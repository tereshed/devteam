package indexer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// TaskIndexer интерфейс для индексации задач в векторной БД
type TaskIndexer interface {
	// IndexTask индексирует одну задачу
	IndexTask(ctx context.Context, taskID uuid.UUID) error
	// DeleteTask удаляет задачу из индекса
	DeleteTask(ctx context.Context, taskID uuid.UUID) error
	// DeleteProjectTasks удаляет все задачи проекта из индекса
	DeleteProjectTasks(ctx context.Context, projectID uuid.UUID) error
	// IndexProjectTasks индексирует все задачи проекта (массовая индексация)
	IndexProjectTasks(ctx context.Context, projectID uuid.UUID) error
}

type taskIndexer struct {
	taskRepo    repository.TaskRepository
	messageRepo repository.TaskMessageRepository
	vectorRepo  repository.VectorRepository
	logger      *slog.Logger
}

// NewTaskIndexer создает новый экземпляр TaskIndexer
func NewTaskIndexer(
	taskRepo repository.TaskRepository,
	messageRepo repository.TaskMessageRepository,
	vectorRepo repository.VectorRepository,
	logger *slog.Logger,
) TaskIndexer {
	return &taskIndexer{
		taskRepo:    taskRepo,
		messageRepo: messageRepo,
		vectorRepo:  vectorRepo,
		logger:      logger,
	}
}

func (i *taskIndexer) IndexTask(ctx context.Context, taskID uuid.UUID) error {
	task, err := i.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Получаем сообщения задачи
	messages, _, err := i.messageRepo.ListByTaskID(ctx, taskID, repository.TaskMessageFilter{
		Limit: 100, // Лимит сообщений для индексации
	})
	if err != nil {
		return fmt.Errorf("failed to get task messages: %w", err)
	}

	return i.indexTaskWithData(ctx, task, messages)
}

// indexTaskWithData выполняет индексацию задачи с уже загруженными данными
func (i *taskIndexer) indexTaskWithData(ctx context.Context, task *models.Task, messages []models.TaskMessage) error {
	// Собираем документ
	contents := i.buildTaskDocuments(task, messages)
	if len(contents) == 0 {
		return nil // Early return для пустых задач
	}

	// Гарантированно удаляем старые чанки перед вставкой новых (Upsert)
	// Мы используем DeleteByContentID, который в Weaviate удаляет все объекты с этим contentId.
	// Поскольку мы всегда устанавливаем contentId равным task.ID.String() для всех чанков,
	// это удалит все предыдущие чанки этой задачи.
	err := i.vectorRepo.DeleteByContentID(ctx, task.ProjectID.String(), task.ID.String())
	if err != nil {
		i.logger.Warn("failed to delete old task chunks", "task_id", task.ID, "error", err)
		// Продолжаем, так как это может быть первая индексация
	}

	for idx, content := range contents {
		// Создаем векторный документ
		// ВАЖНО: Мы используем task.ID.String() как contentId для ВСЕХ чанков.
		// Это позволяет удалять их все разом через DeleteByContentID.
		contentID := task.ID.String()

		doc := models.NewVectorDocument(
			contentID,
			content,
			models.ContentType("task"),
		)
		doc.Category = "task_history"
		doc.SetMetadata("task_id", task.ID.String())
		doc.SetMetadata("project_id", task.ProjectID.String())
		doc.SetMetadata("status", string(task.Status))
		doc.SetMetadata("priority", string(task.Priority))
		if len(contents) > 1 {
			doc.SetMetadata("chunk_index", idx)
			doc.SetMetadata("total_chunks", len(contents))
		}
		if task.AssignedAgentID != nil {
			doc.SetMetadata("assigned_agent_id", task.AssignedAgentID.String())
		}
		if task.CompletedAt != nil {
			doc.SetMetadata("completed_at", task.CompletedAt.Format(time.RFC3339))
		}

		// Сохраняем в Weaviate через VectorRepository
		_, err = i.vectorRepo.Create(ctx, task.ProjectID.String(), doc)
		if err != nil {
			return fmt.Errorf("failed to create vector document for chunk %d: %w", idx, err)
		}
	}

	return nil
}

func (i *taskIndexer) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	// Нам нужно знать projectID для удаления из коллекции
	task, err := i.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			return nil // Уже удалена
		}
		return fmt.Errorf("failed to get task for deletion: %w", err)
	}

	err = i.vectorRepo.DeleteByContentID(ctx, task.ProjectID.String(), task.ID.String())
	if err != nil {
		return fmt.Errorf("failed to delete task from vector index: %w", err)
	}

	return nil
}

func (i *taskIndexer) DeleteProjectTasks(ctx context.Context, projectID uuid.UUID) error {
	// Удаляем все документы типа "task" для данного проекта
	err := i.vectorRepo.DeleteByContentType(ctx, projectID.String(), models.ContentType("task"), "task_history")
	if err != nil {
		return fmt.Errorf("failed to delete project tasks from vector index: %w", err)
	}
	return nil
}

func (i *taskIndexer) IndexProjectTasks(ctx context.Context, projectID uuid.UUID) error {
	offset := 0
	batchSize := 100

	for {
		tasks, _, err := i.taskRepo.List(ctx, repository.TaskFilter{
			ProjectID: &projectID,
			Limit:     batchSize,
			Offset:    offset,
			OrderBy:   "created_at",
			OrderDir:  "asc",
		})
		if err != nil {
			return fmt.Errorf("failed to list tasks for indexing: %w", err)
		}

		if len(tasks) == 0 {
			break
		}

		// TODO: fix N+1 query by adding ListByTaskIDs to TaskMessageRepository
		for _, task := range tasks {
			messages, _, err := i.messageRepo.ListByTaskID(ctx, task.ID, repository.TaskMessageFilter{
				Limit: 100,
			})
			if err != nil {
				i.logger.Error("failed to get messages for task", "task_id", task.ID, "error", err)
				continue
			}

			if err := i.indexTaskWithData(ctx, &task, messages); err != nil {
				i.logger.Error("failed to index task", "task_id", task.ID, "error", err)
			}
		}

		if len(tasks) < batchSize {
			break
		}
		offset += batchSize
	}

	return nil
}

const (
	// Эвристика: 1 токен ≈ 4 символа. Лимит 8k токенов ≈ 32000 символов.
	// Берем с запасом 24000 символов на чанк.
	maxChunkChars = 24000
)

// buildTaskDocuments собирает текстовые представления задачи (возможно несколько чанков)
func (i *taskIndexer) buildTaskDocuments(task *models.Task, messages []models.TaskMessage) []string {
	var sb strings.Builder
	sb.Grow(2048) // Оптимизация аллокаций

	// Title
	sb.WriteString("--- TASK TITLE ---\n")
	sb.WriteString(task.Title)
	sb.WriteString("\n\n")

	// Prompt/Description
	if task.Description != "" {
		sb.WriteString("--- TASK PROMPT ---\n")
		sb.WriteString(i.sanitizeText(task.Description))
		sb.WriteString("\n\n")
	}

	// Result
	if task.Result != nil && *task.Result != "" {
		sb.WriteString("--- AGENT RESULT ---\n")
		sb.WriteString(i.sanitizeText(*task.Result))
		sb.WriteString("\n\n")
	}

	// Discussion
	if len(messages) > 0 {
		sb.WriteString("--- DISCUSSION ---\n")
		for _, msg := range messages {
			role := "User"
			if msg.SenderType == models.SenderTypeAgent {
				role = "Agent"
			}
			msgContent := i.sanitizeText(msg.Content)
			
			// Если одно сообщение слишком длинное, обрезаем его
			if len(msgContent) > maxChunkChars {
				msgContent = msgContent[:maxChunkChars] + "... [TRUNCATED]"
			}
			
			sb.WriteString(fmt.Sprintf("[%s]: %s\n", role, msgContent))
		}
	}

	fullText := sb.String()
	if strings.TrimSpace(fullText) == "" {
		return nil
	}

	// Чанкирование по длине (эвристика токенов)
	// Используем []rune для корректной работы с UTF-8 (русский язык)
	runes := []rune(fullText)
	if len(runes) <= maxChunkChars {
		return []string{fullText}
	}

	// Разбиваем на чанки
	var chunks []string
	for start := 0; start < len(runes); start += maxChunkChars {
		end := start + maxChunkChars
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}

	return chunks
}

var (
	// Простые регулярки для маскирования секретов
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[-_]?key|secret|password|token|auth|credential)(["']?\s*[:=]\s*["']?)([a-zA-Z0-9\-_.~]{4,})`),
		regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9\-_.~]{15,})`),
	}
)

// sanitizeText маскирует секреты в тексте
func (i *taskIndexer) sanitizeText(text string) string {
	result := text
	for _, pattern := range secretPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			groups := pattern.FindStringSubmatch(match)
			if len(groups) >= 3 {
				// groups[0] - full match
				// groups[1] - key name / prefix
				// groups[2] - separator / secret value
				// groups[3] - secret value (if exists)
				if len(groups) >= 4 {
					return groups[1] + groups[2] + "********"
				}
				return groups[1] + "********"
			}
			return "********"
		})
	}
	return result
}
