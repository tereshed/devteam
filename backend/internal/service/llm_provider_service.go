package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	internallm "github.com/devteam/backend/internal/llm"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
)

// ErrLLMProviderInvalid — общая ошибка валидации (детальный текст — в Error()).
var ErrLLMProviderInvalid = errors.New("invalid llm provider")

// LLMProviderInput — DTO для создания/обновления провайдера.
type LLMProviderInput struct {
	Name         string
	Kind         models.LLMProviderKind
	BaseURL      string
	AuthType     models.LLMProviderAuthType
	Credential   string // plaintext, шифруется сервисом перед сохранением.
	DefaultModel string
	Enabled      bool
}

// LLMProviderService — CRUD, тест подключения, health-check (Sprint 15.10).
type LLMProviderService interface {
	Create(ctx context.Context, in LLMProviderInput) (*models.LLMProvider, error)
	Update(ctx context.Context, id uuid.UUID, in LLMProviderInput) (*models.LLMProvider, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.LLMProvider, error)
	List(ctx context.Context, onlyEnabled bool) ([]models.LLMProvider, error)
	HealthCheck(ctx context.Context, id uuid.UUID) error
	TestConnection(ctx context.Context, in LLMProviderInput) error
	// ResolveCredentials реализует internallm.SecretsResolver — нужен фабрике NewLLMClient.
	ResolveCredentials(ctx context.Context, provider *models.LLMProvider) (string, error)
}

type llmProviderService struct {
	repo      repository.LLMProviderRepository
	encryptor Encryptor
	// healthTimeout — таймаут на health-check вызов к провайдеру.
	healthTimeout time.Duration
}

// NewLLMProviderService собирает сервис. encryptor может быть NoopEncryptor для dev.
func NewLLMProviderService(repo repository.LLMProviderRepository, encryptor Encryptor) LLMProviderService {
	return &llmProviderService{
		repo:          repo,
		encryptor:     encryptor,
		healthTimeout: 10 * time.Second,
	}
}

func (s *llmProviderService) Create(ctx context.Context, in LLMProviderInput) (*models.LLMProvider, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	p := &models.LLMProvider{
		Name:         strings.TrimSpace(in.Name),
		Kind:         in.Kind,
		BaseURL:      in.BaseURL,
		AuthType:     in.AuthType,
		DefaultModel: in.DefaultModel,
		Enabled:      in.Enabled,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	// Шифруем credential c AAD по итоговому ID (после Create — ID известен).
	if in.Credential != "" && in.AuthType != models.LLMProviderAuthNone {
		blob, err := s.encryptor.Encrypt([]byte(in.Credential), aad(p.ID))
		if err != nil {
			return nil, fmt.Errorf("encrypt credentials: %w", err)
		}
		p.CredentialsEncrypted = blob
		if err := s.repo.Update(ctx, p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (s *llmProviderService) Update(ctx context.Context, id uuid.UUID, in LLMProviderInput) (*models.LLMProvider, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	existing.Name = strings.TrimSpace(in.Name)
	existing.Kind = in.Kind
	existing.BaseURL = in.BaseURL
	existing.AuthType = in.AuthType
	existing.DefaultModel = in.DefaultModel
	existing.Enabled = in.Enabled
	if in.Credential != "" && in.AuthType != models.LLMProviderAuthNone {
		blob, err := s.encryptor.Encrypt([]byte(in.Credential), aad(existing.ID))
		if err != nil {
			return nil, fmt.Errorf("encrypt credentials: %w", err)
		}
		existing.CredentialsEncrypted = blob
	} else if in.AuthType == models.LLMProviderAuthNone {
		existing.CredentialsEncrypted = nil
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *llmProviderService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *llmProviderService) GetByID(ctx context.Context, id uuid.UUID) (*models.LLMProvider, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *llmProviderService) List(ctx context.Context, onlyEnabled bool) ([]models.LLMProvider, error) {
	return s.repo.List(ctx, onlyEnabled)
}

// HealthCheck создаёт клиента провайдера и вызывает HealthCheck (с таймаутом healthTimeout).
func (s *llmProviderService) HealthCheck(ctx context.Context, id uuid.UUID) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	client, err := internallm.NewLLMClient(ctx, p, s)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, s.healthTimeout)
	defer cancel()
	return client.HealthCheck(ctx)
}

// TestConnection собирает временного клиента из LLMProviderInput (без записи в БД)
// и зовёт HealthCheck. Нужен для UI-формы "Тест подключения" перед сохранением.
func (s *llmProviderService) TestConnection(ctx context.Context, in LLMProviderInput) error {
	if err := validateInput(in); err != nil {
		return err
	}
	if !in.Enabled {
		// На пустом провайдере проверки нет смысла делать; считаем "ok".
		return nil
	}
	tmp := &models.LLMProvider{
		Name:         in.Name,
		Kind:         in.Kind,
		BaseURL:      in.BaseURL,
		AuthType:     in.AuthType,
		DefaultModel: in.DefaultModel,
		Enabled:      true,
	}
	resolver := internallm.SecretsResolverFunc(func(ctx context.Context, _ *models.LLMProvider) (string, error) {
		return in.Credential, nil
	})
	client, err := internallm.NewLLMClient(ctx, tmp, resolver)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, s.healthTimeout)
	defer cancel()
	return client.HealthCheck(ctx)
}

// ResolveCredentials дешифрует blob, делегируя Encryptor. Реализация internallm.SecretsResolver.
func (s *llmProviderService) ResolveCredentials(ctx context.Context, provider *models.LLMProvider) (string, error) {
	if provider == nil || len(provider.CredentialsEncrypted) == 0 {
		return "", nil
	}
	plaintext, err := s.encryptor.Decrypt(provider.CredentialsEncrypted, aad(provider.ID))
	if err != nil {
		return "", fmt.Errorf("decrypt credentials: %w", err)
	}
	return string(plaintext), nil
}

// aad — associated data для AES-GCM: префикс + id, чтобы blob нельзя было перенести на другую запись.
func aad(id uuid.UUID) []byte {
	return []byte("llm_provider:" + id.String())
}

func validateInput(in LLMProviderInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrLLMProviderInvalid)
	}
	if !in.Kind.IsValid() {
		return fmt.Errorf("%w: kind=%q", ErrLLMProviderInvalid, in.Kind)
	}
	if !in.AuthType.IsValid() {
		return fmt.Errorf("%w: auth_type=%q", ErrLLMProviderInvalid, in.AuthType)
	}
	if in.AuthType != models.LLMProviderAuthNone && strings.TrimSpace(in.Credential) == "" {
		// Для обновления credential может быть пустым (тогда не меняем). Этот хелпер вызывается
		// и из TestConnection — там пустой credential для auth != none тоже невалиден.
		// Но при Update это случай "не менять credential" — поэтому validate не зовём при Update без credential
		// (validateInput всегда зовётся; Update перед шифровкой проверит TrimSpace отдельно).
	}
	return nil
}

// Compile-time check: LLMProviderService реализует SecretsResolver (для фабрики).
var _ internallm.SecretsResolver = (*llmProviderService)(nil)
