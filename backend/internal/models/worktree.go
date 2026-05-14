package models

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorktreeState — состояние git worktree.
// Соответствует CHECK chk_worktrees_state из миграции 036.
type WorktreeState string

const (
	WorktreeStateAllocated WorktreeState = "allocated"
	WorktreeStateInUse     WorktreeState = "in_use"
	WorktreeStateReleased  WorktreeState = "released"
)

// IsValid проверяет валидность состояния.
func (s WorktreeState) IsValid() bool {
	switch s {
	case WorktreeStateAllocated, WorktreeStateInUse, WorktreeStateReleased:
		return true
	default:
		return false
	}
}

// Worktree — git worktree для изолированного запуска параллельных sandbox-агентов.
//
// БЕЗОПАСНОСТЬ — критично:
//   - Поле Path в структуре ОТСУТСТВУЕТ намеренно, в БД соответствующей колонки также нет.
//     Путь ВСЕГДА вычисляется методом ComputePath(worktreesRoot) детерминированно из
//     типизированных uuid.UUID. Это исключает path traversal через подмену БД.
//   - BranchName форсится backend'ом по шаблону "task-<task_uuid>-wt-<wt_uuid>";
//     CHECK chk_worktrees_branch_name_format в БД отвергнет любое другое значение.
//   - BaseBranch валидируется CHECK chk_worktrees_base_branch_safe (regex ^[a-zA-Z0-9._/]
//     [a-zA-Z0-9._/-]{0,127}$) — отказ при ведущем "-" (git flag injection).
type Worktree struct {
	ID           uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TaskID       uuid.UUID     `gorm:"type:uuid;not null" json:"task_id"`
	SubtaskID    *uuid.UUID    `gorm:"type:uuid" json:"subtask_id,omitempty"`
	BaseBranch   string        `gorm:"type:varchar(128);not null" json:"base_branch"`
	BranchName   string        `gorm:"type:varchar(128);not null" json:"branch_name"`
	State        WorktreeState `gorm:"type:varchar(16);not null;default:'allocated'" json:"state"`
	AgentJobID   *int64        `gorm:"type:bigint" json:"agent_job_id,omitempty"`
	AllocatedAt  time.Time     `gorm:"type:timestamp with time zone;default:now()" json:"allocated_at"`
	ReleasedAt   *time.Time    `gorm:"type:timestamp with time zone" json:"released_at,omitempty"`
}

// TableName возвращает имя таблицы.
func (Worktree) TableName() string {
	return "worktrees"
}

// BeforeCreate генерирует UUID и форсит BranchName, если он не задан.
// BranchName формируется ТОЛЬКО здесь — никогда не приходит снаружи.
func (w *Worktree) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	w.BranchName = MakeWorktreeBranchName(w.TaskID, w.ID)
	return nil
}

// ComputePath — детерминированный путь к worktree.
// ВСЕГДА используется этот метод; БД-колонки с путём НЕТ.
//
// Защиты:
//   - На вход — только типизированные uuid.UUID (UUID.String() даёт строго
//     "8-4-4-4-12" формат, без / и ..).
//   - filepath.Join нормализует разделители для текущей ОС.
//   - filepath.Clean убирает "." и нормализует ..
//   - Префикс-проверка гарантирует, что результат не вышел за worktreesRoot
//     (defence-in-depth — даже при ошибке в коде Join'а).
//
// Возвращает ошибку если результат каким-то образом ушёл за пределы корня.
func (w *Worktree) ComputePath(worktreesRoot string) (string, error) {
	if worktreesRoot == "" {
		return "", fmt.Errorf("worktrees root is empty")
	}
	if w.TaskID == uuid.Nil || w.ID == uuid.Nil {
		return "", fmt.Errorf("task_id and id are required to compute worktree path")
	}
	root := filepath.Clean(worktreesRoot)
	candidate := filepath.Clean(filepath.Join(root, w.TaskID.String(), w.ID.String()))
	// Защита от path traversal (defence-in-depth, см. branch_validator + DB CHECK).
	if !strings.HasPrefix(candidate+string(filepath.Separator), root+string(filepath.Separator)) {
		return "", fmt.Errorf("computed worktree path escapes root: %q (root=%q)", candidate, root)
	}
	return candidate, nil
}

// MakeWorktreeBranchName строит имя ветки в каноническом формате.
// Используется в BeforeCreate и в WorktreeManager.
func MakeWorktreeBranchName(taskID, worktreeID uuid.UUID) string {
	return fmt.Sprintf("task-%s-wt-%s", taskID, worktreeID)
}

// worktreeBranchNameRe соответствует CHECK chk_worktrees_branch_name_format из миграции 036.
var worktreeBranchNameRe = regexp.MustCompile(
	`^task-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}-wt-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

// ValidateWorktreeBranchName — ранняя Go-валидация имени ветки.
// Возвращает true если имя соответствует БД-формату (две UUID v4 в нижнем регистре).
func ValidateWorktreeBranchName(name string) bool {
	return worktreeBranchNameRe.MatchString(name)
}
