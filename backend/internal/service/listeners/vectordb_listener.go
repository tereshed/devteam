package listeners

import (
	"context"
	"log/slog"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/pkg/vectordb"
)

// VectorDBListener слушает события и обновляет Weaviate
type VectorDBListener struct {
	vectorDB *vectordb.Client
	eventBus events.EventBus
	log      *slog.Logger
}

// NewVectorDBListener создает новый слушатель для Weaviate
func NewVectorDBListener(vdb *vectordb.Client, bus events.EventBus, log *slog.Logger) *VectorDBListener {
	if log == nil {
		log = slog.Default()
	}
	return &VectorDBListener{
		vectorDB: vdb,
		eventBus: bus,
		log:      log,
	}
}

// Start запускает прослушивание событий
func (l *VectorDBListener) Start(ctx context.Context) {
	ch, unsubscribe := l.eventBus.Subscribe("vectordb_listener", 100)
	defer unsubscribe()

	l.log.Info("VectorDBListener started")

	for {
		select {
		case <-ctx.Done():
			l.log.Info("VectorDBListener stopping")
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			l.handleEvent(ctx, ev)
		}
	}
}

func (l *VectorDBListener) handleEvent(ctx context.Context, ev events.DomainEvent) {
	switch e := ev.(type) {
	case events.ProjectDeleted:
		l.handleProjectDeleted(ctx, e)
	}
}

func (l *VectorDBListener) handleProjectDeleted(ctx context.Context, ev events.ProjectDeleted) {
	projectID := ev.ProjectID.String()
	l.log.Info("Handling project deletion in VectorDB", "projectID", projectID)

	// Попытка удаления коллекции с ретраями (упрощенно)
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		// Используем Background контекст с таймаутом, чтобы удаление не прервалось при отмене ctx запроса
		deleteCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := l.vectorDB.DeleteCollection(deleteCtx, projectID)
		cancel()

		if err == nil {
			l.log.Info("Successfully deleted Weaviate collection", "projectID", projectID)
			return
		}

		l.log.Error("Failed to delete Weaviate collection", 
			"projectID", projectID, 
			"attempt", i+1, 
			"error", err)
		
		if i < maxRetries-1 {
			time.Sleep(time.Second * time.Duration(2*(i+1)))
		}
	}

	l.log.Error("Failed to delete Weaviate collection after all retries. Manual cleanup required.", 
		"projectID", projectID)
}
