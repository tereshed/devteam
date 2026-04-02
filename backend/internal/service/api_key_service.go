package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/repository"
)

var (
	ErrApiKeyNotFound    = errors.New("api key not found")
	ErrApiKeyRevoked     = errors.New("api key has been revoked")
	ErrApiKeyExpired     = errors.New("api key has expired")
	ErrApiKeyAccessDenied = errors.New("access denied: api key does not belong to user")
)

// ApiKeyService определяет интерфейс для сервиса API-ключей
type ApiKeyService interface {
	// CreateKey создает новый API-ключ и возвращает сырой ключ (показывается один раз)
	CreateKey(ctx context.Context, userID uuid.UUID, name string, scopes string, expiresAt *time.Time) (*models.ApiKey, string, error)
	// ValidateKey проверяет API-ключ и возвращает пользователя-владельца
	ValidateKey(ctx context.Context, rawKey string) (*models.ApiKey, *models.User, error)
	// ListKeys возвращает все ключи пользователя
	ListKeys(ctx context.Context, userID uuid.UUID) ([]models.ApiKey, error)
	// RevokeKey отзывает ключ (только владелец или админ)
	RevokeKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error
	// DeleteKey удаляет ключ (только владелец или админ)
	DeleteKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error
}

// apiKeyService реализация ApiKeyService
type apiKeyService struct {
	apiKeyRepo repository.ApiKeyRepository
	userRepo   repository.UserRepository
}

// NewApiKeyService создает новый сервис API-ключей
func NewApiKeyService(
	apiKeyRepo repository.ApiKeyRepository,
	userRepo repository.UserRepository,
) ApiKeyService {
	return &apiKeyService{
		apiKeyRepo: apiKeyRepo,
		userRepo:   userRepo,
	}
}

// CreateKey создает новый API-ключ
func (s *apiKeyService) CreateKey(ctx context.Context, userID uuid.UUID, name string, scopes string, expiresAt *time.Time) (*models.ApiKey, string, error) {
	// Генерируем сырой ключ: wibe_ + 32 байта random hex = wibe_ + 64 hex символа
	rawKey, err := generateRawKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate api key: %w", err)
	}

	// Префикс для идентификации: первые 8 символов после "wibe_"
	keyPrefix := rawKey[:12] // "wibe_" + 7 символов

	// Хешируем ключ для хранения в БД
	keyHash := hashApiKey(rawKey)

	if scopes == "" {
		scopes = "*"
	}

	apiKey := &models.ApiKey{
		UserID:    userID,
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
	}

	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return nil, "", err
	}

	return apiKey, rawKey, nil
}

// ValidateKey проверяет API-ключ и возвращает данные ключа и пользователя
func (s *apiKeyService) ValidateKey(ctx context.Context, rawKey string) (*models.ApiKey, *models.User, error) {
	keyHash := hashApiKey(rawKey)

	apiKey, err := s.apiKeyRepo.GetByKeyHash(ctx, keyHash)
	if err != nil {
		if errors.Is(err, repository.ErrApiKeyNotFound) {
			return nil, nil, ErrApiKeyNotFound
		}
		return nil, nil, err
	}

	// Проверяем, не отозван ли ключ
	if apiKey.IsRevoked() {
		return nil, nil, ErrApiKeyRevoked
	}

	// Проверяем, не истек ли ключ
	if apiKey.IsExpired() {
		return nil, nil, ErrApiKeyExpired
	}

	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, nil, err
	}

	// Обновляем last_used_at (в фоне, не блокируем запрос)
	go func() {
		_ = s.apiKeyRepo.UpdateLastUsed(context.Background(), apiKey.ID)
	}()

	return apiKey, user, nil
}

// ListKeys возвращает все ключи пользователя
func (s *apiKeyService) ListKeys(ctx context.Context, userID uuid.UUID) ([]models.ApiKey, error) {
	return s.apiKeyRepo.ListByUserID(ctx, userID)
}

// RevokeKey отзывает ключ
func (s *apiKeyService) RevokeKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, repository.ErrApiKeyNotFound) {
			return ErrApiKeyNotFound
		}
		return err
	}

	// Проверяем доступ: только владелец или админ
	if apiKey.UserID != requestingUserID && !isAdmin {
		return ErrApiKeyAccessDenied
	}

	return s.apiKeyRepo.Revoke(ctx, keyID)
}

// DeleteKey удаляет ключ
func (s *apiKeyService) DeleteKey(ctx context.Context, keyID uuid.UUID, requestingUserID uuid.UUID, isAdmin bool) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		if errors.Is(err, repository.ErrApiKeyNotFound) {
			return ErrApiKeyNotFound
		}
		return err
	}

	// Проверяем доступ: только владелец или админ
	if apiKey.UserID != requestingUserID && !isAdmin {
		return ErrApiKeyAccessDenied
	}

	return s.apiKeyRepo.Delete(ctx, keyID)
}

// generateRawKey генерирует сырой API-ключ формата wibe_<64 hex>
func generateRawKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "wibe_" + hex.EncodeToString(bytes), nil
}

// hashApiKey создает SHA256 хеш API-ключа
func hashApiKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}
