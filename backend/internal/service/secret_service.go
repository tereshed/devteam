package service

import (
	"fmt"
	"regexp"

	"github.com/devteam/backend/pkg/crypto"
	"github.com/google/uuid"
)

var placeholderRe = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`)

// SecretService — shared encryption/decryption logic for all secret types
// (project_secrets, user_secrets, agent_secrets). Uses pkg/crypto.AESEncryptor
// with AAD = record ID.
type SecretService struct {
	encryptor Encryptor
}

func NewSecretService(encryptor Encryptor) *SecretService {
	return &SecretService{encryptor: encryptor}
}

func (s *SecretService) Encrypt(id uuid.UUID, plaintext string) ([]byte, error) {
	blob, err := s.encryptor.Encrypt([]byte(plaintext), []byte(id.String()))
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	if len(blob) < crypto.MinCiphertextBlobLen {
		return nil, fmt.Errorf("%w: encryptor produced unexpectedly small blob", ErrEncryptorNotConfigured)
	}
	return blob, nil
}

func (s *SecretService) Decrypt(id uuid.UUID, ciphertext []byte) (string, error) {
	plain, err := s.encryptor.Decrypt(ciphertext, []byte(id.String()))
	if err != nil {
		return "", fmt.Errorf("decrypt secret %s: %w", id, err)
	}
	return string(plain), nil
}

// ResolveEnvPlaceholders replaces ${VAR_NAME} placeholders with values from secrets map.
// Returns resolved map and list of unresolved placeholder names.
func ResolveEnvPlaceholders(raw map[string]string, secrets map[string]string) (map[string]string, []string) {
	var missing []string
	result := make(map[string]string, len(raw))
	for k, v := range raw {
		result[k] = placeholderRe.ReplaceAllStringFunc(v, func(ph string) string {
			name := ph[2 : len(ph)-1]
			if val, ok := secrets[name]; ok {
				return val
			}
			missing = append(missing, name)
			return ph
		})
	}
	return result, missing
}
