package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// --- фейки ---

type fakeAssistantMCPRepo struct {
	enabled []models.AssistantMCPServer
}

func (f *fakeAssistantMCPRepo) ListByProject(context.Context, uuid.UUID) ([]models.AssistantMCPServer, error) {
	return f.enabled, nil
}
func (f *fakeAssistantMCPRepo) ListEnabledByProject(context.Context, uuid.UUID) ([]models.AssistantMCPServer, error) {
	return f.enabled, nil
}
func (f *fakeAssistantMCPRepo) GetByID(context.Context, uuid.UUID) (*models.AssistantMCPServer, error) {
	return nil, nil
}
func (f *fakeAssistantMCPRepo) Create(context.Context, *models.AssistantMCPServer) error { return nil }
func (f *fakeAssistantMCPRepo) Update(context.Context, *models.AssistantMCPServer) error { return nil }
func (f *fakeAssistantMCPRepo) Delete(context.Context, uuid.UUID) error                  { return nil }

type fakeAssistantSecretResolver struct{ secrets map[string]string }

func (f *fakeAssistantSecretResolver) Resolve(_ context.Context, _ *models.Project, name string) (string, error) {
	v, ok := f.secrets[name]
	if !ok {
		return "", fmt.Errorf("secret %q not found", name)
	}
	return v, nil
}

func mustJSONHeaders(t *testing.T, m map[string]string) datatypes.JSON {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal headers: %v", err)
	}
	return datatypes.JSON(b)
}

// --- тесты ---

func TestValidateAssistantMCPServer(t *testing.T) {
	base := func() *models.AssistantMCPServer {
		return &models.AssistantMCPServer{Name: "srv", Transport: models.MCPTransportHTTP, URL: "https://x.example.com/mcp"}
	}
	t.Run("ok http", func(t *testing.T) {
		if err := validateAssistantMCPServer(base()); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})
	t.Run("ok sse", func(t *testing.T) {
		c := base()
		c.Transport = models.MCPTransportSSE
		if err := validateAssistantMCPServer(c); err != nil {
			t.Fatalf("unexpected: %v", err)
		}
	})
	t.Run("stdio rejected", func(t *testing.T) {
		c := base()
		c.Transport = models.MCPTransportStdio
		if err := validateAssistantMCPServer(c); err != ErrAssistantMCPInvalidTransport {
			t.Fatalf("got %v, want ErrAssistantMCPInvalidTransport", err)
		}
	})
	t.Run("empty name", func(t *testing.T) {
		c := base()
		c.Name = "  "
		if err := validateAssistantMCPServer(c); err != ErrAssistantMCPInvalidName {
			t.Fatalf("got %v, want ErrAssistantMCPInvalidName", err)
		}
	})
	t.Run("bad url scheme", func(t *testing.T) {
		c := base()
		c.URL = "ftp://x/mcp"
		if err := validateAssistantMCPServer(c); err != ErrAssistantMCPInvalidURL {
			t.Fatalf("got %v, want ErrAssistantMCPInvalidURL", err)
		}
	})
	t.Run("empty url", func(t *testing.T) {
		c := base()
		c.URL = ""
		if err := validateAssistantMCPServer(c); err != ErrAssistantMCPInvalidURL {
			t.Fatalf("got %v, want ErrAssistantMCPInvalidURL", err)
		}
	})
}

func TestResolveEnabledConfigs_SecretSubstitution(t *testing.T) {
	repo := &fakeAssistantMCPRepo{enabled: []models.AssistantMCPServer{
		{
			Name:                "GitHub",
			Transport:           models.MCPTransportHTTP,
			URL:                 "https://mcp.example.com/",
			Headers:             mustJSONHeaders(t, map[string]string{"Authorization": "Bearer ${secret:GH_TOKEN}", "X-Static": "v"}),
			RequireConfirmation: true,
		},
	}}
	svc := NewAssistantMCPServerService(repo, &fakeAssistantSecretResolver{secrets: map[string]string{"GH_TOKEN": "ghp_abc"}})

	project := &models.Project{ID: uuid.New()}
	got, err := svc.ResolveEnabledConfigs(context.Background(), project)
	if err != nil {
		t.Fatalf("ResolveEnabledConfigs: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 config, got %d", len(got))
	}
	c := got[0]
	if !c.RequireConfirmation {
		t.Error("expected RequireConfirmation=true")
	}
	if c.Config.Headers["Authorization"] != "Bearer ghp_abc" {
		t.Errorf("secret not substituted: %q", c.Config.Headers["Authorization"])
	}
	if c.Config.Headers["X-Static"] != "v" {
		t.Errorf("static header lost: %q", c.Config.Headers["X-Static"])
	}
	if string(c.Config.Transport) != "http" {
		t.Errorf("transport = %q, want http", c.Config.Transport)
	}
}

func TestResolveEnabledConfigs_MissingSecretFails(t *testing.T) {
	repo := &fakeAssistantMCPRepo{enabled: []models.AssistantMCPServer{
		{Name: "S", Transport: models.MCPTransportHTTP, URL: "https://x/", Headers: mustJSONHeaders(t, map[string]string{"Authorization": "Bearer ${secret:MISSING}"})},
	}}
	svc := NewAssistantMCPServerService(repo, &fakeAssistantSecretResolver{secrets: map[string]string{}})
	if _, err := svc.ResolveEnabledConfigs(context.Background(), &models.Project{ID: uuid.New()}); err == nil {
		t.Fatal("expected error for missing secret")
	}
}
