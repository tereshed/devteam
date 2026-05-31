package sandbox

// DefaultSandboxNanoCPUs — дефолтный лимит CPU для sandbox (1 полное ядро в наносекундах Docker).
// При ResourceLimit.NanoCPUs <= 0 в HostConfig всегда подставляется не меньше этого значения (задача 5.9).
const DefaultSandboxNanoCPUs int64 = 1_000_000_000

// DefaultResourceLimitPolicy — встроенные полы/потолки (fallback при пустом WithResourceLimitPolicy / нулевых полях).
// Числовые дефолты должны совпадать с backend/internal/config/sandbox_config.go (SANDBOX_* при пустом env).
func DefaultResourceLimitPolicy() ResourceLimitPolicy {
	return ResourceLimitPolicy{
		// 2 GiB пол: 1 GiB не хватает на компиляцию Go-проектов с графом gin (OOM при
		// `go build ./...`). ДОЛЖНО совпадать с config.defaultSandboxMemoryFloorBytes.
		MemoryFloorBytes: 2 << 30, // 2 GiB
		MemoryCeilBytes:  16 << 30,
		PidsFloor:        100,
		PidsCeil:         8192,
		DefaultNanoCPUs:  DefaultSandboxNanoCPUs,
		NanoCPUsCeil:     16_000_000_000,
	}
}

func normalizeResourceLimitPolicy(p ResourceLimitPolicy) ResourceLimitPolicy {
	d := DefaultResourceLimitPolicy()
	out := p
	if out.MemoryFloorBytes <= 0 {
		out.MemoryFloorBytes = d.MemoryFloorBytes
	}
	if out.MemoryCeilBytes <= 0 {
		out.MemoryCeilBytes = d.MemoryCeilBytes
	}
	if out.MemoryCeilBytes < out.MemoryFloorBytes {
		out.MemoryFloorBytes = d.MemoryFloorBytes
		out.MemoryCeilBytes = d.MemoryCeilBytes
	}
	if out.PidsFloor <= 0 {
		out.PidsFloor = d.PidsFloor
	}
	if out.PidsCeil <= 0 {
		out.PidsCeil = d.PidsCeil
	}
	if out.PidsCeil < out.PidsFloor {
		out.PidsFloor = d.PidsFloor
		out.PidsCeil = d.PidsCeil
	}
	if out.DefaultNanoCPUs < DefaultSandboxNanoCPUs {
		out.DefaultNanoCPUs = DefaultSandboxNanoCPUs
	}
	if out.NanoCPUsCeil <= 0 {
		out.NanoCPUsCeil = d.NanoCPUsCeil
	}
	if out.NanoCPUsCeil < out.DefaultNanoCPUs {
		out.NanoCPUsCeil = out.DefaultNanoCPUs
	}
	return out
}
