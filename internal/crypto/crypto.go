package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

const ivLength = 12 // AES-GCM standard nonce size

// GenerateKey creates a new 256-bit AES key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// ExportKey encodes a raw key as base64url (no padding), matching the web app format.
func ExportKey(key []byte) string {
	s := base64.RawStdEncoding.EncodeToString(key)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	return s
}

// ImportKey decodes a base64url key string back to raw bytes.
func ImportKey(encoded string) ([]byte, error) {
	s := strings.ReplaceAll(encoded, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	return base64.RawStdEncoding.DecodeString(s)
}

// Encrypt encrypts data with AES-256-GCM.
// Returns [12-byte IV][ciphertext + GCM auth tag], matching the web app format.
func Encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	iv := make([]byte, ivLength)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	ciphertext := gcm.Seal(nil, iv, plaintext, nil)

	// Prepend IV: [12-byte IV][ciphertext + tag]
	result := make([]byte, ivLength+len(ciphertext))
	copy(result[:ivLength], iv)
	copy(result[ivLength:], ciphertext)

	return result, nil
}

// Decrypt decrypts data encrypted with AES-256-GCM.
// Expects [12-byte IV][ciphertext + GCM auth tag] format.
func Decrypt(data, key []byte) ([]byte, error) {
	if len(data) < ivLength {
		return nil, fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	iv := data[:ivLength]
	ciphertext := data[ivLength:]

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// EncryptFile reads a file's contents, encrypts it, and returns the ciphertext.
func EncryptFile(plaintext []byte, key []byte) ([]byte, error) {
	return Encrypt(plaintext, key)
}

// DecryptFile decrypts ciphertext and returns the original file contents.
func DecryptFile(ciphertext []byte, key []byte) ([]byte, error) {
	return Decrypt(ciphertext, key)
}
