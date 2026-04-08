package crypto

import (
	"crypto/rand"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey32(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return k
}

func TestNewAESEncryptor_InvalidKeyLen(t *testing.T) {
	_, err := NewAESEncryptor([]byte("short"))
	require.Error(t, err)
}

func TestAESEncryptor_RoundTrip(t *testing.T) {
	e, err := NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	aad := []byte("550e8400-e29b-41d4-a716-446655440000")
	plain := []byte("hello credential")

	blob, err := e.Encrypt(plain, aad)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(blob), minBlobLen)

	got, err := e.Decrypt(blob, aad)
	require.NoError(t, err)
	assert.Equal(t, plain, got)
}

func TestAESEncryptor_AADMismatch(t *testing.T) {
	e, err := NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	aadEnc := []byte("11111111-1111-1111-1111-111111111111")
	aadDec := []byte("22222222-2222-2222-2222-222222222222")

	blob, err := e.Encrypt([]byte("secret"), aadEnc)
	require.NoError(t, err)

	_, err = e.Decrypt(blob, aadDec)
	require.Error(t, err)
}

func TestAESEncryptor_UnsupportedVersion(t *testing.T) {
	e, err := NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	aad := []byte("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")

	blob, err := e.Encrypt([]byte("x"), aad)
	require.NoError(t, err)
	blob[0] = 0x02

	_, err = e.Decrypt(blob, aad)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedCiphertextVersion))
}

func TestAESEncryptor_CorruptSealed(t *testing.T) {
	e, err := NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	aad := []byte("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	blob, err := e.Encrypt([]byte("data"), aad)
	require.NoError(t, err)
	if len(blob) > 14 {
		blob[len(blob)-1] ^= 0xff
	}

	_, err = e.Decrypt(blob, aad)
	require.Error(t, err)
}

func TestAESEncryptor_ShortBlob(t *testing.T) {
	e, err := NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	aad := []byte("cccccccc-cccc-cccc-cccc-cccccccccccc")

	for _, blob := range [][]byte{nil, {}, {0x01}, make([]byte, 28)} {
		_, err := e.Decrypt(blob, aad)
		require.ErrorIs(t, err, ErrInvalidCiphertext)
	}
}

func TestAESEncryptor_EmptyPlaintext(t *testing.T) {
	e, err := NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	aad := []byte("dddddddd-dddd-dddd-dddd-dddddddddddd")

	blob, err := e.Encrypt(nil, aad)
	require.NoError(t, err)
	got, err := e.Decrypt(blob, aad)
	require.NoError(t, err)
	assert.Len(t, got, 0)
}

func TestAESEncryptor_Concurrent(t *testing.T) {
	e, err := NewAESEncryptor(testKey32(t))
	require.NoError(t, err)
	aad := []byte("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			blob, encErr := e.Encrypt([]byte("payload"), aad)
			require.NoError(t, encErr)
			got, decErr := e.Decrypt(blob, aad)
			require.NoError(t, decErr)
			assert.Equal(t, []byte("payload"), got)
		}()
	}
	wg.Wait()
}
