package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devteam/backend/internal/middleware"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mock via testify/mock ---

type MockAgentRolePromptRepo struct {
	mock.Mock
}

func (m *MockAgentRolePromptRepo) GetByRole(ctx context.Context, role string) (*models.AgentRolePrompt, error) {
	args := m.Called(ctx, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AgentRolePrompt), args.Error(1)
}

func (m *MockAgentRolePromptRepo) List(ctx context.Context) ([]models.AgentRolePrompt, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AgentRolePrompt), args.Error(1)
}

func (m *MockAgentRolePromptRepo) Upsert(ctx context.Context, prompt *models.AgentRolePrompt) error {
	args := m.Called(ctx, prompt)
	return args.Error(0)
}

// --- Router helper ---

func setupRolePromptRouter(h *AgentRolePromptHandler, userID uuid.UUID, role models.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("userRole", string(role))
		c.Next()
	})
	r.Use(middleware.AdminOnlyMiddleware())
	r.GET("/admin/agent-role-prompts", h.List)
	r.GET("/admin/agent-role-prompts/:role", h.GetByRole)
	r.PUT("/admin/agent-role-prompts/:role", h.Update)
	return r
}

// --- Tests ---

func TestRolePrompt_RBAC_NonAdmin_Forbidden(t *testing.T) {
	repo := new(MockAgentRolePromptRepo)
	h := NewAgentRolePromptHandler(repo)
	userID := uuid.New()
	router := setupRolePromptRouter(h, userID, models.RoleUser)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/admin/agent-role-prompts"},
		{"GET", "/admin/agent-role-prompts/assistant"},
		{"PUT", "/admin/agent-role-prompts/assistant"},
	}

	for _, ep := range endpoints {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(ep.method, ep.path, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code, "%s %s should be 403 for non-admin", ep.method, ep.path)
	}

	repo.AssertNotCalled(t, "List", mock.Anything)
	repo.AssertNotCalled(t, "GetByRole", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}

func TestRolePrompt_List_OK(t *testing.T) {
	repo := new(MockAgentRolePromptRepo)
	desc := "desc"
	prompts := []models.AgentRolePrompt{
		{ID: uuid.New(), Role: "assistant", Content: "prompt-a", Description: &desc, UpdatedAt: time.Now().UTC()},
		{ID: uuid.New(), Role: "router", Content: "prompt-r", UpdatedAt: time.Now().UTC()},
	}
	repo.On("List", mock.Anything).Return(prompts, nil)

	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h, uuid.New(), models.RoleAdmin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/agent-role-prompts", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result []agentRolePromptResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Len(t, result, 2)
	repo.AssertExpectations(t)
}

func TestRolePrompt_GetByRole_OK(t *testing.T) {
	repo := new(MockAgentRolePromptRepo)
	p := &models.AgentRolePrompt{
		ID: uuid.New(), Role: "router", Content: "router prompt", UpdatedAt: time.Now().UTC(),
	}
	repo.On("GetByRole", mock.Anything, "router").Return(p, nil)

	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h, uuid.New(), models.RoleAdmin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/agent-role-prompts/router", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result agentRolePromptResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "router", result.Role)
	assert.Equal(t, "router prompt", result.Content)
	repo.AssertExpectations(t)
}

func TestRolePrompt_GetByRole_NotFound(t *testing.T) {
	repo := new(MockAgentRolePromptRepo)
	repo.On("GetByRole", mock.Anything, "nonexistent").Return(nil, repository.ErrAgentRolePromptNotFound)

	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h, uuid.New(), models.RoleAdmin)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/admin/agent-role-prompts/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	repo.AssertExpectations(t)
}

func TestRolePrompt_Update_OK(t *testing.T) {
	repo := new(MockAgentRolePromptRepo)
	existing := &models.AgentRolePrompt{
		ID: uuid.New(), Role: "planner", Content: "old", UpdatedAt: time.Now().UTC(),
	}
	repo.On("GetByRole", mock.Anything, "planner").Return(existing, nil)
	repo.On("Upsert", mock.Anything, mock.Anything).Return(nil)

	userID := uuid.New()
	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h, userID, models.RoleAdmin)

	body, _ := json.Marshal(updateRolePromptRequest{Content: "new content"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/admin/agent-role-prompts/planner", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result agentRolePromptResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "new content", result.Content)
	assert.Equal(t, "planner", result.Role)
	require.NotNil(t, result.UpdatedBy)
	assert.Equal(t, userID, *result.UpdatedBy)
	repo.AssertExpectations(t)
}

func TestRolePrompt_Update_EmptyContent_400(t *testing.T) {
	repo := new(MockAgentRolePromptRepo)
	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h, uuid.New(), models.RoleAdmin)

	body := []byte(`{"content":""}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/admin/agent-role-prompts/assistant", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertNotCalled(t, "GetByRole", mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "Upsert", mock.Anything, mock.Anything)
}
