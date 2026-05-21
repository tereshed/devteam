package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// ---------------------------------------------------------------------------
// In-memory mock for MCPServerRegistryRepository
// ---------------------------------------------------------------------------

type mockMCPServerRegistryRepo struct {
	mu   sync.Mutex
	data map[uuid.UUID]*models.MCPServerRegistry
}

func newMockMCPServerRegistryRepo() *mockMCPServerRegistryRepo {
	return &mockMCPServerRegistryRepo{data: make(map[uuid.UUID]*models.MCPServerRegistry)}
}

func (m *mockMCPServerRegistryRepo) List(_ context.Context, onlyActive bool) ([]models.MCPServerRegistry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]models.MCPServerRegistry, 0, len(m.data))
	for _, s := range m.data {
		if onlyActive && !s.IsActive {
			continue
		}
		out = append(out, *s)
	}
	return out, nil
}

func (m *mockMCPServerRegistryRepo) GetByName(_ context.Context, name string) (*models.MCPServerRegistry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.data {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, repository.ErrMCPServerRegistryNotFound
}

func (m *mockMCPServerRegistryRepo) GetByID(_ context.Context, id uuid.UUID) (*models.MCPServerRegistry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.data[id]
	if !ok {
		return nil, repository.ErrMCPServerRegistryNotFound
	}
	return s, nil
}

func (m *mockMCPServerRegistryRepo) Create(_ context.Context, srv *models.MCPServerRegistry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Enforce unique name constraint.
	for _, existing := range m.data {
		if existing.Name == srv.Name {
			return fmt.Errorf("duplicate key value violates unique constraint")
		}
	}
	if srv.ID == uuid.Nil {
		srv.ID = uuid.New()
	}
	m.data[srv.ID] = srv
	return nil
}

func (m *mockMCPServerRegistryRepo) Update(_ context.Context, srv *models.MCPServerRegistry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[srv.ID]; !ok {
		return repository.ErrMCPServerRegistryNotFound
	}
	// Enforce unique name constraint.
	for _, existing := range m.data {
		if existing.Name == srv.Name && existing.ID != srv.ID {
			return fmt.Errorf("duplicate key value violates unique constraint")
		}
	}
	m.data[srv.ID] = srv
	return nil
}

func (m *mockMCPServerRegistryRepo) Delete(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.data[id]
	if !ok {
		return repository.ErrMCPServerRegistryNotFound
	}
	s.IsActive = false
	return nil
}

// seed adds a record and returns it for convenience.
func (m *mockMCPServerRegistryRepo) seed(srv models.MCPServerRegistry) models.MCPServerRegistry {
	m.mu.Lock()
	defer m.mu.Unlock()
	if srv.ID == uuid.Nil {
		srv.ID = uuid.New()
	}
	m.data[srv.ID] = &srv
	return srv
}

// ---------------------------------------------------------------------------
// Router helper
// ---------------------------------------------------------------------------

func setupMCPRegistryRouter(repo *mockMCPServerRegistryRepo, role models.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	svc := service.NewMCPServerRegistryService(repo)
	h := NewMCPServerRegistryHandler(svc)

	r.Use(func(c *gin.Context) {
		c.Set("userID", uuid.New())
		c.Set("userRole", string(role))
		c.Next()
	})
	r.Use(middleware.AdminOnlyMiddleware())

	r.GET("/admin/mcp-servers", h.List)
	r.GET("/admin/mcp-servers/:id", h.Get)
	r.POST("/admin/mcp-servers", h.Create)
	r.PUT("/admin/mcp-servers/:id", h.Update)
	r.DELETE("/admin/mcp-servers/:id", h.Delete)

	return r
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// 1. RBAC: non-admin gets 403
func TestMCPServerRegistry_NonAdmin_Returns403(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	r := setupMCPRegistryRouter(repo, models.RoleUser)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/admin/mcp-servers"},
		{"GET", "/admin/mcp-servers/" + uuid.New().String()},
		{"POST", "/admin/mcp-servers"},
		{"PUT", "/admin/mcp-servers/" + uuid.New().String()},
		{"DELETE", "/admin/mcp-servers/" + uuid.New().String()},
	}
	for _, ep := range endpoints {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(ep.method, ep.path, nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code, "%s %s should be 403", ep.method, ep.path)
	}
}

// 2. List: happy path
func TestMCPServerRegistry_List(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	repo.seed(models.MCPServerRegistry{Name: "srv-a", Transport: models.MCPTransportStdio, IsActive: true})
	repo.seed(models.MCPServerRegistry{Name: "srv-b", Transport: models.MCPTransportHTTP, IsActive: false})
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/mcp-servers", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []models.MCPServerRegistry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 2)
}

// 3. List with only_active=true
func TestMCPServerRegistry_List_OnlyActive(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	repo.seed(models.MCPServerRegistry{Name: "active", Transport: models.MCPTransportSSE, IsActive: true})
	repo.seed(models.MCPServerRegistry{Name: "disabled", Transport: models.MCPTransportStdio, IsActive: false})
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/mcp-servers?only_active=true", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []models.MCPServerRegistry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
	assert.Equal(t, "active", items[0].Name)
}

// 4. Get: happy path + not found
func TestMCPServerRegistry_Get(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	srv := repo.seed(models.MCPServerRegistry{
		Name: "my-mcp", Transport: models.MCPTransportHTTP, Scope: models.MCPScopeGlobal, IsActive: true,
	})
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	t.Run("found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin/mcp-servers/"+srv.ID.String(), nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var got models.MCPServerRegistry
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		assert.Equal(t, srv.ID, got.ID)
		assert.Equal(t, "my-mcp", got.Name)
	})

	t.Run("not_found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin/mcp-servers/"+uuid.New().String(), nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// 5. Create: happy path (stdio, global) → 201
func TestMCPServerRegistry_Create(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	body := map[string]interface{}{
		"name":      "new-server",
		"transport": "stdio",
		"command":   "/usr/local/bin/mcp",
		"args":      []string{"--verbose"},
		"scope":     "global",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/mcp-servers", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var created models.MCPServerRegistry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "new-server", created.Name)
	assert.Equal(t, models.MCPTransportStdio, created.Transport)
	assert.Equal(t, models.MCPScopeGlobal, created.Scope)
	assert.True(t, created.IsActive)
	assert.NotEqual(t, uuid.Nil, created.ID)
}

// 6. Create: invalid transport → 400
func TestMCPServerRegistry_Create_InvalidTransport(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	body := map[string]interface{}{
		"name":      "bad-transport",
		"transport": "grpc",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/mcp-servers", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// 7. Create: duplicate name → 409
func TestMCPServerRegistry_Create_DuplicateName(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	repo.seed(models.MCPServerRegistry{Name: "dup", Transport: models.MCPTransportStdio, IsActive: true})
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	body := map[string]interface{}{
		"name":      "dup",
		"transport": "stdio",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/mcp-servers", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// 8. Update: happy path → 200
func TestMCPServerRegistry_Update(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	srv := repo.seed(models.MCPServerRegistry{
		Name: "old-name", Transport: models.MCPTransportStdio, Scope: models.MCPScopeGlobal,
		IsActive: true, Args: datatypes.JSON("[]"), EnvTemplate: datatypes.JSON("{}"),
	})
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	isActive := false
	body := map[string]interface{}{
		"name":        "new-name",
		"transport":   "http",
		"url":         "http://localhost:9000",
		"scope":       "project",
		"is_active":   isActive,
		"args":        []string{},
		"env_template": map[string]string{},
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/mcp-servers/"+srv.ID.String(), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var updated models.MCPServerRegistry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, models.MCPTransportHTTP, updated.Transport)
	assert.Equal(t, models.MCPScopeProject, updated.Scope)
	assert.False(t, updated.IsActive)
}

// 9. Update: not found → 404
func TestMCPServerRegistry_Update_NotFound(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	body := map[string]interface{}{
		"name":      "whatever",
		"transport": "stdio",
	}
	b, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/admin/mcp-servers/"+uuid.New().String(), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// 10. Delete (soft): happy path → 204
func TestMCPServerRegistry_Delete(t *testing.T) {
	repo := newMockMCPServerRegistryRepo()
	srv := repo.seed(models.MCPServerRegistry{
		Name: "to-delete", Transport: models.MCPTransportStdio, IsActive: true,
	})
	r := setupMCPRegistryRouter(repo, models.RoleAdmin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/admin/mcp-servers/"+srv.ID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify the server is now inactive.
	repo.mu.Lock()
	assert.False(t, repo.data[srv.ID].IsActive)
	repo.mu.Unlock()
}
