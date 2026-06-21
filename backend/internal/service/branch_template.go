package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/devteam/backend/internal/sandbox"
	"github.com/google/uuid"
)

// Шаблон имён веток — per-project конвенция именования git-веток (см. docs/rules).
//
// Шаблон — это free-form строка: литералы сохраняются как есть, а плейсхолдеры
// {name} (и {name|fallback}) подставляются. Поддерживаемые плейсхолдеры:
//
//	{ticket}      — внешний ключ задачи (external_key, напр. DEV-123), verbatim.
//	              Наличие {ticket} БЕЗ fallback делает ключ обязательным.
//	{ticket|slug} — ключ тикета, а при его отсутствии — fallback-плейсхолдер.
//	{slug},{title}— слугифицированный заголовок (lower, [a-z0-9-], ≤40).
//	{short_id}    — первые 8 символов UUID задачи.
//	{id}          — полный UUID задачи.
//	{date}        — YYYYMMDD (UTC); также {yyyy},{mm},{dd}.
//
// Один и тот же шаблон служит двумя целями: генерирует авто-ветку (RenderBranchName)
// и задаёт «жёсткий формат» — выведенный из него regex (CompileBranchPattern)
// валидирует ручные override'ы, чтобы они не обходили конвенцию.

// DefaultMRTitleTemplate воспроизводит историческое поведение публикатора PR
// (`PolyMaths: <title>`). Применяется, когда у проекта не задан свой шаблон тайтла.
const DefaultMRTitleTemplate = "PolyMaths: {title}"

// DefaultBranchTemplate воспроизводит историческое поведение generateBranchName
// (task/<short_id>-<slug>). Применяется, когда у проекта не задан свой шаблон
// или когда рендер пользовательского шаблона дал невалидное имя (safety-fallback).
const DefaultBranchTemplate = "task/{short_id}-{slug}"

const branchSlugMaxLen = 40

var (
	// ErrBranchTemplateInvalid — шаблон содержит неизвестный плейсхолдер или
	// порождает невалидное (не git-ref-safe) имя ветки.
	ErrBranchTemplateInvalid = errors.New("invalid branch name template")
	// ErrInvalidExternalKey — external_key не соответствует безопасному формату.
	ErrInvalidExternalKey = errors.New("invalid external key")
	// ErrExternalKeyRequired — шаблон проекта требует {ticket}, а ключ не передан.
	ErrExternalKeyRequired = errors.New("external key is required for this project")
	// ErrBranchNamingLocked — проект запрещает ручной override имени ветки.
	ErrBranchNamingLocked = errors.New("manual branch name override is locked for this project")
	// ErrBranchPatternMismatch — переданное имя ветки не соответствует формату проекта.
	ErrBranchPatternMismatch = errors.New("branch name does not match the project format")
	// ErrTaskInvalidBranch — переданный override имени ветки не git-ref-safe.
	ErrTaskInvalidBranch = errors.New("invalid branch name")
	// ErrMRTitleTemplateInvalid — шаблон тайтла MR содержит неизвестный плейсхолдер.
	ErrMRTitleTemplateInvalid = errors.New("invalid MR title template")
)

// mrSpaceRe схлопывает повторяющиеся пробелы в тайтле MR (например после пустого
// плейсхолдера).
var mrSpaceRe = regexp.MustCompile(`\s{2,}`)

// externalKeyRe — безопасный формат ключа тикета (DEV-123, ABC-4567, FEAT_12).
// Должен начинаться с буквы/цифры; далее буквы/цифры/'-'/'_'; ≤64 символов.
// НЕ навязывает строго PREFIX-NUMBER (разные трекеры), но гарантирует
// git-ref-safe и shell-safe строку для verbatim-подстановки в ветку.
var externalKeyRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// branchPlaceholderRe выделяет {name} либо {name|fallback}.
var branchPlaceholderRe = regexp.MustCompile(`\{([a-z_]+)(?:\|([a-z_]+))?\}`)

