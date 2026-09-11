// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

type fileKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

func newFileKeyMaterialProvider(cfg *FileKeyConfig) (KeyMaterialProvider, error) {
	// HMAC mode: load only the symmetric key
	if cfg.HMACKey != "" {
		data, err := os.ReadFile(cfg.HMACKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read HMAC key file %q: %w", cfg.HMACKey, err)
		}
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("HMAC key file %q: content must be standard base64-encoded: %w", cfg.HMACKey, err)
		}
		if len(key) == 0 {
			return nil, fmt.Errorf("HMAC key file %q is empty after base64 decoding", cfg.HMACKey)
		}
		return &fileKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	// Asymmetric mode: load cert + private key
	certPEM, err := os.ReadFile(cfg.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file %q: %w", cfg.Certificate, err)
	}
	keyPEM, err := os.ReadFile(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %q: %w", cfg.PrivateKey, err)
	}
	certPEM = decodeIfBase64(certPEM)
	keyPEM = decodeIfBase64(keyPEM)
	certPEM = normalizeLineEndings(certPEM)
	keyPEM = normalizeLineEndings(keyPEM)
	reader, err := parseCertificateData(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &fileKeyMaterialProvider{baseKeyMaterialProvider{reader: reader}}, nil
}
