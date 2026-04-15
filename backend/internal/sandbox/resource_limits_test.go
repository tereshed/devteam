package sandbox

import (
	"math"
	"testing"
)

func Test_effectiveMemoryBytes(t *testing.T) {
	const floor = 1 << 30
	const ceil = 16 << 30
	if g := effectiveMemoryBytes(0, floor, ceil); g != floor {
		t.Fatalf("0 -> floor: got %d", g)
	}
	if g := effectiveMemoryBytes(-1, floor, ceil); g != floor {
		t.Fatalf("negative -> floor: got %d", g)
	}
	if g := effectiveMemoryBytes(512, floor, ceil); g != floor {
		t.Fatalf("512 MB below floor -> floor: got %d", g)
	}
	if g := effectiveMemoryBytes(2048, floor, ceil); g != 2048*1024*1024 {
		t.Fatalf("2048 MB: got %d", g)
	}
	if g := effectiveMemoryBytes(20000, floor, ceil); g != ceil {
		t.Fatalf("above ceil -> ceil: got %d", g)
	}
}

func Test_effectivePidsLimit(t *testing.T) {
	if g := effectivePidsLimit(0, 100, 8192); g != 100 {
		t.Fatalf("0 -> floor: %d", g)
	}
	if g := effectivePidsLimit(50, 100, 8192); g != 100 {
		t.Fatalf("50 -> floor: %d", g)
	}
	if g := effectivePidsLimit(500, 100, 8192); g != 500 {
		t.Fatalf("500: %d", g)
	}
	if g := effectivePidsLimit(10000, 100, 8192); g != 8192 {
		t.Fatalf("10000 -> ceil: %d", g)
	}
}

func Test_effectiveNanoCPUs(t *testing.T) {
	def := int64(1_000_000_000)
	ceil := int64(16_000_000_000)
	if g := effectiveNanoCPUs(0, def, ceil); g != def {
		t.Fatalf("0 -> default: %d", g)
	}
	if g := effectiveNanoCPUs(-1, def, ceil); g != def {
		t.Fatalf("-1 -> default: %d", g)
	}
	if g := effectiveNanoCPUs(2_000_000_000, def, ceil); g != 2_000_000_000 {
		t.Fatalf("2 CPUs: %d", g)
	}
	if g := effectiveNanoCPUs(20_000_000_000, def, ceil); g != ceil {
		t.Fatalf("above ceil -> ceil: %d", g)
	}
}

func Test_maxAllowedMemoryMB(t *testing.T) {
	ceil := int64(16 << 30)
	got := maxAllowedMemoryMB(ceil)
	if got != 16384 {
		t.Fatalf("16 GiB ceil -> 16384 MB cap, got %d", got)
	}
	if maxAllowedMemoryMB(0) != int64(math.MaxInt64/(1024*1024)) {
		t.Fatal("zero ceilBytes should use int64-safe cap")
	}
}

func TestSandboxOptions_ValidateResourceLimits(t *testing.T) {
	pol := DefaultResourceLimitPolicy()
	base := SandboxOptions{
		ResourceLimit: ResourceLimit{},
	}
	if err := base.ValidateResourceLimits(pol); err != nil {
		t.Fatal(err)
	}
	badDisk := base
	badDisk.ResourceLimit = ResourceLimit{DiskMB: 1}
	if err := badDisk.ValidateResourceLimits(pol); err == nil {
		t.Fatal("expected error for disk_mb")
	}
	badNano := base
	badNano.ResourceLimit = ResourceLimit{NanoCPUs: -1}
	if err := badNano.ValidateResourceLimits(pol); err == nil {
		t.Fatal("expected error for negative nano")
	}
	tooBigNano := base
	tooBigNano.ResourceLimit = ResourceLimit{NanoCPUs: pol.NanoCPUsCeil + 1}
	if err := tooBigNano.ValidateResourceLimits(pol); err == nil {
		t.Fatal("expected error for nano above ceil")
	}
	badMem := base
	badMem.ResourceLimit = ResourceLimit{MemoryMB: -1}
	if err := badMem.ValidateResourceLimits(pol); err == nil {
		t.Fatal("expected error for negative memory")
	}
	hugeMem := base
	hugeMem.ResourceLimit = ResourceLimit{MemoryMB: math.MaxInt32}
	if err := hugeMem.ValidateResourceLimits(pol); err == nil {
		t.Fatal("expected error for huge memory_mb")
	}
	badPids := base
	badPids.ResourceLimit = ResourceLimit{PIDsLimit: -1}
	if err := badPids.ValidateResourceLimits(pol); err == nil {
		t.Fatal("expected error for negative pids")
	}
	tooManyPids := base
	tooManyPids.ResourceLimit = ResourceLimit{PIDsLimit: int(pol.PidsCeil) + 1}
	if err := tooManyPids.ValidateResourceLimits(pol); err == nil {
		t.Fatal("expected error for pids above ceil")
	}
}

func Test_normalizeResourceLimitPolicy_enforcesMinimumDefaultNano(t *testing.T) {
	p := normalizeResourceLimitPolicy(ResourceLimitPolicy{
		MemoryFloorBytes: 1 << 30,
		MemoryCeilBytes:  16 << 30,
		PidsFloor:        100,
		PidsCeil:         8192,
		DefaultNanoCPUs:  1,
		NanoCPUsCeil:     16_000_000_000,
	})
	if p.DefaultNanoCPUs != DefaultSandboxNanoCPUs {
		t.Fatalf("default nano: %d", p.DefaultNanoCPUs)
	}
}