// branchTicketBareRe — {ticket} без fallback. Наличие ⇒ external_key обязателен.
var branchTicketBareRe = regexp.MustCompile(`\{ticket\}`)

// Пост-обработка: схлопывание разделителей и зачистка краёв.
var (
	branchDashRunRe  = regexp.MustCompile(`[-_]{2,}`)
	branchSlashSepRe = regexp.MustCompile(`/[-_]+`)
	branchSepSlashRe = regexp.MustCompile(`[-_]+/`)
	branchSlashRunRe = regexp.MustCompile(`/{2,}`)
)

// BranchVars — значения для подстановки в шаблон имени ветки.
type BranchVars struct {
	TaskID      uuid.UUID
	Title       string
	ExternalKey string
	Now         time.Time
}

// ValidateExternalKey проверяет формат внешнего ключа (fail-loud при создании задачи).
func ValidateExternalKey(key string) error {
	if !externalKeyRe.MatchString(key) {
		return fmt.Errorf("%w: %q (allowed: letters/digits/-/_, ≤64 chars, e.g. DEV-123)", ErrInvalidExternalKey, key)
	}
	return nil
}

// TemplateRequiresTicket возвращает true, если шаблон содержит {ticket} без fallback
// — тогда external_key обязателен для задачи.
func TemplateRequiresTicket(tmpl string) bool {
	return branchTicketBareRe.MatchString(tmpl)
}

func knownBranchPlaceholder(name string) bool {
	switch name {
	case "ticket", "slug", "title", "short_id", "id", "date", "yyyy", "mm", "dd":
		return true
	default:
		return false
	}
}

func slugifyForBranch(title string) string {
	slug := strings.ToLower(title)
	slug = branchNameSlugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > branchSlugMaxLen {
		slug = slug[:branchSlugMaxLen]
		slug = strings.TrimSuffix(slug, "-")
	}
	return slug
}

func resolveBranchPlaceholder(name string, vars BranchVars) (string, error) {
	switch name {
	case "ticket":
		return vars.ExternalKey, nil
	case "slug", "title":
		return slugifyForBranch(vars.Title), nil
	case "short_id":
		return vars.TaskID.String()[:8], nil
	case "id":
		return vars.TaskID.String(), nil
	case "date":
		return vars.Now.UTC().Format("20060102"), nil
	case "yyyy":
		return vars.Now.UTC().Format("2006"), nil
	case "mm":
		return vars.Now.UTC().Format("01"), nil
	case "dd":
		return vars.Now.UTC().Format("02"), nil
	default:
		return "", fmt.Errorf("%w: unknown placeholder {%s}", ErrBranchTemplateInvalid, name)
	}
}

// cleanupBranch схлопывает повторяющиеся разделители (в т.ч. вокруг '/', что важно,
// когда плейсхолдер оказался пустым: "issue/_slug" → "issue/slug") и срезает края.
func cleanupBranch(s string) string {
	s = branchDashRunRe.ReplaceAllStringFunc(s, func(run string) string { return string(run[0]) })
	s = branchSlashSepRe.ReplaceAllString(s, "/")
	s = branchSepSlashRe.ReplaceAllString(s, "/")
	s = branchSlashRunRe.ReplaceAllString(s, "/")
	return strings.Trim(s, "-_/")
}

