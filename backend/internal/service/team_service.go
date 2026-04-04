package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
)

var (
	ErrTeamNotFound    = errors.New("team not found")
	ErrTeamInvalidName = errors.New("team name cannot be empty")
)

// TeamService минимальная бизнес-обёртка над TeamRepository.
type TeamService interface {
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error)
	Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error)
}

type teamService struct {
	teamRepo repository.TeamRepository
}

// NewTeamService создаёт сервис команд.
func NewTeamService(teamRepo repository.TeamRepository) TeamService {
	return &teamService{teamRepo: teamRepo}
}

func (s *teamService) GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error) {
	team, err := s.teamRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}
	return team, nil
}

func (s *teamService) Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error) {
	team, err := s.teamRepo.GetByProjectID(ctx, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamNotFound) {
			return nil, ErrTeamNotFound
		}
		return nil, err
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if trimmed == "" {
			return nil, ErrTeamInvalidName
		}
		team.Name = trimmed
	}
	if err := s.teamRepo.Update(ctx, team); err != nil {
		return nil, err
	}
	return s.teamRepo.GetByProjectID(ctx, projectID)
}
