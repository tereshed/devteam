package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// orchestration_query_service.go — Sprint 5 review fix #1 (layer violation).
//
// Read-only фасад над v2-репозиториями для observability-операций (MCP-инструменты,
// HTTP debug-эндпоинты, frontend Task Detail v2). Транспорт-слой (MCP/HTTP) НЕ
// должен дёргать ArtifactRepository / RouterDecisionRepository / WorktreeRepository
// напрямую — все вызовы идут через этот сервис.
//
// Write-операции (cancel) — отдельно через TaskLifecycleService.RequestCancel
// (она уже инкапсулирует Notify + UPDATE).

var (
	// ErrArtifactNotInTask — переиспользуем для caller'а; маппится из ErrArtifactNotFound.
	ErrArtifactNotInTask = errors.New("artifact not found")
)

// OrchestrationQueryService — read-only operations над v2-данными оркестратора.
type OrchestrationQueryService struct {
	artifactRepo  repository.ArtifactRepository
	decisionRepo  repository.RouterDecisionRepository
	worktreeRepo  repository.WorktreeRepository
}

// NewOrchestrationQueryService — конструктор.
func NewOrchestrationQueryService(
	artifactRepo repository.ArtifactRepository,
	decisionRepo repository.RouterDecisionRepository,
	worktreeRepo repository.WorktreeRepository,
) *OrchestrationQueryService {
	return &OrchestrationQueryService{
		artifactRepo: artifactRepo,
		decisionRepo: decisionRepo,
		worktreeRepo: worktreeRepo,
	}
}

// ListArtifacts — артефакты задачи. onlyReady=true (default для UI) фильтрует
// только status=ready. Возвращает metadata (без полного content).
func (s *OrchestrationQueryService) ListArtifacts(ctx context.Context, taskID uuid.UUID, onlyReady bool) ([]models.Artifact, error) {
	return s.artifactRepo.ListMetadataByTaskID(ctx, taskID, onlyReady)
}

// GetArtifact — полный артефакт (включая content). Для debug-вью.
func (s *OrchestrationQueryService) GetArtifact(ctx context.Context, artifactID uuid.UUID) (*models.Artifact, error) {
	a, err := s.artifactRepo.GetByID(ctx, artifactID)
	if err != nil {
		if errors.Is(err, repository.ErrArtifactNotFound) {
			return nil, ErrArtifactNotInTask
		}
		return nil, fmt.Errorf("get artifact %s: %w", artifactID, err)
	}
	return a, nil
}

// ListRouterDecisions — лог Router-решений по задаче. encrypted_raw_response
// НЕ возвращается (через MCP/HTTP read-стек не отдаётся; для дебага в БД напрямую).
func (s *OrchestrationQueryService) ListRouterDecisions(ctx context.Context, taskID uuid.UUID) ([]models.RouterDecision, error) {
	return s.decisionRepo.ListByTaskID(ctx, taskID, false)
}

// ListWorktrees — git worktree'и задачи (debug-view).
func (s *OrchestrationQueryService) ListWorktrees(ctx context.Context, taskID uuid.UUID) ([]models.Worktree, error) {
	return s.worktreeRepo.ListByTaskID(ctx, taskID)
}
