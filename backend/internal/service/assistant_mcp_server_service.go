package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/devteam/backend/internal/mcp/mcpclient"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// Ошибки валидации конфига MCP-сервера ассистента.
var (
	ErrAssistantMCPInvalidName      = errors.New("assistant mcp server: name is required")
	ErrAssistantMCPInvalidTransport = errors.New("assistant mcp server: transport must be http or sse")
	ErrAssistantMCPInvalidURL       = errors.New("assistant mcp server: url must be a valid http(s) URL")
	ErrAssistantMCPInvalidHeaders   = errors.New("assistant mcp server: headers must be a JSON object of strings")
)

// ResolvedMCPServer — готовый к подключению конфиг коннектора (секреты подставлены)
// плюс политика подтверждения вызовов.
type ResolvedMCPServer struct {
	Config              mcpclient.ServerConfig
	RequireConfirmation bool
}

// AssistantMCPServerService — CRUD и резолв per-project MCP-серверов ассистента.
type AssistantMCPServerService interface {
	List(ctx context.Context, projectID uuid.UUID) ([]models.AssistantMCPServer, error)
	Get(ctx context.Context, id uuid.UUID) (*models.AssistantMCPServer, error)
	Create(ctx context.Context, cfg *models.AssistantMCPServer) error
	Update(ctx context.Context, cfg *models.AssistantMCPServer) error
	Delete(ctx context.Context, id uuid.UUID) error
	// ResolveEnabledConfigs возвращает РАЗРЕШЁННЫЕ конфиги включённых серверов проекта
	// (секреты ${secret:NAME} в заголовках подставлены в plaintext через SecretResolver).
	// project обязателен — нужен для резолва секретов.
	ResolveEnabledConfigs(ctx context.Context, project *models.Project) ([]ResolvedMCPServer, error)
}

type assistantMCPServerService struct {
	repo     repository.AssistantMCPServerRepository
	resolver SecretResolver // может быть nil; тогда ${secret:} в заголовках → ошибка резолва
}

// NewAssistantMCPServerService создаёт сервис MCP-серверов ассистента.
func NewAssistantMCPServerService(repo repository.AssistantMCPServerRepository, resolver SecretResolver) AssistantMCPServerService {
	return &assistantMCPServerService{repo: repo, resolver: resolver}
}

func (s *assistantMCPServerService) List(ctx context.Context, projectID uuid.UUID) ([]models.AssistantMCPServer, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *assistantMCPServerService) Get(ctx context.Context, id uuid.UUID) (*models.AssistantMCPServer, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *assistantMCPServerService) Create(ctx context.Context, cfg *models.AssistantMCPServer) error {
	if err := validateAssistantMCPServer(cfg); err != nil {
		return err
	}
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.URL = strings.TrimSpace(cfg.URL)
	return s.repo.Create(ctx, cfg)
}

func (s *assistantMCPServerService) Update(ctx context.Context, cfg *models.AssistantMCPServer) error {
	if err := validateAssistantMCPServer(cfg); err != nil {
		return err
	}
	cfg.Name = strings.TrimSpace(cfg.Name)
	cfg.URL = strings.TrimSpace(cfg.URL)
	return s.repo.Update(ctx, cfg)
}

func (s *assistantMCPServerService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *assistantMCPServerService) ResolveEnabledConfigs(ctx context.Context, project *models.Project) ([]ResolvedMCPServer, error) {
	if project == nil {
		return nil, fmt.Errorf("assistant mcp: project is required to resolve configs")
	}
	servers, err := s.repo.ListEnabledByProject(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	out := make([]ResolvedMCPServer, 0, len(servers))
	for i := range servers {
		sv := &servers[i]
		headers, err := s.resolveHeaders(ctx, project, sv)
		if err != nil {
			return nil, fmt.Errorf("assistant mcp server %q: %w", sv.Name, err)
		}
		out = append(out, ResolvedMCPServer{
			Config: mcpclient.ServerConfig{
				Name:      sv.Name,
				Transport: mcpclient.Transport(string(sv.Transport)),
				URL:       sv.URL,
				Headers:   headers,
			},
			RequireConfirmation: sv.RequireConfirmation,
		})
	}
	return out, nil
}

func (s *assistantMCPServerService) resolveHeaders(ctx context.Context, project *models.Project, sv *models.AssistantMCPServer) (map[string]string, error) {
	if len(sv.Headers) == 0 {
		return nil, nil
	}
	var raw map[string]string
	if err := json.Unmarshal(sv.Headers, &raw); err != nil {
		return nil, ErrAssistantMCPInvalidHeaders
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		resolved, err := s.resolveSecretPlaceholders(ctx, project, v)
		if err != nil {
			return nil, fmt.Errorf("header %q: %w", k, err)
		}
		out[k] = resolved
	}
	return out, nil
}

// resolveSecretPlaceholders подставляет ${secret:NAME} → plaintext (in-process: без
// env-индирекции, в отличие от sandbox-пути). Переиспользует mcpSecretRE.
func (s *assistantMCPServerService) resolveSecretPlaceholders(ctx context.Context, project *models.Project, v string) (string, error) {
	matches := mcpSecretRE.FindAllStringSubmatch(v, -1)
	if len(matches) == 0 {
		return v, nil
	}
	resolved := v
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		if name == "" {
			return "", fmt.Errorf("empty secret name")
		}
		if s.resolver == nil {
			return "", fmt.Errorf("secret resolver not configured")
		}
		plain, err := s.resolver.Resolve(ctx, project, name)
		if err != nil {
			return "", fmt.Errorf("resolve secret %q: %w", name, err)
		}
		resolved = strings.ReplaceAll(resolved, m[0], plain)
	}
	return resolved, nil
}

func validateAssistantMCPServer(cfg *models.AssistantMCPServer) error {
	if cfg == nil || strings.TrimSpace(cfg.Name) == "" {
		return ErrAssistantMCPInvalidName
	}
	// Remote-only: только http/sse (stdio запрещён — RCE в backend-процессе).
	switch cfg.Transport {
	case models.MCPTransportHTTP, models.MCPTransportSSE:
	default:
		return ErrAssistantMCPInvalidTransport
	}
	u, err := url.Parse(strings.TrimSpace(cfg.URL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ErrAssistantMCPInvalidURL
	}
	if len(cfg.Headers) > 0 {
		var hm map[string]string
		if err := json.Unmarshal(cfg.Headers, &hm); err != nil {
			return ErrAssistantMCPInvalidHeaders
		}
	}
	return nil
}
