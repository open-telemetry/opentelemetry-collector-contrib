// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"crypto"
	"crypto/x509"
)

// baseKeyMaterialProvider holds the common fields and delegation methods shared
// by all four concrete key-material provider types. Embed it instead of
// duplicating the three Get* methods in every provider.
type baseKeyMaterialProvider struct {
	reader  *certificateReader
	hmacKey []byte
}

func (b *baseKeyMaterialProvider) GetPrivateKey() crypto.Signer {
	if b.reader == nil {
		return nil
	}
	return b.reader.GetPrivateKey()
}

func (b *baseKeyMaterialProvider) GetCertificate() *x509.Certificate {
	if b.reader == nil {
		return nil
	}
	return b.reader.GetCertificate()
}

func (b *baseKeyMaterialProvider) GetHMACKey() []byte { return b.hmacKey }
