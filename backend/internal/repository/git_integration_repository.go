package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GitIntegrationCredentialAAD — AAD для AES-GCM шифрования полей записи.
// Конвенция docs/rules/main.md §2.3 п.5: AAD = id записи (UUID PK).
// Это защищает от cross-row substitution: подмена access_token_enc блобом из чужой
// строки → расшифровка падает с GCM-tag-mismatch (id другой → AAD не сойдётся).
func GitIntegrationCredentialAAD(id uuid.UUID) []byte {
	return []byte("git_integration_credential:" + id.String())
}

// GitIntegrationCredentialRepository — CRUD по git_integration_credentials.
//
// Контракт: шифрование/дешифрование делает СЕРВИС (см. service.GitIntegrationService),
// repository работает с уже-зашифрованными blob-полями. Repository:
//   - проверяет минимальную длину blob'ов (≥29 байт = MinCiphertextBlobLen), чтобы
//     не дать случайно записать plaintext;
//   - один user — один provider (UNIQUE constraint).
type GitIntegrationCredentialRepository interface {
	// Upsert атомарно создаёт/обновляет запись по (user_id, provider, host, account_login).
	// Для INSERT-сценария: caller обязан выставить cred.ID до вызова (AAD = id).
	Upsert(ctx context.Context, cred *models.GitIntegrationCredential) error
	// GetByUserAndProvider возвращает ПЕРВЫЙ аккаунт провайдера (фолбэк для legacy-резолва,
	// когда у проекта/репо не выбран конкретный git_integration_credential_id).
	GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (*models.GitIntegrationCredential, error)
	// GetByID возвращает аккаунт по id (для резолва выбранного проектом/репо аккаунта).
	GetByID(ctx context.Context, id uuid.UUID) (*models.GitIntegrationCredential, error)
	// UpdateTokens обновляет зашифрованные токены + срок жизни (после refresh), НЕ меняя id
	// (AAD шифрования = id; FK-ссылки сохраняются). refreshTokenEnc пустой — refresh не меняем.
	UpdateTokens(ctx context.Context, id uuid.UUID, accessTokenEnc, refreshTokenEnc []byte, expiresAt, lastRefreshedAt *time.Time) error
	// ListByUserAndProvider — все аккаунты пользователя для провайдера (мульти-аккаунт).
	ListByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) ([]models.GitIntegrationCredential, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.GitIntegrationCredential, error)
	// DeleteByID удаляет конкретный аккаунт (мульти-аккаунт disconnect).
	DeleteByID(ctx context.Context, userID, id uuid.UUID) error
	DeleteByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) error
	// DeleteLegacyUnlabeled удаляет строку без account_login для (user, provider, host) —
	// одноразовая миграция при первом re-auth после апгрейда, когда логин уже захвачен.
	DeleteLegacyUnlabeled(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider, host string) error
}

type gitIntegrationCredentialRepository struct {
	db *gorm.DB
}

// NewGitIntegrationCredentialRepository — конструктор.
func NewGitIntegrationCredentialRepository(db *gorm.DB) GitIntegrationCredentialRepository {
	return &gitIntegrationCredentialRepository{db: db}
}

func (r *gitIntegrationCredentialRepository) Upsert(ctx context.Context, cred *models.GitIntegrationCredential) error {
	if cred == nil || cred.UserID == uuid.Nil {
		return ErrInvalidInput
	}
	if !cred.Provider.IsValid() {
		return fmt.Errorf("%w: invalid provider %q", ErrInvalidInput, cred.Provider)
	}
	if len(cred.AccessTokenEnc) < crypto.MinCiphertextBlobLen {
		return fmt.Errorf("access_token_enc too short (%d bytes), refusing to write — looks unencrypted", len(cred.AccessTokenEnc))
	}
	if len(cred.RefreshTokenEnc) > 0 && len(cred.RefreshTokenEnc) < crypto.MinCiphertextBlobLen {
		return fmt.Errorf("refresh_token_enc too short (%d bytes), refusing to write — looks unencrypted", len(cred.RefreshTokenEnc))
	}
	if len(cred.ByoClientSecretEnc) > 0 && len(cred.ByoClientSecretEnc) < crypto.MinCiphertextBlobLen {
		return fmt.Errorf("byo_client_secret_enc too short (%d bytes), refusing to write — looks unencrypted", len(cred.ByoClientSecretEnc))
	}
	if cred.ID == uuid.Nil {
		cred.ID = uuid.New()
	}

	// Настоящий ON CONFLICT (user_id, provider) DO UPDATE на уровне БД.
	//
	// Важно: ID обновляется тоже. Caller (сервис) шифрует токены с AAD = cred.ID;
	// если оставить старый id, AAD при следующей расшифровке не сойдётся (GCM tag mismatch).
	// Поэтому перезаписываем id под тот же блоб шифротекстов, который пришёл с этим Upsert.
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "provider"}, {Name: "host"}, {Name: "account_login"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"id",
			"byo_client_id",
			"byo_client_secret_enc",
			"access_token_enc",
			"refresh_token_enc",
			"token_type",
			"scopes",
			"expires_at",
			"last_refreshed_at",
			"updated_at",
		}),
	}).Create(cred).Error
}

