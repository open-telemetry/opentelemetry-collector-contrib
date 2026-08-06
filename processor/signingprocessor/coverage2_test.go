// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"hash"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// newKeyMaterialProvider dispatch — env, k8s_secret error, bao error
// ---------------------------------------------------------------------------

func TestNewKeyMaterialProviderEnv(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
	t.Setenv("NKM_CERT", string(certPEM))
	t.Setenv("NKM_KEY", string(keyPEM))

	cfg := &Config{
		Algorithm: "RS256",
		KeySource: KeySourceConfig{
			Type: KeySourceEnv,
			Env:  &EnvKeyConfig{CertEnvVar: "NKM_CERT", KeyEnvVar: "NKM_KEY"},
		},
	}
	prov, err := newKeyMaterialProvider(context.Background(), cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.GetPrivateKey() == nil || prov.GetCertificate() == nil {
		t.Error("provider returned nil key or cert")
	}
}

func TestNewKeyMaterialProviderK8sError(t *testing.T) {
	// k8s provider will fail outside a cluster — we just verify the dispatch
	// reaches the k8s branch and surfaces an error (not a panic or wrong branch).
	cfg := &Config{
		Algorithm: "RS256",
		KeySource: KeySourceConfig{
			Type: KeySourceK8sSecret,
			K8sSecret: &K8sSecretConfig{
				Name: "signing-secret", Namespace: "default",
				CertKey: "tls.crt", KeyKey: "tls.key",
			},
		},
	}
	_, err := newKeyMaterialProvider(context.Background(), cfg, zap.NewNop())
	if err == nil {
		t.Skip("k8s client unexpectedly succeeded (running inside a cluster?)")
	}
}

func TestNewKeyMaterialProviderBaoError(t *testing.T) {
	cfg := &Config{
		Algorithm: "RS256",
		KeySource: KeySourceConfig{
			Type: KeySourceBao,
			Bao: &BaoKeyConfig{
				Address:    "http://127.0.0.1:19999", // nothing listening
				SecretPath: "secret/data/signing",
				CertField:  "certificate",
				KeyField:   "private_key",
			},
		},
	}
	_, err := newKeyMaterialProvider(context.Background(), cfg, zap.NewNop())
	if err == nil {
		t.Skip("bao client unexpectedly succeeded")
	}
}

func TestNewKeyMaterialProviderUnknownType(t *testing.T) {
	cfg := &Config{
		Algorithm: "RS256",
		KeySource:     KeySourceConfig{Type: "unknown"},
	}
	_, err := newKeyMaterialProvider(context.Background(), cfg, zap.NewNop())
	if err == nil {
		t.Error("expected error for unknown key source type")
	}
}

// ---------------------------------------------------------------------------
// createLogsProcessor — invalid config type
// ---------------------------------------------------------------------------

func TestCreateLogsProcessorInvalidConfig(t *testing.T) {
	f := NewFactory()
	settings := processortest.NewNopSettings(f.Type())
	_, err := f.CreateLogs(context.Background(), settings, &struct{ x int }{}, &logSink{})
	if err == nil {
		t.Error("expected error for invalid config type")
	}
}

// ---------------------------------------------------------------------------
// parseCertificateData — PKCS8 non-RSA key (EC key)
// ---------------------------------------------------------------------------

func TestParseCertificateDataPKCS8NonRSA(t *testing.T) {
	certPEM, _, _, _ := generateTestPEM(t)

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(ecKey)
	if err != nil {
		t.Fatalf("marshal EC key: %v", err)
	}
	ecPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	_, err = parseCertificateData(certPEM, ecPEM)
	if err == nil {
		t.Error("expected error: EC key is not RSA")
	}
}

// ---------------------------------------------------------------------------
// faultyProvider — returns nil key and cert (for error-path tests)
// ---------------------------------------------------------------------------

type faultyProvider struct{}

func (f *faultyProvider) GetPrivateKey() crypto.Signer    { return nil }
func (f *faultyProvider) GetCertificate() *x509.Certificate { return nil }
func (f *faultyProvider) GetHMACKey() []byte                   { return nil }

var _ KeyMaterialProvider = (*faultyProvider)(nil)

// ---------------------------------------------------------------------------
// ConsumeLogs error path — hash.Write failure
// ---------------------------------------------------------------------------

// alwaysErrHash is a hash.Hash whose Write always returns an error.
type alwaysErrHash struct{}

func (h *alwaysErrHash) Write(_ []byte) (int, error) { return 0, errors.New("hash write error") }
func (h *alwaysErrHash) Sum(b []byte) []byte          { return b }
func (h *alwaysErrHash) Reset()                       {}
func (h *alwaysErrHash) Size() int                    { return 32 }
func (h *alwaysErrHash) BlockSize() int               { return 64 }

func TestConsumeLogsSignError(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)
	prov, _ := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: certFile, KeyFile: keyFile})

	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256"},
		provider:     prov,
		nextLogs:     &logSink{},
		hashFunc:     func() hash.Hash { return &alwaysErrHash{} },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("test")
	lr.SetTimestamp(pcommon.Timestamp(1000))

	if err := p.ConsumeLogs(context.Background(), ld); err == nil {
		t.Error("expected error from ConsumeLogs when hash.Write fails")
	}
}

// ---------------------------------------------------------------------------
// buildCertificateRef — nil certificate
// ---------------------------------------------------------------------------

func TestBuildCertificateRefNilCert(t *testing.T) {
	_, err := buildCertificateRef(&faultyProvider{}, CertificateRefFingerprint)
	if err == nil {
		t.Error("expected error when provider returns nil certificate")
	}
}

// ---------------------------------------------------------------------------
// newProcessor — key file missing propagates error
// ---------------------------------------------------------------------------

func TestNewProcessorMissingKeyFiles(t *testing.T) {
	cfg := &Config{
		Algorithm: "RS256",
		CertificateRef: CertificateRefFingerprint,
		KeySource:      KeySourceConfig{Type: KeySourceFile, File: &FileKeyConfig{CertFile: "/no/such/cert.pem", KeyFile: "/no/such/key.pem"}},
	}
	f := NewFactory()
	settings := processortest.NewNopSettings(f.Type())
	_, err := newProcessor(cfg, &logSink{}, settings)
	if err == nil {
		t.Error("expected error when key files do not exist")
	}
}

// keep crypto imported
var _ = crypto.SHA256
