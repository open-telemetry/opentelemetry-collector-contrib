// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
)

// AgentPackage describes how collector executable updates downloaded by the
// supervisor are identified and verified. The archive format is not configured
// here; the supervisor detects it at update time from the download URL and
// Content-Type header.
type AgentPackage struct {
	// AgentBinary is the name of the collector binary as it appears inside the
	// downloaded archive. Used to locate the binary in archives that bundle
	// multiple files (e.g. tar.gz).
	AgentBinary string `mapstructure:"agent_binary"`
	// Verifier configures how downloaded packages are verified.
	Verifier Verifier `mapstructure:"verifier"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Verifier configures how downloaded packages are verified.
type Verifier struct {
	// Type selects the verification method. An empty string disables verification.
	Type string `mapstructure:"type"`
	// Cosign configures the Cosign (Sigstore) signature verifier. Only used when
	// Type is "cosign".
	Cosign CosignSignatureVerifier `mapstructure:"cosign"`
	// prevent unkeyed literal initialization
	_ struct{}
}

const (
	// VerifierTypeNone disables package signature verification.
	VerifierTypeNone = ""
	// VerifierTypeCosign verifies packages signed with Cosign keyless signing.
	VerifierTypeCosign = "cosign"

	// Defaults matching binaries produced by the OpenTelemetry Collector
	// Releases repository. See the specification for details.
	defaultCosignRepository    = "open-telemetry/opentelemetry-collector-releases"
	defaultCosignIssuer        = "https://token.actions.githubusercontent.com"
	defaultCosignSubjectRegExp = `^https://github.com/open-telemetry/opentelemetry-collector-releases/.github/workflows/base-release.yaml@refs/tags/[^/]*$`
)

// Validate validates the verifier configuration.
func (v Verifier) Validate() error {
	switch v.Type {
	case VerifierTypeNone:
		return nil
	case VerifierTypeCosign:
		if len(v.Cosign.Identities) == 0 {
			return errors.New("cosign::identities must not be empty")
		}
		return nil
	default:
		return fmt.Errorf("unsupported verifier type: %q", v.Type)
	}
}

// CosignSignatureVerifier configures verification of packages signed with
// Cosign keyless signing. The signature is expected to be a Sigstore bundle.
// https://docs.sigstore.dev/cosign/signing/overview/
//
// Each identity is validated by confmap; Verifier.Validate requires at least
// one identity when the cosign verifier is selected.
type CosignSignatureVerifier struct {
	// CertGithubWorkflowRepository is the GitHub repository expected in the
	// signing certificate. Set to the empty string to skip this check.
	CertGithubWorkflowRepository string `mapstructure:"github_workflow_repository"`
	// Identities is a list of identities accepted as the package signer. Only
	// one needs to match the certificate for verification to pass.
	Identities []AgentSignatureIdentity `mapstructure:"identities"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// AgentSignatureIdentity is an OIDC issuer/subject pair identifying a trusted
// signer. Issuer and Subject match exactly; IssuerRegExp and SubjectRegExp
// match with a regular expression. Exactly one of each pair must be set.
type AgentSignatureIdentity struct {
	// Issuer is the exact OIDC issuer for the identity.
	Issuer string `mapstructure:"issuer"`
	// Subject is the exact OIDC subject for the identity.
	Subject string `mapstructure:"subject"`
	// IssuerRegExp is a regular expression matching the OIDC issuer.
	IssuerRegExp string `mapstructure:"issuer_regex"`
	// SubjectRegExp is a regular expression matching the OIDC subject.
	SubjectRegExp string `mapstructure:"subject_regex"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate validates the identity configuration.
func (a AgentSignatureIdentity) Validate() error {
	if a.Issuer != "" && a.IssuerRegExp != "" {
		return errors.New("cannot specify both issuer and issuer_regex")
	}
	if a.Subject != "" && a.SubjectRegExp != "" {
		return errors.New("cannot specify both subject and subject_regex")
	}
	if a.Issuer == "" && a.IssuerRegExp == "" {
		return errors.New("must specify one of issuer or issuer_regex")
	}
	if a.Subject == "" && a.SubjectRegExp == "" {
		return errors.New("must specify one of subject or subject_regex")
	}
	return nil
}
