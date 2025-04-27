// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package aesprovider // import "github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/aesprovider"

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

const (
	schemaName = "aes"
	// This environment variable holds a base64-encoded AES key, either 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
	keyEnvVar = "OTEL_AES_CREDENTIAL_PROVIDER"
)

type provider struct {
	logger *zap.Logger
	key    []byte
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
		base64Key, ok := os.LookupEnv(keyEnvVar)
		if !ok {
			return nil, fmt.Errorf("env var %q not set, required for %q provider", keyEnvVar, schemaName)
		}
		key, err := base64.StdEncoding.DecodeString(base64Key)
		if err != nil {
			return nil, fmt.Errorf("%q provider uri failed to base64 decode key: %w", schemaName, err)
		}
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

func (p *provider) decrypt(cipherText string) (string, error) {
	cipherBytes, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(p.key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(cipherBytes) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, cipherBytes := cipherBytes[:nonceSize], cipherBytes[nonceSize:]

	clearBytes, err := aesGCM.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", err
	}

	return string(clearBytes), nil
}
