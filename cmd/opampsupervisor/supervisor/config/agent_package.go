// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
)

// AgentPackage represents options for installing an agent package when it is received
// through a PackagesAvailable message.
type AgentPackage struct {
	// AgentBinary is the name of the agent binary in the Archive that will be installed.
	AgentBinary string `mapstructure:"agent_binary"`
	// Archive is the format of the package that will be installed. This specifies how
	// the package will be processed.
	Archive Archive `mapstructure:"archive"`
	// Verifier is the verifier of the package that will be installed. This is used to
	// verify the authenticity of the package.
	Verifier Verifier `mapstructure:"verifier"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate validates the agent package configuration.
func (a AgentPackage) Validate() error {
	if err := a.Archive.Validate(a.AgentBinary != ""); err != nil {
		return err
	}

	if err := a.Verifier.Validate(); err != nil {
		return err
	}

	return nil
}

// Archive represents the format of the package that will be installed.
type Archive string

const (
	// ArchiveDefault is the default value for the archive format.
	// It is used when there is no formatting on the package.
	ArchiveDefault Archive = ""
	// ArchiveTarGzip is the archive format for tar.gz files.
	ArchiveTarGzip Archive = "tar.gz"

	// TODO: Add support for other archive formats below.
)

// ErrAgentBinaryRequiredError is the error returned when the agent binary is required for the given archive format.
var ErrAgentBinaryRequiredError = errors.New("agent::package::agent_binary must be specified for given archive")

// Validate validates the archive format is supported. It also verifies that the agent binary is configured if the archive format requires it.
func (a Archive) Validate(binaryConfigured bool) error {
	switch a {
	case ArchiveTarGzip:
		if !binaryConfigured {
			return ErrAgentBinaryRequiredError
		}
		return nil
	case ArchiveDefault:
		return nil
	default:
		return fmt.Errorf("unsupported archive format: %s", a)
	}
}

const (
	// VerifierTypeCosign is the type of the Cosign signature verifier.
	VerifierTypeCosign = "cosign"
	// VerifierTypeNone is the type of the no-op verifier.
	VerifierTypeNone = ""

	// TODO: Add support for other verifiers.
)

// Verifier represents the verifier of the package that will be installed.
type Verifier struct {
	// Type is the type of the verifier.
	Type string `mapstructure:"type"`
	// Cosign is the Cosign signature verifier.
	Cosign CosignSignatureVerifier `mapstructure:"cosign,omitempty"`

	// TODO: Add support for other verifiers.

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate validates the verifier configuration.
// Only one verifier can be configured at a time.
// By default, Cosign is used as the verifier.
func (v Verifier) Validate() error {
	switch v.Type {
	case VerifierTypeCosign:
		return v.Cosign.Validate()
	case VerifierTypeNone:
		return nil
	default:
		return fmt.Errorf("invalid verifier type: %s", v.Type)
	}
}

// CosignSignatureVerifier represents options for verifying the agent signature from the PackagesAvailable message.
// You can read more about Cosign and signing here.
// https://docs.sigstore.dev/cosign/signing/overview/
type CosignSignatureVerifier struct {
	// TODO: The Fulcio root certificate can be specified via SIGSTORE_ROOT_FILE for now
	// But we should add it as a config option.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35931

	// github_workflow_repository defines the expected repository field
	// on the sigstore certificate.
	CertGithubWorkflowRepository string `mapstructure:"github_workflow_repository"`

	// Identities is a list of valid identities to use when verifying the agent.
	// Only one needs to match the identity on the certificate for signature
	// verification to pass.
	Identities []AgentSignatureIdentity `mapstructure:"identities"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate validates the agent signature configuration.
func (a CosignSignatureVerifier) Validate() error {
	for i, ident := range a.Identities {
		if err := ident.Validate(); err != nil {
			return fmt.Errorf("agent::identities[%d]: %w", i, err)
		}
	}

	return nil
}

// AgentSignatureIdentity represents an Issuer/Subject pair that identifies
// the signer of the agent. This allows restricting the valid signers, such
// that only very specific sources are trusted as signers for an agent.
// You can read more about Cosign and signing here.
// Issuer and Subject are used to strictly match their values.
// IssuerRegExp and SubjectRegExp can be used instead to match the values using a regular expression.
// These are the values that are used to verify the identity of the signer.
// https://docs.sigstore.dev/cosign/signing/overview/
type AgentSignatureIdentity struct {
	// Issuer is the OIDC Issuer for the identity
	Issuer string `mapstructure:"issuer"`
	// Subject is the OIDC Subject for the identity
	Subject string `mapstructure:"subject"`
	// IssuerRegExp is a regular expression for matching the OIDC Issuer for the identity
	IssuerRegExp string `mapstructure:"issuer_regex"`
	// SubjectRegExp is a regular expression for matching the OIDC Subject for the identity.
	SubjectRegExp string `mapstructure:"subject_regex"`

	// prevent unkeyed literal initialization
	_ struct{}
}

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
