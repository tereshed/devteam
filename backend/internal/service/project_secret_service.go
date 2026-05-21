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
	ErrProjectSecretNotFound   = errors.New("project secret not found")
	ErrProjectSecretInvalidKey = errors.New("invalid secret key_name (must match ^[A-Z][A-Z0-9_]{0,127}$)")
	ErrProjectSecretValidation = errors.New("project secret validation failed")
)

type ProjectSecretService struct {
	repo    repository.ProjectSecretRepository
	secrets *SecretService
	logger  *slog.Logger
}

func NewProjectSecretService(
	repo repository.ProjectSecretRepository,
	secrets *SecretService,
	logger *slog.Logger,
) *ProjectSecretService {
	return &ProjectSecretService{
		repo:    repo,
		secrets: secrets,
		logger:  logger,
	}
}

type SetProjectSecretInput struct {
	ProjectID uuid.UUID
	KeyName   string
	Value     string
}

type SetProjectSecretOutput struct {
	SecretID  uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
	KeyName   string    `json:"key_name"`
}

func (s *ProjectSecretService) Set(ctx context.Context, in SetProjectSecretInput) (*SetProjectSecretOutput, error) {
	if in.ProjectID == uuid.Nil {
		return nil, fmt.Errorf("%w: project_id is required", ErrProjectSecretValidation)
	}
	if !models.ValidateAgentSecretKeyName(in.KeyName) {
		return nil, ErrProjectSecretInvalidKey
	}
	if in.Value == "" {
		return nil, fmt.Errorf("%w: value must be non-empty", ErrProjectSecretValidation)
	}

	s.logger.Info("project secret set",
		slog.String("key_name", in.KeyName),
		slog.String("value", "<redacted>"),
		slog.String("project_id", in.ProjectID.String()),
	)

	existing, err := s.repo.GetByName(ctx, in.ProjectID, in.KeyName)
	if err != nil && !errors.Is(err, repository.ErrProjectSecretNotFound) {
		return nil, fmt.Errorf("check existing secret: %w", err)
	}

	if existing != nil {
		blob, encErr := s.secrets.Encrypt(existing.ID, in.Value)
		if encErr != nil {
			return nil, encErr
		}
		existing.EncryptedValue = blob
		if upErr := s.repo.Update(ctx, existing); upErr != nil {
			return nil, fmt.Errorf("update project secret: %w", upErr)
		}
		return &SetProjectSecretOutput{
			SecretID:  existing.ID,
			ProjectID: existing.ProjectID,
			KeyName:   existing.KeyName,
		}, nil
	}

	secret := &models.ProjectSecret{
		ID:        uuid.New(),
		ProjectID: in.ProjectID,
		KeyName:   in.KeyName,
	}
	blob, encErr := s.secrets.Encrypt(secret.ID, in.Value)
	if encErr != nil {
		return nil, encErr
	}
	secret.EncryptedValue = blob

	if createErr := s.repo.Create(ctx, secret); createErr != nil {
		return nil, fmt.Errorf("persist secret: %w", createErr)
	}
	return &SetProjectSecretOutput{
		SecretID:  secret.ID,
		ProjectID: secret.ProjectID,
		KeyName:   secret.KeyName,
	}, nil
}

func (s *ProjectSecretService) List(ctx context.Context, projectID uuid.UUID) ([]models.ProjectSecret, error) {
	return s.repo.ListByProjectID(ctx, projectID)
}

func (s *ProjectSecretService) Delete(ctx context.Context, secretID uuid.UUID) error {
	if err := s.repo.Delete(ctx, secretID); err != nil {
		if errors.Is(err, repository.ErrProjectSecretNotFound) {
			return ErrProjectSecretNotFound
		}
		return fmt.Errorf("delete project secret %s: %w", secretID, err)
	}
	return nil
}

// GetAllDecrypted returns all secrets for a project as a map[keyName]plaintext.
// Used by BuildArtifacts for bulk placeholder resolution.
func (s *ProjectSecretService) GetAllDecrypted(ctx context.Context, projectID uuid.UUID) (map[string]string, error) {
	secrets, err := s.repo.GetAllDecrypted(ctx, projectID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(secrets))
	for _, sec := range secrets {
		plain, decErr := s.secrets.Decrypt(sec.ID, sec.EncryptedValue)
		if decErr != nil {
			return nil, fmt.Errorf("decrypt project secret %s/%s: %w", projectID, sec.KeyName, decErr)
		}
		result[sec.KeyName] = plain
	}
	return result, nil
}
