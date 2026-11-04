// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"crypto/x509"
)

// KeyMaterialProvider supplies the key material used for verification.
//
// For asymmetric algorithms (RS256, RS512, ES256, EdDSA):
//   - GetCertificate() returns the X.509 certificate whose public key verifies signatures
//   - GetHMACKey() returns nil
//
// For HMAC-SHA256:
//   - GetHMACKey() returns the raw symmetric secret
//   - GetCertificate() returns nil
type KeyMaterialProvider interface {
	GetCertificate() *x509.Certificate
	GetHMACKey() []byte
}
