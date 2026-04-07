package gitprovider

import (
	"fmt"
	"strings"
)

var _ Factory = (*DefaultFactory)(nil)

// DefaultFactory creates GitProvider instances by provider type string.
type DefaultFactory struct{}

// NewFactory returns a Factory for creating GitProvider instances.
// Inject into services via constructors (DI).
func NewFactory() Factory {
	return &DefaultFactory{}
}

// Create returns a GitProvider for the given providerType and credentials.
// The providerType is normalized (trimmed and lowercased) before matching.
// Supported types: "github", "local". Returns ErrUnknownProvider otherwise.
func (f *DefaultFactory) Create(providerType string, creds Credentials) (GitProvider, error) {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "github":
		return NewGitHubProvider(creds), nil
	case "local":
		return NewLocalGitProvider(creds), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownProvider, providerType)
	}
}
