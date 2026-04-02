package password

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultCost - стандартная стоимость хеширования bcrypt
	DefaultCost = bcrypt.DefaultCost
)

// Hash создает хеш пароля
func Hash(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Verify проверяет пароль против хеша
func Verify(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
