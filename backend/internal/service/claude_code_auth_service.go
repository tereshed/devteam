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

type claudeCodeAuthService struct {
	repo      repository.ClaudeCodeSubscriptionRepository
	encryptor Encryptor
	oauth     ClaudeCodeOAuthProvider
	// refreshSkew — обновлять токен заранее, за этот интервал до expires_at (по умолчанию 5m).
	refreshSkew time.Duration
}

// NewClaudeCodeAuthService собирает сервис.
func NewClaudeCodeAuthService(
	repo repository.ClaudeCodeSubscriptionRepository,
	encryptor Encryptor,
	oauth ClaudeCodeOAuthProvider,
) ClaudeCodeAuthService {
	return &claudeCodeAuthService{
		repo:        repo,
		encryptor:   encryptor,
		oauth:       oauth,
		refreshSkew: 5 * time.Minute,
	}
}

func (s *claudeCodeAuthService) InitDeviceCode(ctx context.Context, userID uuid.UUID) (*ClaudeCodeDeviceInit, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("user id required")
	}
	return s.oauth.InitDeviceCode(ctx)
}

func (s *claudeCodeAuthService) CompleteDeviceCode(ctx context.Context, userID uuid.UUID, deviceCode string) (*ClaudeCodeAuthStatus, error) {
	if userID == uuid.Nil || strings.TrimSpace(deviceCode) == "" {
		return nil, fmt.Errorf("user id and device code required")
	}
	tok, err := s.oauth.PollDeviceToken(ctx, deviceCode)
	if err != nil {
		return nil, err
	}
	sub, err := s.persistToken(ctx, userID, tok)
	if err != nil {
		return nil, err
	}
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
func (s *claudeCodeAuthService) RefreshOne(ctx context.Context, sub *models.ClaudeCodeSubscription) error {
	if len(sub.OAuthRefreshTokenEnc) == 0 {
		return fmt.Errorf("subscription has no refresh token")
	}
	refresh, err := s.decrypt(sub.OAuthRefreshTokenEnc, refreshAAD(sub.UserID))
	if err != nil {
		return err
	}
	tok, err := s.oauth.RefreshToken(ctx, refresh)
	if err != nil {
		return err
	}
	if tok.RefreshToken == "" {
		// Anthropic может не вернуть refresh_token при refresh — сохраним прежний.
		tok.RefreshToken = refresh
	}
	_, err = s.persistToken(ctx, sub.UserID, tok)
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
