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
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
)

// Helper function to create DeviceConfig for tests
func createTestDeviceConfig(hostName, hostIP string, hostPort int, username, password, keyFile string) DeviceConfig {
	return DeviceConfig{
		Device: DeviceInfo{
			Host: HostInfo{
				Name: hostName,
				IP:   hostIP,
				Port: hostPort,
			},
		},
		Auth: AuthConfig{
			Username: username,
			Password: configopaque.String(password),
			KeyFile:  keyFile,
		},
	}
}

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

func TestEstablishDeviceConnection_NoAuthProvided(t *testing.T) {
	logger := zap.NewNop()

	// Test with no password and no key file
	deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", 22, "testuser", "", "")
	_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no authentication method provided")
}

func TestEstablishDeviceConnection_InvalidKeyFile(t *testing.T) {
	logger := zap.NewNop()

	// Test with invalid key file path
	deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", 22, "testuser", "", "/nonexistent/path/to/key")
	_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load SSH key")
}

func TestEstablishDeviceConnection_PasswordAuth(t *testing.T) {
	logger := zap.NewNop()

	// Test with password authentication
	// This will fail to connect (no SSH server), but we verify the auth methods are built correctly
	deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", 22, "testuser", "testpass", "")
	_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection failure, not auth method building
	require.Error(t, err)
	// The error should be about SSH connection, not authentication method building
	assert.NotContains(t, err.Error(), "no authentication method provided")
}

func TestEstablishDeviceConnection_KeyFileAuth(t *testing.T) {
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

	// Test with key file authentication
	// This will fail to connect (no SSH server), but we verify the auth methods are built correctly
	deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", 22, "testuser", "", keyFile)
	_, err = EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection failure, not auth method building
	require.Error(t, err)
	// The error should be about SSH connection, not authentication method building
	assert.NotContains(t, err.Error(), "failed to load SSH key")
	assert.NotContains(t, err.Error(), "no authentication method provided")
}

func TestEstablishDeviceConnection_BothPasswordAndKey(t *testing.T) {
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

	// Test with both password and key file authentication
	deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", 22, "testuser", "testpass", keyFile)
	_, err = EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection failure, not auth method building
	require.Error(t, err)
	// The error should be about SSH connection, not authentication method building
	assert.NotContains(t, err.Error(), "failed to load SSH key")
	assert.NotContains(t, err.Error(), "at least one of auth.password or auth.key_file is required")
}

// Edge case tests

func TestEstablishDeviceConnection_EmptyHostName(t *testing.T) {
	logger := zap.NewNop()

	// Host name is optional, empty string should be valid
	deviceConfig := createTestDeviceConfig("", "192.168.1.1", 22, "testuser", "testpass", "")
	_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection, not validation
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "device.host.name")
}

func TestEstablishDeviceConnection_PortBoundaries(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name string
		port int
	}{
		{
			name: "port one",
			port: 1,
		},
		{
			name: "standard SSH port",
			port: 22,
		},
		{
			name: "high port",
			port: 65535,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", tt.port, "testuser", "testpass", "")
			_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)
			// Should fail with connection error, not validation error
			if err != nil {
				assert.NotContains(t, err.Error(), "is required")
			}
		})
	}
}

func TestEstablishDeviceConnection_IPv6Address(t *testing.T) {
	logger := zap.NewNop()

	// Test with IPv6 address
	deviceConfig := createTestDeviceConfig("test-device", "2001:db8::1", 22, "testuser", "testpass", "")
	_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection, not validation
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "device.host.ip is required")
}

func TestEstablishDeviceConnection_SpecialCharactersInPassword(t *testing.T) {
	logger := zap.NewNop()

	// Test with special characters in password
	deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", 22, "testuser", "p@$$w0rd!#%&*()[]{}|<>?/\\\"'`~", "")
	_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection, not validation
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "at least one of auth.password or auth.key_file is required")
}

func TestEstablishDeviceConnection_NonStandardPort(t *testing.T) {
	logger := zap.NewNop()

	// Test with non-standard SSH port
	deviceConfig := createTestDeviceConfig("test-device", "192.168.1.1", 2222, "testuser", "testpass", "")
	_, err := EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection, not validation
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "device.host.port is required")
}

func TestEstablishDeviceConnection_AllFieldsPopulated(t *testing.T) {
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

	// Test with all fields populated including optional host name
	deviceConfig := createTestDeviceConfig("cisco-switch-01", "192.168.1.1", 22, "admin", "password123", keyFile)
	_, err = EstablishDeviceConnection(t.Context(), deviceConfig, logger)

	// Should fail due to SSH connection, but all validation passed
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "is required")
	assert.NotContains(t, err.Error(), "at least one of")
}
