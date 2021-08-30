package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldGetMemory(t *testing.T) {
	m := MemUsageMiB()
	require.NotNil(t, m)
}

func Test_ShouldVerifySignature(t *testing.T) {
	body := []byte("hello there")
	secret := "mysecret"
	hash := hmac.New(sha256.New, []byte(secret))
	_, _ = hash.Write(body)
	actualHash := hex.EncodeToString(hash.Sum(nil))
	err := VerifySignature(secret, actualHash, body)
	require.NoError(t, err)
}