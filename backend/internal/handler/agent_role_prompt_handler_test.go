package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockRolePromptRepo struct {
	prompts map[string]*models.AgentRolePrompt
}

func newMockRolePromptRepo() *mockRolePromptRepo {
	return &mockRolePromptRepo{prompts: make(map[string]*models.AgentRolePrompt)}
}

func (m *mockRolePromptRepo) GetByRole(_ context.Context, role string) (*models.AgentRolePrompt, error) {
	p, ok := m.prompts[role]
	if !ok {
		return nil, repository.ErrAgentRolePromptNotFound
	}
	return p, nil
}

func (m *mockRolePromptRepo) List(_ context.Context) ([]models.AgentRolePrompt, error) {
	result := make([]models.AgentRolePrompt, 0, len(m.prompts))
	for _, p := range m.prompts {
		result = append(result, *p)
	}
	return result, nil
}

func (m *mockRolePromptRepo) Upsert(_ context.Context, prompt *models.AgentRolePrompt) error {
	m.prompts[prompt.Role] = prompt
	return nil
}

func setupRolePromptRouter(h *AgentRolePromptHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/admin/agent-role-prompts")
	{
		g.GET("", h.List)
		g.GET("/:role", h.GetByRole)
		g.PUT("/:role", h.Update)
	}
	return r
}

func TestRolePromptHandler_List(t *testing.T) {
	repo := newMockRolePromptRepo()
	desc := "test description"
	repo.prompts["assistant"] = &models.AgentRolePrompt{
		ID:          uuid.New(),
		Role:        "assistant",
		Content:     "test prompt",
		Description: &desc,
		UpdatedAt:   time.Now(),
	}

	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/agent-role-prompts", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result []agentRolePromptResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "assistant", result[0].Role)
	assert.Equal(t, "test prompt", result[0].Content)
}

func TestRolePromptHandler_List_Empty(t *testing.T) {
	repo := newMockRolePromptRepo()
	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/agent-role-prompts", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result []agentRolePromptResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestRolePromptHandler_GetByRole_Found(t *testing.T) {
	repo := newMockRolePromptRepo()
	repo.prompts["router"] = &models.AgentRolePrompt{
		ID:        uuid.New(),
		Role:      "router",
		Content:   "router prompt",
		UpdatedAt: time.Now(),
	}

	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/agent-role-prompts/router", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result agentRolePromptResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "router", result.Role)
	assert.Equal(t, "router prompt", result.Content)
}

func TestRolePromptHandler_GetByRole_NotFound(t *testing.T) {
	repo := newMockRolePromptRepo()
	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/admin/agent-role-prompts/nonexistent", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRolePromptHandler_Update_Success(t *testing.T) {
	repo := newMockRolePromptRepo()
	repo.prompts["planner"] = &models.AgentRolePrompt{
		ID:        uuid.New(),
		Role:      "planner",
		Content:   "old prompt",
		UpdatedAt: time.Now(),
	}

	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h)

	body := updateRolePromptRequest{
		Content: "updated planner prompt",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/admin/agent-role-prompts/planner", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result agentRolePromptResponse
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "planner", result.Role)
	assert.Equal(t, "updated planner prompt", result.Content)
	assert.Equal(t, "updated planner prompt", repo.prompts["planner"].Content)
}

func TestRolePromptHandler_Update_NotFound(t *testing.T) {
	repo := newMockRolePromptRepo()
	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h)

	body := updateRolePromptRequest{Content: "new"}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/admin/agent-role-prompts/missing", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRolePromptHandler_Update_EmptyContent(t *testing.T) {
	repo := newMockRolePromptRepo()
	repo.prompts["assistant"] = &models.AgentRolePrompt{
		ID:      uuid.New(),
		Role:    "assistant",
		Content: "existing",
	}

	h := NewAgentRolePromptHandler(repo)
	router := setupRolePromptRouter(h)

	body := `{"content": ""}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/admin/agent-role-prompts/assistant", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
