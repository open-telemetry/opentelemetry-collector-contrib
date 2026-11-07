// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"errors"
	"fmt"
)

type inlineKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

func newInlineKeyMaterialProvider(cfg *EnvKeyConfig) (KeyMaterialProvider, error) {
	if cfg.HMACKey != "" {
		key, err := decodeHMACKey(cfg.HMACKey)
		if err != nil {
			return nil, fmt.Errorf("env.hmac_key: %w", err)
		}
		return &inlineKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	certPEM := []byte(cfg.Certificate)
	if len(certPEM) == 0 {
		return nil, errors.New("env.certificate: value is empty")
	}
	certPEM = decodeIfBase64(certPEM)
	certPEM = normalizeLineEndings(certPEM)
	cert, err := parseCertificatePEM(certPEM)
	if err != nil {
		return nil, err
	}
	return &inlineKeyMaterialProvider{baseKeyMaterialProvider{cert: cert}}, nil
}
