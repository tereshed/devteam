package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devteam/backend/internal/config"
	"github.com/devteam/backend/internal/models"
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/crypto"
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

	refreshAAD := []byte("antigravity_subscription:refresh:" + sub.UserID.String())
	refreshTokenBytes, err := encryptor.Decrypt(sub.OAuthRefreshTokenEnc, refreshAAD)
	if err != nil {
		log.Fatalf("decrypt refresh token: %v", err)
	}
	refreshToken := string(refreshTokenBytes)

	// Create oauth provider
	prov := service.NewAntigravityOAuthProvider(service.AntigravityOAuthConfig{
		ClientID:      cfg.AntigravityOAuth.ClientID,
		ClientSecret:  cfg.AntigravityOAuth.ClientSecret,
		DeviceCodeURL: cfg.AntigravityOAuth.DeviceCodeURL,
		TokenURL:      cfg.AntigravityOAuth.TokenURL,
		RevokeURL:     cfg.AntigravityOAuth.RevokeURL,
		Scopes:        cfg.AntigravityOAuth.Scopes,
	})

	fmt.Printf("ClientID: %s\n", cfg.AntigravityOAuth.ClientID)
	fmt.Printf("ClientSecret: %s\n", cfg.AntigravityOAuth.ClientSecret)
	fmt.Printf("TokenURL: %s\n", cfg.AntigravityOAuth.TokenURL)

	tok, err := prov.RefreshToken(context.Background(), refreshToken)
	if err != nil {
		log.Fatalf("RefreshToken failed: %v", err)
	}

	fmt.Printf("Refresh success! AccessToken: %s\n", tok.AccessToken)
}
