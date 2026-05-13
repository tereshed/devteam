package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"gopkg.in/yaml.v3"
)

// FreeClaudeProxyConfig — конфиг прокси free-claude-proxy, формируемый из таблицы llm_providers.
// Структура соответствует deployment/free-claude-proxy/config.example.yaml (Sprint 15.16).
type FreeClaudeProxyConfig struct {
	Port         int                              `yaml:"port"`
	ServiceToken string                           `yaml:"service_token"`
	Providers    []FreeClaudeProxyConfigProvider  `yaml:"providers"`
	Routes       map[string]string                `yaml:"routes"`
}

// FreeClaudeProxyConfigProvider — апстрим в конфиге прокси.
type FreeClaudeProxyConfigProvider struct {
	Name         string `yaml:"name"`
	Kind         string `yaml:"kind"`
	BaseURL      string `yaml:"base_url"`
	APIKey       string `yaml:"api_key,omitempty"`
	DefaultModel string `yaml:"default_model,omitempty"`
}

// defaultProxyRouteModels — Sprint 15.m1: «anthropic-style» имена моделей, на которые
// бэкенд по умолчанию маршрутизирует Claude Code → первый по алфавиту upstream-провайдер.
// Список расширяется через WithDefaultRouteModels (например, при появлении Sonnet/Haiku новой версии).
var defaultProxyRouteModels = []string{
	"claude-3-5-sonnet-20240620",
	"claude-3-haiku-20240307",
}

// FreeClaudeProxyConfigBuilder — собирает конфиг прокси из БД и записывает на диск (Sprint 15.17).
type FreeClaudeProxyConfigBuilder struct {
	providers    repository.LLMProviderRepository
	secrets      ClaudeCodeProxySecrets
	port         int
	serviceToken string
	routeModels  []string
}

// WithDefaultRouteModels позволяет переопределить список «anthropic-style» имён моделей
// для дефолтного routing'а (Sprint 15.m1).
func (b *FreeClaudeProxyConfigBuilder) WithDefaultRouteModels(models []string) *FreeClaudeProxyConfigBuilder {
	if len(models) > 0 {
		b.routeModels = append([]string(nil), models...)
	}
	return b
}

// ClaudeCodeProxySecrets — источник дешифрованных API-ключей для апстримов.
// В качестве реализации передаётся LLMProviderService (он же реализует SecretsResolver).
type ClaudeCodeProxySecrets interface {
	ResolveCredentials(ctx context.Context, provider *models.LLMProvider) (string, error)
}

// NewFreeClaudeProxyConfigBuilder собирает builder.
// serviceToken — Bearer, который прокси будет требовать в Authorization-заголовке от sandbox-агентов.
func NewFreeClaudeProxyConfigBuilder(
	providers repository.LLMProviderRepository,
	secrets ClaudeCodeProxySecrets,
	port int,
	serviceToken string,
) *FreeClaudeProxyConfigBuilder {
	if port <= 0 {
		port = 8787
	}
	return &FreeClaudeProxyConfigBuilder{
		providers:    providers,
		secrets:      secrets,
		port:         port,
		serviceToken: serviceToken,
		routeModels:  append([]string(nil), defaultProxyRouteModels...),
	}
}

// Build собирает конфиг из включённых провайдеров (kind != anthropic*/free_claude_proxy).
// Routes пробрасывают любой "anthropic-style" model name на первый совместимый провайдер.
func (b *FreeClaudeProxyConfigBuilder) Build(ctx context.Context) (*FreeClaudeProxyConfig, error) {
	all, err := b.providers.List(ctx, true /*onlyEnabled*/)
	if err != nil {
		return nil, fmt.Errorf("list llm providers: %w", err)
	}
	cfg := &FreeClaudeProxyConfig{
		Port:         b.port,
		ServiceToken: b.serviceToken,
		Routes:       map[string]string{},
	}
	for i := range all {
		p := &all[i]
		// Прокси сам экспонирует Anthropic-совместимый API, так что Anthropic-провайдеры
		// здесь не нужны, и сам прокси не должен ссылаться сам на себя.
		switch p.Kind {
		case models.LLMProviderKindAnthropic,
			models.LLMProviderKindAnthropicOAuth,
			models.LLMProviderKindFreeClaudeProxy:
			continue
		}
		apiKey, err := b.secrets.ResolveCredentials(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("resolve credentials for %s: %w", p.Name, err)
		}
		cfg.Providers = append(cfg.Providers, FreeClaudeProxyConfigProvider{
			Name:         p.Name,
			Kind:         string(p.Kind),
			BaseURL:      p.BaseURL,
			APIKey:       apiKey,
			DefaultModel: p.DefaultModel,
		})
	}
	sort.Slice(cfg.Providers, func(i, j int) bool {
		return cfg.Providers[i].Name < cfg.Providers[j].Name
	})

	// Sprint 15.m1: routing-таблица берётся из b.routeModels (с дефолтом defaultProxyRouteModels).
	// Первый по алфавиту upstream — единый таргет для всех известных «anthropic-style» имён.
	if len(cfg.Providers) > 0 {
		first := cfg.Providers[0].Name
		for _, m := range b.routeModels {
			cfg.Routes[m] = first
		}
	}
	return cfg, nil
}

// Marshal сериализует конфиг в YAML.
func (c *FreeClaudeProxyConfig) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

// WriteFile собирает конфиг и атомарно записывает его по пути targetPath.
// Используется при изменении llm_providers (вызывается из LLMProviderService после CRUD).
func (b *FreeClaudeProxyConfigBuilder) WriteFile(ctx context.Context, targetPath string) error {
	if strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("free-claude-proxy config: target path is empty")
	}
	cfg, err := b.Build(ctx)
	if err != nil {
		return err
	}
	payload, err := cfg.Marshal()
	if err != nil {
		return fmt.Errorf("marshal proxy config: %w", err)
	}
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".free-claude-proxy-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, targetPath); err != nil {
		return fmt.Errorf("rename %s: %w", targetPath, err)
	}
	return nil
}
