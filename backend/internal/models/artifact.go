package models

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ArtifactKind — тип артефакта. НЕ enum, чтобы новые типы добавлялись без миграции.
// Список ниже — известные/стандартные типы; код может встретить и другие.
//
// Жизненный цикл артефактов задачи:
//
//	plan → review(plan) → decomposition → review(decomposition) → subtask_description
//	→ review(subtask_description) → code_diff → review(code_diff) → merged_code →
//	review(merged_code) → test_result
type ArtifactKind string

const (
	ArtifactKindPlan                ArtifactKind = "plan"
	ArtifactKindDecomposition       ArtifactKind = "decomposition"
	ArtifactKindSubtaskDescription  ArtifactKind = "subtask_description"
	ArtifactKindCodeDiff            ArtifactKind = "code_diff"
	ArtifactKindMergedCode          ArtifactKind = "merged_code"
	ArtifactKindReview              ArtifactKind = "review"
	ArtifactKindTestResult          ArtifactKind = "test_result"
)

// ArtifactStatus — соответствует CHECK chk_artifacts_status в миграции 033.
type ArtifactStatus string

const (
	ArtifactStatusReady      ArtifactStatus = "ready"
	ArtifactStatusSuperseded ArtifactStatus = "superseded"
)

// IsValid проверяет валидность статуса артефакта.
func (s ArtifactStatus) IsValid() bool {
	switch s {
	case ArtifactStatusReady, ArtifactStatusSuperseded:
		return true
	default:
		return false
	}
}

// Artifact — однородная единица результата работы агента.
// Router получает только metadata + Summary (никогда Content) — бюджет контекста LLM.
// Полный Content загружают специалисты по ID.
type Artifact struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TaskID         uuid.UUID      `gorm:"type:uuid;not null" json:"task_id"`
	ParentID       *uuid.UUID     `gorm:"type:uuid" json:"parent_id"` // review.parent_id = plan.id
	ProducerAgent  string         `gorm:"type:varchar(255);not null" json:"producer_agent"`
	Kind           ArtifactKind   `gorm:"type:varchar(64);not null" json:"kind"`
	Summary        string         `gorm:"type:varchar(500);not null" json:"summary"`
	Content        datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"content" swaggertype:"object"`
	Status         ArtifactStatus `gorm:"type:varchar(32);not null;default:'ready'" json:"status"`
	Iteration      int            `gorm:"type:integer;not null;default:0" json:"iteration"`
	CreatedAt      time.Time      `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

// TableName возвращает имя таблицы.
func (Artifact) TableName() string {
	return "artifacts"
}

// BeforeCreate генерирует UUID если не задан.
func (a *Artifact) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// ValidateArtifactSummary проверяет summary: не пустой (после trim), длина ≤ 500 рун.
//
// ВАЖНО: используется utf8.RuneCountInString, не len(). Postgres VARCHAR(N) считает
// символы (codepoints), а не байты, поэтому Go-валидация должна быть в тех же
// единицах. Иначе кириллический summary в 500 символов (≈1000 UTF-8 байт)
// ошибочно отвергался бы Go при валидной для БД длине.
func ValidateArtifactSummary(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	return utf8.RuneCountInString(s) <= 500
}
