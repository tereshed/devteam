package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devteam/backend/internal/indexer"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/vectordb"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

var (
	indexerOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "indexer_operations_total",
		Help: "Total number of indexing operations",
	}, []string{"operation", "status"})

	indexerOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "indexer_operation_duration_seconds",
		Help:    "Duration of indexing operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	indexerFailedOperations = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "indexer_failed_operations_queue_size",
		Help: "Number of failed operations in DLQ",
	}, []string{"project_id"})
)

const (
	StateIdle     = "idle"
	StateIndexing = "indexing"
	StateFailed   = "failed"

	LockTTL              = 2 * time.Minute
	LockWatchdogInterval = 1 * time.Minute

	MaxRetries         = 3
	InitialRetryDelay  = 1 * time.Second
	MaxRetryDelay      = 30 * time.Second
)

// Locker определяет интерфейс распределенного лока для предотвращения параллельной индексации одного проекта.
type Locker interface {
	// Lock пытается захватить лок. Возвращает lockID для последующего продления или удаления.
	Lock(ctx context.Context, key string, ttl time.Duration) (lockID string, err error)
	// Unlock освобождает лок по ID.
	Unlock(ctx context.Context, key, lockID string) error
	// Refresh продлевает TTL существующего лока.
	Refresh(ctx context.Context, key, lockID string, ttl time.Duration) error
}

// IndexStatus описывает текущее состояние процесса индексации
type IndexStatus struct {
	State      string    // idle | indexing | failed
	Progress   float64   // 0.0 to 1.0
	RunID      string    // current index_run_id
	StartTime  time.Time
	LastError  string
}

// IndexerService координирует индексацию данных проекта в векторную БД
type IndexerService interface {
	FullIndex(ctx context.Context, projectID string) error

	// Single updates
	IndexCode(ctx context.Context, projectID string, filePath string) error
	IndexTask(ctx context.Context, projectID string, taskID string) error
	IndexMessage(ctx context.Context, projectID string, messageID string) error

	// Bulk updates
	IndexCodes(ctx context.Context, projectID string, filePaths []string) error
	IndexTasks(ctx context.Context, projectID string, taskIDs []string) error
	IndexMessages(ctx context.Context, projectID string, messageIDs []string) error

	// Deletions
	DeleteCode(ctx context.Context, projectID string, filePath string) error
	DeleteCodes(ctx context.Context, projectID string, filePaths []string) error
	DeleteTask(ctx context.Context, projectID string, taskID string) error
	DeleteMessage(ctx context.Context, projectID string, messageID string) error

	// Utilities
	MoveCode(ctx context.Context, projectID string, oldPath, newPath string) error
	GetIndexStatus(ctx context.Context, projectID string) (IndexStatus, error)
}

type indexerService struct {
	logger     *slog.Logger
	vectorDB   *vectordb.Client
	codeIdx    indexer.CodeIndexer
	taskIdx    indexer.TaskIndexer
	convIdx    indexer.ConversationIndexer
	projectSvc ProjectService
	syncRepo   repository.SyncStateRepository
	locker     Locker
}

func NewIndexerService(
	logger *slog.Logger,
	vectorDB *vectordb.Client,
	codeIdx indexer.CodeIndexer,
	taskIdx indexer.TaskIndexer,
	convIdx indexer.ConversationIndexer,
	projectSvc ProjectService,
	syncRepo repository.SyncStateRepository,
	locker Locker,
) IndexerService {
	return &indexerService{
		logger:     logger,
		vectorDB:   vectorDB,
		codeIdx:    codeIdx,
		taskIdx:    taskIdx,
		convIdx:    convIdx,
		projectSvc: projectSvc,
		syncRepo:   syncRepo,
		locker:     locker,
	}
}

// --- Security & Helpers ---

func (s *indexerService) sanitizePath(path string) (string, error) {
	clean := filepath.Clean(path)
	// Защита от Path Traversal: запрещаем абсолютные пути и выход за пределы корня через ".."
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("path traversal attempt: %s", clean)
	}
	return clean, nil
}

func (s *indexerService) sanitizeLog(msg string) string {
	// Защита от Log Forging: удаляем переносы строк
	replacer := strings.NewReplacer("\n", " ", "\r", " ")
	return replacer.Replace(msg)
}

