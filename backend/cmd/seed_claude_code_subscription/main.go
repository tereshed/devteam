// Sprint 15 e2e: одноразовая утилита для записи Claude Code OAuth-подписки
// (long-lived setup-token из `claude setup-token`) в claude_code_subscriptions
// без поднятия device-flow.
//
// AAD совпадает с конвенцией ClaudeCodeAuthService.persistToken:
//   access:  "claude_code_subscription:access:<user_id>"
//   refresh: "claude_code_subscription:refresh:<user_id>"
//
// Использование:
//   USER_ID=<uuid> \
//   CLAUDE_CODE_OAUTH_ACCESS_TOKEN=sk-ant-oat01-... \
//   ENCRYPTION_KEY=<hex32> \
//     go run ./cmd/seed_claude_code_subscription
//
// Опционально: CLAUDE_CODE_OAUTH_REFRESH_TOKEN, CLAUDE_CODE_OAUTH_EXPIRES_AT (RFC3339).
// Идемпотентно: использует Upsert по user_id.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/repository"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("env %s is required", k)
	}
	return v
}

func main() {
	userID := uuid.MustParse(mustEnv("USER_ID"))
	accessToken := mustEnv("CLAUDE_CODE_OAUTH_ACCESS_TOKEN")
	refreshToken := os.Getenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN")
	expiresAtRaw := os.Getenv("CLAUDE_CODE_OAUTH_EXPIRES_AT")

	var expiresAt *time.Time
	if expiresAtRaw != "" {
		t, err := time.Parse(time.RFC3339, expiresAtRaw)
		if err != nil {
			log.Fatalf("CLAUDE_CODE_OAUTH_EXPIRES_AT must be RFC3339: %v", err)
		}
		expiresAt = &t
	}

	encHex := mustEnv("ENCRYPTION_KEY")
	key, err := hex.DecodeString(encHex)
	if err != nil || len(key) != 32 {
		log.Fatalf("ENCRYPTION_KEY must be 32 bytes hex: %v", err)
	}

	dsn := os.Getenv("DSN")
	if dsn == "" {
		dsn = "host=localhost port=5433 user=yugabyte password=yugabyte dbname=yugabyte sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	enc, err := crypto.NewAESEncryptor(key)
	if err != nil {
		log.Fatalf("encryptor: %v", err)
	}

	accessAAD := []byte("claude_code_subscription:access:" + userID.String())
	refreshAAD := []byte("claude_code_subscription:refresh:" + userID.String())

	accessEnc, err := enc.Encrypt([]byte(accessToken), accessAAD)
	if err != nil {
		log.Fatalf("encrypt access token: %v", err)
	}

	var refreshEnc []byte
	if refreshToken != "" {
		refreshEnc, err = enc.Encrypt([]byte(refreshToken), refreshAAD)
		if err != nil {
			log.Fatalf("encrypt refresh token: %v", err)
		}
	}

	now := time.Now()
	sub := &models.ClaudeCodeSubscription{
		UserID:               userID,
		OAuthAccessTokenEnc:  accessEnc,
		OAuthRefreshTokenEnc: refreshEnc,
		TokenType:            "Bearer",
		Scopes:               os.Getenv("CLAUDE_CODE_OAUTH_SCOPES"),
		ExpiresAt:            expiresAt,
		LastRefreshedAt:      &now,
	}

	repo := repository.NewClaudeCodeSubscriptionRepository(db)
	if err := repo.Upsert(context.Background(), sub); err != nil {
		log.Fatalf("upsert subscription: %v", err)
	}
	fmt.Println("subscription_id:", sub.ID, "user_id:", sub.UserID)
}
