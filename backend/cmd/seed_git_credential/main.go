// Одноразовая утилита для создания зашифрованного git_credential без HTTP-handler'а.
// Запуск: PAT=ghp_xxx USER_ID=<uuid> PROJECT_ID=<uuid> go run ./cmd/seed_git_credential
// Шифрует токен AES-256-GCM (ENCRYPTION_KEY из env, тот же, что использует API),
// вставляет запись в git_credentials и обновляет projects.git_credential_id + git_provider.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"

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
	pat := mustEnv("PAT")
	userID := uuid.MustParse(mustEnv("USER_ID"))
	projectID := uuid.MustParse(mustEnv("PROJECT_ID"))
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

	cred := &models.GitCredential{
		ID:       uuid.New(),
		UserID:   userID,
		Provider: models.GitCredentialProviderGitHub,
		AuthType: models.GitCredentialAuthToken,
		Label:    "kt-test-repo PAT",
	}
	enc1, err := enc.Encrypt([]byte(pat), []byte(cred.ID.String()))
	if err != nil {
		log.Fatalf("encrypt: %v", err)
	}
	cred.EncryptedValue = enc1

	repo := repository.NewGitCredentialRepository(db)
	if err := repo.Create(context.Background(), cred); err != nil {
		log.Fatalf("create credential: %v", err)
	}
	fmt.Println("credential_id:", cred.ID)

	// Привязываем к проекту + переключаем провайдер на github
	if err := db.Exec(`UPDATE projects SET git_credential_id = ?, git_provider = 'github' WHERE id = ?`, cred.ID, projectID).Error; err != nil {
		log.Fatalf("attach to project: %v", err)
	}
	fmt.Println("attached to project:", projectID)
}
