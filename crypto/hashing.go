package crypto

import (
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	pw, err := bcrypt.GenerateFromPassword([]byte(password), 13)
	return string(pw), err
}

func VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func Sum(input string) string {
	sh := sha256.New()
	sh.Write([]byte(input))
	return hex.EncodeToString(sh.Sum(nil))
}
