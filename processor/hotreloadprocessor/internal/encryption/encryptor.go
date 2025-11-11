// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// Encryptor defines the interface for encryption operations.
type Encryptor interface {
	// Encrypt encrypts the content using the configured encryption method.
	Encrypt(content []byte) ([]byte, error)
	// Decrypt decrypts the content using the configured encryption method.
	Decrypt(encrypted []byte) ([]byte, error)
	// Close releases resources used by the encryptor.
	Close() error
}

// FileEncryptor encrypts and decrypts files using AES-GCM with zstd compression.
type FileEncryptor struct {
	key     []byte
	encoder *zstd.Encoder
	decoder *zstd.Decoder
}

var _ Encryptor = (*FileEncryptor)(nil)

// NewFileEncryptor creates a new FileEncryptor from a string of at least 32 characters.
func NewFileEncryptor(keyString string) (Encryptor, error) {
	if len(keyString) < 32 {
		return nil, fmt.Errorf(
			"key string too short: minimum length is 32 characters, got %d",
			len(keyString),
		)
	}

	hasher := sha256.New()
	hasher.Write([]byte(keyString))
	key := hasher.Sum(nil)

	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		encoder.Close()
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}

	return &FileEncryptor{
		key:     key,
		encoder: encoder,
		decoder: decoder,
	}, nil
}

// Encrypt compresses and then encrypts the content using AES-GCM.
func (f *FileEncryptor) Encrypt(content []byte) ([]byte, error) {
	compressed := f.encoder.EncodeAll(content, make([]byte, 0, len(content)))

	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := aesgcm.Seal(nonce, nonce, compressed, nil)
	return ciphertext, nil
}

// Decrypt decrypts and then decompresses the content using AES-GCM.
func (f *FileEncryptor) Decrypt(encrypted []byte) ([]byte, error) {
	if len(encrypted) < 12 {
		return nil, fmt.Errorf("encrypted content too short")
	}
	nonce := encrypted[:12]
	ciphertext := encrypted[12:]

	block, err := aes.NewCipher(f.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	compressed, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt content: %w", err)
	}

	decompressed, err := f.decoder.DecodeAll(compressed, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress content: %w", err)
	}

	return decompressed, nil
}

// Close releases resources used by the encryptor.
func (f *FileEncryptor) Close() error {
	f.encoder.Close()
	f.decoder.Close()
	return nil
}
