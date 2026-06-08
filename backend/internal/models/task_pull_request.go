package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TaskPullRequest — открытый Pull Request по задаче в конкретном репозитории (мульти-репо).
// Одна задача может затронуть несколько репо → по одному PR на каждый затронутый репо.
// Done-гейт оркестратора считает задачу done только когда PR открыт по всем затронутым репо.
type TaskPullRequest struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TaskID    uuid.UUID `gorm:"type:uuid;not null;index" json:"task_id"`
	RepoSlug  string    `gorm:"type:varchar(64);not null" json:"repo_slug"`
	PRNumber  int       `gorm:"column:pr_number;not null" json:"pr_number"`
	PRURL     string    `gorm:"column:pr_url;type:varchar(1024);not null" json:"pr_url"`
	CreatedAt time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

// TableName возвращает имя таблицы
func (TaskPullRequest) TableName() string {
	return "task_pull_requests"
}

// BeforeCreate генерирует UUID если не задан
func (t *TaskPullRequest) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}
