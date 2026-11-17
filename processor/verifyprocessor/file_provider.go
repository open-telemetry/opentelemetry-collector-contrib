// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"fmt"
	"os"
)

type fileKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

func newFileKeyMaterialProvider(cfg *FileKeyConfig) (KeyMaterialProvider, error) {
	if cfg.HMACKey != "" {
		data, err := os.ReadFile(cfg.HMACKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read HMAC key file %q: %w", cfg.HMACKey, err)
		}
		key, err := decodeHMACKey(string(data))
		if err != nil {
			return nil, fmt.Errorf("HMAC key file %q: %w", cfg.HMACKey, err)
		}
		return &fileKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	certPEM, err := os.ReadFile(cfg.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file %q: %w", cfg.Certificate, err)
	}
	certPEM = decodeIfBase64(certPEM)
	certPEM = normalizeLineEndings(certPEM)
	cert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return nil, err
	}
	return &fileKeyMaterialProvider{baseKeyMaterialProvider{cert: cert}}, nil
}
