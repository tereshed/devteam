package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/devteam/backend/internal/domain/events"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

// ProviderAntigravityOAuth — имя провайдера для IntegrationConnectionChanged события.
const ProviderAntigravityOAuth = "antigravity_oauth"

// AntigravityAuthService — управление OAuth-подпиской Antigravity на пользователя.
type AntigravityAuthService interface {
	// InitDeviceCode инициирует device-flow и возвращает данные для отображения пользователю.
	InitDeviceCode(ctx context.Context, userID uuid.UUID) (*AntigravityDeviceInit, error)
	// CompleteDeviceCode единичный poll для уже инициированного device_code.
	CompleteDeviceCode(ctx context.Context, userID uuid.UUID, deviceCode string) (*AntigravityAuthStatus, error)
	// Status — текущая подписка пользователя (без секретных полей).
	Status(ctx context.Context, userID uuid.UUID) (*AntigravityAuthStatus, error)
	// Revoke удаляет подписку и отзывает токен у провайдера.
	Revoke(ctx context.Context, userID uuid.UUID) error
	// AccessTokenForSandbox возвращает дешифрованный access-токен для проброса в sandbox.
	AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error)
	// RefreshOne — обновляет токены конкретной подписки.
	RefreshOne(ctx context.Context, sub *models.AntigravitySubscription) error
	// SaveManualToken записывает access/refresh-токены, полученные пользователем out-of-band.
	SaveManualToken(ctx context.Context, userID uuid.UUID, tok *AntigravityOAuthToken) (*AntigravityAuthStatus, error)
}

// AntigravityAuthStatus — публичный статус подписки.
type AntigravityAuthStatus struct {
	Connected       bool       `json:"connected"`
	TokenType       string     `json:"token_type,omitempty"`
	Scopes          string     `json:"scopes,omitempty"`
	ExpiresAt       *time.Time `json:"expires_at,omitempty"`
	LastRefreshedAt *time.Time `json:"last_refreshed_at,omitempty"`
}

type antigravityAuthService struct {
	repo          repository.AntigravitySubscriptionRepository
	encryptor     Encryptor
	oauth         AntigravityOAuthProvider
	refreshSkew   time.Duration
	deviceCodes   DeviceCodeStore
	deviceCodeTTL time.Duration
	refreshGroup  singleflight.Group
	bus           events.EventBus
}

// NewAntigravityAuthService собирает сервис.
func NewAntigravityAuthService(
	repo repository.AntigravitySubscriptionRepository,
	encryptor Encryptor,
	oauth AntigravityOAuthProvider,
) AntigravityAuthService {
	return &antigravityAuthService{
		repo:          repo,
		encryptor:     encryptor,
		oauth:         oauth,
		refreshSkew:   5 * time.Minute,
		deviceCodes:   NewInMemoryDeviceCodeStore(),
		deviceCodeTTL: 30 * time.Minute,
	}
}

// WithAntigravityDeviceStore позволяет подменить store.
func WithAntigravityDeviceStore(svc AntigravityAuthService, store DeviceCodeStore) AntigravityAuthService {
	if cs, ok := svc.(*antigravityAuthService); ok && store != nil {
		cs.deviceCodes = store
	}
	return svc
}

// WithAntigravityEventBus подключает EventBus для публикации IntegrationConnectionChanged.
func WithAntigravityEventBus(svc AntigravityAuthService, bus events.EventBus) AntigravityAuthService {
	if cs, ok := svc.(*antigravityAuthService); ok {
		cs.bus = bus
	}
	return svc
}

func (s *antigravityAuthService) InitDeviceCode(ctx context.Context, userID uuid.UUID) (*AntigravityDeviceInit, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("user id required")
	}
	init, err := s.oauth.InitDeviceCode(ctx)
	if err != nil {
		return nil, err
	}
	ttl := s.deviceCodeTTL
	if init.ExpiresIn > 0 && init.ExpiresIn+5*time.Minute > ttl {
		ttl = init.ExpiresIn + 5*time.Minute
	}
	s.deviceCodes.Put(init.DeviceCode, userID, ttl)
	return init, nil
}

