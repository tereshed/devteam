// Package zhipu — Zhipu AI (GLM), OpenAI-совместимый.
package zhipu

import (
	"github.com/devteam/backend/pkg/llm"
	"github.com/devteam/backend/pkg/llm/providers/oaicompat"
)

const (
	DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	DefaultModel   = "glm-4-plus"
)

// NewClient создаёт OpenAI-совместимый клиент для Zhipu AI.
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
