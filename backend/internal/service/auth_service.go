package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wibe-flutter-gin-template/backend/internal/models"
	"github.com/wibe-flutter-gin-template/backend/internal/repository"
	"github.com/wibe-flutter-gin-template/backend/pkg/jwt"
	passwordpkg "github.com/wibe-flutter-gin-template/backend/pkg/password"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// AuthService определяет интерфейс для сервиса авторизации
type AuthService interface {
	Register(ctx context.Context, email, password string) (*models.User, error)
	Login(ctx context.Context, email, password string) (*models.User, string, string, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, string, error)
	Logout(ctx context.Context, userID uuid.UUID) error
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*models.User, error)
}

// authService реализация AuthService
type authService struct {
	userRepo         repository.UserRepository
	refreshTokenRepo repository.RefreshTokenRepository
	jwtManager       *jwt.Manager
}

// NewAuthService создает новый сервис авторизации
func NewAuthService(
	userRepo repository.UserRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	jwtManager *jwt.Manager,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		jwtManager:       jwtManager,
	}
}

// Register регистрирует нового пользователя
func (s *authService) Register(ctx context.Context, email, password string) (*models.User, error) {
	// Проверяем, существует ли пользователь
	_, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, repository.ErrUserNotFound) {
		return nil, err
	}

	// Хешируем пароль
	passwordHash, err := passwordpkg.Hash(password)
	if err != nil {
		return nil, err
	}

	// Создаем пользователя
	user := &models.User{
		Email:        email,
		PasswordHash: passwordHash,
		Role:         models.RoleUser,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrUserExists) {
			return nil, ErrUserAlreadyExists
		}
		return nil, err
	}

	return user, nil
}

// Login выполняет вход пользователя
func (s *authService) Login(ctx context.Context, email, password string) (*models.User, string, string, error) {
	// Получаем пользователя
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, "", "", ErrInvalidCredentials
		}
		return nil, "", "", err
	}

	// Проверяем пароль
	if !passwordpkg.Verify(password, user.PasswordHash) {
		return nil, "", "", ErrInvalidCredentials
	}

	// Генерируем токены
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return nil, "", "", err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken()
	if err != nil {
		return nil, "", "", err
	}

	// Сохраняем refresh токен в БД
	tokenHash := hashToken(refreshToken)
	refreshTokenModel := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(s.jwtManager.GetRefreshTokenTTL()),
	}

	if err := s.refreshTokenRepo.Create(ctx, refreshTokenModel); err != nil {
		return nil, "", "", err
	}

	return user, accessToken, refreshToken, nil
}

// RefreshToken обновляет access token используя refresh token
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	// Хешируем токен для поиска в БД
	tokenHash := hashToken(refreshToken)

	// Получаем токен из БД
	tokenModel, err := s.refreshTokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) {
			return "", "", ErrInvalidRefreshToken
		}
		return "", "", err
	}

	// Проверяем валидность токена
	if !tokenModel.IsValid() {
		return "", "", ErrInvalidRefreshToken
	}

	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, tokenModel.UserID)
	if err != nil {
		return "", "", err
	}

	// Генерируем новые токены
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, string(user.Role))
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}

	// Отзываем старый токен и создаем новый
	if err := s.refreshTokenRepo.Revoke(ctx, tokenModel.ID); err != nil {
		return "", "", err
	}

	newTokenHash := hashToken(newRefreshToken)
	newRefreshTokenModel := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: newTokenHash,
		ExpiresAt: time.Now().Add(s.jwtManager.GetRefreshTokenTTL()),
	}

	if err := s.refreshTokenRepo.Create(ctx, newRefreshTokenModel); err != nil {
		return "", "", err
	}

	return accessToken, newRefreshToken, nil
}

// Logout выполняет выход пользователя (отзывает все refresh токены)
func (s *authService) Logout(ctx context.Context, userID uuid.UUID) error {
	return s.refreshTokenRepo.RevokeAllForUser(ctx, userID)
}

// GetCurrentUser получает данные текущего пользователя
func (s *authService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// hashToken создает SHA256 хеш токена для хранения в БД
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
