// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

type inlineKeyMaterialProvider struct {
	baseKeyMaterialProvider
}

// newInlineKeyMaterialProvider constructs a KeyMaterialProvider from inline
// string values. Fields are expected to already hold resolved content —
// i.e. the actual PEM text or base64-encoded key, not env-var names.
// In a collector config, use ${env:VAR_NAME} substitution to supply the values.
func newInlineKeyMaterialProvider(cfg *EnvKeyConfig) (KeyMaterialProvider, error) {
	if cfg.HMACKey != "" {
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(cfg.HMACKey))
		if err != nil {
			return nil, fmt.Errorf("env.hmac_key: must be standard base64-encoded: %w", err)
		}
		if len(key) == 0 {
			return nil, errors.New("env.hmac_key: empty after base64 decoding")
		}
		return &inlineKeyMaterialProvider{baseKeyMaterialProvider{hmacKey: key}}, nil
	}

	certPEM := []byte(cfg.Certificate)
	if len(certPEM) == 0 {
		return nil, errors.New("env.certificate: value is empty")
	}
	keyPEM := []byte(cfg.PrivateKey)
	if len(keyPEM) == 0 {
		return nil, errors.New("env.private_key: value is empty")
	}
	certPEM = decodeIfBase64(certPEM)
	keyPEM = decodeIfBase64(keyPEM)
	certPEM = normalizeLineEndings(certPEM)
	keyPEM = normalizeLineEndings(keyPEM)
	reader, err := parseCertificateData(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &inlineKeyMaterialProvider{baseKeyMaterialProvider{reader: reader}}, nil
}
