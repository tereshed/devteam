package vectordb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// NewClient Tests
// ========================================

func TestNewClient_Success(t *testing.T) {
	cfg := &Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := NewClient(cfg)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.weaviate)
	assert.Equal(t, cfg, client.config)
}

func TestNewClient_DefaultScheme(t *testing.T) {
	cfg := &Config{
		Host:   "localhost:8080",
		Scheme: "", // Пустой - должен стать "http"
	}

	client, err := NewClient(cfg)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "http", client.config.Scheme)
}

func TestNewClient_NilConfig(t *testing.T) {
	_, err := NewClient(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestNewClient_EmptyHost(t *testing.T) {
	cfg := &Config{
		Host:   "",
		Scheme: "http",
	}

	_, err := NewClient(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host cannot be empty")
}

func TestClient_GetClient(t *testing.T) {
	cfg := &Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	weaviateClient := client.GetClient()

	assert.NotNil(t, weaviateClient)
	assert.Equal(t, client.weaviate, weaviateClient)
}

func TestClient_Close(t *testing.T) {
	cfg := &Config{
		Host:   "localhost:8080",
		Scheme: "http",
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// Close не должен возвращать ошибку
	err = client.Close()

	assert.NoError(t, err)
}

// ========================================
// Collection Management Tests
// ========================================

func TestClient_GetClassName(t *testing.T) {
	cfg := &Config{Host: "localhost:8080"}
	client, _ := NewClient(cfg)

	tests := []struct {
		name      string
		projectID string
		want      string
		wantErr   bool
	}{
		{
			name:      "Valid UUID",
			projectID: "550e8400-e29b-41d4-a716-446655440000",
			want:      "DevTeam_Project_550e8400e29b41d4a716446655440000",
			wantErr:   false,
		},
		{
			name:      "Invalid UUID",
			projectID: "invalid-uuid",
			wantErr:   true,
		},
		{
			name:      "Empty ID",
			projectID: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.GetClassName(tt.projectID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// ========================================
// Config Tests
// ========================================

func TestConfig_DefaultValues(t *testing.T) {
	cfg := &Config{}

	assert.Empty(t, cfg.Host)
	assert.Empty(t, cfg.Scheme)
}

