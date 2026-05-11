package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/devteam/backend/internal/handler/dto"
)

func TestToolDefinitionsList_Success(t *testing.T) {
	svc := new(mockToolDefinitionService)
	h := makeToolDefinitionsListHandler(svc)
	ctx := testUserCtx(t)

	svc.On("ListActiveCatalog", mock.Anything).Return([]dto.ToolDefinitionListItemResponse{
		{ID: "a", Name: "n", Description: "d", Category: "c", IsBuiltin: true},
	}, nil)

	result, structured, err := h(ctx, nil, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	data := structured.(*Response).Data.([]dto.ToolDefinitionListItemResponse)
	require.Len(t, data, 1)
	assert.Equal(t, "n", data[0].Name)
	svc.AssertExpectations(t)
}

func TestToolDefinitionsList_NoAuth(t *testing.T) {
	svc := new(mockToolDefinitionService)
	h := makeToolDefinitionsListHandler(svc)

	result, _, err := h(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	svc.AssertNotCalled(t, "ListActiveCatalog")
}
