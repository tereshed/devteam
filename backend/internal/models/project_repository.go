package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProjectRepository — один git-репозиторий в составе проекта. Проект может содержать
// несколько репозиториев (например, отдельный под UI и под высоконагруженную часть).
// RoleDescription читает decomposer, чтобы раскладывать подзадачи по нужному репо.
type ProjectRepository struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"project_id"`
	Slug              string         `gorm:"type:varchar(64);not null" json:"slug"`
	DisplayName       string         `gorm:"type:varchar(255);not null" json:"display_name"`
	RoleDescription   string         `gorm:"type:text;not null;default:''" json:"role_description"`
	GitProvider       GitProvider    `gorm:"type:varchar(50);not null;default:'local'" json:"git_provider"`
	GitURL            string         `gorm:"type:varchar(1024);not null" json:"git_url"`
	GitDefaultBranch  string         `gorm:"type:varchar(255);not null;default:'main'" json:"git_default_branch"`
	GitCredentialsID  *uuid.UUID     `gorm:"type:uuid" json:"git_credentials_id"`
	GitCredential     *GitCredential `gorm:"foreignKey:GitCredentialsID" json:"git_credential,omitempty"`
	// GitIntegrationCredentialID — выбранный OAuth-аккаунт провайдера для этого репо
	// (мульти-аккаунт). NULL → наследует выбор проекта / фолбэк на первый аккаунт провайдера.
	GitIntegrationCredentialID *uuid.UUID `gorm:"type:uuid" json:"git_integration_credential_id"`
	VectorCollection           string     `gorm:"type:varchar(255)" json:"vector_collection"`
	LastIndexedCommit string         `gorm:"type:varchar(255);not null;default:''" json:"last_indexed_commit"`
	Status            ProjectStatus  `gorm:"type:varchar(50);not null;default:'active'" json:"status"`
	IsPrimary         bool           `gorm:"not null;default:false" json:"is_primary"`
	SortOrder         int            `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt         time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы
func (ProjectRepository) TableName() string {
	return "project_repositories"
}

// BeforeCreate генерирует UUID если не задан
func (r *ProjectRepository) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
