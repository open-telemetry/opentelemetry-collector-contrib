// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// The fixtures are the real Sigstore bundle and sha256 published for
// otelcol-contrib v0.158.0 (linux/amd64) plus a snapshot of the public
// Sigstore trusted root, so verification runs offline.
const testArtifact = "otelcol-contrib_0.158.0_linux_amd64.tar.gz"

func loadFixtures(t *testing.T) (sig, digest []byte, trustedRoot root.TrustedMaterial) {
	t.Helper()
	sig, err := os.ReadFile(filepath.Join("testdata", testArtifact+".sigstore.json"))
	require.NoError(t, err)
	shaFile, err := os.ReadFile(filepath.Join("testdata", testArtifact+".sha256"))
	require.NoError(t, err)
	digest, err = hex.DecodeString(string(shaFile[:64]))
	require.NoError(t, err)
	rootJSON, err := os.ReadFile(filepath.Join("testdata", "trusted_root.json"))
	require.NoError(t, err)
	trustedRoot, err = root.NewTrustedRootFromJSON(rootJSON)
	require.NoError(t, err)
	return sig, digest, trustedRoot
}

func TestCosignVerifier(t *testing.T) {
	sig, digest, trustedRoot := loadFixtures(t)
	defaultCfg := config.DefaultSupervisor().Agent.Package.Verifier.Cosign
	wrongDigest := append([]byte{}, digest...)
	wrongDigest[0] ^= 1

	testCases := []struct {
		name        string
		cfg         func(config.CosignSignatureVerifier) config.CosignSignatureVerifier
		sig         []byte
		digest      []byte
		expectedErr string
	}{
		{
			name:   "default config accepts release bundle",
			digest: digest,
		},
		{
			name:   "repository check can be disabled",
			digest: digest,
			cfg: func(c config.CosignSignatureVerifier) config.CosignSignatureVerifier {
				c.CertGithubWorkflowRepository = ""
				return c
			},
		},
		{
			name:   "any identity may match",
			digest: digest,
			cfg: func(c config.CosignSignatureVerifier) config.CosignSignatureVerifier {
				c.Identities = append([]config.AgentSignatureIdentity{{
					Issuer:  "https://accounts.google.com",
					Subject: "someone@example.com",
				}}, c.Identities...)
				return c
			},
		},
		{
			name:        "tampered artifact",
			digest:      wrongDigest,
			expectedErr: "artifact does not match digest",
		},
		{
			name:   "repository mismatch",
			digest: digest,
			cfg: func(c config.CosignSignatureVerifier) config.CosignSignatureVerifier {
				c.CertGithubWorkflowRepository = "example/fork"
				return c
			},
			expectedErr: "expected GithubWorkflowRepository to be \"example/fork\"",
		},
		{
			name:   "subject mismatch",
			digest: digest,
			cfg: func(c config.CosignSignatureVerifier) config.CosignSignatureVerifier {
				c.Identities = []config.AgentSignatureIdentity{{
					Issuer:  "https://token.actions.githubusercontent.com",
					Subject: "https://github.com/example/fork/.github/workflows/release.yaml@refs/tags/v1.0.0",
				}}
				return c
			},
			expectedErr: "no matching CertificateIdentity found",
		},
		{
			name:   "issuer mismatch",
			digest: digest,
			cfg: func(c config.CosignSignatureVerifier) config.CosignSignatureVerifier {
				c.Identities[0].Issuer = "https://accounts.google.com"
				return c
			},
			expectedErr: "no matching CertificateIdentity found",
		},
		{
			name:        "malformed bundle",
			digest:      digest,
			sig:         []byte("b64_certificate b64_signature"),
			expectedErr: "parse sigstore bundle",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaultCfg
			// copy so per-case mutation does not leak between cases
			cfg.Identities = append([]config.AgentSignatureIdentity{}, defaultCfg.Identities...)
			if tc.cfg != nil {
				cfg = tc.cfg(cfg)
			}
			v, err := newCosignVerifierWithTrustedMaterial(cfg, trustedRoot)
			require.NoError(t, err)
			assert.Equal(t, config.VerifierTypeCosign, v.Type())

			testSig := sig
			if tc.sig != nil {
				testSig = tc.sig
			}
			err = v.(*cosignVerifier).verify(testSig, verify.WithArtifactDigest("sha256", tc.digest))
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCosignVerifier_VerifyRejectsWrongPackage(t *testing.T) {
	sig, _, trustedRoot := loadFixtures(t)
	v, err := newCosignVerifierWithTrustedMaterial(config.DefaultSupervisor().Agent.Package.Verifier.Cosign, trustedRoot)
	require.NoError(t, err)
	require.ErrorContains(t, v.Verify([]byte("not the release tarball"), sig), "verify sigstore bundle: failed to verify signature")
}

func TestNewCosignVerifier_InvalidRegex(t *testing.T) {
	_, _, trustedRoot := loadFixtures(t)
	cfg := config.CosignSignatureVerifier{
		Identities: []config.AgentSignatureIdentity{{Issuer: "https://token.actions.githubusercontent.com", SubjectRegExp: "^(unclosed"}},
	}
	_, err := newCosignVerifierWithTrustedMaterial(cfg, trustedRoot)
	require.ErrorContains(t, err, "identities[0] subject")
}
