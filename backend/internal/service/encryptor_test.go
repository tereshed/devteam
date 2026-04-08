package service

import (
	"testing"

	"github.com/devteam/backend/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopEncryptor_Decrypt_PassthroughShortOrPlaintext(t *testing.T) {
	var n NoopEncryptor
	out, err := n.Decrypt([]byte("tok"), nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("tok"), out)
}

func TestNoopEncryptor_Decrypt_RejectsV1ShapedBlob(t *testing.T) {
	var n NoopEncryptor
	// Минимально длинный «похожий на v1» blob: версия 0x01 + 12 нулей nonce + 16 нулей под tag.
	blob := make([]byte, crypto.MinCiphertextBlobLen)
	blob[0] = crypto.FormatVersionV1

	_, err := n.Decrypt(blob, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoopDecryptBlobRequiresKey)
}