func (s *indexerService) logInfo(msg string, args ...any) {
	s.logger.Info(s.sanitizeLog(msg), args...)
}

func (s *indexerService) logError(msg string, err error, args ...any) {
	fullArgs := append(args, slog.Any("error", err))
	s.logger.Error(s.sanitizeLog(msg), fullArgs...)
}

// withRetry выполняет функцию с механизмом Exponential Backoff
func (s *indexerService) withRetry(ctx context.Context, operation string, fn func() error) error {
	start := time.Now()
	defer func() {
		indexerOperationDuration.WithLabelValues(operation).Observe(time.Since(start).Seconds())
	}()

	var lastErr error
	delay := InitialRetryDelay

	for i := 0; i < MaxRetries; i++ {
		err := fn()
		if err == nil {
			indexerOperationsTotal.WithLabelValues(operation, "success").Inc()
			return nil
		}

		lastErr = err
		s.logError("Operation failed, retrying", err, "operation", operation, "attempt", i+1)

		// Exponential backoff с джиттером
		jitter := time.Duration(rand.Int63n(int64(delay / 2)))
		select {
		case <-time.After(delay + jitter):
			delay *= 2
			if delay > MaxRetryDelay {
				delay = MaxRetryDelay
			}
		case <-ctx.Done():
			indexerOperationsTotal.WithLabelValues(operation, "cancelled").Inc()
			return ctx.Err()
		}
	}

	indexerOperationsTotal.WithLabelValues(operation, "failed").Inc()
	return fmt.Errorf("operation %s failed after %d retries: %w", operation, MaxRetries, lastErr)
}

// addToDLQ записывает неудачную операцию в очередь для последующей обработки
func (s *indexerService) addToDLQ(ctx context.Context, projectID, operation, entityID string, lastErr error) {
	uid, err := uuid.Parse(projectID)
	if err != nil {
		return
	}

	op := &repository.FailedOperation{
		ProjectID:  uid,
		Operation:  operation,
		EntityID:   entityID,
		LastError:  lastErr.Error(),
		RetryCount: 0,
	}

	if err := s.syncRepo.AddFailedOperation(ctx, op); err != nil {
		s.logError("Failed to add operation to DLQ", err, "project_id", projectID, "operation", operation, "entity_id", entityID)
	} else {
		indexerFailedOperations.WithLabelValues(projectID).Inc()
	}
}

// --- Interface Implementation (Stubs) ---

