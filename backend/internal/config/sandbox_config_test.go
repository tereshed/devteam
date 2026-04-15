package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearSandboxEnv(t *testing.T) {
	t.Helper()
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
		t.Setenv(k, "")
	}
}

func TestLoad_SandboxDefaults_EmptyEnv(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int64(1<<30), cfg.Sandbox.MemoryFloorBytes)
	assert.Equal(t, int64(16<<30), cfg.Sandbox.MemoryCeilBytes)
	assert.Equal(t, int64(100), cfg.Sandbox.PidsFloor)
	assert.Equal(t, int64(8192), cfg.Sandbox.PidsCeil)
	assert.Equal(t, int64(1_000_000_000), cfg.Sandbox.DefaultNanoCPUs)
	assert.Equal(t, int64(16_000_000_000), cfg.Sandbox.NanoCPUsCeil)
	assert.Equal(t, 30*time.Minute, cfg.Sandbox.DefaultTaskTimeout)
	assert.Equal(t, 5, cfg.Sandbox.MaxConcurrent)
}

func TestLoad_SandboxParsesEnv(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_MEMORY_FLOOR_BYTES", "2147483648") // 2 GiB
	t.Setenv("SANDBOX_MEMORY_CEIL_BYTES", "3221225472") // 3 GiB
	t.Setenv("SANDBOX_PIDS_FLOOR", "256")
	t.Setenv("SANDBOX_PIDS_CEIL", "4096")
	t.Setenv("SANDBOX_DEFAULT_NANO_CPUS", "2000000000")
	t.Setenv("SANDBOX_NANO_CPUS_CEIL", "8000000000")
	t.Setenv("SANDBOX_DEFAULT_TIMEOUT", "45m")
	t.Setenv("SANDBOX_MAX_CONCURRENT", "12")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int64(2<<30), cfg.Sandbox.MemoryFloorBytes)
	assert.Equal(t, int64(3<<30), cfg.Sandbox.MemoryCeilBytes)
	assert.Equal(t, int64(256), cfg.Sandbox.PidsFloor)
	assert.Equal(t, int64(4096), cfg.Sandbox.PidsCeil)
	assert.Equal(t, int64(2_000_000_000), cfg.Sandbox.DefaultNanoCPUs)
	assert.Equal(t, int64(8_000_000_000), cfg.Sandbox.NanoCPUsCeil)
	assert.Equal(t, 45*time.Minute, cfg.Sandbox.DefaultTaskTimeout)
	assert.Equal(t, 12, cfg.Sandbox.MaxConcurrent)
}

func TestLoad_SandboxMemoryCeilBelowFloor(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_MEMORY_FLOOR_BYTES", "10737418240") // 10 GiB
	t.Setenv("SANDBOX_MEMORY_CEIL_BYTES", "1073741824")   // 1 GiB

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
	assert.True(t, strings.Contains(err.Error(), "MemoryCeilBytes") || strings.Contains(err.Error(), "SANDBOX_MEMORY"), err.Error())
}

func TestLoad_SandboxPidsCeilBelowFloor(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_PIDS_FLOOR", "5000")
	t.Setenv("SANDBOX_PIDS_CEIL", "100")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxNanoCeilBelowDefault(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_DEFAULT_NANO_CPUS", "4000000000")
	t.Setenv("SANDBOX_NANO_CPUS_CEIL", "2000000000")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxDefaultTimeoutInvalidDuration(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_DEFAULT_TIMEOUT", "not-a-duration")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
	assert.True(t, strings.Contains(err.Error(), "SANDBOX_DEFAULT_TIMEOUT"), err.Error())
}

func TestLoad_SandboxDefaultTimeoutZero(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_DEFAULT_TIMEOUT", "0s")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxDefaultTimeoutNegative(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_DEFAULT_TIMEOUT", "-1h")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxMaxConcurrentBelowOne(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_MAX_CONCURRENT", "0")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
	assert.True(t, strings.Contains(err.Error(), "SANDBOX_MAX_CONCURRENT"), err.Error())
}

func TestLoad_SandboxMemoryAboveSanity(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_MEMORY_CEIL_BYTES", "200000000000000") // > 128 GiB sanity

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxMaxConcurrentAboveSanity(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_MAX_CONCURRENT", "10000")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxParseIntInvalid(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_MEMORY_FLOOR_BYTES", "12abc")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxParseIntOverflowStyle(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_DEFAULT_NANO_CPUS", "9999999999999999999999999999999")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxZeroMeansDefaultForMemory(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_MEMORY_FLOOR_BYTES", "0")
	t.Setenv("SANDBOX_MEMORY_CEIL_BYTES", "0")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, int64(1<<30), cfg.Sandbox.MemoryFloorBytes)
	assert.Equal(t, int64(16<<30), cfg.Sandbox.MemoryCeilBytes)
}

func TestLoad_SandboxDefaultNanoBelowMinimum(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_DEFAULT_NANO_CPUS", "500000000")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}

func TestLoad_SandboxDefaultTimeoutExceedsMax(t *testing.T) {
	clearSandboxEnv(t)
	t.Setenv("ENV", "development")
	t.Setenv("SANDBOX_DEFAULT_TIMEOUT", "8000h")

	_, err := Load()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid sandbox config"), err.Error())
}
