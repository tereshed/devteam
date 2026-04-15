package sandbox

// ResourceLimitPolicy — полы, потолки и дефолт CPU для маппинга ResourceLimit → Docker cgroup (5.9).
// Значения задаются при создании DockerSandboxRunner (константы по умолчанию или конфиг 5.10).
type ResourceLimitPolicy struct {
	MemoryFloorBytes int64
	MemoryCeilBytes  int64
	PidsFloor        int64
	PidsCeil         int64
	DefaultNanoCPUs  int64
	NanoCPUsCeil     int64
}

// effectiveMemoryBytes возвращает байты RAM для docker.Resources.Memory (чистая функция).
// requestedMB <= 0 — floorBytes; иначе clamp к [floorBytes, ceilBytes]. Вызывающий обязан отклонить
// недопустимый ввод в ValidateResourceLimits до умножения MB→байты (overflow).
func effectiveMemoryBytes(requestedMB int, floorBytes, ceilBytes int64) int64 {
	if requestedMB <= 0 {
		return floorBytes
	}
	mb := int64(requestedMB)
	b := mb * (1024 * 1024)
	if b < floorBytes {
		return floorBytes
	}
	if b > ceilBytes {
		return ceilBytes
	}
	return b
}

// effectivePidsLimit — clamp к [floor, ceil] (чистая функция).
func effectivePidsLimit(requested int, floor, ceil int64) int64 {
	p := int64(requested)
	if p < floor {
		p = floor
	}
	if p > ceil {
		p = ceil
	}
	return p
}

// effectiveNanoCPUs — при requested <= 0 подставляется defaultWhenNonPositive; иначе clamp сверху ceilNano.
func effectiveNanoCPUs(requestedNano, defaultWhenNonPositive, ceilNano int64) int64 {
	if requestedNano <= 0 {
		return defaultWhenNonPositive
	}
	if requestedNano > ceilNano {
		return ceilNano
	}
	return requestedNano
}