func (s *indexerService) FullIndex(ctx context.Context, projectID string) error {
	uid, err := uuid.Parse(projectID)
	if err != nil {
		return fmt.Errorf("invalid projectID: %w", err)
	}

	// 1. Захват распределенного лока
	lockKey := fmt.Sprintf("indexer_lock_%s", projectID)
	lockID, err := s.locker.Lock(ctx, lockKey, LockTTL)
	if err != nil {
		s.logError("Failed to acquire indexer lock", err, "project_id", projectID)
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	// Watchdog для продления лока
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(LockWatchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				// Пытаемся освободить лок при завершении (успешном или нет)
				unlockCtx, unlockCancel := context.WithTimeout(context.Background(), 5*time.Second)
				s.locker.Unlock(unlockCtx, lockKey, lockID)
				unlockCancel()
				return
			case <-ticker.C:
				if err := s.locker.Refresh(ctx, lockKey, lockID, LockTTL); err != nil {
					s.logError("Failed to refresh indexer lock", err, "project_id", projectID)
					// Если не удалось продлить лок, останавливаем процесс индексации
					cancel()
					return
				}
			}
		}
	}()
	defer func() {
		cancel()
		wg.Wait()
	}()

	// 2. Подготовка сессии индексации
	runID := uuid.New().String()
	startTime := time.Now()

	s.logInfo("Starting full index", "project_id", projectID, "run_id", runID)

	state := &repository.ProjectSyncState{
		ProjectID:    uid,
		CurrentState: StateIndexing,
		Progress:     0,
		StartTime:    startTime,
		ActiveRunID:  runID, // Пока не переключаем "активный", используем как текущий
	}
	if err := s.syncRepo.UpsertProjectState(ctx, state); err != nil {
		return fmt.Errorf("failed to initialize project sync state: %w", err)
	}

	// 3. Запуск индексации через errgroup
	g, gCtx := errgroup.WithContext(ctx)

	// Индексация кода
	g.Go(func() error {
		// Получаем путь к репозиторию через ProjectService
		_, err := s.projectSvc.GetByID(gCtx, uuid.Nil, models.RoleAdmin, uid)
		if err != nil {
			return err
		}
		// Используем LocalPath, если он добавлен, или формируем путь до клона 
		// (он вычисляется менеджером песочниц/импорта)
		// На данный момент в модели нет LocalPath, используем фиктивный путь - для рефакторинга
		// В следующей задаче это нужно реализовать через Sandbox/Storage
		return s.codeIdx.IndexProject(gCtx, indexer.IndexingRequest{
			ProjectID: uid,
			RepoPath:  "/tmp/repos/" + uid.String(),
		})
	})

	// Индексация задач
	g.Go(func() error {
		return s.taskIdx.IndexProjectTasks(gCtx, uid)
	})

	// Индексация переписок
	g.Go(func() error {
		return s.convIdx.IndexProjectConversations(gCtx, uid)
	})

	// Ожидание завершения
	if err := g.Wait(); err != nil {
		s.logError("Full indexing failed", err, "project_id", projectID, "run_id", runID)
		
		// Обновляем статус на failed
		state.CurrentState = StateFailed
		state.LastError = err.Error()
		s.syncRepo.UpsertProjectState(context.Background(), state) // Используем Background для гарантированного сохранения ошибки
		return err
	}

	// 4. Успешное завершение: Атомарное переключение ActiveRunID
	state.CurrentState = StateIdle
	state.Progress = 1.0
	state.LastError = ""
	if err := s.syncRepo.UpsertProjectState(ctx, state); err != nil {
		return fmt.Errorf("failed to finalize project sync state: %w", err)
	}

	s.logInfo("Full indexing completed successfully", "project_id", projectID, "run_id", runID)
	return nil
}

func (s *indexerService) IndexCode(ctx context.Context, projectID string, filePath string) error {
	path, err := s.sanitizePath(filePath)
	if err != nil {
		s.logError("Invalid path provided for indexing", err, "project_id", projectID, "path", filePath)
		return err
	}

	uid, _ := uuid.Parse(projectID)
	err = s.withRetry(ctx, "IndexCode", func() error {
		_, err := s.projectSvc.GetByID(ctx, uuid.Nil, models.RoleAdmin, uid)
		if err != nil {
			return err
		}
		return s.codeIdx.IndexProject(ctx, indexer.IndexingRequest{
			ProjectID: uid,
			RepoPath:  "/tmp/repos/" + uid.String(),
		})
	})

	if err != nil {
		s.addToDLQ(ctx, projectID, "index_code", path, err)
		return err
	}

	return nil
}

func (s *indexerService) IndexTask(ctx context.Context, projectID string, taskID string) error {
	tuid, err := uuid.Parse(taskID)
	if err != nil {
		return err
	}

	err = s.withRetry(ctx, "IndexTask", func() error {
		return s.taskIdx.IndexTask(ctx, tuid)
	})

	if err != nil {
		s.addToDLQ(ctx, projectID, "index_task", taskID, err)
		return err
	}
	return nil
}

func (s *indexerService) IndexMessage(ctx context.Context, projectID string, messageID string) error {
	// _ = muid // to be defined correctly when message indexation is implemented
	// _ = puid

	err := s.withRetry(ctx, "IndexMessage", func() error {
		// Для сообщения нам нужен еще conversationID, но в интерфейсе IndexerService его нет.
		// Предполагаем, что IndexMessage в будущем будет принимать или искать его.
		// Пока используем заглушку, так как ConversationIndexer требует больше данных.
		return nil 
	})

	if err != nil {
		s.addToDLQ(ctx, projectID, "index_message", messageID, err)
		return err
	}
	return nil
}

