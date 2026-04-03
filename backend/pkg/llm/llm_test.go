package llm_test

import (
	"testing"

	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/factory"
)

func TestFactory(t *testing.T) {
	f := factory.New()

	tests := []struct {
		name    string
		pType   llm.ProviderType
		config  llm.Config
		wantErr bool
	}{
		{
			name:  "OpenAI",
			pType: llm.ProviderOpenAI,
			config: llm.Config{
				APIKey:  "test-key",
				BaseURL: "https://api.openai.com/v1",
			},
			wantErr: false,
		},
		{
			name:  "Anthropic",
			pType: llm.ProviderAnthropic,
			config: llm.Config{
				APIKey:  "test-key",
				BaseURL: "https://api.anthropic.com",
			},
			wantErr: false,
		},
		{
			name:  "Gemini",
			pType: llm.ProviderGemini,
			config: llm.Config{
				APIKey:  "test-key",
				BaseURL: "https://generativelanguage.googleapis.com",
			},
			wantErr: false,
		},
		{
			name:  "Deepseek",
			pType: llm.ProviderDeepseek,
			config: llm.Config{
				APIKey:  "test-key",
				BaseURL: "https://api.deepseek.com",
			},
			wantErr: false,
		},
		{
			name:  "Qwen",
			pType: llm.ProviderQwen,
			config: llm.Config{
				APIKey:  "test-key",
				BaseURL: "https://dashscope.aliyuncs.com",
			},
			wantErr: false,
		},
		{
			name:    "Unknown",
			pType:   "unknown",
			config:  llm.Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := f.CreateProvider(tt.pType, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("CreateProvider() returned nil provider")
			}
		})
	}
}
