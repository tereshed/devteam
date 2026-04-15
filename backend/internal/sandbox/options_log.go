package sandbox

import (
	"fmt"
	"sort"
	"strings"
)

// LogSafe возвращает представление опций для логов/метрик без утечки секретов из EnvVars
// и без вывода полных Instruction/Context (только длины).
//
// Внимание: fmt.Printf("%+v", opts) обходит Stringer и напечатает сырые поля — для логирования
// используйте opts.LogSafe() или fmt.Sprintf("%s", opts) при наличии fmt.Stringer.
func (o SandboxOptions) LogSafe() string {
	return fmt.Sprintf(
		"SandboxOptions{TaskID:%q ProjectID:%q Backend:%q Image:%q RepoURL:%q Branch:%q Instruction:%s Context:%s EnvVars:%s Timeout:%v StopGracePeriod:%v DisableNetwork:%v ResourceLimit:{NanoCPUs:%d MemoryMB:%d DiskMB:%d PIDsLimit:%d}}",
		o.TaskID,
		o.ProjectID,
		o.Backend,
		o.Image,
		maskRepoURL(o.RepoURL),
		o.Branch,
		byteLenDesc(o.Instruction),
		byteLenDesc(o.Context),
		maskEnvVarsForLog(o.EnvVars),
		o.Timeout,
		o.StopGracePeriod,
		o.DisableNetwork,
		o.ResourceLimit.NanoCPUs,
		o.ResourceLimit.MemoryMB,
		o.ResourceLimit.DiskMB,
		o.ResourceLimit.PIDsLimit,
	)
}

// String реализует fmt.Stringer: то же, что LogSafe, чтобы случайный %s / %v не сливал ключи.
func (o SandboxOptions) String() string {
	return o.LogSafe()
}

func byteLenDesc(s string) string {
	if s == "" {
		return "<empty>"
	}
	return fmt.Sprintf("<%d bytes>", len(s))
}

// maskRepoURL скрывает userinfo в clone URL (аналог маскирования в entrypoint).
// Для SCP-формы без «://» (например token@github.com:org/repo.git) маскирует всё до последнего «@» перед хостом.
func maskRepoURL(raw string) string {
	if raw == "" {
		return ""
	}
	scheme := strings.Index(raw, "://")
	if scheme < 0 {
		at := strings.LastIndex(raw, "@")
		if at > 0 {
			return "***@" + raw[at+1:]
		}
		return raw
	}
	at := strings.LastIndex(raw, "@")
	if at <= scheme+3 {
		return raw
	}
	return raw[:scheme+3] + "***@" + raw[at+1:]
}

func maskEnvVarsForLog(m map[string]string) string {
	if m == nil {
		return "nil"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("map[")
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		if sensitiveEnvKey(k) {
			b.WriteString(maskSecretValue(m[k]))
		} else {
			b.WriteString(fmt.Sprintf("%q", m[k]))
		}
	}
	b.WriteByte(']')
	return b.String()
}

func sensitiveEnvKey(k string) bool {
	ku := strings.ToUpper(k)
	if ku == strings.ToUpper(EnvAnthropicAPIKey) {
		return true
	}
	return strings.Contains(ku, "API_KEY") ||
		strings.Contains(ku, "SECRET") ||
		strings.Contains(ku, "TOKEN") ||
		strings.Contains(ku, "PASSWORD") ||
		strings.Contains(ku, "KEY")
}

func maskSecretValue(v string) string {
	r := []rune(v)
	switch {
	case len(r) == 0:
		return `""`
	case len(r) <= 6:
		return `"***"`
	case strings.HasPrefix(v, "sk-"):
		if len(r) <= 14 {
			return `"sk-***"`
		}
		return fmt.Sprintf("%q", string(r[:8])+"***"+string(r[len(r)-4:]))
	default:
		if len(r) <= 10 {
			return `"***"`
		}
		return fmt.Sprintf("%q", string(r[:3])+"***"+string(r[len(r)-2:]))
	}
}
