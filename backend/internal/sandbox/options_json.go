package sandbox

import "encoding/json"

type sandboxOptionsWire struct {
	TaskID         string            `json:"task_id"`
	ProjectID      string            `json:"project_id"`
	Backend        CodeBackendType   `json:"backend"`
	Image          string            `json:"image"`
	RepoURL        string            `json:"repo_url"`
	Branch         string            `json:"branch"`
	Instruction    string            `json:"instruction"`
	Context        string            `json:"context"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	Timeout        string            `json:"timeout"`
	DisableNetwork bool              `json:"disable_network"`
	ResourceLimit  ResourceLimit     `json:"resource_limit"`
}

// MarshalJSON сериализует опции с той же политикой маскирования, что LogSafe: секретные env, маскированный RepoURL,
// Instruction/Context только как длина (строка вида "<N bytes>").
func (o SandboxOptions) MarshalJSON() ([]byte, error) {
	w := sandboxOptionsWire{
		TaskID:         o.TaskID,
		ProjectID:      o.ProjectID,
		Backend:        o.Backend,
		Image:          o.Image,
		RepoURL:        maskRepoURL(o.RepoURL),
		Branch:         o.Branch,
		Instruction:    byteLenDesc(o.Instruction),
		Context:        byteLenDesc(o.Context),
		EnvVars:        maskEnvVarsMapForJSON(o.EnvVars),
		Timeout:        o.Timeout.String(),
		DisableNetwork: o.DisableNetwork,
		ResourceLimit:  o.ResourceLimit,
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
