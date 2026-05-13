package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

// ClaudeCodeAuthService — управление OAuth-подпиской Claude Code на пользователя (Sprint 15.12).
type ClaudeCodeAuthService interface {
	// InitDeviceCode инициирует device-flow и возвращает данные для отображения пользователю.
	InitDeviceCode(ctx context.Context, userID uuid.UUID) (*ClaudeCodeDeviceInit, error)
	// CompleteDeviceCode единичный poll для уже инициированного device_code.
	// Возвращает текущий статус подписки. ErrAuthorizationPending означает «ещё не подтверждено».
	CompleteDeviceCode(ctx context.Context, userID uuid.UUID, deviceCode string) (*ClaudeCodeAuthStatus, error)
	// Status — текущая подписка пользователя (без секретных полей).
	Status(ctx context.Context, userID uuid.UUID) (*ClaudeCodeAuthStatus, error)
	// Revoke удаляет подписку и (best-effort) отзывает токен у провайдера.
	Revoke(ctx context.Context, userID uuid.UUID) error
	// AccessTokenForSandbox возвращает дешифрованный access-токен для проброса в sandbox (15.14).
	// Если токен истёк и есть refresh_token — пытается обновить.
	AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error)
	// RefreshOne — обновляет токены конкретной подписки (используется воркером 15.13).
	RefreshOne(ctx context.Context, sub *models.ClaudeCodeSubscription) error
}

