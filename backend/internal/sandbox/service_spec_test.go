package sandbox

import (
	"strings"
	"testing"
)

func validPostgresService() ServiceSpec {
	return ServiceSpec{
		Alias: "db",
		Image: "postgres:16-alpine",
		Port:  5432,
		Env: map[string]string{
			EnvPostgresDB:       "test_dev_ss",
			EnvPostgresUser:     "tester",
			EnvPostgresPassword: "s3cr3t",
		},
	}
}

func TestServiceSpec_validateStructural(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		if err := validPostgresService().validateStructural(); err != nil {
			t.Fatalf("expected valid, got %v", err)
		}
	})

	cases := map[string]func(*ServiceSpec){
		"bad alias upper":     func(s *ServiceSpec) { s.Alias = "DB" },
		"bad alias leading -": func(s *ServiceSpec) { s.Alias = "-db" },
		"empty alias":         func(s *ServiceSpec) { s.Alias = "" },
		"empty image":         func(s *ServiceSpec) { s.Image = "  " },
		"port zero":           func(s *ServiceSpec) { s.Port = 0 },
		"port too big":        func(s *ServiceSpec) { s.Port = 70000 },
		"bad env key":         func(s *ServiceSpec) { s.Env["BAD KEY"] = "x" },
		"control in env val":  func(s *ServiceSpec) { s.Env[EnvPostgresUser] = "a\nb" },
		"seed too large":      func(s *ServiceSpec) { s.SeedSQL = strings.Repeat("x", maxServiceSeedBytes+1) },
		"ready timeout neg":   func(s *ServiceSpec) { s.ReadyTimeoutSecs = -1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := validPostgresService()
			mutate(&s)
			if err := s.validateStructural(); err == nil {
				t.Fatalf("expected error for %q, got nil", name)
			}
		})
	}
}

func TestAppendServiceConnEnv(t *testing.T) {
	s := validPostgresService()
	s.Env[EnvPostgresPassword] = "p@ss:w/ord" // символы, требующие URL-экранирования
	out := appendServiceConnEnv(nil, []ServiceSpec{s})

	want := map[string]string{
		EnvPostgresHost:     "db",
		EnvPostgresPort:     "5432",
		EnvPostgresDB:       "test_dev_ss",
		EnvPostgresUser:     "tester",
		EnvPostgresPassword: "p@ss:w/ord",
	}
	got := envSliceToMap(out)
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	url := got[EnvDatabaseURL]
	if !strings.HasPrefix(url, "postgresql://tester:") || !strings.Contains(url, "@db:5432/test_dev_ss") {
		t.Errorf("DATABASE_URL malformed: %q", url)
	}
	if strings.Contains(url, "p@ss:w/ord") {
		t.Errorf("DATABASE_URL must URL-escape password, got raw: %q", url)
	}
	if got[EnvServiceReadyTimeout] == "" {
		t.Errorf("SERVICE_READY_TIMEOUT must be set")
	}
}

func TestAppendServiceConnEnv_NoServices(t *testing.T) {
	if out := appendServiceConnEnv([]string{"X=1"}, nil); len(out) != 1 {
		t.Fatalf("expected passthrough, got %v", out)
	}
}

func TestSandboxOptions_Clone_DeepCopiesServices(t *testing.T) {
	orig := SandboxOptions{Services: []ServiceSpec{validPostgresService()}}
	clone := orig.Clone()
	clone.Services[0].Env[EnvPostgresPassword] = "mutated"
	if orig.Services[0].Env[EnvPostgresPassword] == "mutated" {
		t.Fatal("Clone must deep-copy ServiceSpec.Env (mutation leaked to original)")
	}
}

func TestMergeSandboxEnv_InjectsServiceConn(t *testing.T) {
	opts := SandboxOptions{
		RepoURL:  "https://example.com/r.git",
		Branch:   "main",
		Backend:  CodeBackendClaudeCode,
		Services: []ServiceSpec{validPostgresService()},
	}
	got := envSliceToMap(mergeSandboxEnv(opts))
	if got[EnvPostgresHost] != "db" {
		t.Errorf("POSTGRES_HOST = %q, want db", got[EnvPostgresHost])
	}
	if got[EnvDatabaseURL] == "" {
		t.Errorf("DATABASE_URL must be injected")
	}
}

func TestSandboxOptions_MarshalJSON_MasksServiceSecrets(t *testing.T) {
	opts := SandboxOptions{Services: []ServiceSpec{func() ServiceSpec {
		s := validPostgresService()
		s.SeedSQL = "INSERT INTO t VALUES (1);"
		return s
	}()}}
	blob, err := opts.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(blob)
	if strings.Contains(js, "s3cr3t") {
		t.Errorf("POSTGRES_PASSWORD leaked into JSON: %s", js)
	}
	if strings.Contains(js, "INSERT INTO") {
		t.Errorf("seed SQL leaked into JSON: %s", js)
	}
}

// envSliceToMap парсит []string{"K=V"} в карту (последнее значение ключа побеждает).
func envSliceToMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, kv := range env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}
