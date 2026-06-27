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
		"SandboxOptions{TaskID:%q ProjectID:%q Backend:%q Image:%q RepoURL:%q Branch:%q Instruction:%s Context:%s EnvVars:%s ProjectEnv:%s Timeout:%v StopGracePeriod:%v DisableNetwork:%v ResourceLimit:{NanoCPUs:%d MemoryMB:%d DiskMB:%d PIDsLimit:%d} Services:%s InjectedEnvFiles:%s}",
		o.TaskID,
		o.ProjectID,
		o.Backend,
		o.Image,
		maskRepoURL(o.RepoURL),
		o.Branch,
		byteLenDesc(o.Instruction),
		byteLenDesc(o.Context),
		maskEnvVarsForLog(o.EnvVars),
		maskProjectEnvForLog(o.ProjectEnv),
		o.Timeout,
		o.StopGracePeriod,
		o.DisableNetwork,
		o.ResourceLimit.NanoCPUs,
		o.ResourceLimit.MemoryMB,
		o.ResourceLimit.DiskMB,
		o.ResourceLimit.PIDsLimit,
		servicesLogSafe(o.Services),
		injectedEnvFilesLogSafe(o.InjectedEnvFiles),
	)
}

// injectedEnvFilesLogSafe — представление «инъекции env-файлов» для логов: для каждого
// имя/папка цели (не секрет) + длина содержимого (содержимое — секрет, не печатаем).
func injectedEnvFilesLogSafe(specs []InjectedEnvFileSpec) string {
	if len(specs) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i := range specs {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "{file:%q dir:%q content:%s}", specs[i].FileName, specs[i].TargetDir, byteLenDesc(specs[i].Content))
	}
	b.WriteByte(']')
	return b.String()
}

// servicesLogSafe — представление сервис-сайдкаров для логов: alias/image/port +
// маскированный env (POSTGRES_PASSWORD), без сырого seed.
func servicesLogSafe(services []ServiceSpec) string {
	if len(services) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range services {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "{alias:%q image:%q port:%d env:%s seed:%s}",
			s.Alias, s.Image, s.Port, maskEnvVarsForLog(s.Env), byteLenDesc(s.SeedSQL))
	}
	b.WriteByte(']')
	return b.String()
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

// maskProjectEnvForLog — представление ProjectEnv для логов: ВСЕ значения скрыты
// (произвольные пользовательские ключи вроде DATABASE_URL не угадываются эвристикой
// sensitiveEnvKey, поэтому маскируем безусловно). Печатаем только отсортированные имена.
func maskProjectEnvForLog(m map[string]string) string {
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
		b.WriteString(`=***`)
	}
	b.WriteByte(']')
	return b.String()
}

func sensitiveEnvKey(k string) bool {
	ku := strings.ToUpper(k)
	switch ku {
	case strings.ToUpper(EnvAnthropicAPIKey),
		strings.ToUpper(EnvClaudeCodeOAuthToken),
		strings.ToUpper(EnvAnthropicAuthToken):
		return true
	}
	// Sprint 16.C: HERMES_MCP_* — секреты MCP-серверов Hermes (резолв ${secret:NAME}).
	// Не имеют слова KEY/TOKEN в имени (например HERMES_MCP_GITHUB_GITHUB_PERSONAL_ACCESS_TOKEN
	// сматчится по TOKEN, но HERMES_MCP_FOO_BAR — нет). Поэтому редактируем по префиксу.
	if strings.HasPrefix(ku, "HERMES_MCP_") {
		return true
	}
	// MCP_* — секреты MCP-серверов Claude Code (env-индирекция). Редактируем по префиксу.
	if strings.HasPrefix(ku, "MCP_") {
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
