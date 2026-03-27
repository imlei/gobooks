// 遵循产品需求 v1.0
package web

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// NewOpaqueSessionToken returns a random opaque cookie value (hex-encoded 32 bytes)
// and the SHA-256 hex digest stored in sessions.token_hash.
func NewOpaqueSessionToken() (cookieValue string, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(raw), hex.EncodeToString(sum[:]), nil
}

// TokenHashFromCookieValue converts the cookie value (hex-encoded 32 bytes) into the
// digest form stored in sessions.token_hash.
func TokenHashFromCookieValue(cookieValue string) (string, error) {
	raw, err := hex.DecodeString(cookieValue)
	if err != nil || len(raw) != 32 {
		return "", fmt.Errorf("invalid session token")
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
