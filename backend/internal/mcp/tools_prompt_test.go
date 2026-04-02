package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/service"
	"gorm.io/datatypes"
)

// --- prompt_list ---

func TestPromptList_Success(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptListHandler(svc)

	svc.On("List", mock.Anything).Return([]models.Prompt{
		{ID: uuid.New(), Name: "prompt1", Description: "desc1", IsActive: true},
		{ID: uuid.New(), Name: "prompt2", Description: "desc2", IsActive: false}, // inactive — filtered
		{ID: uuid.New(), Name: "prompt3", Description: "desc3", IsActive: true},
	}, nil)

	result, structured, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	resp := structured.(*Response)
	data := resp.Data.(*PromptListData)
	assert.Equal(t, 2, data.Count)        // только активные
	assert.Len(t, data.Prompts, 2)
	assert.Equal(t, "prompt1", data.Prompts[0].Name)
	assert.Equal(t, "prompt3", data.Prompts[1].Name)
	svc.AssertExpectations(t)
}

func TestPromptList_Empty(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptListHandler(svc)

	svc.On("List", mock.Anything).Return([]models.Prompt{}, nil)

	result, structured, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*PromptListData)
	assert.Equal(t, 0, data.Count)
	assert.Empty(t, data.Prompts)
	svc.AssertExpectations(t)
}

func TestPromptList_ServiceError(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptListHandler(svc)

	svc.On("List", mock.Anything).Return(nil, assert.AnError)

	result, _, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestPromptList_Pagination(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptListHandler(svc)

	// 5 активных промптов
	prompts := make([]models.Prompt, 5)
	for i := range prompts {
		prompts[i] = models.Prompt{ID: uuid.New(), Name: "p" + string(rune('A'+i)), IsActive: true}
	}
	svc.On("List", mock.Anything).Return(prompts, nil)

	limit := 2
	offset := 1
	result, structured, err := handler(context.Background(), nil, &PromptListParams{Limit: &limit, Offset: &offset})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*PromptListData)
	assert.Equal(t, 5, data.Count)   // total
	assert.Len(t, data.Prompts, 2)   // limit=2
	svc.AssertExpectations(t)
}

// --- prompt_get ---

func TestPromptGet_NilParams(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	result, _, err := handler(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestPromptGet_BothIDAndName(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	result, _, err := handler(context.Background(), nil, &PromptGetParams{
		ID:   uuid.New().String(),
		Name: "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError) // "specify only one"
}

func TestPromptGet_EmptyBoth(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	result, _, err := handler(context.Background(), nil, &PromptGetParams{})
	require.NoError(t, err)
	assert.True(t, result.IsError) // "either id or name is required"
}

func TestPromptGet_InvalidUUID(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	result, _, err := handler(context.Background(), nil, &PromptGetParams{ID: "not-a-uuid"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestPromptGet_ByID_Success(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	id := uuid.New()
	now := time.Now()
	prompt := &models.Prompt{
		ID:          id,
		Name:        "test-prompt",
		Description: "A test prompt",
		Template:    "Hello {{name}}",
		JSONSchema:  datatypes.JSON(`{"type":"object"}`),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	svc.On("GetByID", mock.Anything, id).Return(prompt, nil)

	result, structured, err := handler(context.Background(), nil, &PromptGetParams{ID: id.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	resp := structured.(*Response)
	assert.Contains(t, resp.Details, "test-prompt")

	data := resp.Data.(*PromptGetData)
	assert.Equal(t, id.String(), data.ID)
	assert.Equal(t, "test-prompt", data.Name)
	assert.Equal(t, "Hello {{name}}", data.Template)
	assert.True(t, data.IsActive)
	assert.NotNil(t, data.JSONSchema)
	svc.AssertExpectations(t)
}

func TestPromptGet_ByName_Success(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	prompt := &models.Prompt{
		ID:       uuid.New(),
		Name:     "my-prompt",
		IsActive: true,
	}
	svc.On("GetByName", mock.Anything, "my-prompt").Return(prompt, nil)

	result, structured, err := handler(context.Background(), nil, &PromptGetParams{Name: "my-prompt"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*PromptGetData)
	assert.Equal(t, "my-prompt", data.Name)
	svc.AssertExpectations(t)
}

func TestPromptGet_ByID_NotFound(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(nil, service.ErrPromptNotFound)

	result, _, err := handler(context.Background(), nil, &PromptGetParams{ID: id.String()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertExpectations(t)
}

func TestPromptGet_InactiveWarning(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	id := uuid.New()
	prompt := &models.Prompt{ID: id, Name: "old-prompt", IsActive: false}
	svc.On("GetByID", mock.Anything, id).Return(prompt, nil)

	result, structured, err := handler(context.Background(), nil, &PromptGetParams{ID: id.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	resp := structured.(*Response)
	assert.Contains(t, resp.Details, "WARNING: inactive")
	svc.AssertExpectations(t)
}

func TestPromptGet_InvalidJSONSchema(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	id := uuid.New()
	prompt := &models.Prompt{
		ID:         id,
		Name:       "bad-schema",
		JSONSchema: datatypes.JSON(`{not valid json`),
		IsActive:   true,
	}
	svc.On("GetByID", mock.Anything, id).Return(prompt, nil)

	result, structured, err := handler(context.Background(), nil, &PromptGetParams{ID: id.String()})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data := structured.(*Response).Data.(*PromptGetData)
	// JSONSchema невалидный — должен быть nil (или не включён)
	assert.Nil(t, data.JSONSchema)
	svc.AssertExpectations(t)
}

func TestPromptGet_NameTooLong(t *testing.T) {
	svc := new(mockPromptService)
	handler := makePromptGetHandler(svc)

	longName := strings.Repeat("a", 256)
	result, _, err := handler(context.Background(), nil, &PromptGetParams{Name: longName})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
