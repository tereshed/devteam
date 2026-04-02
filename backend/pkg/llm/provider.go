package llm

import "context"

// Provider defines the interface for LLM providers
type Provider interface {
	// Generate generates a response based on the request
	Generate(ctx context.Context, req Request) (*Response, error)
}
