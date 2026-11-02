// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"strings"
)

// baseKeyMaterialProvider implements KeyMaterialProvider.
// Concrete providers embed it and set either the certificate or the HMAC key.
type baseKeyMaterialProvider struct {
	cert    *x509.Certificate
	hmacKey []byte
}

func decodeHMACKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("HMAC key is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed)
	if err == nil && len(decoded) > 0 {
		return decoded, nil
	}
	return []byte(trimmed), nil
}

func (b *baseKeyMaterialProvider) GetCertificate() *x509.Certificate { return b.cert }

func (b *baseKeyMaterialProvider) GetHMACKey() []byte { return b.hmacKey }
