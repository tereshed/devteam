package service

import (
	"context"
	"errors"
	"strings"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrTeamNotFound    = errors.New("team not found")
	ErrTeamInvalidName = errors.New("team name cannot be empty")

	ErrTeamAgentNotFound           = errors.New("agent not found")
	ErrTeamAgentInvalidModel       = errors.New("invalid model")
	ErrTeamAgentInvalidCodeBackend = errors.New("invalid code_backend")
	ErrTeamAgentConflict           = errors.New("agent update conflict")
)

// TeamService минимальная бизнес-обёртка над TeamRepository.
type TeamService interface {
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error)
	Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error)
	PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error)
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

// isAgentFKViolation — нарушение FK при SaveAgent.
// В контракте PatchAgent (13.3) на практике это в первую очередь prompt_id → prompts;
// team_id с фронта не меняется. При появлении новых FK не смешивать все 23503 в один UX-«конфликт» без разбора ConstraintName.
func isAgentFKViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

const maxAgentModelLen = 128

func (s *teamService) PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error) {
	agent, err := s.teamRepo.GetAgentInProject(ctx, projectID, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return nil, ErrTeamAgentNotFound
		}
		return nil, err
	}

	if req.ModelPresent() {
		if req.ModelClear() {
			agent.Model = nil
		} else if v, ok := req.ModelValue(); ok {
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				agent.Model = nil
			} else if len(trimmed) > maxAgentModelLen {
				return nil, ErrTeamAgentInvalidModel
			} else {
				agent.Model = &trimmed
			}
		}
	}

	if req.PromptIDPresent() {
		if req.PromptIDClear() {
			agent.PromptID = nil
			agent.Prompt = nil
		} else if id, ok := req.PromptIDValue(); ok {
			agent.PromptID = &id
			agent.Prompt = nil
		}
	}

	if req.CodeBackendPresent() {
		if req.CodeBackendClear() {
			agent.CodeBackend = nil
		} else if v, ok := req.CodeBackendValue(); ok {
			cb := models.CodeBackend(v)
			if !cb.IsValid() {
				return nil, ErrTeamAgentInvalidCodeBackend
			}
			agent.CodeBackend = &cb
		}
	}

	if req.IsActivePresent() {
		if v, ok := req.IsActiveValue(); ok {
			agent.IsActive = v
		}
	}

	if err := s.teamRepo.SaveAgent(ctx, agent); err != nil {
		if isAgentFKViolation(err) {
			return nil, ErrTeamAgentConflict
		}
		return nil, err
	}

	return s.teamRepo.GetByProjectID(ctx, projectID)
}
