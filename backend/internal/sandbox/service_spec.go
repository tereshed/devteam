package sandbox

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Эфемерные сервис-сайдкары прогона (Sprint 22). Для прогонов, объявивших
// Services, runner поднимает по контейнеру на сервис в той же per-run bridge-сети
// (с alias-DNS), чтобы агент-тестер мог гонять интеграционные тесты против реальной
// БД. Изначальный кейс — одноразовый PostgreSQL для Alembic/SQL-миграций.
//
// Connection-vars инжектятся в env агента (см. mergeSandboxEnv → appendServiceConnEnv);
// entrypoint ждёт готовности по /dev/tcp (см. EnvServiceReadyTimeout).
const (
	EnvPostgresHost     = "POSTGRES_HOST"
	EnvPostgresPort     = "POSTGRES_PORT"
	EnvPostgresDB       = "POSTGRES_DB"
	EnvPostgresUser     = "POSTGRES_USER"
	EnvPostgresPassword = "POSTGRES_PASSWORD"
	// EnvDatabaseURL — готовая строка подключения postgresql://user:pass@alias:port/db.
	EnvDatabaseURL = "DATABASE_URL"
	// EnvServiceReadyTimeout — потолок ожидания сервиса в entrypoint (секунды).
	EnvServiceReadyTimeout = "SERVICE_READY_TIMEOUT"
)

const (
	// DefaultServiceReadyTimeoutSecs — дефолтный потолок ожидания готовности сервиса.
	DefaultServiceReadyTimeoutSecs = 60
	// maxServiceReadyTimeoutSecs — верхняя граница (защита от вечного ожидания).
	maxServiceReadyTimeoutSecs = 600
	// maxServiceSeedBytes — потолок размера seed SQL (CopyToContainer в память).
	maxServiceSeedBytes = 256 * 1024
	// maxServiceEnvValueBytes — потолок длины значения env сервис-контейнера.
	maxServiceEnvValueBytes = 8 * 1024
)

// serviceAliasRE — RFC1123-label: alias становится DNS-именем сервиса в bridge-сети
// (агент обращается как `alias:port`). Должен быть безопасен для DNS и shell.
var serviceAliasRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// ServiceSpec — описание одного эфемерного сервис-контейнера прогона.
type ServiceSpec struct {
	// Alias — сетевой alias/hostname в bridge-сети (например "db"). DNS-safe.
	Alias string
	// Image — образ сервиса; проверяется против allowlist раннера (allowedServiceImages).
	Image string
	// Env — переменные окружения сервис-контейнера (POSTGRES_DB/USER/PASSWORD и т.п.).
	// НЕ проходят ValidateEnvKeys (это конфиг сервиса, не агента) — валидируются здесь.
	Env map[string]string
	// Port — порт, на который агент подключается к сервису (5432 для postgres).
	Port int
	// ReadyTimeoutSecs — потолок ожидания готовности (0 → DefaultServiceReadyTimeoutSecs).
	ReadyTimeoutSecs int
	// SeedSQL — необязательный сид; копируется в /docker-entrypoint-initdb.d/seed.sql
	// ДО старта контейнера (официальный postgres-образ прогоняет его на первой инициализации).
	SeedSQL string
	// ResourceLimit — cgroup-лимиты сервис-контейнера (0-поля → полы/потолки политики).
	ResourceLimit ResourceLimit
}

// EffectiveReadyTimeoutSecs возвращает потолок ожидания готовности с дефолтом.
func (s ServiceSpec) EffectiveReadyTimeoutSecs() int {
	if s.ReadyTimeoutSecs > 0 {
		return s.ReadyTimeoutSecs
	}
	return DefaultServiceReadyTimeoutSecs
}

// validateStructural проверяет поля спеки независимо от allowlist образов раннера
// (image-allowlist сверяется в RunTask, как и для агент-образа). Вызывается из
// SandboxOptions.validateWithoutResourceLimits.
func (s ServiceSpec) validateStructural() error {
	if !serviceAliasRE.MatchString(s.Alias) {
		return fmt.Errorf("%w: service alias %q must match ^[a-z][a-z0-9-]{0,62}$", ErrInvalidOptions, s.Alias)
	}
	if strings.TrimSpace(s.Image) == "" {
		return fmt.Errorf("%w: service %q image is empty", ErrInvalidOptions, s.Alias)
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("%w: service %q port %d out of range 1..65535", ErrInvalidOptions, s.Alias, s.Port)
	}
	if s.ReadyTimeoutSecs < 0 || s.ReadyTimeoutSecs > maxServiceReadyTimeoutSecs {
		return fmt.Errorf("%w: service %q ready_timeout out of range 0..%d", ErrInvalidOptions, s.Alias, maxServiceReadyTimeoutSecs)
	}
	for k, v := range s.Env {
		if !isSafeEnvKeyToken(k) {
			return fmt.Errorf("%w: service %q env key %q invalid", ErrInvalidOptions, s.Alias, k)
		}
		if strings.ContainsAny(v, "\n\r\x00") {
			return fmt.Errorf("%w: service %q env value for %q contains control characters", ErrInvalidOptions, s.Alias, k)
		}
		if len(v) > maxServiceEnvValueBytes {
			return fmt.Errorf("%w: service %q env value for %q too long", ErrInvalidOptions, s.Alias, k)
		}
	}
	if len(s.SeedSQL) > maxServiceSeedBytes {
		return fmt.Errorf("%w: service %q seed SQL exceeds %d bytes", ErrInvalidOptions, s.Alias, maxServiceSeedBytes)
	}
	return nil
}

// serviceEnvSlice конвертит map env сервис-контейнера в детерминированный []string.
func serviceEnvSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(env))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}

// appendServiceConnEnv дописывает в env агента connection-vars первого сервиса
// (POSTGRES_*/DATABASE_URL/SERVICE_READY_TIMEOUT). Только первый сервис получает
// удобные POSTGRES_*; остальные доступны по своему alias. Вызывается последней
// строкой mergeSandboxEnv → высший приоритет (перекрывает попытки shadow через EnvVars).
func appendServiceConnEnv(out []string, services []ServiceSpec) []string {
	if len(services) == 0 {
		return out
	}
	s := services[0]
	db := s.Env[EnvPostgresDB]
	user := s.Env[EnvPostgresUser]
	pass := s.Env[EnvPostgresPassword]
	port := strconv.Itoa(s.Port)
	out = append(out,
		EnvPostgresHost+"="+s.Alias,
		EnvPostgresPort+"="+port,
		EnvPostgresDB+"="+db,
		EnvPostgresUser+"="+user,
		EnvPostgresPassword+"="+pass,
		EnvDatabaseURL+"="+buildPostgresURL(user, pass, s.Alias, port, db),
		EnvServiceReadyTimeout+"="+strconv.Itoa(s.EffectiveReadyTimeoutSecs()),
	)
	return out
}

// buildPostgresURL собирает строку подключения с экранированием user/pass
// (пароль может содержать @ / : — net/url.UserPassword кодирует корректно).
func buildPostgresURL(user, pass, host, port, db string) string {
	u := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(user, pass),
		Host:   host + ":" + port,
		Path:   "/" + db,
	}
	return u.String()
}
