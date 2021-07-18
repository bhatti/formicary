package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"golang.org/x/crypto/scrypt"
)

// GenerateKey creates random key with size such as 32
func GenerateKey(size int) ([]byte, error) {
	key := make([]byte, size)

	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// DeriveKey derives key using password and salt
func DeriveKey(password, salt []byte) (key []byte, newSalt []byte, err error) {
	if salt == nil {
		if salt, err = GenerateKey(32); err != nil {
			return nil, nil, err
		}
	}
	newSalt = salt
	key, err = scrypt.Key(password, salt, 1048576, 8, 1, 32)
	return
}

// SHA256Key key
func SHA256Key(password string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return hasher.Sum(nil)[0:32]
}

// SHA256 hash
func SHA256(key string) string {
	hasher := sha256.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Encrypt data
func Encrypt(key, data []byte) ([]byte, error) {
	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

// Decrypt data
func Decrypt(key, data []byte) ([]byte, error) {
	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
