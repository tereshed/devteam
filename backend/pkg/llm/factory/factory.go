package factory

import (
	"fmt"

	"github.com/wibe-flutter-gin-template/backend/pkg/llm"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm/providers/anthropic"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm/providers/deepseek"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm/providers/gemini"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm/providers/openai"
	"github.com/wibe-flutter-gin-template/backend/pkg/llm/providers/qwen"
)

// Factory creates LLM providers
type Factory struct {
	providers map[llm.ProviderType]func(llm.Config) (llm.Provider, error)
}

// New creates a new Factory
func New() *Factory {
	f := &Factory{
		providers: make(map[llm.ProviderType]func(llm.Config) (llm.Provider, error)),
	}

	f.RegisterProvider(llm.ProviderOpenAI, func(c llm.Config) (llm.Provider, error) {
		return openai.NewClient(c)
	})
	f.RegisterProvider(llm.ProviderAnthropic, func(c llm.Config) (llm.Provider, error) {
		return anthropic.NewClient(c)
	})
	f.RegisterProvider(llm.ProviderGemini, func(c llm.Config) (llm.Provider, error) {
		return gemini.NewClient(c)
	})
	f.RegisterProvider(llm.ProviderDeepseek, func(c llm.Config) (llm.Provider, error) {
		return deepseek.NewClient(c)
	})
	f.RegisterProvider(llm.ProviderQwen, func(c llm.Config) (llm.Provider, error) {
		return qwen.NewClient(c)
	})

	return f
}

// RegisterProvider registers a provider factory function
func (f *Factory) RegisterProvider(pType llm.ProviderType, factory func(llm.Config) (llm.Provider, error)) {
	f.providers[pType] = factory
}

// CreateProvider creates a provider instance
func (f *Factory) CreateProvider(pType llm.ProviderType, config llm.Config) (llm.Provider, error) {
	factory, ok := f.providers[pType]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", pType)
	}
	return factory(config)
}
