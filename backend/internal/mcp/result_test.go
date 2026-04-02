package mcp

import (
	"encoding/json"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- OK / Err / ValidationErr ---

func TestOK(t *testing.T) {
	type payload struct {
		Value string `json:"value"`
	}

	result, structured, err := OK("success details", &payload{Value: "test"})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	resp, ok := structured.(*Response)
	require.True(t, ok)
	assert.Equal(t, StatusOK, resp.Status)
	assert.Equal(t, "success details", resp.Details)
	assert.NotNil(t, resp.Data)

	assertResponseJSON(t, result, StatusOK, "success details")
}

func TestErr(t *testing.T) {
	result, structured, err := Err("something failed", assert.AnError)

	require.NoError(t, err) // error всегда nil (HTTP 2xx)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	resp, ok := structured.(*Response)
	require.True(t, ok)
	assert.Equal(t, StatusError, resp.Status)
	assert.Equal(t, "something failed", resp.Details)
	assert.Nil(t, resp.Data)

	assertResponseJSON(t, result, StatusError, "something failed")
}

func TestErr_NilError(t *testing.T) {
	result, _, err := Err("no internal error", nil)

	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestValidationErr(t *testing.T) {
	result, structured, err := ValidationErr("field X is required")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	resp, ok := structured.(*Response)
	require.True(t, ok)
	assert.Equal(t, StatusError, resp.Status)
	assert.Equal(t, "field X is required", resp.Details)
	assert.Nil(t, resp.Data)
}

func TestOK_IsError_Status_Contract(t *testing.T) {
	// Контракт: OK → IsError=false, Status="ok"
	result, structured, _ := OK("ok", nil)
	assert.False(t, result.IsError)
	assert.Equal(t, StatusOK, structured.(*Response).Status)

	// Контракт: Err → IsError=true, Status="error"
	result, structured, _ = Err("err", nil)
	assert.True(t, result.IsError)
	assert.Equal(t, StatusError, structured.(*Response).Status)

	// Контракт: ValidationErr → IsError=true, Status="error"
	result, structured, _ = ValidationErr("val")
	assert.True(t, result.IsError)
	assert.Equal(t, StatusError, structured.(*Response).Status)
}

// --- Truncate ---

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"empty string", "", 10, ""},
		{"within limit", "hello", 10, "hello"},
		{"exact limit", "hello", 5, "hello"},
		{"exceeds limit", "hello world", 5, "hello..."},
		{"unicode", "Привет мир!", 6, "Привет..."},
		{"zero max", "hello", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Truncate(tt.input, tt.max))
		})
	}
}

// --- PaginateDefaults ---

func TestPaginateDefaults(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name           string
		limitPtr       *int
		offsetPtr      *int
		defaultLimit   int
		maxLimit       int
		expectedLimit  int
		expectedOffset int
	}{
		{"nil/nil → defaults", nil, nil, 50, 100, 50, 0},
		{"custom limit", intPtr(20), nil, 50, 100, 20, 0},
		{"limit exceeds max", intPtr(200), nil, 50, 100, 100, 0},
		{"custom offset", nil, intPtr(10), 50, 100, 50, 10},
		{"both custom", intPtr(30), intPtr(5), 50, 100, 30, 5},
		{"zero limit → default", intPtr(0), nil, 50, 100, 50, 0},
		{"negative limit → default", intPtr(-1), nil, 50, 100, 50, 0},
		{"negative offset → 0", nil, intPtr(-5), 50, 100, 50, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit, offset := PaginateDefaults(tt.limitPtr, tt.offsetPtr, tt.defaultLimit, tt.maxLimit)
			assert.Equal(t, tt.expectedLimit, limit)
			assert.Equal(t, tt.expectedOffset, offset)
		})
	}
}

// --- Paginate ---

func TestPaginate(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		name     string
		limit    int
		offset   int
		expected []int
	}{
		{"first page", 3, 0, []int{1, 2, 3}},
		{"second page", 3, 3, []int{4, 5, 6}},
		{"last partial page", 3, 9, []int{10}},
		{"offset beyond", 3, 15, []int{}},
		{"large limit", 100, 0, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Paginate(items, tt.limit, tt.offset)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPaginate_EmptySlice(t *testing.T) {
	result := Paginate([]string{}, 10, 0)
	assert.Empty(t, result)
}

// --- Helpers ---

func assertResponseJSON(t *testing.T, result *gomcp.CallToolResult, expectedStatus, expectedDetails string) {
	t.Helper()
	require.Len(t, result.Content, 1)

	tc, ok := result.Content[0].(*gomcp.TextContent)
	require.True(t, ok, "expected *mcp.TextContent")

	var resp Response
	err := json.Unmarshal([]byte(tc.Text), &resp)
	require.NoError(t, err, "Content should be valid JSON")
	assert.Equal(t, expectedStatus, resp.Status)
	assert.Equal(t, expectedDetails, resp.Details)
}
