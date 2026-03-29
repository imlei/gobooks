package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

const aiSecretPrefix = "enc:v1:"

var (
	errAISecretKeyNotConfigured = errors.New("AI secret encryption key is not configured")
	aiSecretKeyMu               sync.RWMutex
	aiSecretKey                 []byte
)

// ConfigureAISecretKey loads the base64-encoded 32-byte key used for AI secret encryption.
func ConfigureAISecretKey(encoded string) error {
	encoded = strings.TrimSpace(encoded)

	aiSecretKeyMu.Lock()
	defer aiSecretKeyMu.Unlock()

	if encoded == "" {
		aiSecretKey = nil
		return nil
	}

	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("decode AI secret key: %w", err)
	}
	if len(key) != 32 {
		return fmt.Errorf("AI secret key must decode to exactly 32 bytes")
	}

	aiSecretKey = append([]byte(nil), key...)
	return nil
}

func encryptAISecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}

	key, err := currentAISecretKey()
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return aiSecretPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptAISecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}
	if !strings.HasPrefix(secret, aiSecretPrefix) {
		return secret, nil
	}

	key, err := currentAISecretKey()
	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, aiSecretPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted AI secret: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("encrypted AI secret is malformed")
	}

	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt AI secret: %w", err)
	}
	return string(plaintext), nil
}

func currentAISecretKey() ([]byte, error) {
	aiSecretKeyMu.RLock()
	defer aiSecretKeyMu.RUnlock()

	if len(aiSecretKey) == 0 {
		return nil, errAISecretKeyNotConfigured
	}
	return append([]byte(nil), aiSecretKey...), nil
}