func (s *indexerService) IndexCodes(ctx context.Context, projectID string, filePaths []string) error {
	s.logInfo("Indexing %d files", "count", len(filePaths), "project_id", projectID)
	for _, path := range filePaths {
		_, err := s.sanitizePath(path)
		if err != nil {
			s.logError("Invalid path provided in bulk indexing", err, "project_id", projectID, "path", path)
			return err
		}
	}
	for _, path := range filePaths {
		if err := s.IndexCode(ctx, projectID, path); err != nil {
			return err
		}
	}
	return nil
}

func (s *indexerService) IndexTasks(ctx context.Context, projectID string, taskIDs []string) error {
	for _, id := range taskIDs {
		if err := s.IndexTask(ctx, projectID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *indexerService) IndexMessages(ctx context.Context, projectID string, messageIDs []string) error {
	for _, id := range messageIDs {
		if err := s.IndexMessage(ctx, projectID, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *indexerService) DeleteCode(ctx context.Context, projectID string, filePath string) error {
	path, err := s.sanitizePath(filePath)
	if err != nil {
		return err
	}

	uid, _ := uuid.Parse(projectID)
	err = s.withRetry(ctx, "DeleteCode", func() error {
		// Мы используем детерминированные ID в Weaviate (code_{projectID}_{sha256_16(filePath)})
		// Но текущий векторный репозиторий может требовать другие параметры.
		// В текущей реализации CodeIndexer удаление идет через SyncState ID.
		state, err := s.syncRepo.GetByPath(ctx, uid, path)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		
		// Удаляем из вектора по ID из SyncState
		err = s.vectorDB.DeleteByContentID(ctx, projectID, state.ID.String())
		if err != nil {
			return err
		}
		
		return s.syncRepo.Delete(ctx, uid, path)
	})

	if err != nil {
		s.addToDLQ(ctx, projectID, "delete_code", path, err)
		return err
	}
	return nil
}

func (s *indexerService) DeleteCodes(ctx context.Context, projectID string, filePaths []string) error {
	s.logInfo("Deleting %d files", "count", len(filePaths), "project_id", projectID)
	for _, path := range filePaths {
		_, err := s.sanitizePath(path)
		if err != nil {
			s.logError("Invalid path provided in bulk deletion", err, "project_id", projectID, "path", path)
			return err
		}
	}
	for _, path := range filePaths {
		if err := s.DeleteCode(ctx, projectID, path); err != nil {
			return err
		}
	}
	return nil
}

func (s *indexerService) DeleteTask(ctx context.Context, projectID string, taskID string) error {
	tuid, _ := uuid.Parse(taskID)
	err := s.withRetry(ctx, "DeleteTask", func() error {
		return s.taskIdx.DeleteTask(ctx, tuid)
	})
	if err != nil {
		s.addToDLQ(ctx, projectID, "delete_task", taskID, err)
		return err
	}
	return nil
}

func (s *indexerService) DeleteMessage(ctx context.Context, projectID string, messageID string) error {
	muid, _ := uuid.Parse(messageID)
	puid, _ := uuid.Parse(projectID)
	err := s.withRetry(ctx, "DeleteMessage", func() error {
		return s.convIdx.DeleteMessage(ctx, puid, muid)
	})
	if err != nil {
		s.addToDLQ(ctx, projectID, "delete_message", messageID, err)
		return err
	}
	return nil
}

func (s *indexerService) MoveCode(ctx context.Context, projectID string, oldPath, newPath string) error {
	// Стратегия: Запись нового -> Удаление старого
	if err := s.IndexCode(ctx, projectID, newPath); err != nil {
		return err
	}
	return s.DeleteCode(ctx, projectID, oldPath)
}

func (s *indexerService) GetIndexStatus(ctx context.Context, projectID string) (IndexStatus, error) {
	uid, err := uuid.Parse(projectID)
	if err != nil {
		return IndexStatus{}, fmt.Errorf("invalid projectID: %w", err)
	}

	state, err := s.syncRepo.GetProjectState(ctx, uid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return IndexStatus{State: StateIdle}, nil
		}
		return IndexStatus{}, fmt.Errorf("failed to get project state: %w", err)
	}

	return IndexStatus{
		State:     state.CurrentState,
		Progress:  state.Progress,
		RunID:     state.ActiveRunID,
		StartTime: state.StartTime,
		LastError: state.LastError,
	}, nil
}
