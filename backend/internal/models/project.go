package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// GitProvider тип git-провайдера проекта
type GitProvider string

const (
	GitProviderGitHub    GitProvider = "github"
	GitProviderGitLab    GitProvider = "gitlab"
	GitProviderBitbucket GitProvider = "bitbucket"
	GitProviderLocal     GitProvider = "local"
)

// IsValid проверяет валидность провайдера
func (gp GitProvider) IsValid() bool {
	switch gp {
	case GitProviderGitHub, GitProviderGitLab, GitProviderBitbucket, GitProviderLocal:
		return true
	default:
		return false
	}
}

// ProjectStatus статус проекта
type ProjectStatus string

const (
	ProjectStatusActive         ProjectStatus = "active"
	ProjectStatusPaused         ProjectStatus = "paused"
	ProjectStatusArchived       ProjectStatus = "archived"
	ProjectStatusIndexing       ProjectStatus = "indexing"
	ProjectStatusIndexingFailed ProjectStatus = "indexing_failed"
	ProjectStatusReady          ProjectStatus = "ready"
)

// IsValid проверяет валидность статуса
func (ps ProjectStatus) IsValid() bool {
	switch ps {
	case ProjectStatusActive, ProjectStatusPaused, ProjectStatusArchived,
		ProjectStatusIndexing, ProjectStatusIndexingFailed, ProjectStatusReady:
		return true
	default:
		return false
	}
}

// Project центральная сущность — связывает репозиторий, команду агентов и контекст
type Project struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name              string         `gorm:"type:varchar(255);not null" json:"name"`
	Description       string         `gorm:"type:text" json:"description"`
	GitProvider       GitProvider    `gorm:"type:varchar(50);not null;default:'local'" json:"git_provider"`
	GitURL            string         `gorm:"type:varchar(1024)" json:"git_url"`
	GitDefaultBranch  string         `gorm:"type:varchar(255);not null;default:'main'" json:"git_default_branch"`
	LastIndexedCommit string         `gorm:"type:varchar(255);not null;default:''" json:"last_indexed_commit"`
	GitCredentialsID  *uuid.UUID     `gorm:"type:uuid" json:"git_credentials_id"`
	GitCredential     *GitCredential `gorm:"foreignKey:GitCredentialsID" json:"git_credential,omitempty"`
	// GitIntegrationCredentialID — выбранный OAuth-аккаунт провайдера (мульти-аккаунт).
	// NULL → фолбэк на первый аккаунт провайдера владельца (обратная совместимость).
	GitIntegrationCredentialID *uuid.UUID `gorm:"type:uuid" json:"git_integration_credential_id"`
	VectorCollection           string     `gorm:"type:varchar(255)" json:"vector_collection"`
	TechStack         datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"tech_stack" swaggertype:"object"`
	Status            ProjectStatus  `gorm:"type:varchar(50);not null;default:'active'" json:"status"`
	Settings          datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"settings" swaggertype:"object"`
	UserID            uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	User              *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	// Repositories — git-репозитории проекта (мульти-репо). Для существующих проектов
	// миграция 078 создаёт одну primary-строку slug='main' из полей git_* выше.
	Repositories []ProjectRepository `gorm:"foreignKey:ProjectID" json:"repositories,omitempty"`
	CreatedAt    time.Time           `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt    time.Time           `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы
func (Project) TableName() string {
	return "projects"
}

// PrimaryRepo возвращает primary-репозиторий проекта (или nil, если репозитории не загружены/пусты).
func (p *Project) PrimaryRepo() *ProjectRepository {
	for i := range p.Repositories {
		if p.Repositories[i].IsPrimary {
			return &p.Repositories[i]
		}
	}
	if len(p.Repositories) > 0 {
		return &p.Repositories[0]
	}
	return nil
}

// RepoBySlug возвращает репозиторий по slug (или nil, если не найден).
func (p *Project) RepoBySlug(slug string) *ProjectRepository {
	for i := range p.Repositories {
		if p.Repositories[i].Slug == slug {
			return &p.Repositories[i]
		}
	}
	return nil
}

// BeforeCreate генерирует UUID если не задан
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}