// RenderBranchName рендерит шаблон в имя ветки. Пустой шаблон ⇒ DefaultBranchTemplate.
// Если в шаблоне нет ни {short_id}, ни {id}, в хвост дописывается short_id —
// гарантия уникальности ветки между задачами с одинаковым ключом/заголовком.
// Возвращает ошибку, если имя не прошло git-ref-safe валидацию (caller откатывается
// на дефолт). Никогда не должен ронять создание задачи сам по себе.
func RenderBranchName(tmpl string, vars BranchVars) (string, error) {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		tmpl = DefaultBranchTemplate
	}
	hasID := strings.Contains(tmpl, "{short_id}") || strings.Contains(tmpl, "{id}")

	var renderErr error
	out := branchPlaceholderRe.ReplaceAllStringFunc(tmpl, func(m string) string {
		sub := branchPlaceholderRe.FindStringSubmatch(m)
		name, fallback := sub[1], sub[2]
		val, err := resolveBranchPlaceholder(name, vars)
		if err != nil {
			renderErr = err
			return ""
		}
		if val == "" && fallback != "" {
			val, err = resolveBranchPlaceholder(fallback, vars)
			if err != nil {
				renderErr = err
				return ""
			}
		}
		return val
	})
	if renderErr != nil {
		return "", renderErr
	}

	out = cleanupBranch(out)
	if !hasID {
		short := vars.TaskID.String()[:8]
		if out == "" {
			out = short
		} else {
			out = out + "-" + short
		}
	}
	if out == "" {
		return "", fmt.Errorf("%w: rendered empty", ErrBranchTemplateInvalid)
	}
	if err := sandbox.ValidateBranchName(out); err != nil {
		return "", fmt.Errorf("%w: %v", ErrBranchTemplateInvalid, err)
	}
	return out, nil
}

func branchPlaceholderSubPattern(name string) (string, error) {
	switch name {
	case "ticket":
		return `[A-Za-z0-9][A-Za-z0-9_-]*`, nil
	case "slug", "title":
		return `[a-z0-9]+(?:-[a-z0-9]+)*`, nil
	case "short_id":
		return `[0-9a-f]{8}`, nil
	case "id":
		return `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`, nil
	case "date":
		return `[0-9]{8}`, nil
	case "yyyy":
		return `[0-9]{4}`, nil
	case "mm", "dd":
		return `[0-9]{2}`, nil
	default:
		return "", fmt.Errorf("%w: unknown placeholder {%s}", ErrBranchTemplateInvalid, name)
	}
}

