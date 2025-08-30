package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Token generates a cryptographically-random token of a given length
// hex encoded
func Token(bytes int) (string, string, error) {
	out := make([]byte, bytes)
	if _, err := rand.Read(out); err != nil {
		return "", "", fmt.Errorf("generate random token bytes: %w", err)
	}
	token := hex.EncodeToString(out)
	hash := Sum(token)
	return token, hash, nil
}