func (r *gitIntegrationCredentialRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.GitIntegrationCredential, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var cred models.GitIntegrationCredential
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cred).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGitIntegrationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

func (r *gitIntegrationCredentialRepository) UpdateTokens(ctx context.Context, id uuid.UUID, accessTokenEnc, refreshTokenEnc []byte, expiresAt, lastRefreshedAt *time.Time) error {
	if id == uuid.Nil {
		return ErrInvalidInput
	}
	if len(accessTokenEnc) < crypto.MinCiphertextBlobLen {
		return fmt.Errorf("access_token_enc too short (%d bytes), refusing to write — looks unencrypted", len(accessTokenEnc))
	}
	updates := map[string]interface{}{
		"access_token_enc":  accessTokenEnc,
		"expires_at":        expiresAt,
		"last_refreshed_at": lastRefreshedAt,
	}
	if len(refreshTokenEnc) > 0 {
		if len(refreshTokenEnc) < crypto.MinCiphertextBlobLen {
			return fmt.Errorf("refresh_token_enc too short (%d bytes), refusing to write — looks unencrypted", len(refreshTokenEnc))
		}
		updates["refresh_token_enc"] = refreshTokenEnc
	}
	res := r.db.WithContext(ctx).Model(&models.GitIntegrationCredential{}).
		Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrGitIntegrationNotFound
	}
	return nil
}

func (r *gitIntegrationCredentialRepository) ListByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) ([]models.GitIntegrationCredential, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if !provider.IsValid() {
		return nil, fmt.Errorf("%w: invalid provider %q", ErrInvalidInput, provider)
	}
	var creds []models.GitIntegrationCredential
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		Order("account_login ASC, created_at ASC").
		Find(&creds).Error
	if err != nil {
		return nil, err
	}
	return creds, nil
}

func (r *gitIntegrationCredentialRepository) DeleteByID(ctx context.Context, userID, id uuid.UUID) error {
	if userID == uuid.Nil || id == uuid.Nil {
		return ErrInvalidInput
	}
	res := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.GitIntegrationCredential{})
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ErrGitIntegrationNotFound
	}
	return nil
}

func (r *gitIntegrationCredentialRepository) DeleteLegacyUnlabeled(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider, host string) error {
	if userID == uuid.Nil {
		return ErrInvalidInput
	}
	return r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ? AND host = ? AND account_login = ''", userID, provider, host).
		Delete(&models.GitIntegrationCredential{}).Error
}

func (r *gitIntegrationCredentialRepository) GetByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) (*models.GitIntegrationCredential, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	if !provider.IsValid() {
		return nil, fmt.Errorf("%w: invalid provider %q", ErrInvalidInput, provider)
	}
	var cred models.GitIntegrationCredential
	// Мульти-аккаунт фолбэк: предпочитаем аккаунт с заполненным account_login (не legacy
	// ''-строку) и самый свежий по дате — чтобы не выбрать протухший/осиротевший legacy-аккаунт.
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		Order("(account_login = '') ASC, created_at DESC").
		First(&cred).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGitIntegrationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &cred, nil
}

func (r *gitIntegrationCredentialRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]models.GitIntegrationCredential, error) {
	if userID == uuid.Nil {
		return nil, ErrInvalidInput
	}
	var creds []models.GitIntegrationCredential
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("provider ASC").
		Find(&creds).Error
	if err != nil {
		return nil, err
	}
	return creds, nil
}

func (r *gitIntegrationCredentialRepository) DeleteByUserAndProvider(ctx context.Context, userID uuid.UUID, provider models.GitIntegrationProvider) error {
	if userID == uuid.Nil {
		return ErrInvalidInput
	}
	if !provider.IsValid() {
		return fmt.Errorf("%w: invalid provider %q", ErrInvalidInput, provider)
	}
	res := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		Delete(&models.GitIntegrationCredential{})
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ErrGitIntegrationNotFound
	}
	return nil
}
