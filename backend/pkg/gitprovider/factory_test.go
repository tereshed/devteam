package gitprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFactory_Create(t *testing.T) {
	t.Parallel()
	f := NewFactory()
	tests := []struct {
		name             string
		providerType     string
		wantConcreteType any
		wantErr          bool
	}{
		{"github", "github", &GitHubProvider{}, false},
		{"local", "local", &LocalGitProvider{}, false},
		{"case insensitive upper", "GitHub", &GitHubProvider{}, false},
		{"case insensitive all caps", "GITHUB", &GitHubProvider{}, false},
		{"case insensitive local", "LOCAL", &LocalGitProvider{}, false},
		{"trimmed spaces", "  github  ", &GitHubProvider{}, false},
		{"unknown gitlab", "gitlab", nil, true},
		{"unknown bitbucket", "bitbucket", nil, true},
		{"empty string", "", nil, true},
		{"whitespace only", "   ", nil, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := f.Create(tt.providerType, Credentials{Token: "test-token"})
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrUnknownProvider)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.IsType(t, tt.wantConcreteType, got)
		})
	}
}

func TestNewFactory_ReturnsNonNil(t *testing.T) {
	t.Parallel()
	f := NewFactory()
	assert.NotNil(t, f)
}
