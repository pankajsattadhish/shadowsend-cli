package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// Streaming parameters matching web app
	StreamMagic      = "PHNT"
	StreamVersion    = 1
	StreamChunkSize  = 64 * 1024 // 64KB chunks
	StreamNonceSize  = 12
	StreamTagSize    = 16
	StreamHeaderSize = 28 // 4 magic + 4 version + 4 chunk_size + 4 total_chunks + 12 base_nonce
)

// StreamHeader represents the streaming encryption header
type StreamHeader struct {
	Magic       [4]byte
	Version     uint32
	ChunkSize   uint32
	TotalChunks uint32
	BaseNonce   [StreamNonceSize]byte
}

// isStreamingFormat checks if data starts with PHNT magic
func isStreamingFormat(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == StreamMagic
}

// deriveStreamNonce derives nonce for chunk i using STREAM construction
// Nonce format: [7-byte prefix][4-byte counter BE][1-byte last_block_flag]
func deriveStreamNonce(baseNonce []byte, chunkIndex uint32, isLastChunk bool) []byte {
	nonce := make([]byte, StreamNonceSize)
	
	// Copy first 7 bytes from base nonce
	copy(nonce[0:7], baseNonce[0:7])
	
	// Counter in big-endian at bytes 7-10
	binary.BigEndian.PutUint32(nonce[7:11], chunkIndex)
	
	// Last block flag at byte 11
	if isLastChunk {
		nonce[11] = 0x01
	} else {
		nonce[11] = 0x00
	}
	
	return nonce
}

// EncryptStream encrypts data using streaming AES-256-GCM with STREAM construction
func EncryptStream(plaintext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes")
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Generate random base nonce
	baseNonce := make([]byte, StreamNonceSize)
	if _, err := io.ReadFull(rand.Reader, baseNonce); err != nil {
		return nil, fmt.Errorf("failed to generate base nonce: %w", err)
	}
	
	// Calculate total chunks
	totalChunks := uint32(0)
	remaining := len(plaintext)
	for remaining > 0 {
		totalChunks++
		chunkLen := StreamChunkSize
		if remaining < StreamChunkSize {
			chunkLen = remaining
		}
		remaining -= chunkLen
	}
	
	if totalChunks == 0 {
		totalChunks = 1 // Handle empty file
	}
	
	// Build header
	var header StreamHeader
	copy(header.Magic[:], StreamMagic)
	header.Version = StreamVersion
	header.ChunkSize = StreamChunkSize
	header.TotalChunks = totalChunks
	copy(header.BaseNonce[:], baseNonce)
	
	// Serialize header
	headerBuf := new(bytes.Buffer)
	headerBuf.Write(header.Magic[:])
	binary.Write(headerBuf, binary.LittleEndian, header.Version)
	binary.Write(headerBuf, binary.LittleEndian, header.ChunkSize)
	binary.Write(headerBuf, binary.LittleEndian, header.TotalChunks)
	headerBuf.Write(header.BaseNonce[:])
	
	// Encrypt chunks
	var result bytes.Buffer
	result.Write(headerBuf.Bytes())
	
	offset := 0
	for i := uint32(0); i < totalChunks; i++ {
		chunkStart := offset
		chunkEnd := offset + StreamChunkSize
		if chunkEnd > len(plaintext) {
			chunkEnd = len(plaintext)
		}
		
		chunk := plaintext[chunkStart:chunkEnd]
		offset = chunkEnd
		
		isLast := i == totalChunks-1
		nonce := deriveStreamNonce(baseNonce, i, isLast)
		
		// Encrypt chunk
		ciphertext := gcm.Seal(nil, nonce, chunk, nil)
		
		// Write: [nonce][ciphertext+tag]
		result.Write(nonce)
		result.Write(ciphertext)
	}
	
	return result.Bytes(), nil
}

// DecryptStream decrypts streaming format data
func DecryptStream(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext) < StreamHeaderSize {
		return nil, fmt.Errorf("ciphertext too short for streaming format")
	}
	
	// Check magic
	if !isStreamingFormat(ciphertext) {
		return nil, fmt.Errorf("not streaming format")
	}
	
	// Parse header
	header := StreamHeader{
		Magic:     [4]byte{ciphertext[0], ciphertext[1], ciphertext[2], ciphertext[3]},
		Version:   binary.LittleEndian.Uint32(ciphertext[4:8]),
		ChunkSize: binary.LittleEndian.Uint32(ciphertext[8:12]),
		TotalChunks: binary.LittleEndian.Uint32(ciphertext[12:16]),
	}
	copy(header.BaseNonce[:], ciphertext[16:28])
	
	if header.Version != StreamVersion {
		return nil, fmt.Errorf("unsupported version: %d", header.Version)
	}
	
	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Calculate expected plaintext size
	offset := StreamHeaderSize
	var plaintext bytes.Buffer
	
	for i := uint32(0); i < header.TotalChunks; i++ {
		// Read nonce
		if offset+StreamNonceSize > len(ciphertext) {
			return nil, fmt.Errorf("unexpected end of stream at chunk %d", i)
		}
		nonce := ciphertext[offset : offset+StreamNonceSize]
		offset += StreamNonceSize
		
		// Calculate chunk plaintext size
		isLast := i == header.TotalChunks-1
		var chunkPlaintextSize int
		if isLast {
			// Last chunk: remainder
			totalPlaintextSize := int(header.TotalChunks-1) * int(header.ChunkSize)
			lastChunkSize := len(ciphertext) - StreamHeaderSize
			lastChunkSize -= int(header.TotalChunks) * StreamNonceSize // nonces
			lastChunkSize -= int(header.TotalChunks) * StreamTagSize   // tags
			chunkPlaintextSize = lastChunkSize - totalPlaintextSize
			if chunkPlaintextSize <= 0 {
				chunkPlaintextSize = 0
			}
		} else {
			chunkPlaintextSize = int(header.ChunkSize)
		}
		
		// Read and decrypt chunk
		chunkCiphertextSize := chunkPlaintextSize + StreamTagSize
		if offset+chunkCiphertextSize > len(ciphertext) {
			return nil, fmt.Errorf("unexpected end of stream at chunk %d ciphertext", i)
		}
		
		chunkCiphertext := ciphertext[offset : offset+chunkCiphertextSize]
		offset += chunkCiphertextSize
		
		// Verify nonce matches expected (optional but recommended)
		expectedNonce := deriveStreamNonce(header.BaseNonce[:], i, isLast)
		if !bytes.Equal(nonce, expectedNonce) {
			return nil, fmt.Errorf("nonce mismatch at chunk %d", i)
		}
		
		// Decrypt
		chunkPlaintext, err := gcm.Open(nil, nonce, chunkCiphertext, nil)
		if err != nil {
			return nil, fmt.Errorf("decryption failed at chunk %d: %w", i, err)
		}
		
		plaintext.Write(chunkPlaintext)
	}
	
	return plaintext.Bytes(), nil
}

// DecryptAuto auto-detects format and decrypts (streaming or legacy)
func DecryptAuto(ciphertext, key []byte) ([]byte, error) {
	if isStreamingFormat(ciphertext) {
		return DecryptStream(ciphertext, key)
	}
	return Decrypt(ciphertext, key)
}