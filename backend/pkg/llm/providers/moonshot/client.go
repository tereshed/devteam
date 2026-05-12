// Package moonshot — Moonshot AI (Kimi), OpenAI-совместимый.
package moonshot

import (
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/providers/oaicompat"
)

const (
	DefaultBaseURL = "https://api.moonshot.cn/v1"
	DefaultModel   = "moonshot-v1-8k"
)

// NewClient создаёт OpenAI-совместимый клиент для Moonshot.
func NewClient(c llm.Config) (*oaicompat.Client, error) {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return oaicompat.NewClient(oaicompat.Config{
		APIKey:       c.APIKey,
		BaseURL:      baseURL,
		DefaultModel: DefaultModel,
	})
}
