package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Дефолты секции sandbox совпадают с internal/sandbox.DefaultResourceLimitPolicy,
// DefaultSandboxNanoCPUs и DefaultSandboxTimeout — не расходовать без причины.
const (
	defaultSandboxMemoryFloorBytes int64 = 1 << 30 // 1 GiB
	defaultSandboxMemoryCeilBytes  int64 = 16 << 30
	defaultSandboxPidsFloor        int64 = 100
	defaultSandboxPidsCeil         int64 = 8192
	defaultSandboxNanoCPUs         int64 = 1_000_000_000
	defaultSandboxNanoCPUsCeil     int64 = 16_000_000_000
	defaultSandboxMaxConcurrent    int   = 5
)

// Верхние sanity-пределы (один процесс API / защита от DoS через env).
const (
	sandboxSanityMaxMemoryBytes int64 = 128 << 30 // 128 GiB
	sandboxSanityMaxNanoCPUs    int64 = 256 * 1_000_000_000
	sandboxSanityMaxPids        int64 = 1 << 22 // 4194304
	sandboxSanityMaxConcurrent  int   = 4096
)

const minSandboxDefaultNanoCPUs int64 = 1_000_000_000

// SandboxConfig — операционные параметры sandbox (env SANDBOX_*), задача 5.10.
type SandboxConfig struct {
	// MemoryFloorBytes — пол RAM в байтах (SANDBOX_MEMORY_FLOOR_BYTES). Пустой env или 0 → дефолт 1 GiB.
	MemoryFloorBytes int64
	// MemoryCeilBytes — потолок RAM (SANDBOX_MEMORY_CEIL_BYTES). Пустой env или 0 → 16 GiB; должен быть ≥ MemoryFloorBytes.
	MemoryCeilBytes int64
	// PidsFloor / PidsCeil — диапазон pids (SANDBOX_PIDS_FLOOR / SANDBOX_PIDS_CEIL). 0 в env → дефолты 100 / 8192.
	PidsFloor int64
	PidsCeil  int64
	// DefaultNanoCPUs — CPU по умолчанию при NanoCPUs ≤ 0 в опциях (SANDBOX_DEFAULT_NANO_CPUS). Не ниже 1e9.
	DefaultNanoCPUs int64
	// NanoCPUsCeil — верхний предел NanoCPUs (SANDBOX_NANO_CPUS_CEIL). Должен быть ≥ DefaultNanoCPUs.
	NanoCPUsCeil int64
	// DefaultTaskTimeout — таймаут задачи при SandboxOptions.Timeout ≤ 0 (SANDBOX_DEFAULT_TIMEOUT, time.ParseDuration). Строго > 0.
	DefaultTaskTimeout time.Duration
	// MaxConcurrent — макс. параллельных sandbox на процесс (SANDBOX_MAX_CONCURRENT), резерв под очередь; ≥ 1.
	MaxConcurrent int
}

