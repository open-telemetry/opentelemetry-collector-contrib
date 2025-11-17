// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	"github.com/sigstore/rekor/pkg/client"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

const (
	rekorClientURL = "https://rekor.sigstore.dev"
)

// cosignVerifier is the Verifier that verifies the authenticity of a package using Cosign.
type cosignVerifier struct {
	checkOpts *cosign.CheckOpts
}

var _ Verifier = &cosignVerifier{}

// newCosignVerifier creates a new CosignVerifier.
// It creates a cosign.CheckOpts from the signature options and returns a cosignVerifier.
// These options provide information needed to verify the signature of the package.
// The options consist of:
// - public Fulcio certificates to verify the identity of the signature,
// - public Fulcio intermediate certificates to verify the identity of the signature,
// - public Rekor keys to verify the integrity of the signature against a transparency log,
// - public CT log keys to verify the integrity of the signature against a transparency log,
// - a set of identities that the signature must match.
// More information about the cosign.CheckOpts can be found in the specification
// (../../specification/README.md#collector-executable-updates-flow).
func newCosignVerifier(config config.CosignSignatureVerifier) (Verifier, error) {
	ctx := context.Background()

	// Get the public Fulcio certificates.
	rootCerts, err := fulcio.GetRoots()
	if err != nil {
		return nil, fmt.Errorf("fetch root certs: %w", err)
	}

	// Get the public Fulcio intermediate certificates.
	intermediateCerts, err := fulcio.GetIntermediates()
	if err != nil {
		return nil, fmt.Errorf("fetch intermediate certs: %w", err)
	}

	// Get public rekor keys
	rekorClient, err := client.GetRekorClient(rekorClientURL)
	if err != nil {
		return nil, fmt.Errorf("create rekot client: %w", err)
	}
	rekorKeys, err := cosign.GetRekorPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("get rekor public keys: %w", err)
	}

	// Get public CT log keys
	ctLogPubKeys, err := cosign.GetCTLogPubs(ctx)
	if err != nil {
		return nil, fmt.Errorf("get CT log public keys: %w", err)
	}

	// Create the identities to verify the signature against.
	identities := make([]cosign.Identity, 0, len(config.Identities))
	for _, ident := range config.Identities {
		identities = append(identities, cosign.Identity{
			Issuer:        ident.Issuer,
			IssuerRegExp:  ident.IssuerRegExp,
			Subject:       ident.Subject,
			SubjectRegExp: ident.SubjectRegExp,
		})
	}

	return &cosignVerifier{
		checkOpts: &cosign.CheckOpts{
			RootCerts:                    rootCerts,
			IntermediateCerts:            intermediateCerts,
			CertGithubWorkflowRepository: config.CertGithubWorkflowRepository,
			Identities:                   identities,
			RekorClient:                  rekorClient,
			RekorPubKeys:                 rekorKeys,
			CTLogPubKeys:                 ctLogPubKeys,
		},
	}, nil
}

// Verify verifies the authenticity of a package using Cosign.
// It parses the package signature into a base64 encoded certificate and signature,
// and then verifies the signature using the verifyPackageSignature function.
func (c *cosignVerifier) Verify(packageBytes, signature []byte) error {
	b64Cert, b64Signature, err := c.parsePackageSignature(signature)
	if err != nil {
		return fmt.Errorf("could not parse package signature: %w", err)
	}
	if err = c.verifyPackageSignature(context.Background(), packageBytes, b64Cert, b64Signature); err != nil {
		return fmt.Errorf("could not verify package signature: %w", err)
	}

	return nil
}

// parsePackageSignature parses the package signature into a base64 encoded certificate and signature.
// The signature is expected to be formatted as a space separated cert and signature.
// The certificate and signature are returned as base64 encoded bytes.
func (cosignVerifier) parsePackageSignature(signature []byte) (b64Cert, b64Signature []byte, err error) {
	splitSignature := bytes.SplitN(signature, []byte(" "), 2)
	if len(splitSignature) != 2 {
		return nil, nil, errors.New("signature must be formatted as a space separated cert and signature")
	}

	return splitSignature[0], splitSignature[1], nil
}

// verifyPackageSignature verifies that the b64Signature is a valid signature for packageBytes.
// b64Cert is used to validate the identity of the signature against the identities in the
// provided checkOpts.
func (c *cosignVerifier) verifyPackageSignature(ctx context.Context, packageBytes, b64Cert, b64Signature []byte) error {
	decodedCert, err := base64.StdEncoding.AppendDecode(nil, b64Cert)
	if err != nil {
		return fmt.Errorf("b64 decode cert: %w", err)
	}

	ociSig, err := static.NewSignature(packageBytes, string(b64Signature), static.WithCertChain(decodedCert, nil))
	if err != nil {
		return fmt.Errorf("create signature: %w", err)
	}

	// VerifyBlobSignature uses the provided checkOpts to verify the signature of the package.
	// Specifically it uses the public Fulcio certificates to verify the identity of the signature and
	// a Rekor client to verify the validity of the signature against a transparency log.
	_, err = cosign.VerifyBlobSignature(ctx, ociSig, c.checkOpts)
	if err != nil {
		return fmt.Errorf("verify blob: %w", err)
	}

	return nil
}
