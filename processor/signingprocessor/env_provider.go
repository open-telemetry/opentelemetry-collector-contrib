// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"encoding/base64"
	"fmt"
	"os"
)

type envKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

func newEnvKeyMaterialProvider(cfg *EnvKeyConfig) (KeyMaterialProvider, error) {
	// HMAC mode: load only the symmetric key
	if cfg.HMACKey != "" {
		raw := os.Getenv(cfg.HMACKey)
		if raw == "" {
			return nil, fmt.Errorf("environment variable %q is not set or empty", cfg.HMACKey)
		}
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("environment variable %q: HMAC key must be standard base64-encoded: %w", cfg.HMACKey, err)
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("environment variable %q: HMAC key is empty after base64 decoding", cfg.HMACKey)
		}
		return &envKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	// Asymmetric mode: load cert + private key
	certPEM := []byte(os.Getenv(cfg.Certificate))
	if len(certPEM) == 0 {
		return nil, fmt.Errorf("environment variable %q is not set or empty", cfg.Certificate)
	}
	keyPEM := []byte(os.Getenv(cfg.PrivateKey))
	if len(keyPEM) == 0 {
		return nil, fmt.Errorf("environment variable %q is not set or empty", cfg.PrivateKey)
	}
	certPEM = decodeIfBase64(certPEM)
	keyPEM = decodeIfBase64(keyPEM)
	certPEM = normalizeLineEndings(certPEM)
	keyPEM = normalizeLineEndings(keyPEM)
	reader, err := parseCertificateData(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &envKeyMaterialProvider{baseKeyMaterialProvider{reader: reader}}, nil
}
