// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package verifier verifies the signatures of collector executable packages
// downloaded by the supervisor.
package verifier

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// Verifier verifies the signature of a downloaded package.
type Verifier interface {
	// Verify returns nil if signature is a valid signature for packageBytes.
	Verify(packageBytes, signature []byte) error
	// Type returns the verifier type, matching the configured verifier type.
	Type() string
}

// NewVerifier returns the Verifier for the given verifier configuration.
func NewVerifier(verifier config.Verifier) (Verifier, error) {
	switch verifier.Type {
	case config.VerifierTypeNone:
		return &noneVerifier{}, nil
	default:
		return nil, fmt.Errorf("unsupported verifier type: %q", verifier.Type)
	}
}

// noneVerifier performs no verification and accepts every package.
type noneVerifier struct{}

func (noneVerifier) Verify(_, _ []byte) error { return nil }

func (noneVerifier) Type() string { return config.VerifierTypeNone }
