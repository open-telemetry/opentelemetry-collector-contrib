// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"crypto"
	"crypto/x509"
)

// KeyMaterialProvider supplies the key material used for signing.
//
// For asymmetric algorithms (RS256, RS512, ES256, EdDSA):
//   - GetPrivateKey() returns the signing key (implements crypto.Signer)
//   - GetCertificate() returns the X.509 certificate for the public key
//   - GetHMACKey() returns nil
//
// For HMAC-SHA256:
//   - GetHMACKey() returns the raw symmetric secret
//   - GetPrivateKey() returns nil
//   - GetCertificate() returns nil
type KeyMaterialProvider interface {
	GetPrivateKey() crypto.Signer
	GetCertificate() *x509.Certificate
	GetHMACKey() []byte
}
