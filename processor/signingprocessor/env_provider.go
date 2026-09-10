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
	if cfg.HMACKeyEnvVar != "" {
		raw := os.Getenv(cfg.HMACKeyEnvVar)
		if raw == "" {
			return nil, fmt.Errorf("environment variable %q is not set or empty", cfg.HMACKeyEnvVar)
		}
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, fmt.Errorf("environment variable %q: HMAC key must be standard base64-encoded: %w", cfg.HMACKeyEnvVar, err)
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("environment variable %q: HMAC key is empty after base64 decoding", cfg.HMACKeyEnvVar)
		}
		return &envKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	// Asymmetric mode: load cert + private key
	certPEM := []byte(os.Getenv(cfg.CertEnvVar))
	if len(certPEM) == 0 {
		return nil, fmt.Errorf("environment variable %q is not set or empty", cfg.CertEnvVar)
	}
	keyPEM := []byte(os.Getenv(cfg.KeyEnvVar))
	if len(keyPEM) == 0 {
		return nil, fmt.Errorf("environment variable %q is not set or empty", cfg.KeyEnvVar)
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
