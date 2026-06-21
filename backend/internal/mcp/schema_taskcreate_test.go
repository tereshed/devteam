package mcp

import (
	"encoding/json"
	"testing"
)

// TestSchemaTaskCreateDeclaresExternalKey проверяет, что схема task_create —
// валидный JSON и объявляет параметр external_key (иначе ассистент не сможет
// его заполнить, и хард-гейт ErrExternalKeyRequired будет бить вслепую).
func TestSchemaTaskCreateDeclaresExternalKey(t *testing.T) {
	if !json.Valid(schemaTaskCreate) {
		t.Fatal("schemaTaskCreate is not valid JSON")
	}
	var parsed struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schemaTaskCreate, &parsed); err != nil {
		t.Fatalf("unmarshal schemaTaskCreate: %v", err)
	}
	if _, ok := parsed.Properties["external_key"]; !ok {
		t.Fatal("schemaTaskCreate.properties must declare external_key")
	}
}
