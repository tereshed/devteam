package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrAgentSecretNotFound — sentinel, чтобы caller отличал "не нашли" от "ошибка БД"
// и мог вернуть пользователю осмысленное сообщение, а не падать тихо.
var ErrAgentSecretNotFound = errors.New("agent secret not found")

// agentSecretListColumns — НЕ включает encrypted_value, чтобы случайно не утянуть
// зашифрованный blob при list-операциях (ленивая загрузка через GetByName).
const agentSecretListColumns = "id, agent_id, key_name, created_at, updated_at"

// AgentSecretRepository — CRUD по зашифрованным секретам агентов.
//
// Контракт: encrypted_value записывается УЖЕ зашифрованным (через pkg/crypto.AESEncryptor,
// AAD = id.String()). Repository не шифрует/дешифрует — это делает сервис-слой.
type AgentSecretRepository interface {
	Create(ctx context.Context, secret *models.AgentSecret) error
	GetByName(ctx context.Context, agentID uuid.UUID, keyName string) (*models.AgentSecret, error)
	ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]models.AgentSecret, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type agentSecretRepository struct {
	db *gorm.DB
}

// NewAgentSecretRepository — конструктор.
func NewAgentSecretRepository(db *gorm.DB) AgentSecretRepository {
	return &agentSecretRepository{db: db}
}

func (r *agentSecretRepository) Create(ctx context.Context, secret *models.AgentSecret) error {
	if !models.ValidateAgentSecretKeyName(secret.KeyName) {
		return fmt.Errorf("invalid agent secret key_name: %q", secret.KeyName)
	}
	if len(secret.EncryptedValue) < 29 {
		// 29 = MinCiphertextBlobLen (1 version + 12 nonce + 16 GCM tag).
		// Защита от случайной записи нешифрованных данных.
		return fmt.Errorf("encrypted_value too short (%d bytes), refusing to write — looks unencrypted", len(secret.EncryptedValue))
	}
	if err := r.db.WithContext(ctx).Create(secret).Error; err != nil {
		return fmt.Errorf("failed to create agent secret: %w", err)
	}
	return nil
}

func (r *agentSecretRepository) GetByName(ctx context.Context, agentID uuid.UUID, keyName string) (*models.AgentSecret, error) {
	var s models.AgentSecret
	err := r.db.WithContext(ctx).
		Where("agent_id = ? AND key_name = ?", agentID, keyName).
		First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentSecretNotFound
		}
		return nil, fmt.Errorf("failed to get agent secret %s/%s: %w", agentID, keyName, err)
	}
	return &s, nil
}

func (r *agentSecretRepository) ListByAgentID(ctx context.Context, agentID uuid.UUID) ([]models.AgentSecret, error) {
	var secrets []models.AgentSecret
	err := r.db.WithContext(ctx).
		Select(agentSecretListColumns). // encrypted_value не тащим
		Where("agent_id = ?", agentID).
		Order("key_name ASC").
		Find(&secrets).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list agent secrets for agent %s: %w", agentID, err)
	}
	return secrets, nil
}

func (r *agentSecretRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.AgentSecret{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete agent secret %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAgentSecretNotFound
	}
	return nil
}
