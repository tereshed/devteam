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
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─────────────────────────────────────────────────────────────────────────────
// In-memory mocks (handler-test scope)
// ─────────────────────────────────────────────────────────────────────────────

type handlerMemAgentRepo struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*models.Agent
	byName map[string]*models.Agent
}

func newHandlerMemAgentRepo() *handlerMemAgentRepo {
	return &handlerMemAgentRepo{byID: map[uuid.UUID]*models.Agent{}, byName: map[string]*models.Agent{}}
}

func (r *handlerMemAgentRepo) Create(_ context.Context, a *models.Agent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[a.Name]; exists {
		return repository.ErrAgentNameTaken
	}
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	cp := *a
	r.byID[a.ID] = &cp
	r.byName[a.Name] = &cp
	return nil
}
func (r *handlerMemAgentRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrAgentNotFound
	}
	cp := *a
	return &cp, nil
}
func (r *handlerMemAgentRepo) GetByIDForUpdate(_ context.Context, id uuid.UUID) (*models.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return nil, repository.ErrAgentNotFound
	}
	cp := *a
	return &cp, nil
}
func (r *handlerMemAgentRepo) GetByUserAndRole(_ context.Context, userID uuid.UUID, role string) (*models.Agent, error) {
	for _, a := range r.byID {
		if a.UserID != nil && *a.UserID == userID && string(a.Role) == role {
			cp := *a
			return &cp, nil
		}
	}
	return nil, repository.ErrAgentNotFound
}

func (r *handlerMemAgentRepo) GetByName(_ context.Context, name string) (*models.Agent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byName[name]
	if !ok {
		return nil, repository.ErrAgentNotFound
	}
	cp := *a
	return &cp, nil
}
func (r *handlerMemAgentRepo) List(_ context.Context, f repository.AgentFilter) ([]models.Agent, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]models.Agent, 0, len(r.byID))
	for _, a := range r.byID {
		if f.UserID != nil && (a.UserID == nil || *a.UserID != *f.UserID) {
			continue
		}
		out = append(out, *a)
	}
	return out, int64(len(out)), nil
}
func (r *handlerMemAgentRepo) Update(_ context.Context, a *models.Agent, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[a.ID]; !ok {
		return repository.ErrAgentNotFound
	}
	cp := *a
	cp.UpdatedAt = time.Now().UTC()
	r.byID[a.ID] = &cp
	r.byName[a.Name] = &cp
	return nil
}
func (r *handlerMemAgentRepo) Delete(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return repository.ErrAgentNotFound
	}
	delete(r.byID, id)
	delete(r.byName, a.Name)
	return nil
}

type handlerMemSecretRepo struct{}

func (r *handlerMemSecretRepo) Create(_ context.Context, _ *models.AgentSecret) error         { return nil }
func (r *handlerMemSecretRepo) GetByName(_ context.Context, _ uuid.UUID, _ string) (*models.AgentSecret, error) {
	return nil, repository.ErrAgentSecretNotFound
}
func (r *handlerMemSecretRepo) ListByAgentID(_ context.Context, _ uuid.UUID) ([]models.AgentSecret, error) {
	return nil, nil
}
func (r *handlerMemSecretRepo) Delete(_ context.Context, _ uuid.UUID) error               { return nil }
func (r *handlerMemSecretRepo) DeleteByAgentID(_ context.Context, _ uuid.UUID) error      { return nil }

type handlerMemTxManager struct{ mu sync.Mutex }

func (m *handlerMemTxManager) WithTransaction(_ context.Context, fn func(ctx context.Context) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(context.Background())
}

type handlerMemLlmCredRepo struct {
	mu    sync.Mutex
	creds map[string]bool
}

func newHandlerMemLlmCredRepo() *handlerMemLlmCredRepo {
	return &handlerMemLlmCredRepo{creds: map[string]bool{}}
}

func (r *handlerMemLlmCredRepo) seed(userID uuid.UUID, provider models.UserLLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.creds[userID.String()+":"+string(provider)] = true
}

func (r *handlerMemLlmCredRepo) GetByUserAndProvider(_ context.Context, userID uuid.UUID, provider models.UserLLMProvider) (*models.UserLlmCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.creds[userID.String()+":"+string(provider)]; ok {
		return &models.UserLlmCredential{ID: uuid.New(), UserID: userID, Provider: provider}, nil
	}
	return nil, repository.ErrUserLlmCredentialNotFound
}
func (r *handlerMemLlmCredRepo) ListByUserID(_ context.Context, _ uuid.UUID) ([]models.UserLlmCredential, error) {
	return nil, nil
}
func (r *handlerMemLlmCredRepo) Create(_ context.Context, _ *models.UserLlmCredential) error  { return nil }
func (r *handlerMemLlmCredRepo) Update(_ context.Context, _ *models.UserLlmCredential) error  { return nil }
func (r *handlerMemLlmCredRepo) DeleteByUserAndProvider(_ context.Context, _ uuid.UUID, _ models.UserLLMProvider) (int64, error) {
	return 0, nil
}
func (r *handlerMemLlmCredRepo) CreateAudit(_ context.Context, _ *models.UserLlmCredentialAudit) error {
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Setup
// ─────────────────────────────────────────────────────────────────────────────

type agentMyTestFixture struct {
	handler     *AgentMyHandler
	agentRepo   *handlerMemAgentRepo
	llmCredRepo *handlerMemLlmCredRepo
	svc         *service.AgentService
}

func setupAgentMyFixture(t *testing.T) *agentMyTestFixture {
	t.Helper()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	enc, err := crypto.NewAESEncryptor(key)
	require.NoError(t, err)

	agentRepo := newHandlerMemAgentRepo()
	llmCredRepo := newHandlerMemLlmCredRepo()

	svc := service.NewAgentService(agentRepo, &handlerMemSecretRepo{}, enc, &handlerMemTxManager{})
	svc.WithLlmCredRepo(llmCredRepo)

	return &agentMyTestFixture{
		handler:     NewAgentMyHandler(svc),
		agentRepo:   agentRepo,
		llmCredRepo: llmCredRepo,
		svc:         svc,
	}
}

func setupMyAgentRouter(h *AgentMyHandler, userID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})
	g := r.Group("/me/agents")
	{
		g.GET("", h.List)
		g.GET("/:id", h.Get)
		g.PUT("/:id", h.Update)
	}
	return r
}