func loadSandboxConfig() (SandboxConfig, error) {
	var err error
	out := SandboxConfig{}

	out.MemoryFloorBytes, err = parseSandboxInt64("SANDBOX_MEMORY_FLOOR_BYTES", "MemoryFloorBytes", 0, defaultSandboxMemoryFloorBytes, sandboxSanityMaxMemoryBytes, true)
	if err != nil {
		return SandboxConfig{}, err
	}
	out.MemoryCeilBytes, err = parseSandboxInt64("SANDBOX_MEMORY_CEIL_BYTES", "MemoryCeilBytes", 0, defaultSandboxMemoryCeilBytes, sandboxSanityMaxMemoryBytes, true)
	if err != nil {
		return SandboxConfig{}, err
	}
	if out.MemoryCeilBytes < out.MemoryFloorBytes {
		return SandboxConfig{}, fmt.Errorf("MemoryCeilBytes must be >= MemoryFloorBytes (check %s and %s)", "SANDBOX_MEMORY_CEIL_BYTES", "SANDBOX_MEMORY_FLOOR_BYTES")
	}

	out.PidsFloor, err = parseSandboxInt64("SANDBOX_PIDS_FLOOR", "PidsFloor", 0, defaultSandboxPidsFloor, sandboxSanityMaxPids, true)
	if err != nil {
		return SandboxConfig{}, err
	}
	out.PidsCeil, err = parseSandboxInt64("SANDBOX_PIDS_CEIL", "PidsCeil", 0, defaultSandboxPidsCeil, sandboxSanityMaxPids, true)
	if err != nil {
		return SandboxConfig{}, err
	}
	if out.PidsCeil < out.PidsFloor {
		return SandboxConfig{}, fmt.Errorf("PidsCeil must be >= PidsFloor (check %s and %s)", "SANDBOX_PIDS_CEIL", "SANDBOX_PIDS_FLOOR")
	}

	out.DefaultNanoCPUs, err = parseSandboxInt64("SANDBOX_DEFAULT_NANO_CPUS", "DefaultNanoCPUs", minSandboxDefaultNanoCPUs, defaultSandboxNanoCPUs, sandboxSanityMaxNanoCPUs, true)
	if err != nil {
		return SandboxConfig{}, err
	}
	out.NanoCPUsCeil, err = parseSandboxInt64("SANDBOX_NANO_CPUS_CEIL", "NanoCPUsCeil", minSandboxDefaultNanoCPUs, defaultSandboxNanoCPUsCeil, sandboxSanityMaxNanoCPUs, true)
	if err != nil {
		return SandboxConfig{}, err
	}
	if out.NanoCPUsCeil < out.DefaultNanoCPUs {
		return SandboxConfig{}, fmt.Errorf("NanoCPUsCeil must be >= DefaultNanoCPUs (check %s and %s)", "SANDBOX_NANO_CPUS_CEIL", "SANDBOX_DEFAULT_NANO_CPUS")
	}

	out.DefaultTaskTimeout, err = parseSandboxDefaultTimeout()
	if err != nil {
		return SandboxConfig{}, err
	}

	out.MaxConcurrent, err = parseSandboxMaxConcurrent()
	if err != nil {
		return SandboxConfig{}, err
	}

	return out, nil
}

// parseSandboxInt64 читает int64 из env: пусто → defaultVal; при zeroMeansDefault значение 0 → defaultVal.
// Иначе v должно удовлетворять minVal <= v <= maxVal.
func parseSandboxInt64(envKey, fieldName string, minVal, defaultVal, maxVal int64, zeroMeansDefault bool) (int64, error) {
	raw := os.Getenv(envKey)
	if raw == "" {
		return defaultVal, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: parse int64: %w", envKey, err)
	}
	if zeroMeansDefault && v == 0 {
		return defaultVal, nil
	}
	if v < minVal {
		return 0, fmt.Errorf("%s (%s): must be at least %d", fieldName, envKey, minVal)
	}
	if v > maxVal {
		return 0, fmt.Errorf("%s (%s): exceeds maximum allowed value for this deployment", fieldName, envKey)
	}
	return v, nil
}

func parseSandboxDefaultTimeout() (time.Duration, error) {
	const envKey = "SANDBOX_DEFAULT_TIMEOUT"
	const defaultDur = 30 * time.Minute
	raw := os.Getenv(envKey)
	if raw == "" {
		return defaultDur, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: parse duration: %w", envKey, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: duration must be positive, got %v", envKey, d)
	}
	const maxDur = 7 * 24 * time.Hour
	if d > maxDur {
		return 0, fmt.Errorf("%s: duration exceeds maximum (%v)", envKey, maxDur)
	}
	return d, nil
}

func parseSandboxMaxConcurrent() (int, error) {
	const envKey = "SANDBOX_MAX_CONCURRENT"
	raw := os.Getenv(envKey)
	if raw == "" {
		return defaultSandboxMaxConcurrent, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: parse int: %w", envKey, err)
	}
	if v < 1 {
		return 0, fmt.Errorf("%s: must be >= 1, got %d", envKey, v)
	}
	if v > int64(sandboxSanityMaxConcurrent) {
		return 0, fmt.Errorf("%s: exceeds maximum allowed value (%d)", envKey, sandboxSanityMaxConcurrent)
	}
	return int(v), nil
}
