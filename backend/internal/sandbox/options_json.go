package sandbox

import "encoding/json"

type sandboxOptionsWire struct {
	TaskID          string            `json:"task_id"`
	ProjectID       string            `json:"project_id"`
	Backend         CodeBackendType   `json:"backend"`
	Image           string            `json:"image"`
	RepoURL         string            `json:"repo_url"`
	Branch          string            `json:"branch"`
	Instruction     string            `json:"instruction"`
	Context         string            `json:"context"`
	EnvVars         map[string]string `json:"env_vars,omitempty"`
	ProjectEnv      map[string]string `json:"project_env,omitempty"`
	Timeout         string            `json:"timeout"`
	StopGracePeriod string            `json:"stop_grace_period"`
	DisableNetwork  bool              `json:"disable_network"`
	ResourceLimit   ResourceLimit     `json:"resource_limit"`
	Services        []serviceSpecWire `json:"services,omitempty"`
	InjectedEnvFile *injectedEnvWire  `json:"injected_env_file,omitempty"`
}

// injectedEnvWire — представление InjectedEnvFileSpec для логов/JSON: имя/папка цели
// (не секрет) + длина содержимого (содержимое — секрет).
type injectedEnvWire struct {
	FileName  string `json:"file_name"`
	TargetDir string `json:"target_dir,omitempty"`
	Content   string `json:"content"`
}

func maskInjectedEnvFileForWire(spec *InjectedEnvFileSpec) *injectedEnvWire {
	if spec == nil {
		return nil
	}
	return &injectedEnvWire{
		FileName:  spec.FileName,
		TargetDir: spec.TargetDir,
		Content:   byteLenDesc(spec.Content),
	}
}

// serviceSpecWire — представление ServiceSpec для логов/JSON: env с маскированными
// секретами (POSTGRES_PASSWORD), seed только как длина.
type serviceSpecWire struct {
	Alias   string            `json:"alias"`
	Image   string            `json:"image"`
	Env     map[string]string `json:"env,omitempty"`
	Port    int               `json:"port"`
	SeedSQL string            `json:"seed_sql,omitempty"`
}

func maskServicesForWire(services []ServiceSpec) []serviceSpecWire {
	if len(services) == 0 {
		return nil
	}
	out := make([]serviceSpecWire, 0, len(services))
	for _, s := range services {
		out = append(out, serviceSpecWire{
			Alias:   s.Alias,
			Image:   s.Image,
			Env:     maskEnvVarsMapForJSON(s.Env),
			Port:    s.Port,
			SeedSQL: byteLenDesc(s.SeedSQL),
		})
	}
	return out
}

// MarshalJSON сериализует опции с той же политикой маскирования, что LogSafe: секретные env, маскированный RepoURL,
// Instruction/Context только как длина (строка вида "<N bytes>").
func (o SandboxOptions) MarshalJSON() ([]byte, error) {
	w := sandboxOptionsWire{
		TaskID:          o.TaskID,
		ProjectID:       o.ProjectID,
		Backend:         o.Backend,
		Image:           o.Image,
		RepoURL:         maskRepoURL(o.RepoURL),
		Branch:          o.Branch,
		Instruction:     byteLenDesc(o.Instruction),
		Context:         byteLenDesc(o.Context),
		EnvVars:         maskEnvVarsMapForJSON(o.EnvVars),
		ProjectEnv:      maskAllValuesForJSON(o.ProjectEnv),
		Timeout:         o.Timeout.String(),
		StopGracePeriod: o.StopGracePeriod.String(),
		DisableNetwork:  o.DisableNetwork,
		ResourceLimit:   o.ResourceLimit,
		Services:        maskServicesForWire(o.Services),
		InjectedEnvFile: maskInjectedEnvFileForWire(o.InjectedEnvFile),
	}
	return json.Marshal(w)
}

func maskEnvVarsMapForJSON(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if sensitiveEnvKey(k) {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}

// maskAllValuesForJSON скрывает ВСЕ значения (для ProjectEnv: произвольные пользовательские
// ключи — все значения секретные). Имена сохраняются.
func maskAllValuesForJSON(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k := range m {
		out[k] = "***"
	}
	return out
}
