package supervisor

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/sigstore/cosign/v2/cmd/cosign/cli/fulcio"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
)

func downloadPackageContent(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return nil, fmt.Errorf("got non-200 status: %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	return b, nil
}

func verifyPackageIntegrity(packageBytes, expectedHash []byte) error {
	actualHash := sha256.Sum256(packageBytes)
	if subtle.ConstantTimeCompare(actualHash[:], expectedHash) == 0 {
		return errors.New("invalid hash for package")
	}

	return nil
}

// sig is the decoded signature of
func verifyPackageSignature(packageBytes, b64Cert, b64Signature []byte) error {
	// TODO: allow specifying from config
	rootCerts, err := fulcio.GetRoots()
	if err != nil {
		return fmt.Errorf("fetch root certs: %w", err)
	}

	// TODO: allow specifying from config
	intermediateCerts, err := fulcio.GetIntermediates()
	if err != nil {
		return fmt.Errorf("fetch intermediate certs: %w", err)
	}

	co := &cosign.CheckOpts{
		RootCerts:         rootCerts,
		IntermediateCerts: intermediateCerts,
		// TODO: Make configurable
		CertGithubWorkflowRepository: "open-telemetry/opentelemetry-collector-releases",
		// TODO: Make allowed identities configurable
		Identities: []cosign.Identity{
			{
				SubjectRegExp: `^https://github.com/open-telemetry/opentelemetry-collector-releases/.github/workflows/base-release.yaml@refs/tags/[^/]*$`,
				Issuer:        "https://token.actions.githubusercontent.com",
			},
		},
	}

	decodedCert, err := base64.StdEncoding.AppendDecode(nil, b64Cert)
	if err != nil {
		return fmt.Errorf("b64 decode cert: %w", err)
	}

	// TODO: Should cert chain here be settable? Where should it come from?
	ociSig, err := static.NewSignature(packageBytes, string(b64Signature), static.WithCertChain(decodedCert, nil))
	if err != nil {
		return fmt.Errorf("create signature: %w", err)
	}

	_, err = cosign.VerifyBlobSignature(context.TODO(), ociSig, co)
	if err != nil {
		return fmt.Errorf("verify blob: %w", err)
	}

	return nil
}