func createTestUserAgent(t *testing.T, repo *handlerMemAgentRepo, userID uuid.UUID, name string) *models.Agent {
	t.Helper()
	a := &models.Agent{
		ID:            uuid.New(),
		Name:          name,
		Role:          models.AgentRoleAssistant,
		ExecutionKind: models.AgentExecutionKindLLM,
		UserID:        &userID,
		IsActive:      true,
	}
	err := repo.Create(context.Background(), a)
	require.NoError(t, err)
	return a
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — List
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentMyHandler_List_HappyPath(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	createTestUserAgent(t, f.agentRepo, userID, "assistant")

	otherUser := uuid.New()
	createTestUserAgent(t, f.agentRepo, otherUser, "other-assistant")

	router := setupMyAgentRouter(f.handler, userID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/me/agents", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(1), body["total"])
	items := body["items"].([]any)
	assert.Len(t, items, 1)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — Get
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentMyHandler_Get_HappyPath(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, userID, "assistant")

	router := setupMyAgentRouter(f.handler, userID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/me/agents/"+agent.ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body models.Agent
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, agent.ID, body.ID)
}

func TestAgentMyHandler_Get_ABAC_ForbidsOtherUsersAgent(t *testing.T) {
	f := setupAgentMyFixture(t)
	ownerID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, ownerID, "assistant")

	attackerID := uuid.New()
	router := setupMyAgentRouter(f.handler, attackerID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/me/agents/"+agent.ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAgentMyHandler_Get_NotFound(t *testing.T) {
	f := setupAgentMyFixture(t)
	router := setupMyAgentRouter(f.handler, uuid.New())
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/me/agents/"+uuid.New().String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — Update
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentMyHandler_Update_HappyPath(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, userID, "assistant")

	router := setupMyAgentRouter(f.handler, userID)
	body, _ := json.Marshal(map[string]any{
		"system_prompt":        "You are a helpful assistant.",
		"internal_mcp_enabled": true,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/me/agents/"+agent.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var updated models.Agent
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, "You are a helpful assistant.", *updated.SystemPrompt)
	assert.True(t, updated.InternalMCPEnabled)
}

func TestAgentMyHandler_Update_ABAC_ForbidsOtherUsersAgent(t *testing.T) {
	f := setupAgentMyFixture(t)
	ownerID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, ownerID, "assistant")

	attackerID := uuid.New()
	router := setupMyAgentRouter(f.handler, attackerID)
	body, _ := json.Marshal(map[string]any{"is_active": false})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/me/agents/"+agent.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestAgentMyHandler_Update_RejectsTeamID(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, userID, "assistant")

	router := setupMyAgentRouter(f.handler, userID)
	teamID := uuid.New()
	body, _ := json.Marshal(map[string]any{"team_id": teamID.String()})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/me/agents/"+agent.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "team_id")
}

func TestAgentMyHandler_Update_RejectsRoleChange(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, userID, "assistant")

	router := setupMyAgentRouter(f.handler, userID)
	body, _ := json.Marshal(map[string]any{"role": "developer"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/me/agents/"+agent.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "role")
}

func TestAgentMyHandler_Update_RejectsExecutionKindChange(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, userID, "assistant")

	router := setupMyAgentRouter(f.handler, userID)
	body, _ := json.Marshal(map[string]any{"execution_kind": "sandbox"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/me/agents/"+agent.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "execution_kind")
}

// ─────────────────────────────────────────────────────────────────────────────
// Tests — §4.3 Provider validation (through handler)
// ─────────────────────────────────────────────────────────────────────────────

func TestAgentMyHandler_Update_ProviderNotConnected_Returns422(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, userID, "assistant")
	// NOT seeding llm credentials — provider is not connected.

	router := setupMyAgentRouter(f.handler, userID)
	body, _ := json.Marshal(map[string]any{
		"model":         "claude-sonnet-4-6",
		"provider_kind": "anthropic",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/me/agents/%s", agent.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "not connected")
}

func TestAgentMyHandler_Update_ProviderConnected_Succeeds(t *testing.T) {
	f := setupAgentMyFixture(t)
	userID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, userID, "assistant")
	f.llmCredRepo.seed(userID, models.UserLLMProviderAnthropic)

	router := setupMyAgentRouter(f.handler, userID)
	body, _ := json.Marshal(map[string]any{
		"model":         "claude-sonnet-4-6",
		"provider_kind": "anthropic",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/me/agents/%s", agent.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var updated models.Agent
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, "claude-sonnet-4-6", *updated.Model)
}

// ABAC runs before body parse: attacker with garbage body still gets 403, not 400.
func TestAgentMyHandler_Update_ABAC_BeforeBodyParse(t *testing.T) {
	f := setupAgentMyFixture(t)
	ownerID := uuid.New()
	agent := createTestUserAgent(t, f.agentRepo, ownerID, "assistant")

	attackerID := uuid.New()
	router := setupMyAgentRouter(f.handler, attackerID)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/me/agents/"+agent.ID.String(), bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
