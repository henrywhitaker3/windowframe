// Package crypto
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Encrptor struct {
	gcm cipher.AEAD
}

func GenerateAesKey(bits int) (string, error) {
	if bits%8 != 0 || bits > (32*8) || bits <= 16 {
		return "", errors.New("invalid key length")
	}
	key := make([]byte, bits/8)
	_, err := rand.Read(key)
	b64 := base64.StdEncoding.EncodeToString(key)
	return fmt.Sprintf("base64:%d:%s", bits/8, b64), err
}

func parseKey(key string) ([]byte, error) {
	spl := strings.Split(key, ":")
	if len(spl) != 3 {
		return nil, errors.New("invalid key format")
	}
	size, err := strconv.Atoi(spl[1])
	if err != nil {
		return nil, fmt.Errorf("failed to parse key size: %w", err)
	}
	out := make([]byte, size)
	_, err = base64.StdEncoding.Decode(out, []byte(spl[2]))
	return out, err
}

func NewEncryptor(secret string) (*Encrptor, error) {
	key, err := parseKey(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to parse secret: %w", err)
	}
	aes, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to init aes block: %w", err)
	}
	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return nil, fmt.Errorf("failed to init gcm: %w", err)
	}
	return &Encrptor{gcm: gcm}, nil
}

func (e *Encrptor) Encrypt(p []byte) ([]byte, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	_, err := rand.Read(nonce)
	if err != nil {
		return nil, err
	}
	ciphertext := e.gcm.Seal(nonce, nonce, p, nil)
	return ciphertext, nil
}

func (e *Encrptor) Decrypt(c []byte) ([]byte, error) {
	nonceSize := e.gcm.NonceSize()
	nonce, ciphertext := c[:nonceSize], c[nonceSize:]
	return e.gcm.Open(nil, nonce, ciphertext, nil)
}
