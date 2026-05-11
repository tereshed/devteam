package dto

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchAgentRequest_Unmarshal_EmptyObject(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &p))
	assert.False(t, p.ModelPresent())
	assert.False(t, p.PromptIDPresent())
	assert.False(t, p.CodeBackendPresent())
	assert.False(t, p.IsActivePresent())
	assert.False(t, p.ToolBindingsPresent())
}

func TestPatchAgentRequest_Unmarshal_ModelNull(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":null}`), &p))
	assert.True(t, p.ModelPresent())
	assert.True(t, p.ModelClear())
	_, ok := p.ModelValue()
	assert.False(t, ok)
}

func TestPatchAgentRequest_Unmarshal_ModelValue(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"  gpt-4  "}`), &p))
	assert.True(t, p.ModelPresent())
	assert.False(t, p.ModelClear())
	v, ok := p.ModelValue()
	assert.True(t, ok)
	assert.Equal(t, "  gpt-4  ", v)
}

func TestPatchAgentRequest_Unmarshal_PromptIDNull(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"prompt_id":null}`), &p))
	assert.True(t, p.PromptIDPresent())
	assert.True(t, p.PromptIDClear())
}

func TestPatchAgentRequest_Unmarshal_PromptIDValue(t *testing.T) {
	id := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"prompt_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}`), &p))
	got, ok := p.PromptIDValue()
	assert.True(t, ok)
	assert.Equal(t, id, got)
}

func TestPatchAgentRequest_Unmarshal_CodeBackendNull(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"code_backend":null}`), &p))
	assert.True(t, p.CodeBackendPresent())
	assert.True(t, p.CodeBackendClear())
}

func TestPatchAgentRequest_Unmarshal_CodeBackendValue(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"code_backend":"aider"}`), &p))
	v, ok := p.CodeBackendValue()
	assert.True(t, ok)
	assert.Equal(t, "aider", v)
}

func TestPatchAgentRequest_Unmarshal_IsActive(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"is_active":false}`), &p))
	v, ok := p.IsActiveValue()
	assert.True(t, ok)
	assert.False(t, v)
}

func TestPatchAgentRequest_Unmarshal_IsActiveNullRejected(t *testing.T) {
	var p PatchAgentRequest
	err := json.Unmarshal([]byte(`{"is_active":null}`), &p)
	require.Error(t, err)
}

func TestPatchAgentRequest_Unmarshal_ToolBindingsOmit(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"model":"x"}`), &p))
	assert.False(t, p.ToolBindingsPresent())
}

func TestPatchAgentRequest_Unmarshal_ToolBindingsEmptyArray(t *testing.T) {
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(`{"tool_bindings":[]}`), &p))
	assert.True(t, p.ToolBindingsPresent())
	assert.Len(t, p.ToolBindingsRawIDs(), 0)
}

func TestPatchAgentRequest_Unmarshal_ToolBindingsValues(t *testing.T) {
	a := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	b := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	var p PatchAgentRequest
	require.NoError(t, json.Unmarshal([]byte(
		`{"tool_bindings":[{"tool_definition_id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"},{"tool_definition_id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"}]}`,
	), &p))
	assert.True(t, p.ToolBindingsPresent())
	assert.Equal(t, []uuid.UUID{a, b}, p.ToolBindingsRawIDs())
}

func TestPatchAgentRequest_Unmarshal_ToolBindingsNullRejected(t *testing.T) {
	var p PatchAgentRequest
	err := json.Unmarshal([]byte(`{"tool_bindings":null}`), &p)
	require.Error(t, err)
}
