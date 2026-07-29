// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import "fmt"

// AgentPackage describes how collector executable updates downloaded by the
// supervisor are verified. The archive format is not configured here; the
// supervisor detects it at update time from the download URL and Content-Type
// header.
type AgentPackage struct {
	// Verifier configures how downloaded packages are verified.
	Verifier Verifier `mapstructure:"verifier"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Verifier configures how downloaded packages are verified. The verification
// methods themselves (e.g. cosign) are added in later PRs; for now only the
// no-op verifier is supported.
type Verifier struct {
	// Type selects the verification method. An empty string disables verification.
	Type string `mapstructure:"type"`
	// prevent unkeyed literal initialization
	_ struct{}
}

const (
	// VerifierTypeNone disables package signature verification.
	VerifierTypeNone = ""
)

// Validate validates the verifier configuration.
func (v Verifier) Validate() error {
	switch v.Type {
	case VerifierTypeNone:
		return nil
	default:
		return fmt.Errorf("unsupported verifier type: %q", v.Type)
	}
}
