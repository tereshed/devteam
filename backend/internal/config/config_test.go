package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	for _, k := range []string{
		"SANDBOX_MEMORY_FLOOR_BYTES",
		"SANDBOX_MEMORY_CEIL_BYTES",
		"SANDBOX_PIDS_FLOOR",
		"SANDBOX_PIDS_CEIL",
		"SANDBOX_DEFAULT_NANO_CPUS",
		"SANDBOX_NANO_CPUS_CEIL",
		"SANDBOX_DEFAULT_TIMEOUT",
		"SANDBOX_MAX_CONCURRENT",
	} {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}

func TestDecodeEncryptionKeyHex_Valid(t *testing.T) {
	// 32 нулевых байт в hex
	s := "0000000000000000000000000000000000000000000000000000000000000000"
	key, err := DecodeEncryptionKeyHex(s)
	require.NoError(t, err)
	assert.Len(t, key, 32)
	for _, b := range key {
		assert.Equal(t, byte(0), b)
	}
}

func TestDecodeEncryptionKeyHex_TrimSpace(t *testing.T) {
	s := "  " + strings.Repeat("ab", 32) + "  "
	_, err := DecodeEncryptionKeyHex(s)
	require.NoError(t, err)
}

func TestDecodeEncryptionKeyHex_WrongLength(t *testing.T) {
	_, err := DecodeEncryptionKeyHex("abcd")
	require.Error(t, err)
}

func TestDecodeEncryptionKeyHex_InvalidHex(t *testing.T) {
	s := "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg"
	_, err := DecodeEncryptionKeyHex(s)
	require.Error(t, err)
}

func TestLoad_EncryptionKey_Invalid(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("ENCRYPTION_KEY", strings.Repeat("g", 64))
	_, err := Load()
	require.Error(t, err)
}

func TestLoad_EncryptionKey_Valid(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("ENCRYPTION_KEY", "0000000000000000000000000000000000000000000000000000000000000000")
	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.Encryption.Key, 32)
}

func TestLoad_EncryptionKey_EmptyOptional(t *testing.T) {
	t.Setenv("ENV", "development")
	t.Setenv("ENCRYPTION_KEY", "")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Nil(t, cfg.Encryption.Key)
}

func TestLoad_EncryptionKey_RequiredInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("ENCRYPTION_KEY", "")
	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ENCRYPTION_KEY")
}

func TestLoad_EncryptionKey_RequiredWhenProdAliasOrCase(t *testing.T) {
	for _, env := range []string{"prod", "PROD", " PRODUCTION ", "Production"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("ENV", env)
			t.Setenv("ENCRYPTION_KEY", "")
			_, err := Load()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "ENCRYPTION_KEY")
		})
	}
}

func TestLoad_Environment_Normalized(t *testing.T) {
	t.Setenv("ENV", " Staging ")
	t.Setenv("ENCRYPTION_KEY", "")
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "staging", cfg.Environment)
	assert.False(t, cfg.IsProd())
}

func TestLoad_IsProd_WithValidEncryptionKey(t *testing.T) {
	validKey := "0000000000000000000000000000000000000000000000000000000000000000"
	for _, env := range []string{"production", "PRODUCTION", " prod "} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("ENV", env)
			t.Setenv("ENCRYPTION_KEY", validKey)
			t.Setenv("JWT_SECRET_KEY", "not-the-default-production-placeholder")
			cfg, err := Load()
			require.NoError(t, err)
			assert.True(t, cfg.IsProd())
		})
	}
}
