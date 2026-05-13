// Sprint 15 e2e: одноразовая утилита для регистрации LLM-провайдера через сервис
// (валидация + AES-256-GCM шифрование credentials c тем же AAD, что у API).
//
// Использование:
//
//	NAME=e2e-deepseek-upstream KIND=deepseek BASE_URL=https://api.deepseek.com \
//	AUTH_TYPE=api_key API_KEY=sk-... DEFAULT_MODEL=deepseek-chat \
//	ENCRYPTION_KEY=<hex> \
//	  go run ./cmd/seed_llm_provider
//
// Идемпотентно: если провайдер с таким NAME уже есть — печатает его id и выходит без ошибки.
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
	"github.com/devteam/backend/internal/service"
	"github.com/devteam/backend/pkg/crypto"
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
	name := mustEnv("NAME")
	kind := models.LLMProviderKind(mustEnv("KIND"))
	if !kind.IsValid() {
		log.Fatalf("KIND %q is not a valid LLMProviderKind", kind)
	}
	authType := models.LLMProviderAuthType(mustEnv("AUTH_TYPE"))
	if !authType.IsValid() {
		log.Fatalf("AUTH_TYPE %q is not a valid LLMProviderAuthType", authType)
	}

	baseURL := os.Getenv("BASE_URL")
	defaultModel := os.Getenv("DEFAULT_MODEL")
	apiKey := os.Getenv("API_KEY")
	if authType != models.LLMProviderAuthNone && apiKey == "" {
		log.Fatalf("API_KEY is required when AUTH_TYPE=%s", authType)
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
	repo := repository.NewLLMProviderRepository(db)
	svc := service.NewLLMProviderService(repo, enc)

	ctx := context.Background()

	if existing, err := repo.GetByName(ctx, name); err == nil && existing != nil {
		fmt.Println("provider_id:", existing.ID, "(already exists)")
		return
	} else if err != nil && !errors.Is(err, repository.ErrLLMProviderNotFound) {
		log.Fatalf("lookup by name: %v", err)
	}

	p, err := svc.Create(ctx, service.LLMProviderInput{
		Name:         name,
		Kind:         kind,
		BaseURL:      baseURL,
		AuthType:     authType,
		Credential:   apiKey,
		DefaultModel: defaultModel,
		Enabled:      true,
	})
	if err != nil {
		log.Fatalf("create provider: %v", err)
	}
	fmt.Println("provider_id:", p.ID)
}
