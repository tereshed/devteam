package agentsloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// AgentConfig represents a validated agent configuration (in-memory cache).
type AgentConfig struct {
	Name        string
	Role        string
	PromptName  string
	ModelConfig ModelConfig
	IsActive    bool
	Limits      map[string]interface{}
}

// ModelConfig holds LLM parameters for an agent.
type ModelConfig struct {
	Model       string
	Temperature float64
	MaxTokens   int
	TopP        float64
}

// agentRawYAML is used to unmarshal YAML before schema validation.
type agentRawYAML struct {
	Name        string                 `yaml:"name"`
	Role        string                 `yaml:"role"`
	PromptName  string                 `yaml:"prompt_name"`
	ModelConfig map[string]interface{} `yaml:"model_config"`
	IsActive    bool                   `yaml:"is_active"`
	Limits      map[string]interface{} `yaml:"limits"`
}

var (
	schema     *jsonschema.Schema
	schemaOnce sync.Once

	promptNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

func loadSchema(schemaPath string) error {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	sch, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("parse schema json: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("file://"+schemaPath, sch); err != nil {
		return fmt.Errorf("add schema resource: %w", err)
	}
	schema, err = compiler.Compile("file://" + schemaPath)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	return nil
}

func validatePromptName(name string) error {
	if !promptNameRegex.MatchString(name) {
		return fmt.Errorf("prompt_name must match pattern ^[a-zA-Z0-9_-]+$, got: %q", name)
	}
	return nil
}

func validateTemperatureBounds(temp float64) error {
	if temp < 0.0 || temp > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0, got: %f", temp)
	}
	return nil
}

func validateYAMLFile(yamlPath string) error {
	if schema == nil {
		return fmt.Errorf("schema not loaded")
	}
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
	if err := schema.Validate(jsonDoc); err != nil {
		return fmt.Errorf("jsonschema: %w", err)
	}
	return nil
}

func parseAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw agentRawYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("yaml parse: %w", err)
	}

	// Validate prompt_name for path traversal
	if err := validatePromptName(raw.PromptName); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	// Extract and validate model_config
	model, _ := raw.ModelConfig["model"].(string)
	temp, _ := raw.ModelConfig["temperature"].(float64)
	maxTokens, _ := raw.ModelConfig["max_tokens"].(int)
	topP, _ := raw.ModelConfig["top_p"].(float64)

	if err := validateTemperatureBounds(temp); err != nil {
		return nil, fmt.Errorf("%s: model_config.temperature: %w", filepath.Base(path), err)
	}

	return &AgentConfig{
		Name:       raw.Name,
		Role:       raw.Role,
		PromptName: raw.PromptName,
		ModelConfig: ModelConfig{
			Model:       model,
			Temperature: temp,
			MaxTokens:   maxTokens,
			TopP:        topP,
		},
		IsActive: raw.IsActive,
		Limits:   raw.Limits,
	}, nil
}

// Cache holds all validated agent configs in memory.
type Cache struct {
	mu      sync.RWMutex
	agents  map[string]*AgentConfig
	prompts map[string]*AgentConfig // by prompt_name
}

// NewCache loads and validates all agent YAML configs from dirPath.
func NewCache(dirPath string) (*Cache, error) {
	schemaPath := filepath.Join(dirPath, "agent_schema.json")
	if err := loadSchema(schemaPath); err != nil {
		return nil, fmt.Errorf("agentsloader: load schema: %w", err)
	}

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("agentsloader: read dir: %w", err)
	}

	cache := &Cache{
		agents:  make(map[string]*AgentConfig),
		prompts: make(map[string]*AgentConfig),
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ext := filepath.Ext(file.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(dirPath, file.Name())
		if err := validateYAMLFile(path); err != nil {
			return nil, fmt.Errorf("agentsloader: %s: %w", file.Name(), err)
		}
		cfg, err := parseAgentConfig(path)
		if err != nil {
			return nil, fmt.Errorf("agentsloader: %s: %w", file.Name(), err)
		}
		cache.agents[cfg.Name] = cfg
		cache.prompts[cfg.PromptName] = cfg
	}

	return cache, nil
}

// GetByName returns agent config by name.
func (c *Cache) GetByName(name string) (*AgentConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cfg, ok := c.agents[name]
	return cfg, ok
}

// GetByPromptName returns agent config by prompt_name.
func (c *Cache) GetByPromptName(promptName string) (*AgentConfig, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cfg, ok := c.prompts[promptName]
	return cfg, ok
}

// ValidateRequiredAgents checks that all required pipeline agents are active.
func (c *Cache) ValidateRequiredAgents() error {
	required := []string{"orchestrator", "planner", "developer", "reviewer", "tester"}
	for _, name := range required {
		cfg, ok := c.GetByName(name)
		if !ok {
			return fmt.Errorf("agentsloader: required agent %q not found in config", name)
		}
		if !cfg.IsActive {
			return fmt.Errorf("agentsloader: required agent %q is_active=false", name)
		}
	}
	return nil
}
