package service

import (
	"errors"

	"github.com/devteam/backend/pkg/crypto"
)

// Encryptor — шифрование чувствительных данных (AES-256-GCM в pkg/crypto).
// AAD задаётся сервисом по ID сущности (например []byte(uuid.String())).
// Для git_credentials: при Encrypt при записи в БД использовать тот же AAD, что и при Decrypt здесь — []byte(id.String()).
type Encryptor interface {
	Encrypt(plaintext []byte, associatedData []byte) ([]byte, error)
	Decrypt(ciphertext []byte, associatedData []byte) ([]byte, error)
}

// ErrNoopDecryptBlobRequiresKey — в БД лежит blob формата AESEncryptor, а ENCRYPTION_KEY не задан (NoopEncryptor).
var ErrNoopDecryptBlobRequiresKey = errors.New("cannot decrypt application ciphertext with NoopEncryptor: set ENCRYPTION_KEY")

// NoopEncryptor не шифрует: для тестов и локальной разработки без ENCRYPTION_KEY.
type NoopEncryptor struct{}

func (NoopEncryptor) Encrypt(plaintext, _ []byte) ([]byte, error) {
	return plaintext, nil
}

func (NoopEncryptor) Decrypt(ciphertext, _ []byte) ([]byte, error) {
	// Эвристика «похоже на v1 blob» (pkg/crypto: 0x01 || nonce || sealed). In-band: теоретически
	// возможен ложный срабатывание, если plaintext ≥ MinCiphertextBlobLen и начинается с 0x01.
	if len(ciphertext) >= crypto.MinCiphertextBlobLen && ciphertext[0] == crypto.FormatVersionV1 {
		return nil, ErrNoopDecryptBlobRequiresKey
	}
	return ciphertext, nil
}
