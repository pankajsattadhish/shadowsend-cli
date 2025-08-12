package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptStream_SmallFile(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plaintext := []byte("Hello, ShadowSend!")

	ciphertext, err := EncryptStream(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	// Check magic header
	if string(ciphertext[:4]) != "PHNT" {
		t.Errorf("expected magic PHNT, got %s", ciphertext[:4])
	}

	// Decrypt
	decrypted, err := DecryptStream(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptStream failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data doesn't match plaintext")
	}
}

func TestEncryptStream_ExactChunkSize(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// Exactly one chunk
	plaintext := make([]byte, StreamChunkSize)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := EncryptStream(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	decrypted, err := DecryptStream(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptStream failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data doesn't match plaintext")
	}
}

func TestEncryptStream_MultipleChunks(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	// 200KB = ~4 chunks
	size := 200 * 1024
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, err := EncryptStream(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	decrypted, err := DecryptStream(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptStream failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data doesn't match plaintext")
	}
}

func TestEncryptStream_EmptyFile(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plaintext := []byte{}

	ciphertext, err := EncryptStream(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	decrypted, err := DecryptStream(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptStream failed: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("expected empty decrypted data, got %d bytes", len(decrypted))
	}
}

func TestDecryptAuto_StreamingFormat(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plaintext := []byte("streaming test")

	ciphertext, err := EncryptStream(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	decrypted, err := DecryptAuto(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptAuto failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data doesn't match plaintext")
	}
}

func TestDecryptAuto_LegacyFormat(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plaintext := []byte("legacy test")

	// Use legacy encrypt
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// DecryptAuto should detect and decrypt legacy format
	decrypted, err := DecryptAuto(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptAuto failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data doesn't match plaintext")
	}
}

func TestStreamNonceDerivation(t *testing.T) {
	baseNonce := make([]byte, StreamNonceSize)
	for i := range baseNonce {
		baseNonce[i] = byte(i)
	}

	// First chunk (not last)
	nonce0 := deriveStreamNonce(baseNonce, 0, false)
	if nonce0[11] != 0x00 {
		t.Errorf("expected last_block_flag=0x00 for non-last chunk")
	}

	// Last chunk
	nonceLast := deriveStreamNonce(baseNonce, 5, true)
	if nonceLast[11] != 0x01 {
		t.Errorf("expected last_block_flag=0x01 for last chunk")
	}

	// Nonces should differ
	if bytes.Equal(nonce0, nonceLast) {
		t.Errorf("nonces for different chunks should differ")
	}

	// First 7 bytes should match base nonce
	if !bytes.Equal(nonce0[0:7], baseNonce[0:7]) {
		t.Errorf("first 7 bytes should match base nonce")
	}
}

func TestStream_WrongKey(t *testing.T) {
	key1, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key1: %v", err)
	}

	key2, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key2: %v", err)
	}

	plaintext := []byte("secret data")

	ciphertext, err := EncryptStream(plaintext, key1)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	_, err = DecryptStream(ciphertext, key2)
	if err == nil {
		t.Errorf("expected decryption to fail with wrong key")
	}
}

func TestStream_CorruptedCiphertext(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plaintext := []byte("some data that is longer than a few bytes")
	ciphertext, err := EncryptStream(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	// Corrupt a byte in the first chunk's ciphertext
	ciphertext[50] ^= 0xFF

	_, err = DecryptStream(ciphertext, key)
	if err == nil {
		t.Errorf("expected decryption to fail with corrupted ciphertext")
	}
}

func TestStream_CorruptedHeader(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plaintext := []byte("test")
	ciphertext, err := EncryptStream(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptStream failed: %v", err)
	}

	// Corrupt magic
	ciphertext[0] = 0xFF

	_, err = DecryptStream(ciphertext, key)
	if err == nil {
		t.Errorf("expected decryption to fail with corrupted header")
	}
}

