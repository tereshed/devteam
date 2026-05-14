// Package service / Sprint 16.C-2 — реализация SecretResolver поверх БД.
//
// Сейчас умеет одно: резолвить ${secret:<llm_provider>} в API-ключ из
// user_llm_credentials по project.UserID. Этого достаточно, чтобы Hermes
// MCP-сервера, требующие LLM-ключи, работали (например MCP-обёртки над
// провайдерами — см. mcp.json schemas).
//
// Расширение: появится таблица agent_secrets с произвольными секретами по
// project.ID — добавляем второй блок поиска ниже без правки интерфейса
// SecretResolver. Это и есть смысл явного project-якоря: новые источники
// секретов прибиваются именно к нему, без расползания по сигнатурам.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/devteam/backend/internal/models"
	"gorm.io/gorm"
)

// DatabaseSecretResolver — резолв ${secret:NAME} через БД.
type DatabaseSecretResolver struct {
	db        *gorm.DB
	encryptor Encryptor
}

// NewDatabaseSecretResolver — конструктор.
// db и encryptor обязательны; при nil Resolve возвращает internal-ошибку.
func NewDatabaseSecretResolver(db *gorm.DB, encryptor Encryptor) *DatabaseSecretResolver {
	return &DatabaseSecretResolver{db: db, encryptor: encryptor}
}

// Resolve — реализация SecretResolver.
//
// Алгоритм:
//  1. Если name — это валидное имя LLM-провайдера (см. models.IsValidUserLLMProvider),
//     ищем в user_llm_credentials по (user_id=project.UserID, provider=name).
//     При успехе расшифровываем (AAD = ID записи, как в user_llm_credential_service).
//  2. Если не нашли (gorm.ErrRecordNotFound) — ErrSecretNotFound, чтобы caller
//     показал понятное сообщение пользователю и не запускал sandbox с пустым токеном.
//  3. (TODO Sprint 17+) — поиск по таблице agent_secrets по project.ID.
//
// Безопасность: возвращаем ErrSecretNotFound (sentinel), а не «»+nil, иначе
// HermesArtifactBuilder тихо положит пустую строку в HERMES_MCP_*_TOKEN — и
// MCP-сервер получит «401 Unauthorized», очень неочевидную для пользователя.
func (r *DatabaseSecretResolver) Resolve(ctx context.Context, project *models.Project, name string) (string, error) {
	if r == nil || r.db == nil || r.encryptor == nil {
		return "", fmt.Errorf("secret resolver not initialized")
	}
	if project == nil {
		return "", fmt.Errorf("project is required for secret resolution (owner anchor)")
	}
	if name == "" {
		return "", fmt.Errorf("secret name is empty")
	}

	// 1) user_llm_credentials по project.UserID + provider=name.
	if models.IsValidUserLLMProvider(name) {
		var cred models.UserLlmCredential
		err := r.db.WithContext(ctx).
			Where("user_id = ? AND provider = ?", project.UserID, name).
			First(&cred).Error
		if err == nil {
			// AAD = ID записи (то же, что и в user_llm_credential_service.GetPlaintext).
			plain, derr := r.encryptor.Decrypt(cred.EncryptedKey, []byte(cred.ID.String()))
			if derr != nil {
				return "", fmt.Errorf("decrypt secret %q: %w", name, derr)
			}
			return string(plain), nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", fmt.Errorf("db error resolving secret %q: %w", name, err)
		}
		// Запись не нашлась — падаем дальше в общий ErrSecretNotFound.
	}

	// 2) Sprint 17+: agent_secrets по project.ID — добавлять сюда же.

	return "", ErrSecretNotFound
}
