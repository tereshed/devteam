package service

// Decryptor расшифровывает данные, зашифрованные AES-256-GCM.
// Реализация из pkg/crypto (задача 4.7) подключается в cmd/api через DI.
type Decryptor interface {
	Decrypt(ciphertext []byte) ([]byte, error)
}

// NoopDecryptor возвращает ciphertext как есть (тесты и этап до реального шифрования).
type NoopDecryptor struct{}

func (NoopDecryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	return ciphertext, nil
}
