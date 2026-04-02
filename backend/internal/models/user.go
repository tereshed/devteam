package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRole представляет роль пользователя в системе
type UserRole string

const (
	RoleGuest UserRole = "guest"
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

// User представляет пользователя в системе
type User struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Email         string         `gorm:"type:varchar(255);uniqueIndex:idx_users_email,where:deleted_at IS NULL;not null" json:"email"`
	PasswordHash  string         `gorm:"type:varchar(255);not null" json:"-"`
	Role          UserRole       `gorm:"type:varchar(50);default:'user';index:idx_users_role,where:deleted_at IS NULL;not null" json:"role"`
	EmailVerified bool           `gorm:"default:false" json:"email_verified"`
	CreatedAt     time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"type:timestamp with time zone;index" json:"-"`
}

// TableName возвращает имя таблицы для модели User
func (User) TableName() string {
	return "users"
}

// BeforeCreate вызывается перед созданием записи
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// IsValidRole проверяет, является ли роль валидной
func IsValidRole(role string) bool {
	switch UserRole(role) {
	case RoleGuest, RoleUser, RoleAdmin:
		return true
	default:
		return false
	}
}
