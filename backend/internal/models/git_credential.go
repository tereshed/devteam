package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GitCredentialProvider тип git-провайдера для credentials
type GitCredentialProvider string

const (
	GitCredentialProviderGitHub    GitCredentialProvider = "github"
	GitCredentialProviderGitLab    GitCredentialProvider = "gitlab"
	GitCredentialProviderBitbucket GitCredentialProvider = "bitbucket"
)

// IsValid проверяет валидность провайдера
func (p GitCredentialProvider) IsValid() bool {
	switch p {
	case GitCredentialProviderGitHub, GitCredentialProviderGitLab, GitCredentialProviderBitbucket:
		return true
	default:
		return false
	}
}

// GitCredentialAuthType тип аутентификации
type GitCredentialAuthType string

const (
	GitCredentialAuthToken  GitCredentialAuthType = "token"
	GitCredentialAuthSSHKey GitCredentialAuthType = "ssh_key"
	GitCredentialAuthOAuth  GitCredentialAuthType = "oauth"
)

// IsValid проверяет валидность типа аутентификации
func (a GitCredentialAuthType) IsValid() bool {
	switch a {
	case GitCredentialAuthToken, GitCredentialAuthSSHKey, GitCredentialAuthOAuth:
		return true
	default:
		return false
	}
}

// GitCredential зашифрованные учётные данные для доступа к Git-репозиториям
type GitCredential struct {
	ID             uuid.UUID             `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID             `gorm:"type:uuid;not null" json:"user_id"`
	User           *User                 `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Provider       GitCredentialProvider  `gorm:"type:varchar(50);not null" json:"provider"`
	AuthType       GitCredentialAuthType  `gorm:"type:varchar(50);not null;default:'token'" json:"auth_type"`
	EncryptedValue []byte                `gorm:"type:bytea;not null" json:"-"`
	Label          string                `gorm:"type:varchar(255);not null" json:"label"`
	CreatedAt      time.Time             `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt      time.Time             `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// TableName возвращает имя таблицы
func (GitCredential) TableName() string {
	return "git_credentials"
}

// BeforeCreate генерирует UUID если не задан
func (gc *GitCredential) BeforeCreate(tx *gorm.DB) error {
	if gc.ID == uuid.Nil {
		gc.ID = uuid.New()
	}
	return nil
}
