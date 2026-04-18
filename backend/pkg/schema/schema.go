package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// Schema represents a compiled JSON Schema for validation.
type Schema struct {
	schema *jsonschema.Schema
}

// Compile loads a JSON Schema from file and compiles it.
func Compile(schemaPath string) (*Schema, error) {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	sch, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse schema json: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("file://"+schemaPath, sch); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	compiled, err := compiler.Compile("file://" + schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return &Schema{schema: compiled}, nil
}

// Validate validates a YAML file against the schema.
func (s *Schema) Validate(yamlPath string) error {
	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}
	var doc interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("yaml parse: %w", err)
	}
	intermediate, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal bridge: %w", err)
	}
	var jsonDoc interface{}
	if err := json.Unmarshal(intermediate, &jsonDoc); err != nil {
		return fmt.Errorf("unmarshal bridge: %w", err)
	}
	if err := s.schema.Validate(jsonDoc); err != nil {
		return fmt.Errorf("jsonschema: %w", err)
	}
	return nil
}

// ValidateBytes validates raw YAML bytes against the schema.
func (s *Schema) ValidateBytes(raw []byte) error {
	var doc interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("yaml parse: %w", err)
	}
	intermediate, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal bridge: %w", err)
	}
	var jsonDoc interface{}
	if err := json.Unmarshal(intermediate, &jsonDoc); err != nil {
		return fmt.Errorf("unmarshal bridge: %w", err)
	}
	if err := s.schema.Validate(jsonDoc); err != nil {
		return fmt.Errorf("jsonschema: %w", err)
	}
	return nil
}
