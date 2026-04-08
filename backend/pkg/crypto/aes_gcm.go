package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

const (
	// FormatVersionV1 — единственная поддерживаемая версия blob (первый байт).
	FormatVersionV1 = 0x01
	nonceSize       = 12
	minBlobLen      = 1 + nonceSize + 16 // version + nonce + минимум GCM tag
	// MinCiphertextBlobLen — минимальная длина blob (версия + nonce + GCM tag); для эвристик вне пакета.
	MinCiphertextBlobLen = minBlobLen
)

// AESEncryptor — AES-256-GCM, потокобезопасен (cipher.AEAD).
type AESEncryptor struct {
	aead cipher.AEAD
}

// NewAESEncryptor создаёт шифратор; key должен быть ровно 32 байта (AES-256).
func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return &AESEncryptor{aead: gcm}, nil
}

// Encrypt возвращает blob: 0x01 || nonce(12) || Seal(plaintext, AAD).
// Одна аллокация: заголовок (версия+nonce) с ёмкостью под ciphertext+tag, Seal дописывает в хвост.
func (e *AESEncryptor) Encrypt(plaintext, associatedData []byte) ([]byte, error) {
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	headerLen := 1 + nonceSize
	out := make([]byte, headerLen, headerLen+len(plaintext)+e.aead.Overhead())
	out[0] = FormatVersionV1
	copy(out[1:headerLen], nonce)
	return e.aead.Seal(out, nonce, plaintext, associatedData), nil
}

// Decrypt разбирает blob той же структуры; AAD должен совпадать с использованным при Encrypt.
func (e *AESEncryptor) Decrypt(blob, associatedData []byte) ([]byte, error) {
	if len(blob) < minBlobLen {
		return nil, ErrInvalidCiphertext
	}
	if blob[0] != FormatVersionV1 {
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedCiphertextVersion, blob[0])
	}
	nonce := blob[1 : 1+nonceSize]
	sealed := blob[1+nonceSize:]
	plain, err := e.aead.Open(nil, nonce, sealed, associatedData)
	if err != nil {
		return nil, err
	}
	return plain, nil
}
