package crypto

import "errors"

var (
	// ErrInvalidCiphertext — blob слишком короткий или иначе непригоден для разбора до вызова GCM Open.
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	// ErrUnsupportedCiphertextVersion — первый байт версии не поддерживается (сейчас только 0x01).
	ErrUnsupportedCiphertextVersion = errors.New("unsupported ciphertext version")
)
