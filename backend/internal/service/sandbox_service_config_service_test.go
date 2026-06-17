package service

import (
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/sandbox"
)

func newTestSandboxServiceCfgSvc() *sandboxServiceConfigService {
	return &sandboxServiceConfigService{allowedImages: sandbox.DefaultAllowedSandboxServiceImages()}
}

func TestApplySandboxServiceRequest_Defaults(t *testing.T) {
	cfg := &models.SandboxServiceConfig{}
	if err := applySandboxServiceRequest(cfg, dto.UpsertSandboxServiceRequest{Alias: "db"}); err != nil {
		t.Fatal(err)
	}
	if cfg.Kind != models.SandboxServiceKindPostgres {
		t.Errorf("kind default = %q", cfg.Kind)
	}
	if cfg.Image != sandboxServiceDefaultImage {
		t.Errorf("image default = %q", cfg.Image)
	}
	if cfg.Port != sandboxServiceDefaultPort {
		t.Errorf("port default = %d", cfg.Port)
	}
	if cfg.SeedKind != models.SandboxSeedNone {
		t.Errorf("seed_kind default = %q", cfg.SeedKind)
	}
	if cfg.ReadyTimeoutSeconds != sandboxServiceDefaultReadyTimeout {
		t.Errorf("ready_timeout default = %d", cfg.ReadyTimeoutSeconds)
	}
}

func TestSandboxServiceValidate(t *testing.T) {
	svc := newTestSandboxServiceCfgSvc()

	t.Run("valid", func(t *testing.T) {
		cfg := &models.SandboxServiceConfig{}
		_ = applySandboxServiceRequest(cfg, dto.UpsertSandboxServiceRequest{
			Alias: "db", DBName: "test_dev_ss", DBUser: "tester",
		})
		if err := svc.validate(cfg); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})

	t.Run("inline seed kept", func(t *testing.T) {
		cfg := &models.SandboxServiceConfig{}
		_ = applySandboxServiceRequest(cfg, dto.UpsertSandboxServiceRequest{
			Alias: "db", SeedKind: "inline", SeedValue: "SELECT 1;",
		})
		if err := svc.validate(cfg); err != nil {
			t.Fatalf("inline seed should be valid: %v", err)
		}
		if cfg.SeedValue != "SELECT 1;" {
			t.Errorf("inline seed dropped: %q", cfg.SeedValue)
		}
	})

	t.Run("none seed value cleared", func(t *testing.T) {
		cfg := &models.SandboxServiceConfig{}
		_ = applySandboxServiceRequest(cfg, dto.UpsertSandboxServiceRequest{
			Alias: "db", SeedKind: "none", SeedValue: "ignored",
		})
		if err := svc.validate(cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.SeedValue != "" {
			t.Errorf("none seed must clear value, got %q", cfg.SeedValue)
		}
	})

	cases := map[string]dto.UpsertSandboxServiceRequest{
		"bad alias":           {Alias: "DB"},
		"bad image":           {Alias: "db", Image: "evil:latest"},
		"repo_file traversal": {Alias: "db", SeedKind: "repo_file", SeedValue: "../etc/passwd"},
		"repo_file absolute":  {Alias: "db", SeedKind: "repo_file", SeedValue: "/etc/passwd"},
		"bad db name":         {Alias: "db", DBName: "bad-name"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &models.SandboxServiceConfig{}
			_ = applySandboxServiceRequest(cfg, req)
			if err := svc.validate(cfg); err == nil {
				t.Fatalf("expected validation error for %q", name)
			}
		})
	}
}
