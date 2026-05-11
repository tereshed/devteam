package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrTeamNotFound    = errors.New("team not found")
	ErrTeamInvalidName = errors.New("team name cannot be empty")

	ErrTeamAgentNotFound           = errors.New("agent not found")
	ErrTeamAgentInvalidModel       = errors.New("invalid model")
	ErrTeamAgentInvalidCodeBackend = errors.New("invalid code_backend")
	ErrTeamAgentConflict           = errors.New("agent update conflict")
	ErrTeamAgentInvalidToolBindings = errors.New("invalid or inactive tool_definition_id in tool_bindings")
)

// TeamService минимальная бизнес-обёртка над TeamRepository.
type TeamService interface {
	GetByProjectID(ctx context.Context, projectID uuid.UUID) (*models.Team, error)
	Update(ctx context.Context, projectID uuid.UUID, req dto.UpdateTeamRequest) (*models.Team, error)
	PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error)
}

type teamService struct {
	teamRepo    repository.TeamRepository
	toolDefRepo repository.ToolDefinitionRepository
}

// NewTeamService создаёт сервис команд.
func NewTeamService(teamRepo repository.TeamRepository, toolDefRepo repository.ToolDefinitionRepository) TeamService {
	return &teamService{teamRepo: teamRepo, toolDefRepo: toolDefRepo}
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

const maxAgentModelLen = 128

const maxAgentToolBindings = 50

func dedupeSortedToolDefinitionIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})
	return out
}

func (s *teamService) PatchAgent(ctx context.Context, projectID, agentID uuid.UUID, req dto.PatchAgentRequest) (*models.Team, error) {
	agent, err := s.teamRepo.GetAgentInProject(ctx, projectID, agentID)
	if err != nil {
		if errors.Is(err, repository.ErrTeamAgentNotFound) {
			return nil, ErrTeamAgentNotFound
		}
		return nil, err
	}

	// Сначала валидация tool_bindings, затем мутация agent: при раннем return указатель agent
	// не отражает «частично применённый» PATCH в памяти (см. 13.3.1 / ревью).
	var bindingIDs []uuid.UUID
	var doReplaceBindings bool
	if req.ToolBindingsPresent() {
		doReplaceBindings = true
		bindingIDs = dedupeSortedToolDefinitionIDs(req.ToolBindingsRawIDs())
		if len(bindingIDs) > maxAgentToolBindings {
			return nil, ErrTeamAgentInvalidToolBindings
		}
		if len(bindingIDs) > 0 {
			n, err := s.toolDefRepo.CountActiveInIDs(ctx, bindingIDs)
			if err != nil {
				return nil, err
			}
			if int(n) != len(bindingIDs) {
				return nil, ErrTeamAgentInvalidToolBindings
			}
		}
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

	if doReplaceBindings {
		if err := s.teamRepo.SaveAgentWithToolBindings(ctx, agent, true, bindingIDs); err != nil {
			if mapped, ok := mapAgentPatchPostgresFK(err); ok {
				return nil, mapped
			}
			return nil, err
		}
		return s.teamRepo.GetByProjectID(ctx, projectID)
	}

	if err := s.teamRepo.SaveAgent(ctx, agent); err != nil {
		if mapped, ok := mapAgentPatchPostgresFK(err); ok {
			return nil, mapped
		}
		return nil, err
	}

	return s.teamRepo.GetByProjectID(ctx, projectID)
}
