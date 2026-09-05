// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"fmt"
	"os"
)

type fileKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

func newFileKeyMaterialProvider(cfg *FileKeyConfig) (KeyMaterialProvider, error) {
	// HMAC mode: load only the symmetric key
	if cfg.HMACKeyFile != "" {
		data, err := os.ReadFile(cfg.HMACKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read HMAC key file %q: %w", cfg.HMACKeyFile, err)
		}
		key := decodeIfBase64(normalizeLineEndings(data))
		if len(key) == 0 {
			return nil, fmt.Errorf("HMAC key file %q is empty", cfg.HMACKeyFile)
		}
		return &fileKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	// Asymmetric mode: load cert + private key
	certPEM, err := os.ReadFile(cfg.CertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file %q: %w", cfg.CertFile, err)
	}
	keyPEM, err := os.ReadFile(cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file %q: %w", cfg.KeyFile, err)
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
