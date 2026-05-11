package service

import (
	"context"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/repository"
)

// ToolDefinitionService — каталог инструментов для UI / MCP.
type ToolDefinitionService interface {
	ListActiveCatalog(ctx context.Context) ([]dto.ToolDefinitionListItemResponse, error)
}

type toolDefinitionService struct {
	repo repository.ToolDefinitionRepository
}

// NewToolDefinitionService создаёт сервис каталога инструментов.
func NewToolDefinitionService(repo repository.ToolDefinitionRepository) ToolDefinitionService {
	return &toolDefinitionService{repo: repo}
}

func (s *toolDefinitionService) ListActiveCatalog(ctx context.Context) ([]dto.ToolDefinitionListItemResponse, error) {
	list, err := s.repo.ListActiveCatalog(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ToolDefinitionListItemResponse, 0, len(list))
	for i := range list {
		td := &list[i]
		out = append(out, dto.ToolDefinitionListItemResponse{
			ID:          td.ID.String(),
			Name:        td.Name,
			Description: td.Description,
			Category:    td.Category,
			IsBuiltin:   td.IsBuiltin,
		})
	}
	return out, nil
}
