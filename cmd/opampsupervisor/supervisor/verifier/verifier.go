// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// Verifier is the interface that verifies the authenticity of a package.
type Verifier interface {
	// Verify verifies the authenticity of a package based on the verifier type.
	Verify(packageBytes, signature []byte) error
}

// NewVerifier creates a new Verifier based on the verifier type.
func NewVerifier(verifier config.Verifier) (Verifier, error) {
	switch verifier.Type {
	case config.VerifierTypeCosign:
		return newCosignVerifier(verifier.Cosign)
	case config.VerifierTypeNone:
		return &noneVerifier{}, nil
	default:
		return nil, fmt.Errorf("unsupported verifier type: %s", verifier.Type)
	}
}

// noneVerifier is the no-op verifier.
type noneVerifier struct{}

// Verify is the no-op verify function. It always returns no error.
func (noneVerifier) Verify(_, _ []byte) error {
	return nil
}
