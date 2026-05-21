package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrMCPServerNotFound      = errors.New("mcp server not found")
	ErrMCPServerValidation    = errors.New("mcp server validation failed")
	ErrMCPServerDuplicateName = errors.New("mcp server with this name already exists")
)

type MCPServerRegistryService struct {
	repo repository.MCPServerRegistryRepository
}

func NewMCPServerRegistryService(repo repository.MCPServerRegistryRepository) *MCPServerRegistryService {
	return &MCPServerRegistryService{repo: repo}
}

func (s *MCPServerRegistryService) List(ctx context.Context, onlyActive bool) ([]models.MCPServerRegistry, error) {
	return s.repo.List(ctx, onlyActive)
}

func (s *MCPServerRegistryService) GetByID(ctx context.Context, id uuid.UUID) (*models.MCPServerRegistry, error) {
	srv, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrMCPServerRegistryNotFound) {
			return nil, ErrMCPServerNotFound
		}
		return nil, err
	}
	return srv, nil
}

type CreateMCPServerInput struct {
	Name        string
	Description string
	Transport   string
	Command     string
	Args        []byte
	URL         string
	EnvTemplate []byte
	Scope       string
	IsActive    *bool
}

func (s *MCPServerRegistryService) Create(ctx context.Context, in CreateMCPServerInput) (*models.MCPServerRegistry, error) {
	transport, scope, err := s.validateTransportScope(in.Transport, in.Scope)
	if err != nil {
		return nil, err
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	srv := &models.MCPServerRegistry{
		Name:        in.Name,
		Description: in.Description,
		Transport:   transport,
		Command:     in.Command,
		Args:        in.Args,
		URL:         in.URL,
		EnvTemplate: in.EnvTemplate,
		Scope:       scope,
		IsActive:    isActive,
	}

	if err := s.repo.Create(ctx, srv); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrMCPServerDuplicateName
		}
		return nil, err
	}
	return srv, nil
}

type UpdateMCPServerInput struct {
	Name        string
	Description string
	Transport   string
	Command     string
	Args        []byte
	URL         string
	EnvTemplate []byte
	Scope       string
	IsActive    *bool
}

func (s *MCPServerRegistryService) Update(ctx context.Context, id uuid.UUID, in UpdateMCPServerInput) (*models.MCPServerRegistry, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrMCPServerRegistryNotFound) {
			return nil, ErrMCPServerNotFound
		}
		return nil, err
	}

	transport, scope, err := s.validateTransportScope(in.Transport, in.Scope)
	if err != nil {
		return nil, err
	}
	if in.Scope == "" {
		scope = existing.Scope
	}

	existing.Name = in.Name
	existing.Description = in.Description
	existing.Transport = transport
	existing.Command = in.Command
	existing.Args = in.Args
	existing.URL = in.URL
	existing.EnvTemplate = in.EnvTemplate
	existing.Scope = scope
	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrMCPServerDuplicateName
		}
		return nil, err
	}
	return existing, nil
}

func (s *MCPServerRegistryService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrMCPServerRegistryNotFound) {
			return ErrMCPServerNotFound
		}
		return err
	}
	return nil
}

func (s *MCPServerRegistryService) validateTransportScope(transport, scope string) (models.MCPTransport, models.MCPScope, error) {
	t := models.MCPTransport(transport)
	if !t.IsValid() {
		return "", "", fmt.Errorf("%w: invalid transport %q (must be stdio, http, or sse)", ErrMCPServerValidation, transport)
	}

	sc := models.MCPScope(scope)
	if scope == "" {
		sc = models.MCPScopeGlobal
	} else if !sc.IsValid() {
		return "", "", fmt.Errorf("%w: invalid scope %q (must be global, project, or agent)", ErrMCPServerValidation, scope)
	}

	return t, sc, nil
}

func isDuplicateKeyError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
