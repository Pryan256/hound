// Package encryption provides AES-256-GCM envelope encryption for sensitive values
// stored in the database (provider access tokens, refresh tokens).
//
// Each encrypted value is self-contained: nonce + ciphertext, base64-encoded.
// Key material comes from the app-level KMS key (production) or ENCRYPTION_KEY env var (dev).
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Encryptor holds the AES-256 key and performs encrypt/decrypt operations.
type Encryptor struct {
	key []byte // 32 bytes
}

// New creates an Encryptor from a 32-byte (256-bit) key.
// keyHex should be a 64-character hex string from the environment.
func New(keyHex string) (*Encryptor, error) {
	if len(keyHex) == 0 {
		// Dev fallback: use a deterministic zero key (never in production)
		return &Encryptor{key: make([]byte, 32)}, nil
	}
	key, err := decodeHex(keyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	return &Encryptor{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns base64(nonce + ciphertext). Each call produces a unique ciphertext.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a value produced by Encrypt.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

func decodeHex(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, errors.New("odd length hex string")
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi, ok1 := unhex(s[i])
		lo, ok2 := unhex(s[i+1])
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid hex character at position %d", i)
		}
		b[i/2] = (hi << 4) | lo
	}
	return b, nil
}

func unhex(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
