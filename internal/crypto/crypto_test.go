package crypto

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_EncryptDecrypt(t *testing.T) {
	password := "mypass"
	key, _, err := DeriveKey([]byte(password), nil)
	require.NoError(t, err)
	data := []byte("random bits of data")

	ciphertext, err := Encrypt(key, data)
	require.NoError(t, err)

	plaintext, err := Decrypt(key, ciphertext)
	require.NoError(t, err)
	require.Equal(t, string(plaintext), string(data))
}