func (s *antigravityAuthService) CompleteDeviceCode(ctx context.Context, userID uuid.UUID, deviceCode string) (*AntigravityAuthStatus, error) {
	if userID == uuid.Nil || strings.TrimSpace(deviceCode) == "" {
		return nil, fmt.Errorf("user id and device code required")
	}
	owner, ok := s.deviceCodes.Get(deviceCode)
	if !ok {
		return nil, ErrDeviceCodeOwnerMismatch
	}
	if owner != userID {
		return nil, ErrDeviceCodeOwnerMismatch
	}
	tok, err := s.oauth.PollDeviceToken(ctx, deviceCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrAuthorizationPending), errors.Is(err, ErrSlowDown):
			return nil, err
		case errors.Is(err, ErrAccessDenied):
			s.publishStatus(ctx, userID, events.IntegrationStatusError, ReasonUserCancelled, nil, nil)
		case errors.Is(err, ErrExpiredToken):
			s.publishStatus(ctx, userID, events.IntegrationStatusError, ReasonExpiredToken, nil, nil)
		case errors.Is(err, ErrOAuthInvalidGrant):
			s.publishStatus(ctx, userID, events.IntegrationStatusError, ReasonInvalidGrant, nil, nil)
		case errors.Is(err, ErrAntigravityOAuthNotConfigured):
			s.publishStatus(ctx, userID, events.IntegrationStatusError, ReasonOAuthNotConfigured, nil, nil)
		default:
			s.publishStatus(ctx, userID, events.IntegrationStatusError, ReasonProviderUnreachable, nil, nil)
		}
		return nil, err
	}
	persistCtx := context.WithoutCancel(ctx)
	sub, err := s.persistToken(persistCtx, userID, tok)
	if err != nil {
		s.publishStatus(persistCtx, userID, events.IntegrationStatusError, ReasonInternalError, nil, nil)
		return nil, err
	}
	s.deviceCodes.Delete(deviceCode)
	now := time.Now().UTC()
	s.publishStatus(persistCtx, userID, events.IntegrationStatusConnected, "", &now, sub.ExpiresAt)
	return toAntigravityStatus(sub), nil
}