// CompileBranchPattern выводит из шаблона якорный regex, описывающий «жёсткий формат».
// Литералы экранируются, плейсхолдеры заменяются на свои под-паттерны. Пустой шаблон
// ⇒ паттерн дефолтного шаблона. Назначение паттерна — валидация РУЧНЫХ override'ов
// (он описывает структуру шаблона, без учёта авто-суффикса и схлопывания пустых
// сегментов — это особенности генерации, а не формы).
func CompileBranchPattern(tmpl string) (*regexp.Regexp, error) {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		tmpl = DefaultBranchTemplate
	}
	var b strings.Builder
	b.WriteString("^")
	last := 0
	for _, loc := range branchPlaceholderRe.FindAllStringSubmatchIndex(tmpl, -1) {
		b.WriteString(regexp.QuoteMeta(tmpl[last:loc[0]]))
		name := tmpl[loc[2]:loc[3]]
		sub, err := branchPlaceholderSubPattern(name)
		if err != nil {
			return nil, err
		}
		if loc[4] >= 0 {
			fsub, err := branchPlaceholderSubPattern(tmpl[loc[4]:loc[5]])
			if err != nil {
				return nil, err
			}
			b.WriteString("(?:" + sub + "|" + fsub + ")")
		} else {
			b.WriteString(sub)
		}
		last = loc[1]
	}
	b.WriteString(regexp.QuoteMeta(tmpl[last:]))
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// ValidateBranchTemplate проверяет шаблон при сохранении настроек проекта (fail-loud
// в UI): все плейсхолдеры известны, тестовый рендер даёт git-ref-safe имя, выведенный
// паттерн компилируется.
func ValidateBranchTemplate(tmpl string) error {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		return nil
	}
	for _, m := range branchPlaceholderRe.FindAllStringSubmatch(tmpl, -1) {
		if !knownBranchPlaceholder(m[1]) {
			return fmt.Errorf("%w: unknown placeholder {%s}", ErrBranchTemplateInvalid, m[1])
		}
		if m[2] != "" && !knownBranchPlaceholder(m[2]) {
			return fmt.Errorf("%w: unknown fallback placeholder {%s}", ErrBranchTemplateInvalid, m[2])
		}
	}
	if _, err := RenderBranchName(tmpl, BranchVars{
		TaskID:      uuid.New(),
		Title:       "sample task title",
		ExternalKey: "DEV-123",
		Now:         time.Now(),
	}); err != nil {
		return err
	}
	if _, err := CompileBranchPattern(tmpl); err != nil {
		return fmt.Errorf("%w: derived pattern: %v", ErrBranchTemplateInvalid, err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Шаблон тайтла MR/PR (per-project). Отличия от ветки: тайтл — свободный текст
// (НЕ git-ref-safe, без слугификации {title}, без авто-суффикса).
// ─────────────────────────────────────────────────────────────────────────────

// MRTitleVars — значения для подстановки в шаблон тайтла MR.
type MRTitleVars struct {
	TaskID      uuid.UUID
	Title       string
	ExternalKey string
	Branch      string
	RepoSlug    string
	Now         time.Time
}

func knownMRPlaceholder(name string) bool {
	switch name {
	case "title", "slug", "ticket", "short_id", "id", "branch", "repo", "date", "yyyy", "mm", "dd":
		return true
	default:
		return false
	}
}

func resolveMRPlaceholder(name string, vars MRTitleVars) string {
	switch name {
	case "title":
		return vars.Title
	case "slug":
		return slugifyForBranch(vars.Title)
	case "ticket":
		return vars.ExternalKey
	case "short_id":
		return vars.TaskID.String()[:8]
	case "id":
		return vars.TaskID.String()
	case "branch":
		return vars.Branch
	case "repo":
		return vars.RepoSlug
	case "date":
		return vars.Now.UTC().Format("20060102")
	case "yyyy":
		return vars.Now.UTC().Format("2006")
	case "mm":
		return vars.Now.UTC().Format("01")
	case "dd":
		return vars.Now.UTC().Format("02")
	default:
		return ""
	}
}

// RenderMRTitle рендерит шаблон тайтла MR. Пустой шаблон ⇒ DefaultMRTitleTemplate.
// Плейсхолдеры подставляются как есть (с fallback {a|b}); неизвестные → пусто.
// Лишние пробелы схлопываются, края тримятся. Никогда не возвращает пустую строку
// (фолбэк на «PolyMaths: <title>»).
func RenderMRTitle(tmpl string, vars MRTitleVars) string {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		tmpl = DefaultMRTitleTemplate
	}
	out := branchPlaceholderRe.ReplaceAllStringFunc(tmpl, func(m string) string {
		sub := branchPlaceholderRe.FindStringSubmatch(m)
		name, fallback := sub[1], sub[2]
		v := resolveMRPlaceholder(name, vars)
		if v == "" && fallback != "" {
			v = resolveMRPlaceholder(fallback, vars)
		}
		return v
	})
	out = strings.TrimSpace(mrSpaceRe.ReplaceAllString(out, " "))
	if out == "" {
		out = strings.TrimSpace("PolyMaths: " + vars.Title)
	}
	return out
}

// ValidateMRTitleTemplate проверяет шаблон тайтла MR при сохранении настроек
// проекта: все плейсхолдеры известны (пустой шаблон допустим — дефолт).
func ValidateMRTitleTemplate(tmpl string) error {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" {
		return nil
	}
	for _, m := range branchPlaceholderRe.FindAllStringSubmatch(tmpl, -1) {
		if !knownMRPlaceholder(m[1]) {
			return fmt.Errorf("%w: unknown placeholder {%s}", ErrMRTitleTemplateInvalid, m[1])
		}
		if m[2] != "" && !knownMRPlaceholder(m[2]) {
			return fmt.Errorf("%w: unknown fallback placeholder {%s}", ErrMRTitleTemplateInvalid, m[2])
		}
	}
	return nil
}
