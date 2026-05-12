package agentsloader

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devteam/backend/pkg/schema"
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
	// SandboxPermissions — Sprint 15.25: рекомендуемые permissions Claude Code CLI для роли.
	SandboxPermissions *SandboxPermissions `yaml:"sandbox_permissions,omitempty"`
}

// SandboxPermissions — структура совпадает с settings.json.permissions Claude Code CLI.
type SandboxPermissions struct {
	Allow       []string `yaml:"allow,omitempty"`
	Deny        []string `yaml:"deny,omitempty"`
	Ask         []string `yaml:"ask,omitempty"`
	DefaultMode string   `yaml:"defaultMode,omitempty"`
}

// ModelConfig holds LLM parameters for an agent.
type ModelConfig struct {
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
	TopP        float64 `yaml:"top_p,omitempty"`
}

// agentRawYAML is used to unmarshal YAML before schema validation.
type agentRawYAML struct {
	Name        string              `yaml:"name"`
	Role        string              `yaml:"role"`
	PromptName  string              `yaml:"prompt_name"`
	ModelConfig ModelConfig         `yaml:"model_config"`
	IsActive    bool                `yaml:"is_active"`
	Limits      interface{}         `yaml:"limits"`
	SandboxPermissions *SandboxPermissions `yaml:"sandbox_permissions"`
}

var promptNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// pipelineRoleKeys — роли пайплайна 6.9; для них индекс по role (один конфиг на роль).
var pipelineRoleKeys = map[string]struct{}{
	"orchestrator": {},
	"planner":      {},
	"developer":    {},
	"reviewer":     {},
	"tester":       {},
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

func validateTopPBounds(topP float64) error {
	if topP < 0.0 || topP > 1.0 {
		return fmt.Errorf("top_p must be between 0.0 and 1.0, got: %f", topP)
	}
	return nil
}

// promptYAMLPath maps prompt_name (e.g. orchestrator_prompt) to backend/prompts/orchestrator.yaml.
func promptYAMLPath(promptsDir, promptName string) (string, error) {
	stem := strings.TrimSuffix(promptName, "_prompt")
	if stem == "" {
		return "", fmt.Errorf("empty stem from prompt_name %q", promptName)
	}
	return filepath.Join(promptsDir, stem+".yaml"), nil
}

func validatePromptFile(promptsDir, promptName string) error {
	p, err := promptYAMLPath(promptsDir, promptName)
	if err != nil {
		return err
	}
	absBase, err := filepath.Abs(promptsDir)
	if err != nil {
		return fmt.Errorf("prompts dir abs: %w", err)
	}
	absP, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("prompt file abs: %w", err)
	}
	rel, err := filepath.Rel(absBase, absP)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("resolved prompt path escapes prompts directory")
	}
	st, err := os.Stat(absP)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("prompt file for prompt_name %q not found (expected %s)", promptName, absP)
		}
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("prompt path is a directory: %s", absP)
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

	if err := validatePromptName(raw.PromptName); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	if err := validateTemperatureBounds(raw.ModelConfig.Temperature); err != nil {
		return nil, fmt.Errorf("%s: model_config.temperature: %w", filepath.Base(path), err)
	}

	if raw.ModelConfig.TopP != 0 {
		if err := validateTopPBounds(raw.ModelConfig.TopP); err != nil {
			return nil, fmt.Errorf("%s: model_config.top_p: %w", filepath.Base(path), err)
		}
	}

	var limits map[string]interface{}
	if raw.Limits != nil {
		limits, _ = raw.Limits.(map[string]interface{})
	}

	return &AgentConfig{
		Name:               raw.Name,
		Role:               raw.Role,
		PromptName:         raw.PromptName,
		ModelConfig:        raw.ModelConfig,
		IsActive:           raw.IsActive,
		Limits:             limits,
		SandboxPermissions: raw.SandboxPermissions,
	}, nil
}

// Cache holds all validated agent configs in memory.
type Cache struct {
	schema           *schema.Schema
	agents           map[string]*AgentConfig
	prompts          map[string]*AgentConfig
	byPipelineRole   map[string]*AgentConfig
}

// NewCache loads and validates all agent YAML configs from agentsDir.
// If promptsDir is non-empty, each prompt_name must resolve to an existing file under promptsDir
// (orchestrator_prompt → promptsDir/orchestrator.yaml).
func NewCache(agentsDir, promptsDir string) (*Cache, error) {
	schemaPath := filepath.Join(agentsDir, "agent_schema.json")
	s, err := schema.Compile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("agentsloader: load schema: %w", err)
	}

	files, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, fmt.Errorf("agentsloader: read dir: %w", err)
	}

	cache := &Cache{
		schema:         s,
		agents:         make(map[string]*AgentConfig),
		prompts:        make(map[string]*AgentConfig),
		byPipelineRole: make(map[string]*AgentConfig),
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		ext := filepath.Ext(file.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		path := filepath.Join(agentsDir, file.Name())
		if err := s.Validate(path); err != nil {
			return nil, fmt.Errorf("agentsloader: %s: %w", file.Name(), err)
		}
		cfg, err := parseAgentConfig(path)
		if err != nil {
			return nil, fmt.Errorf("agentsloader: %s: %w", file.Name(), err)
		}
		if promptsDir != "" {
			if err := validatePromptFile(promptsDir, cfg.PromptName); err != nil {
				return nil, fmt.Errorf("agentsloader: %s: %w", file.Name(), err)
			}
		}
		if _, dup := cache.agents[cfg.Name]; dup {
			return nil, fmt.Errorf("agentsloader: duplicate agent name %q", cfg.Name)
		}
		cache.agents[cfg.Name] = cfg
		cache.prompts[cfg.PromptName] = cfg
		if _, isPipeline := pipelineRoleKeys[cfg.Role]; isPipeline {
			if _, exists := cache.byPipelineRole[cfg.Role]; exists {
				return nil, fmt.Errorf("agentsloader: duplicate pipeline role %q", cfg.Role)
			}
			cache.byPipelineRole[cfg.Role] = cfg
		}
	}

	return cache, nil
}

// GetByName returns agent config by name.
func (c *Cache) GetByName(name string) (*AgentConfig, bool) {
	cfg, ok := c.agents[name]
	return cfg, ok
}

// GetByPromptName returns agent config by prompt_name.
func (c *Cache) GetByPromptName(promptName string) (*AgentConfig, bool) {
	cfg, ok := c.prompts[promptName]
	return cfg, ok
}

// GetByPipelineRole returns config for orchestrator|planner|developer|reviewer|tester.
func (c *Cache) GetByPipelineRole(role string) (*AgentConfig, bool) {
	cfg, ok := c.byPipelineRole[role]
	return cfg, ok
}

// ValidateRequiredAgents checks that all required pipeline agents are active.
func (c *Cache) ValidateRequiredAgents() error {
	required := []string{"orchestrator", "planner", "developer", "reviewer", "tester"}
	for _, name := range required {
		cfg, ok := c.agents[name]
		if !ok {
			return fmt.Errorf("agentsloader: required agent %q not found in config", name)
		}
		if !cfg.IsActive {
			return fmt.Errorf("agentsloader: required agent %q is_active=false", name)
		}
	}
	return nil
}
