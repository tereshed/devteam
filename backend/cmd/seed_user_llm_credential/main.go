// Sprint 15.e2e refactor: одноразовая утилита для записи зашифрованного
// per-user LLM-ключа в user_llm_credentials (без HTTP-handler'а).
//
// AAD совпадает с конвенцией UserLlmCredentialService.GetMasked/GetPlaintext —
// []byte(row.ID.String()). Сидер сам генерирует ID и шифрует под него.
//
// Использование:
//   USER_ID=<uuid> PROVIDER=deepseek API_KEY=sk-... ENCRYPTION_KEY=<hex32> \
//     go run ./cmd/seed_user_llm_credential
//
// Допустимые PROVIDER: openai, anthropic, gemini, deepseek, qwen, openrouter, zhipu.
// Идемпотентно: если для (user_id, provider) уже есть строка — обновляет её.
package main

import (
	"context"
	"encoding/hex"
	"errors"
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
	userID := uuid.MustParse(mustEnv("USER_ID"))
	provider := models.UserLLMProvider(mustEnv("PROVIDER"))
	if !models.IsValidUserLLMProvider(string(provider)) {
		log.Fatalf("PROVIDER %q is not a valid UserLLMProvider", provider)
	}
	apiKey := mustEnv("API_KEY")

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

	repo := repository.NewUserLlmCredentialRepository(db)
	ctx := context.Background()

	existing, err := repo.GetByUserAndProvider(ctx, userID, provider)
	if err != nil && !errors.Is(err, repository.ErrUserLlmCredentialNotFound) {
		log.Fatalf("lookup: %v", err)
	}

	row := &models.UserLlmCredential{
		UserID:   userID,
		Provider: provider,
	}
	if existing != nil {
		row = existing
	} else {
		row.ID = uuid.New()
	}
	enc1, err := enc.Encrypt([]byte(apiKey), []byte(row.ID.String()))
	if err != nil {
		log.Fatalf("encrypt: %v", err)
	}
	row.EncryptedKey = enc1

	if existing != nil {
		if err := repo.Update(ctx, row); err != nil {
			log.Fatalf("update: %v", err)
		}
		fmt.Println("credential_id:", row.ID, "(updated)")
	} else {
		if err := repo.Create(ctx, row); err != nil {
			log.Fatalf("create: %v", err)
		}
		fmt.Println("credential_id:", row.ID, "(created)")
	}
}
