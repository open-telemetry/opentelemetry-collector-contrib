// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
)

func TestBuildAuthMethods_Password(t *testing.T) {
	logger := zap.NewNop()

	auth := AuthConfig{
		Username: "testuser",
		Password: "testpass",
	}

	authMethods, err := buildAuthMethods(auth, logger)
	require.NoError(t, err)
	assert.Len(t, authMethods, 1, "Should have one auth method for password")
}

func TestBuildAuthMethods_KeyFile(t *testing.T) {
	// Create temporary SSH key file
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test_key")

	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Encode private key to PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Write key to file
	keyData := pem.EncodeToMemory(privateKeyPEM)
	err = os.WriteFile(keyFile, keyData, 0o600)
	require.NoError(t, err)

	logger := zap.NewNop()

	auth := AuthConfig{
		Username: "testuser",
		KeyFile:  keyFile,
	}

	authMethods, err := buildAuthMethods(auth, logger)
	require.NoError(t, err)
	assert.Len(t, authMethods, 1, "Should have one auth method for key file")
}

func TestBuildAuthMethods_BothPasswordAndKey(t *testing.T) {
	// Create temporary SSH key file
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test_key")

	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Encode private key to PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Write key to file
	keyData := pem.EncodeToMemory(privateKeyPEM)
	err = os.WriteFile(keyFile, keyData, 0o600)
	require.NoError(t, err)

	logger := zap.NewNop()

	auth := AuthConfig{
		Username: "testuser",
		Password: "testpass",
		KeyFile:  keyFile,
	}

	authMethods, err := buildAuthMethods(auth, logger)
	require.NoError(t, err)
	assert.Len(t, authMethods, 2, "Should have two auth methods when both password and key are provided")
}

func TestBuildAuthMethods_NoAuthProvided(t *testing.T) {
	logger := zap.NewNop()

	auth := AuthConfig{
		Username: "testuser",
		// No password or key file
	}

	authMethods, err := buildAuthMethods(auth, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no authentication method provided")
	assert.Nil(t, authMethods)
}

func TestBuildAuthMethods_InvalidKeyFile(t *testing.T) {
	logger := zap.NewNop()

	auth := AuthConfig{
		Username: "testuser",
		KeyFile:  "/nonexistent/path/to/key",
	}

	authMethods, err := buildAuthMethods(auth, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load SSH key")
	assert.Nil(t, authMethods)
}

func TestPublicKeyAuth_ValidKey(t *testing.T) {
	// Create temporary SSH key file
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "test_key")

	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Encode private key to PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	// Write key to file
	keyData := pem.EncodeToMemory(privateKeyPEM)
	err = os.WriteFile(keyFile, keyData, 0o600)
	require.NoError(t, err)

	authMethod, err := publicKeyAuth(keyFile)
	require.NoError(t, err)
	assert.NotNil(t, authMethod)
}

func TestPublicKeyAuth_InvalidKeyFormat(t *testing.T) {
	// Create temporary file with invalid key data
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "invalid_key")

	err := os.WriteFile(keyFile, []byte("not a valid SSH key"), 0o600)
	require.NoError(t, err)

	authMethod, err := publicKeyAuth(keyFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse private key")
	assert.Nil(t, authMethod)
}

func TestPublicKeyAuth_EncryptedKey(t *testing.T) {
	// Create temporary SSH key file (encrypted)
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "encrypted_key")

	// Generate RSA key and encrypt it using SSH library
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Use SSH library to marshal encrypted key (passphrase protected)
	passphrase := []byte("testpassphrase")
	encryptedPEM, err := cryptossh.MarshalPrivateKeyWithPassphrase(privateKey, "", passphrase)
	require.NoError(t, err)

	// Write encrypted key to file
	err = os.WriteFile(keyFile, pem.EncodeToMemory(encryptedPEM), 0o600)
	require.NoError(t, err)

	authMethod, err := publicKeyAuth(keyFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SSH key is encrypted but passphrase is not supported")
	assert.Nil(t, authMethod)
}

func TestPublicKeyAuth_OpenSSHFormat(t *testing.T) {
	// Create temporary SSH key file in OpenSSH format
	tmpDir := t.TempDir()
	keyFile := filepath.Join(tmpDir, "openssh_key")

	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Convert to OpenSSH format
	sshPrivateKey, err := cryptossh.NewSignerFromKey(privateKey)
	require.NoError(t, err)

	// Marshal to OpenSSH format
	opensshKey, err := cryptossh.MarshalPrivateKey(privateKey, "")
	require.NoError(t, err)

	// Write key to file
	err = os.WriteFile(keyFile, pem.EncodeToMemory(opensshKey), 0o600)
	require.NoError(t, err)

	authMethod, err := publicKeyAuth(keyFile)
	require.NoError(t, err)
	assert.NotNil(t, authMethod)
	_ = sshPrivateKey // Use the variable to avoid unused warning
}