// ClaudeCodeAuthStatus — публичный статус подписки.
type ClaudeCodeAuthStatus struct {
	Connected       bool       `json:"connected"`
	TokenType       string     `json:"token_type,omitempty"`
	Scopes          string     `json:"scopes,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	LastRefreshedAt *time.Time `json:"last_refreshed_at,omitempty"`
}

// ErrDeviceCodeOwnerMismatch — попытка poll'а device_code, который был инициирован другим пользователем.
// Sprint 15.B security fix (B2): защищает от device-code phishing (RFC 8628).
var ErrDeviceCodeOwnerMismatch = errors.New("device_code does not belong to this user")

type claudeCodeAuthService struct {
	repo      repository.ClaudeCodeSubscriptionRepository
	encryptor Encryptor
	oauth     ClaudeCodeOAuthProvider
	// refreshSkew — обновлять токен заранее, за этот интервал до expires_at (по умолчанию 5m).
	refreshSkew time.Duration
	// deviceCodes — эфемерная привязка device_code → user_id (TTL ≥ expires_in устройства).
	deviceCodes DeviceCodeStore
	// deviceCodeTTL — TTL записи; должен быть ≥ expires_in от провайдера + запас.
	deviceCodeTTL time.Duration
	// refreshGroup — Sprint 15.B (B3): coalesces concurrent RefreshOne для одного user_id.
	// Anthropic ротейтит refresh_token при первом use; параллельный refresh из воркера и
	// AccessTokenForSandbox без coalescing давал invalid_grant для второго звонящего.
	refreshGroup singleflight.Group
}

// NewClaudeCodeAuthService собирает сервис.
func NewClaudeCodeAuthService(
	repo repository.ClaudeCodeSubscriptionRepository,
	encryptor Encryptor,
	oauth ClaudeCodeOAuthProvider,
) ClaudeCodeAuthService {
	return &claudeCodeAuthService{
		repo:          repo,
		encryptor:     encryptor,
		oauth:         oauth,
		refreshSkew:   5 * time.Minute,
		deviceCodes:   NewInMemoryDeviceCodeStore(),
		deviceCodeTTL: 30 * time.Minute,
	}
}

// WithDeviceCodeStore позволяет подменить store (для тестов/Redis-бэкенда).
// Возвращает интерфейсное значение, чтобы не ломать DI.
func WithClaudeCodeDeviceStore(svc ClaudeCodeAuthService, store DeviceCodeStore) ClaudeCodeAuthService {
	if cs, ok := svc.(*claudeCodeAuthService); ok && store != nil {
		cs.deviceCodes = store
	}
	return svc
}

func (s *claudeCodeAuthService) InitDeviceCode(ctx context.Context, userID uuid.UUID) (*ClaudeCodeDeviceInit, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("user id required")
	}
	init, err := s.oauth.InitDeviceCode(ctx)
	if err != nil {
		return nil, err
	}
	// Sprint 15.B (B2): запоминаем, что именно этот user_id инициировал device_code.
	// Без этой записи /callback от другого пользователя пройдёт успешно и привяжет чужую подписку.
	ttl := s.deviceCodeTTL
	if init.ExpiresIn > 0 && init.ExpiresIn+5*time.Minute > ttl {
		ttl = init.ExpiresIn + 5*time.Minute
	}
	s.deviceCodes.Put(init.DeviceCode, userID, ttl)
	return init, nil
}

func (s *claudeCodeAuthService) CompleteDeviceCode(ctx context.Context, userID uuid.UUID, deviceCode string) (*ClaudeCodeAuthStatus, error) {
	if userID == uuid.Nil || strings.TrimSpace(deviceCode) == "" {
		return nil, fmt.Errorf("user id and device code required")
	}
	// Sprint 15.B (B2): device_code должен принадлежать тому же user_id, что инициировал flow.
	owner, ok := s.deviceCodes.Get(deviceCode)
	if !ok {
		return nil, ErrDeviceCodeOwnerMismatch
	}
	if owner != userID {
		return nil, ErrDeviceCodeOwnerMismatch
	}
	tok, err := s.oauth.PollDeviceToken(ctx, deviceCode)
	if err != nil {
		return nil, err
	}
	sub, err := s.persistToken(ctx, userID, tok)
	if err != nil {
		return nil, err
	}
	// Поток успешно завершён — освобождаем device_code, чтобы повторный POST вернул mismatch (не reuse).
	s.deviceCodes.Delete(deviceCode)
	return toStatus(sub), nil
}

func (s *claudeCodeAuthService) Status(ctx context.Context, userID uuid.UUID) (*ClaudeCodeAuthStatus, error) {
	sub, err := s.repo.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrClaudeCodeSubscriptionNotFound) {
		return &ClaudeCodeAuthStatus{Connected: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return toStatus(sub), nil
}

func (s *claudeCodeAuthService) Revoke(ctx context.Context, userID uuid.UUID) error {
	sub, err := s.repo.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrClaudeCodeSubscriptionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if token, decErr := s.decrypt(sub.OAuthAccessTokenEnc, accessAAD(sub.UserID)); decErr == nil && token != "" {
		_ = s.oauth.Revoke(ctx, token) // best-effort
	}
	return s.repo.DeleteByUserID(ctx, userID)
}

func (s *claudeCodeAuthService) AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error) {
	sub, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if sub.IsExpired(time.Now(), s.refreshSkew) {
		if err := s.RefreshOne(ctx, sub); err != nil {
			return "", fmt.Errorf("refresh expired token: %w", err)
		}
		// Перечитываем подписку: RefreshOne сохраняет обновлённый blob через Upsert,
		// но локальный sub остаётся со старым access_token.
		updated, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			return "", err
		}
		sub = updated
	}
	return s.decrypt(sub.OAuthAccessTokenEnc, accessAAD(sub.UserID))
}

// RefreshOne обновляет токены конкретной подписки (вызывается воркером 15.13 и из AccessTokenForSandbox).
//
// Sprint 15.B (B3): параллельные вызовы для одного user_id коалесцируются через singleflight —
// второй (и далее) caller ждут результат первого вместо отдельного RefreshToken().
// Это критично, потому что Anthropic ротейтит refresh_token: первый успешный обмен
// инвалидирует исходный refresh, и второй "наивный" вызов получает invalid_grant.
func (s *claudeCodeAuthService) RefreshOne(ctx context.Context, sub *models.ClaudeCodeSubscription) error {
	if sub == nil || sub.UserID == uuid.Nil {
		return fmt.Errorf("subscription with user id required")
	}
	if len(sub.OAuthRefreshTokenEnc) == 0 {
		return fmt.Errorf("subscription has no refresh token")
	}
	// Sprint 15.M3: внутри singleflight используем context.WithoutCancel(ctx).
	// Если HTTP-клиент первого caller'а отменится после успешного refresh у Anthropic,
	// без WithoutCancel persistToken упадёт на context.Canceled и следующий refresh
	// получит invalid_grant (refresh уже ротейтнут). WithoutCancel сохраняет deadline parent'а,
	// но игнорирует cancel — финальный INSERT в БД отрабатывает до конца.
	bgCtx := context.WithoutCancel(ctx)
	_, err, _ := s.refreshGroup.Do(sub.UserID.String(), func() (any, error) {
		// Перечитываем актуальную запись из репо (другой caller мог уже обновить — тогда выйти).
		current, err := s.repo.GetByUserID(bgCtx, sub.UserID)
		if err != nil {
			return nil, err
		}
		if current.LastRefreshedAt != nil && sub.LastRefreshedAt != nil &&
			current.LastRefreshedAt.After(*sub.LastRefreshedAt) {
			// Между вызовами кто-то уже обновил подписку — считаем работу выполненной.
			return nil, nil
		}
		refresh, err := s.decrypt(current.OAuthRefreshTokenEnc, refreshAAD(current.UserID))
		if err != nil {
			return nil, err
		}
		tok, err := s.oauth.RefreshToken(bgCtx, refresh)
		if err != nil {
			return nil, err
		}
		if tok.RefreshToken == "" {
			// Anthropic может не вернуть refresh_token при refresh — сохраним прежний.
			tok.RefreshToken = refresh
		}
		if _, err := s.persistToken(bgCtx, current.UserID, tok); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

func (s *claudeCodeAuthService) persistToken(ctx context.Context, userID uuid.UUID, tok *ClaudeCodeOAuthToken) (*models.ClaudeCodeSubscription, error) {
	accessEnc, err := s.encryptor.Encrypt([]byte(tok.AccessToken), accessAAD(userID))
	if err != nil {
		return nil, fmt.Errorf("encrypt access token: %w", err)
	}
	var refreshEnc []byte
	if tok.RefreshToken != "" {
		refreshEnc, err = s.encryptor.Encrypt([]byte(tok.RefreshToken), refreshAAD(userID))
		if err != nil {
			return nil, fmt.Errorf("encrypt refresh token: %w", err)
		}
	}
	now := time.Now()
	sub := &models.ClaudeCodeSubscription{
		UserID:               userID,
		OAuthAccessTokenEnc:  accessEnc,
		OAuthRefreshTokenEnc: refreshEnc,
		TokenType:            firstNonEmpty(tok.TokenType, "Bearer"),
		Scopes:               tok.Scopes,
		ExpiresAt:            tok.ExpiresAt,
		LastRefreshedAt:      &now,
	}
	if err := s.repo.Upsert(ctx, sub); err != nil {
		return nil, err
	}
	return sub, nil
}

func (s *claudeCodeAuthService) decrypt(blob, aad []byte) (string, error) {
	if len(blob) == 0 {
		return "", nil
	}
	plain, err := s.encryptor.Decrypt(blob, aad)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func toStatus(sub *models.ClaudeCodeSubscription) *ClaudeCodeAuthStatus {
	if sub == nil {
		return &ClaudeCodeAuthStatus{Connected: false}
	}
	return &ClaudeCodeAuthStatus{
		Connected:       true,
		TokenType:       sub.TokenType,
		Scopes:          sub.Scopes,
		ExpiresAt:       sub.ExpiresAt,
		LastRefreshedAt: sub.LastRefreshedAt,
	}
}

func accessAAD(userID uuid.UUID) []byte {
	return []byte("claude_code_subscription:access:" + userID.String())
}

func refreshAAD(userID uuid.UUID) []byte {
	return []byte("claude_code_subscription:refresh:" + userID.String())
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
