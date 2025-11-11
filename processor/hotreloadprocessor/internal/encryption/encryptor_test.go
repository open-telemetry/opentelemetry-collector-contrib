// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encryption

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileEncryptor(t *testing.T) {
	// Create encryptor with test key
	key := "12345678901234567890123456789012"
	encryptor, err := NewFileEncryptor(key)
	require.NoError(t, err)
	defer encryptor.Close()

	// Test with compressible content (repeated data)
	content := []byte("test configuration content " + strings.Repeat("repeated data ", 100))

	// Test encryption
	encrypted, err := encryptor.Encrypt(content)
	require.NoError(t, err)
	assert.NotEqual(t, content, encrypted)
	// Verify the encrypted data is not empty and has at least the nonce
	assert.Greater(t, len(encrypted), 12) // At least the nonce size

	// Test decryption
	decrypted, err := encryptor.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, content, decrypted)
}

func TestNewFileEncryptor(t *testing.T) {
	tests := []struct {
		name      string
		keyString string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "minimum length (32 chars)",
			keyString: "12345678901234567890123456789012",
			wantErr:   false,
		},
		{
			name:      "64 characters",
			keyString: "ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234",
			wantErr:   false,
		},
		{
			name:      "longer than 64 chars",
			keyString: "ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234ABCD1234_extra",
			wantErr:   false,
		},
		{
			name:      "too short",
			keyString: "tooshort",
			wantErr:   true,
			errMsg:    "minimum length is 32 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encryptor, err := NewFileEncryptor(tt.keyString)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, encryptor)
			defer encryptor.Close()

			// Test that encryption/decryption works with compression
			content := []byte("test content " + strings.Repeat("compressible repeated data ", 100))
			encrypted, err := encryptor.Encrypt(content)
			require.NoError(t, err)
			// Verify the encrypted data is not empty and has at least the nonce
			assert.Greater(t, len(encrypted), 12) // At least the nonce size

			// Verify that same input produces same key by checking decryption works
			encryptor2, err := NewFileEncryptor(tt.keyString)
			require.NoError(t, err)
			defer encryptor2.Close()

			decrypted, err := encryptor2.Decrypt(encrypted)
			require.NoError(t, err)
			assert.Equal(t, content, decrypted)
		})
	}
}

func TestCompressionEffectiveness(t *testing.T) {
	encryptor, err := NewFileEncryptor("12345678901234567890123456789012")
	require.NoError(t, err)
	defer encryptor.Close()

	// Test with highly compressible data
	compressibleData := []byte(strings.Repeat("repeated data ", 1000))
	encrypted, err := encryptor.Encrypt(compressibleData)
	require.NoError(t, err)

	// Verify compression is effective
	// Even with encryption overhead, compressed size should be significantly smaller
	assert.Less(t, len(encrypted), len(compressibleData))

	// Test with random (incompressible) data
	randomData := make([]byte, 1000)
	_, err = rand.Read(randomData)
	require.NoError(t, err)

	encryptedRandom, err := encryptor.Encrypt(randomData)
	require.NoError(t, err)

	// Random data shouldn't compress much, size should be larger due to encryption overhead
	assert.Greater(t, len(encryptedRandom), len(randomData))
}

