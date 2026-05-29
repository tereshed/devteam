package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/pkg/crypto"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load config
	os.Setenv("CONFIG_DIR", ".")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Initialize encryptor
	var encryptor service.Encryptor = service.NoopEncryptor{}
	if len(cfg.Encryption.Key) == 32 {
		aesEnc, err := crypto.NewAESEncryptor(cfg.Encryption.Key)
		if err != nil {
			log.Fatalf("aes: %v", err)
		}
		encryptor = aesEnc
	} else {
		fmt.Printf("Warning: Using NoopEncryptor because key length is %d (expected 32)\n", len(cfg.Encryption.Key))
	}

	// Connect to database
	dsn := cfg.Database.DSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	var sub models.AntigravitySubscription
	userID, _ := uuid.Parse("18fec79a-0d8d-4669-95d4-570be1157afd")
	if err := db.Where("user_id = ?", userID).First(&sub).Error; err != nil {
		log.Fatalf("get subscription: %v", err)
	}

	fmt.Printf("Subscription ID: %s\n", sub.ID)
	fmt.Printf("ExpiresAt: %v\n", sub.ExpiresAt)
	fmt.Printf("LastRefreshedAt: %v\n", sub.LastRefreshedAt)

	// Decrypt tokens
	accessAAD := []byte("antigravity_subscription:access:" + sub.UserID.String())
	refreshAAD := []byte("antigravity_subscription:refresh:" + sub.UserID.String())

	accessToken, err := encryptor.Decrypt(sub.OAuthAccessTokenEnc, accessAAD)
	if err != nil {
		log.Fatalf("decrypt access token: %v (blob len=%d, hex=%s)", err, len(sub.OAuthAccessTokenEnc), hex.EncodeToString(sub.OAuthAccessTokenEnc))
	}
	fmt.Printf("Decrypted Access Token: %s\n", string(accessToken))

	refreshToken, err := encryptor.Decrypt(sub.OAuthRefreshTokenEnc, refreshAAD)
	if err != nil {
		log.Fatalf("decrypt refresh token: %v (blob len=%d, hex=%s)", err, len(sub.OAuthRefreshTokenEnc), hex.EncodeToString(sub.OAuthRefreshTokenEnc))
	}
	fmt.Printf("Decrypted Refresh Token: %s\n", string(refreshToken))

	// Try to parse go-keyring token if prefix matches
	if strings.HasPrefix(string(accessToken), "go-keyring-base64:") {
		fmt.Println("Access token has keyring prefix!")
	}
}
