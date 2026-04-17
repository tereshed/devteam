package agentprompts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

func loadJSONSchema(path string) (*jsonschema.Schema, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	sch, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse schema json: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("file://"+path, sch); err != nil {
		return nil, fmt.Errorf("add schema resource: %w", err)
	}
	schema, err := compiler.Compile("file://" + path)
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return schema, nil
}

func validateYAMLFile(schema *jsonschema.Schema, yamlPath string) error {
	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return err
	}
	var doc interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("yaml parse: %w", err)
	}
	// JSON-compatible tree for jsonschema
	intermediate, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal bridge: %w", err)
	}
	var jsonDoc interface{}
	if err := json.Unmarshal(intermediate, &jsonDoc); err != nil {
		return fmt.Errorf("unmarshal bridge: %w", err)
	}
	if err := schema.Validate(jsonDoc); err != nil {
		return fmt.Errorf("jsonschema: %w", err)
	}
	return nil
}
