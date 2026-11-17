// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"bytes"
	"fmt"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// cosignVerifier verifies packages signed with Cosign keyless signing. The
// signature is a Sigstore bundle (the `.sigstore.json` file published
// alongside a release artifact) carrying the signing certificate, signature,
// Rekor transparency log entry, and signed timestamp. Verification is fully
// offline against the Sigstore trusted root fetched at startup.
type cosignVerifier struct {
	verifier   *verify.Verifier
	identities []verify.PolicyOption
}

var _ Verifier = &cosignVerifier{}

// newCosignVerifier fetches the public Sigstore trusted root via TUF and
// returns a verifier for the configured identities.
func newCosignVerifier(cfg config.CosignSignatureVerifier) (Verifier, error) {
	// Public Sigstore only; add a trusted_root path option when
	// someone needs a private Sigstore instance or air-gapped startup.
	trustedRoot, err := root.FetchTrustedRoot()
	if err != nil {
		return nil, fmt.Errorf("fetch sigstore trusted root: %w", err)
	}
	return newCosignVerifierWithTrustedMaterial(cfg, trustedRoot)
}

func newCosignVerifierWithTrustedMaterial(cfg config.CosignSignatureVerifier, trustedMaterial root.TrustedMaterial) (Verifier, error) {
	// Require a transparency log entry, a signed timestamp (from Rekor or a
	// timestamp authority), and an embedded SCT proving the certificate was
	// logged. This mirrors `cosign verify-blob` defaults for keyless bundles.
	v, err := verify.NewVerifier(trustedMaterial,
		verify.WithTransparencyLog(1),
		verify.WithObserverTimestamps(1),
		verify.WithSignedCertificateTimestamps(1),
	)
	if err != nil {
		return nil, fmt.Errorf("create sigstore verifier: %w", err)
	}

	identities := make([]verify.PolicyOption, 0, len(cfg.Identities))
	for i, ident := range cfg.Identities {
		san, err := verify.NewSANMatcher(ident.Subject, ident.SubjectRegExp)
		if err != nil {
			return nil, fmt.Errorf("identities[%d] subject: %w", i, err)
		}
		issuer, err := verify.NewIssuerMatcher(ident.Issuer, ident.IssuerRegExp)
		if err != nil {
			return nil, fmt.Errorf("identities[%d] issuer: %w", i, err)
		}
		certID, err := verify.NewCertificateIdentity(san, issuer, certificate.Extensions{
			GithubWorkflowRepository: cfg.CertGithubWorkflowRepository,
		})
		if err != nil {
			return nil, fmt.Errorf("identities[%d]: %w", i, err)
		}
		identities = append(identities, verify.WithCertificateIdentity(certID))
	}

	return &cosignVerifier{verifier: v, identities: identities}, nil
}

// Verify checks that signature is a Sigstore bundle whose signature covers
// packageBytes and whose certificate matches one of the configured identities.
func (c *cosignVerifier) Verify(packageBytes, signature []byte) error {
	return c.verify(signature, verify.WithArtifact(bytes.NewReader(packageBytes)))
}

func (c *cosignVerifier) verify(signature []byte, artifact verify.ArtifactPolicyOption) error {
	var b bundle.Bundle
	if err := b.UnmarshalJSON(signature); err != nil {
		return fmt.Errorf("parse sigstore bundle: %w", err)
	}
	if _, err := c.verifier.Verify(&b, verify.NewPolicy(artifact, c.identities...)); err != nil {
		return fmt.Errorf("verify sigstore bundle: %w", err)
	}
	return nil
}

func (*cosignVerifier) Type() string { return config.VerifierTypeCosign }
