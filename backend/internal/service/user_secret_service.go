package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrUserSecretNotFound   = errors.New("user secret not found")
	ErrUserSecretInvalidKey = errors.New("invalid secret key_name (must match ^[A-Z][A-Z0-9_]{0,127}$)")
	ErrUserSecretValidation = errors.New("user secret validation failed")
)

type UserSecretService struct {
	repo    repository.UserSecretRepository
	secrets *SecretService
	logger  *slog.Logger
}

func NewUserSecretService(
	repo repository.UserSecretRepository,
	secrets *SecretService,
	logger *slog.Logger,
) *UserSecretService {
	return &UserSecretService{
		repo:    repo,
		secrets: secrets,
		logger:  logger,
	}
}

type SetUserSecretInput struct {
	UserID  uuid.UUID
	KeyName string
	Value   string
}

type SetUserSecretOutput struct {
	SecretID uuid.UUID `json:"id"`
	UserID   uuid.UUID `json:"user_id"`
	KeyName  string    `json:"key_name"`
}

func (s *UserSecretService) Set(ctx context.Context, in SetUserSecretInput) (*SetUserSecretOutput, error) {
	if in.UserID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrUserSecretValidation)
	}
	if !models.ValidateAgentSecretKeyName(in.KeyName) {
		return nil, ErrUserSecretInvalidKey
	}
	if in.Value == "" {
		return nil, fmt.Errorf("%w: value must be non-empty", ErrUserSecretValidation)
	}

	s.logger.Info("user secret set",
		slog.String("key_name", in.KeyName),
		slog.String("value", "<redacted>"),
		slog.String("user_id", in.UserID.String()),
	)

	existing, err := s.repo.GetByName(ctx, in.UserID, in.KeyName)
	if err != nil && !errors.Is(err, repository.ErrUserSecretNotFound) {
		return nil, fmt.Errorf("check existing secret: %w", err)
	}

	if existing != nil {
		blob, encErr := s.secrets.Encrypt(existing.ID, in.Value)
		if encErr != nil {
			return nil, encErr
		}
		existing.EncryptedValue = blob
		if upErr := s.repo.Update(ctx, existing); upErr != nil {
			return nil, fmt.Errorf("update user secret: %w", upErr)
		}
		return &SetUserSecretOutput{
			SecretID: existing.ID,
			UserID:   existing.UserID,
			KeyName:  existing.KeyName,
		}, nil
	}

	secret := &models.UserSecret{
		ID:      uuid.New(),
		UserID:  in.UserID,
		KeyName: in.KeyName,
	}
	blob, encErr := s.secrets.Encrypt(secret.ID, in.Value)
	if encErr != nil {
		return nil, encErr
	}
	secret.EncryptedValue = blob

	if createErr := s.repo.Create(ctx, secret); createErr != nil {
		return nil, fmt.Errorf("persist secret: %w", createErr)
	}
	return &SetUserSecretOutput{
		SecretID: secret.ID,
		UserID:   secret.UserID,
		KeyName:  secret.KeyName,
	}, nil
}

func (s *UserSecretService) List(ctx context.Context, userID uuid.UUID) ([]models.UserSecret, error) {
	return s.repo.ListByUserID(ctx, userID)
}

func (s *UserSecretService) Delete(ctx context.Context, secretID uuid.UUID) error {
	if err := s.repo.Delete(ctx, secretID); err != nil {
		if errors.Is(err, repository.ErrUserSecretNotFound) {
			return ErrUserSecretNotFound
		}
		return fmt.Errorf("delete user secret %s: %w", secretID, err)
	}
	return nil
}

// GetAllDecrypted returns all secrets for a user as a map[keyName]plaintext.
func (s *UserSecretService) GetAllDecrypted(ctx context.Context, userID uuid.UUID) (map[string]string, error) {
	secrets, err := s.repo.GetAllDecrypted(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(secrets))
	for _, sec := range secrets {
		plain, decErr := s.secrets.Decrypt(sec.ID, sec.EncryptedValue)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt user secret %s/%s: %w", userID, sec.KeyName, decErr)
		}
		result[sec.KeyName] = plain
	}
	return result, nil
}
