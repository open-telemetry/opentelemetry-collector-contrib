// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package credentialprovider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

const (
	schemaName    = "credential"
	keyTypeEnvVar = "OTEL_CREDENTIAL_PROVIDER_TYPE"
	keyEnvVar     = "OTEL_CREDENTIAL_PROVIDER"
)

type keyType string

const (
	AES keyType = "AES"
)

type provider struct {
	logger  *zap.Logger
	key     []byte
	keyType keyType
}

// NewFactory creates a new provider factory
func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(
		func(settings confmap.ProviderSettings) confmap.Provider {
			return &provider{
				logger: settings.Logger,
			}
		})
}

func (*provider) Scheme() string {
	return schemaName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}

func (p *provider) Retrieve(_ context.Context, uri string, _ confmap.WatcherFunc) (*confmap.Retrieved, error) {

	if !strings.HasPrefix(uri, schemaName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemaName)
	}

	if p.key == nil {
		// base64 decode env var
		keyTypeStr, ok := os.LookupEnv(keyTypeEnvVar)
		if !ok {
			return nil, fmt.Errorf("env var %q not set, required for %q provider", keyTypeEnvVar, schemaName)
		}
		switch keyTypeStr {
		case string(AES):
		default:
			return nil, fmt.Errorf("%q provider does not support %q key type", schemaName, keyTypeStr)
		}

		base64Key, ok := os.LookupEnv(keyEnvVar)
		if !ok {
			return nil, fmt.Errorf("env var %q not set, required for %q provider", keyEnvVar, schemaName)
		}
		key, err := base64.StdEncoding.DecodeString(base64Key)
		if err != nil {
			return nil, fmt.Errorf("%q provider uri failed to base64 decode key: %w", schemaName, err)
		}
		p.keyType = keyType(keyTypeStr)
		p.key = key
	}

	// Remove schemaName
	cipherText := strings.Replace(uri, schemaName+":", "", 1)

	clearText, err := p.decrypt(cipherText)
	if err != nil {
		return nil, fmt.Errorf("%q provider failed to decrypt value: %w", schemaName, err)
	}

	return confmap.NewRetrieved(clearText)
}

// decrypt is the place for future encryption schemas
func (p *provider) decrypt(cipherText string) (string, error) {

	cipherBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	clearBytes, err := decryptAES(cipherBytes, p.key)

	if err != nil {
		return "", err
	}

	return string(clearBytes), nil
}

// AES decryption function
func decryptAES(cipherBytes, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(cipherBytes) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, cipherBytes := cipherBytes[:nonceSize], cipherBytes[nonceSize:]
	return aesGCM.Open(nil, nonce, cipherBytes, nil)
}
