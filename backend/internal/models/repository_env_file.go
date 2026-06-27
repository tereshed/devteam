package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RepositoryEnvFile — «инъекция env-файла» уровня репозитория проекта. Один файл на
// project_repository (UNIQUE). Содержимое (EncryptedContent) шифруется AES-256-GCM и
// никогда не сериализуется в JSON напрямую. FileName/TargetDir определяют, куда
// sandbox-entrypoint положит файл в рабочей копии репо (TargetDir пуст = корень).
type RepositoryEnvFile struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ProjectRepositoryID uuid.UUID `gorm:"column:project_repository_id;type:uuid;not null" json:"project_repository_id"`
	FileName            string    `gorm:"column:file_name;type:varchar(255);not null" json:"file_name"`
	TargetDir           string    `gorm:"column:target_dir;type:varchar(512);not null;default:''" json:"target_dir"`
	EncryptedContent    []byte    `gorm:"column:encrypted_content;type:bytea;not null" json:"-"`
	CreatedAt           time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt           time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

func (RepositoryEnvFile) TableName() string {
	return "repository_env_files"
}

func (f *RepositoryEnvFile) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
