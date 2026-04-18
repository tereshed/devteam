package agentprompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/devteam/backend/pkg/schema"
	"gopkg.in/yaml.v3"
)

// Composer loads base + role pipeline prompts from a directory and merges system text (task 6.8).
type Composer struct {
	dir         string
	baseSystem  string
	roleSystems map[string]string
	roleUsers   map[string]string
}

// NewComposer loads and validates base_prompt.yaml plus all role files against prompt_schema.json.
func NewComposer(dir string) (*Composer, error) {
	schemaPath := filepath.Join(dir, "prompt_schema.json")
	s, err := schema.Compile(schemaPath)
	if err != nil {
		return nil, err
	}

	basePath := filepath.Join(dir, "base_prompt.yaml")
	if err := s.Validate(basePath); err != nil {
		return nil, fmt.Errorf("agentprompts: base prompt: %w", err)
	}

	baseData, err := os.ReadFile(basePath)
	if err != nil {
		return nil, err
	}
	var baseDoc promptFile
	if err := yaml.Unmarshal(baseData, &baseDoc); err != nil {
		return nil, fmt.Errorf("agentprompts: parse base: %w", err)
	}

	c := &Composer{
		dir:         dir,
		baseSystem:  strings.TrimSpace(baseDoc.System.Content),
		roleSystems: make(map[string]string),
		roleUsers:   make(map[string]string),
	}

	roles := []string{"orchestrator", "planner", "developer", "reviewer", "tester"}
	for _, r := range roles {
		path := filepath.Join(dir, r+".yaml")
		if err := s.Validate(path); err != nil {
			return nil, fmt.Errorf("agentprompts: role %s: %w", r, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var roleDoc promptFile
		if err := yaml.Unmarshal(data, &roleDoc); err != nil {
			return nil, fmt.Errorf("agentprompts: parse %s: %w", r, err)
		}
		if !strings.EqualFold(roleDoc.Meta.Extends, "base_prompt") {
			return nil, fmt.Errorf("agentprompts: %s: meta.extends must be base_prompt", r)
		}
		c.roleSystems[r] = strings.TrimSpace(roleDoc.System.Content)
		c.roleUsers[r] = strings.TrimSpace(roleDoc.UserMessageTemplate)
	}

	return c, nil
}

// ComposeSystem returns merged system instructions (base + role). Role is the agent role string (e.g. planner).
func (c *Composer) ComposeSystem(role string) (string, error) {
	r := strings.ToLower(strings.TrimSpace(role))
	spec, ok := c.roleSystems[r]
	if !ok {
		return "", fmt.Errorf("agentprompts: unknown role %q", r)
	}
	var b strings.Builder
	b.WriteString(c.baseSystem)
	b.WriteString("\n\n---\n\n")
	b.WriteString(spec)
	return b.String(), nil
}

// UserTemplate returns the role user_message_template (may be empty).
func (c *Composer) UserTemplate(role string) (string, error) {
	r := strings.ToLower(strings.TrimSpace(role))
	t, ok := c.roleUsers[r]
	if !ok {
		return "", fmt.Errorf("agentprompts: unknown role %q", r)
	}
	return t, nil
}

// ValidateAllYAMLAgainstSchema checks six pipeline YAML files (for tests and CI).
func ValidateAllYAMLAgainstSchema(dir string) error {
	schemaPath := filepath.Join(dir, "prompt_schema.json")
	s, err := schema.Compile(schemaPath)
	if err != nil {
		return err
	}
	files := []string{
		"base_prompt.yaml",
		"orchestrator.yaml",
		"planner.yaml",
		"developer.yaml",
		"reviewer.yaml",
		"tester.yaml",
	}
	for _, name := range files {
		if err := s.Validate(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

type promptFile struct {
	Meta struct {
		SchemaVersion string `yaml:"schema_version"`
		Kind          string `yaml:"kind"`
		Role          string `yaml:"role"`
		Version       string `yaml:"version"`
		Extends       string `yaml:"extends"`
	} `yaml:"meta"`
	System struct {
		Content string `yaml:"content"`
	} `yaml:"system"`
	UserMessageTemplate string `yaml:"user_message_template"`
}
