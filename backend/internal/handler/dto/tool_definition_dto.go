package dto

// ToolDefinitionListItemResponse — элемент каталога GET /tool-definitions (без parameters_schema).
type ToolDefinitionListItemResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IsBuiltin   bool   `json:"is_builtin"`
}

// ToolBindingPatchItem — один элемент массива tool_bindings в PATCH агента.
type ToolBindingPatchItem struct {
	ToolDefinitionID string `json:"tool_definition_id"`
}