type keyringTokenJSON struct {
	Token *struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		Expiry       string `json:"expiry"`
	} `json:"token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Expiry       string `json:"expiry"`
}

func tryDecodeKeyringToken(tokenStr string) (*AntigravityOAuthToken, error) {
	if !strings.HasPrefix(tokenStr, "go-keyring-base64:") {
		return nil, nil
	}
	encoded := strings.TrimPrefix(tokenStr, "go-keyring-base64:")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode base64 keyring token: %w", err)
	}

	var parsed keyringTokenJSON
	if err := json.Unmarshal(decoded, &parsed); err != nil {
		return nil, fmt.Errorf("parse keyring token JSON: %w", err)
	}

	res := &AntigravityOAuthToken{}
	if parsed.Token != nil {
		res.AccessToken = parsed.Token.AccessToken
		res.RefreshToken = parsed.Token.RefreshToken
		res.TokenType = parsed.Token.TokenType
		if parsed.Token.Expiry != "" {
			if t, err := time.Parse(time.RFC3339, parsed.Token.Expiry); err == nil {
				res.ExpiresAt = &t
			} else if t, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", parsed.Token.Expiry); err == nil {
				res.ExpiresAt = &t
			}
		}
	} else {
		res.AccessToken = parsed.AccessToken
		res.RefreshToken = parsed.RefreshToken
		res.TokenType = parsed.TokenType
		if parsed.Expiry != "" {
			if t, err := time.Parse(time.RFC3339, parsed.Expiry); err == nil {
				res.ExpiresAt = &t
			} else if t, err := time.Parse("2006-01-02T15:04:05.999999999Z07:00", parsed.Expiry); err == nil {
				res.ExpiresAt = &t
			}
		}
	}

	if res.AccessToken == "" {
		return nil, fmt.Errorf("no access_token found in keyring token")
	}
	return res, nil
}

func (s *antigravityAuthService) SaveManualToken(ctx context.Context, userID uuid.UUID, tok *AntigravityOAuthToken) (*AntigravityAuthStatus, error) {
	if tok == nil || tok.AccessToken == "" {
		return nil, fmt.Errorf("manual token: access_token is required")
	}
	
	if decoded, err := tryDecodeKeyringToken(tok.AccessToken); err == nil && decoded != nil {
		tok.AccessToken = decoded.AccessToken
		if decoded.RefreshToken != "" {
			tok.RefreshToken = decoded.RefreshToken
		}
		if decoded.TokenType != "" {
			tok.TokenType = decoded.TokenType
		}
		if decoded.Scopes != "" {
			tok.Scopes = decoded.Scopes
		}
		if decoded.ExpiresAt != nil {
			tok.ExpiresAt = decoded.ExpiresAt
		}
	} else if err != nil {
		return nil, err
	}

	persistCtx := context.WithoutCancel(ctx)
	sub, err := s.persistToken(persistCtx, userID, tok)
	if err != nil {
		s.publishStatus(persistCtx, userID, events.IntegrationStatusError, ReasonInternalError, nil, nil)
		return nil, err
	}
	now := time.Now().UTC()
	s.publishStatus(persistCtx, userID, events.IntegrationStatusConnected, "", &now, sub.ExpiresAt)
	return toAntigravityStatus(sub), nil
}

func (s *antigravityAuthService) Status(ctx context.Context, userID uuid.UUID) (*AntigravityAuthStatus, error) {
	sub, err := s.repo.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrAntigravitySubscriptionNotFound) {
		return &AntigravityAuthStatus{Connected: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return toAntigravityStatus(sub), nil
}

func (s *antigravityAuthService) Revoke(ctx context.Context, userID uuid.UUID) error {
	sub, err := s.repo.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrAntigravitySubscriptionNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if token, decErr := s.decrypt(sub.OAuthAccessTokenEnc, antigravityAccessAAD(sub.UserID)); decErr == nil && token != "" {
		_ = s.oauth.Revoke(ctx, token)
	}
	if err := s.repo.DeleteByUserID(ctx, userID); err != nil {
		return err
	}
	s.publishStatus(ctx, userID, events.IntegrationStatusDisconnected, "", nil, nil)
	return nil
}

func (s *antigravityAuthService) publishStatus(
	ctx context.Context,
	userID uuid.UUID,
	status events.IntegrationConnectionStatus,
	reason string,
	connectedAt, expiresAt *time.Time,
) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(ctx, events.IntegrationConnectionChanged{
		UserID:      userID,
		Provider:    ProviderAntigravityOAuth,
		Status:      status,
		Reason:      reason,
		ConnectedAt: connectedAt,
		ExpiresAt:   expiresAt,
		OccurredAt:  time.Now().UTC(),
	})
}

func (s *antigravityAuthService) AccessTokenForSandbox(ctx context.Context, userID uuid.UUID) (string, error) {
	sub, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if sub.IsExpired(time.Now(), s.refreshSkew) {
		if err := s.RefreshOne(ctx, sub); err != nil {
			return "", fmt.Errorf("refresh expired token: %w", err)
		}
		updated, err := s.repo.GetByUserID(ctx, userID)
		if err != nil {
			return "", err
		}
		sub = updated
	}
	return s.decrypt(sub.OAuthAccessTokenEnc, antigravityAccessAAD(sub.UserID))
}

func (s *antigravityAuthService) RefreshOne(ctx context.Context, sub *models.AntigravitySubscription) error {
	if sub == nil || sub.UserID == uuid.Nil {
		return fmt.Errorf("subscription with user id required")
	}
	if len(sub.OAuthRefreshTokenEnc) == 0 {
		return fmt.Errorf("subscription has no refresh token")
	}
	bgCtx := context.WithoutCancel(ctx)
	_, err, _ := s.refreshGroup.Do(sub.UserID.String(), func() (any, error) {
		current, err := s.repo.GetByUserID(bgCtx, sub.UserID)
		if err != nil {
			return nil, err
		}
		if current.LastRefreshedAt != nil && sub.LastRefreshedAt != nil &&
			current.LastRefreshedAt.After(*sub.LastRefreshedAt) {
			return nil, nil
		}
		refresh, err := s.decrypt(current.OAuthRefreshTokenEnc, antigravityRefreshAAD(current.UserID))
		if err != nil {
			return nil, err
		}
		tok, err := s.oauth.RefreshToken(bgCtx, refresh)
		if err != nil {
			return nil, err
		}
		if tok.RefreshToken == "" {
			tok.RefreshToken = refresh
		}
		if _, err := s.persistToken(bgCtx, current.UserID, tok); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

func (s *antigravityAuthService) persistToken(ctx context.Context, userID uuid.UUID, tok *AntigravityOAuthToken) (*models.AntigravitySubscription, error) {
	accessEnc, err := s.encryptor.Encrypt([]byte(tok.AccessToken), antigravityAccessAAD(userID))
	if err != nil {
		return nil, fmt.Errorf("encrypt access token: %w", err)
	}
	var refreshEnc []byte
	if tok.RefreshToken != "" {
		refreshEnc, err = s.encryptor.Encrypt([]byte(tok.RefreshToken), antigravityRefreshAAD(userID))
		if err != nil {
			return nil, fmt.Errorf("encrypt refresh token: %w", err)
		}
	}
	now := time.Now()
	sub := &models.AntigravitySubscription{
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

func (s *antigravityAuthService) decrypt(blob, aad []byte) (string, error) {
	if len(blob) == 0 {
		return "", nil
	}
	plain, err := s.encryptor.Decrypt(blob, aad)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func toAntigravityStatus(sub *models.AntigravitySubscription) *AntigravityAuthStatus {
	if sub == nil {
		return &AntigravityAuthStatus{Connected: false}
	}
	return &AntigravityAuthStatus{
		Connected:       true,
		TokenType:       sub.TokenType,
		Scopes:          sub.Scopes,
		ExpiresAt:       sub.ExpiresAt,
		LastRefreshedAt: sub.LastRefreshedAt,
	}
}

func antigravityAccessAAD(userID uuid.UUID) []byte {
	return []byte("antigravity_subscription:access:" + userID.String())
}

func antigravityRefreshAAD(userID uuid.UUID) []byte {
	return []byte("antigravity_subscription:refresh:" + userID.String())
}
