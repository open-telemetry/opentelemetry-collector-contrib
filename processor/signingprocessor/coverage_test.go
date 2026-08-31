// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"hash"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// helpers shared across test files
// ---------------------------------------------------------------------------

func generateTestPEM(t *testing.T) (certPEM, keyPEM []byte, key *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM, key
}

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.pem")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

// ---------------------------------------------------------------------------
// log sink (used by consume and coverage2 tests)
// ---------------------------------------------------------------------------

type logSink struct{ logs []plog.Logs }

func (s *logSink) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	s.logs = append(s.logs, ld)
	return nil
}

func (*logSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

var _ consumer.Logs = (*logSink)(nil)

// ---------------------------------------------------------------------------
// Config.Validate
// ---------------------------------------------------------------------------

func TestConfigValidate(t *testing.T) {
	validFile := &FileKeyConfig{CertFile: "c.pem", KeyFile: "k.pem"}

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "valid file defaults",
			cfg:     Config{Algorithm: "RS256", CertificateRef: "fingerprint", KeySource: KeySourceConfig{Type: "file", File: validFile}},
			wantErr: false,
		},
		{
			name:    "valid SHA512 full",
			cfg:     Config{Algorithm: "RS512", CertificateRef: "full", KeySource: KeySourceConfig{Type: "file", File: validFile}},
			wantErr: false,
		},
		{
			name:    "bad hash algorithm",
			cfg:     Config{Algorithm: "MD5", KeySource: KeySourceConfig{Type: "file", File: validFile}},
			wantErr: true,
		},
		{
			name:    "bad certificate_ref",
			cfg:     Config{Algorithm: "RS256", CertificateRef: "base64", KeySource: KeySourceConfig{Type: "file", File: validFile}},
			wantErr: true,
		},
		{
			name:    "invalid key_source type",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "vault"}},
			wantErr: true,
		},
		{
			name:    "k8s_secret missing config block",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "k8s_secret"}},
			wantErr: true,
		},
		{
			name:    "k8s_secret missing name",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "k8s_secret", K8sSecret: &K8sSecretConfig{CertKey: "c", KeyKey: "k"}}},
			wantErr: true,
		},
		{
			name:    "k8s_secret missing cert_key",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "k8s_secret", K8sSecret: &K8sSecretConfig{Name: "s", KeyKey: "k"}}},
			wantErr: true,
		},
		{
			name:    "k8s_secret missing key_key",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "k8s_secret", K8sSecret: &K8sSecretConfig{Name: "s", CertKey: "c"}}},
			wantErr: true,
		},
		{
			name:    "k8s_secret valid with namespace",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "k8s_secret", K8sSecret: &K8sSecretConfig{Name: "s", Namespace: "ns", CertKey: "c", KeyKey: "k"}}},
			wantErr: false,
		},
		{
			name:    "env missing config block",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "env"}},
			wantErr: true,
		},
		{
			name:    "env missing cert_env_var",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "env", Env: &EnvKeyConfig{KeyEnvVar: "K"}}},
			wantErr: true,
		},
		{
			name:    "env missing key_env_var",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "env", Env: &EnvKeyConfig{CertEnvVar: "C"}}},
			wantErr: true,
		},
		{
			name:    "file missing config block",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "file"}},
			wantErr: true,
		},
		{
			name:    "file missing cert_file",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "file", File: &FileKeyConfig{KeyFile: "k.pem"}}},
			wantErr: true,
		},
		{
			name:    "file missing key_file",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "file", File: &FileKeyConfig{CertFile: "c.pem"}}},
			wantErr: true,
		},
		{
			name:    "bao missing config block",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "bao"}},
			wantErr: true,
		},
		{
			name:    "bao missing secret_path",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "bao", Bao: &BaoKeyConfig{CertField: "c", KeyField: "k"}}},
			wantErr: true,
		},
		{
			name:    "bao missing cert_field",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "bao", Bao: &BaoKeyConfig{SecretPath: "s", KeyField: "k"}}},
			wantErr: true,
		},
		{
			name:    "bao missing key_field",
			cfg:     Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "bao", Bao: &BaoKeyConfig{SecretPath: "s", CertField: "c"}}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	if f == nil {
		t.Fatal("NewFactory returned nil")
	}
	cfg := f.CreateDefaultConfig()
	if cfg == nil {
		t.Fatal("CreateDefaultConfig returned nil")
	}
	c := cfg.(*Config)
	if c.Algorithm != "RS256" {
		t.Errorf("default algorithm: got %q, want RS256", c.Algorithm)
	}
	if c.CertificateRef != "fingerprint" {
		t.Errorf("default cert ref: got %q, want fingerprint", c.CertificateRef)
	}
}

func TestCreateLogsProcessorFileProvider(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)

	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)

	cfg := &Config{
		Algorithm:      "RS256",
		CertificateRef: CertificateRefFingerprint,
		KeySource: KeySourceConfig{
			Type: KeySourceFile,
			File: &FileKeyConfig{CertFile: certFile, KeyFile: keyFile},
		},
	}

	f := NewFactory()
	settings := processortest.NewNopSettings(f.Type())
	sink := &logSink{}

	proc, err := f.CreateLogs(t.Context(), settings, cfg, sink)
	if err != nil {
		t.Fatalf("CreateLogs: %v", err)
	}
	if err := proc.Start(t.Context(), componenttest.NewNopHost()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := proc.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseCertificateData / CertificateReader
// ---------------------------------------------------------------------------

func TestParseCertificateData(t *testing.T) {
	certPEM, keyPEM, key := generateTestPEM(t)

	t.Run("valid PKCS1 key", func(t *testing.T) {
		cr, err := parseCertificateData(certPEM, keyPEM)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cr.GetPrivateKey() == nil {
			t.Error("private key is nil")
		}
		if cr.GetCertificate() == nil {
			t.Error("certificate is nil")
		}
		_ = key
	})

	t.Run("valid PKCS8 key", func(t *testing.T) {
		der, _ := x509.MarshalPKCS8PrivateKey(key)
		pkcs8PEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		cr, err := parseCertificateData(certPEM, pkcs8PEM)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cr.GetPrivateKey() == nil {
			t.Error("private key is nil")
		}
	})

	t.Run("empty cert", func(t *testing.T) {
		_, err := parseCertificateData([]byte{}, keyPEM)
		if err == nil {
			t.Error("expected error for empty cert")
		}
	})

	t.Run("empty key", func(t *testing.T) {
		_, err := parseCertificateData(certPEM, []byte{})
		if err == nil {
			t.Error("expected error for empty key")
		}
	})

	t.Run("non-PEM cert", func(t *testing.T) {
		_, err := parseCertificateData([]byte("not pem"), keyPEM)
		if err == nil {
			t.Error("expected error for non-PEM cert")
		}
	})

	t.Run("non-PEM key", func(t *testing.T) {
		_, err := parseCertificateData(certPEM, []byte("not pem"))
		if err == nil {
			t.Error("expected error for non-PEM key")
		}
	})

	t.Run("garbled PEM cert", func(t *testing.T) {
		bad := []byte("-----BEGIN CERTIFICATE-----\ngarbage\n-----END CERTIFICATE-----\n")
		_, err := parseCertificateData(bad, keyPEM)
		if err == nil {
			t.Error("expected error for garbled cert")
		}
	})

	t.Run("unsupported key type", func(t *testing.T) {
		bad := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("garbage")})
		_, err := parseCertificateData(certPEM, bad)
		if err == nil {
			t.Error("expected error for unsupported key type")
		}
	})
}

func TestDecodeIfBase64(t *testing.T) {
	certPEM, _, _ := generateTestPEM(t)

	t.Run("plain PEM returned as-is", func(t *testing.T) {
		result := decodeIfBase64(certPEM)
		if !bytes.Equal(result, certPEM) {
			t.Error("plain PEM should be returned unchanged")
		}
	})

	t.Run("base64-encoded PEM decoded", func(t *testing.T) {
		encoded := []byte(base64.StdEncoding.EncodeToString(certPEM))
		result := decodeIfBase64(encoded)
		if !bytes.Equal(result, certPEM) {
			t.Errorf("base64 decode failed: got %q", result[:20])
		}
	})

	t.Run("empty input", func(t *testing.T) {
		result := decodeIfBase64([]byte{})
		if len(result) != 0 {
			t.Error("empty input should return empty")
		}
	})
}

func TestNormalizeLineEndings(t *testing.T) {
	input := []byte("line1\r\nline2\rline3\n")
	got := string(normalizeLineEndings(input))
	want := "line1\nline2\nline3\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// file provider
// ---------------------------------------------------------------------------

func TestFileKeyMaterialProvider(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)

	prov, err := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: certFile, KeyFile: keyFile})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.GetPrivateKey() == nil {
		t.Error("private key is nil")
	}
	if prov.GetCertificate() == nil {
		t.Error("certificate is nil")
	}
}

func TestFileKeyMaterialProviderMissingFiles(t *testing.T) {
	_, err := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: "/nonexistent/cert.pem", KeyFile: "/nonexistent/key.pem"})
	if err == nil {
		t.Error("expected error for missing cert file")
	}
}

// ---------------------------------------------------------------------------
// env provider
// ---------------------------------------------------------------------------

func TestEnvKeyMaterialProvider(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
	t.Setenv("TEST_CERT", string(certPEM))
	t.Setenv("TEST_KEY", string(keyPEM))

	prov, err := newEnvKeyMaterialProvider(&EnvKeyConfig{CertEnvVar: "TEST_CERT", KeyEnvVar: "TEST_KEY"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.GetPrivateKey() == nil {
		t.Error("private key is nil")
	}
	if prov.GetCertificate() == nil {
		t.Error("certificate is nil")
	}
}

func TestEnvKeyMaterialProviderMissingEnv(t *testing.T) {
	os.Unsetenv("MISSING_CERT")
	os.Unsetenv("MISSING_KEY")
	_, err := newEnvKeyMaterialProvider(&EnvKeyConfig{CertEnvVar: "MISSING_CERT", KeyEnvVar: "MISSING_KEY"})
	if err == nil {
		t.Error("expected error for missing env vars")
	}
}

// ---------------------------------------------------------------------------
// buildCertificateRef
// ---------------------------------------------------------------------------

func TestBuildCertificateRef(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)
	prov, _ := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: certFile, KeyFile: keyFile})

	t.Run("fingerprint", func(t *testing.T) {
		ref, err := buildCertificateRef(prov, CertificateRefFingerprint)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ref) < 7 || ref[:7] != "sha256:" {
			t.Errorf("fingerprint should start with sha256:, got %q", ref[:min(20, len(ref))])
		}
	})

	t.Run("full", func(t *testing.T) {
		ref, err := buildCertificateRef(prov, CertificateRefFull)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ref == "" {
			t.Error("full ref should be non-empty")
		}
		if _, err := base64.StdEncoding.DecodeString(ref); err != nil {
			t.Errorf("full ref is not valid base64: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// processor Start/Shutdown + valueToInterface
// ---------------------------------------------------------------------------

func TestProcessorStartShutdown(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)
	prov, _ := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: certFile, KeyFile: keyFile})

	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		nextLogs:     &logSink{},
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}
	if err := p.Start(t.Context(), componenttest.NewNopHost()); err != nil {
		t.Errorf("Start: %v", err)
	}
	if err := p.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	caps := p.Capabilities()
	if !caps.MutatesData {
		t.Error("expected MutatesData=true")
	}
}

func TestValueToInterface(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)
	prov, _ := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: certFile, KeyFile: keyFile})

	p := &signingProcessor{
		config:   &Config{Algorithm: "RS256"},
		provider: prov,
		hashFunc: func() hash.Hash { return crypto.SHA256.New() },
	}

	tests := []struct {
		name  string
		setup func(v pcommon.Value)
	}{
		{"string", func(v pcommon.Value) { v.SetStr("hello") }},
		{"int", func(v pcommon.Value) { v.SetInt(42) }},
		{"double", func(v pcommon.Value) { v.SetDouble(3.14) }},
		{"bool", func(v pcommon.Value) { v.SetBool(true) }},
		{"bytes", func(v pcommon.Value) { v.SetEmptyBytes().Append(1, 2, 3) }},
		{"slice", func(v pcommon.Value) {
			s := v.SetEmptySlice()
			s.AppendEmpty().SetStr("a")
		}},
		{"map", func(v pcommon.Value) {
			m := v.SetEmptyMap()
			m.PutStr("k", "v")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := pcommon.NewValueEmpty()
			tt.setup(v)
			result := p.valueToInterface(v)
			if result == nil {
				t.Error("valueToInterface returned nil for non-empty value")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// newProcessor error paths
// ---------------------------------------------------------------------------

func TestNewProcessorUnsupportedHash(t *testing.T) {
	if err := (&Config{Algorithm: "MD5", KeySource: KeySourceConfig{Type: KeySourceFile, File: &FileKeyConfig{CertFile: "c", KeyFile: "k"}}}).Validate(); err == nil {
		t.Error("Validate() should reject MD5")
	}
}

// ---------------------------------------------------------------------------
// serializeLogRecord — non-string body (nil branch)
// ---------------------------------------------------------------------------

func TestSerializeLogRecordNonStringBody(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)
	prov, _ := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: certFile, KeyFile: keyFile})

	p := &signingProcessor{
		config:   &Config{Algorithm: "RS256"},
		provider: prov,
		hashFunc: func() hash.Hash { return crypto.SHA256.New() },
	}

	lr := plog.NewLogRecord()
	lr.Body().SetInt(99)
	lr.SetTimestamp(pcommon.Timestamp(1000000))

	b, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serializeLogRecord: %v", err)
	}
	if len(b) == 0 {
		t.Error("expected non-empty serialized payload")
	}
}

// ---------------------------------------------------------------------------
// TestLoadConfig (config_test.go)
// ---------------------------------------------------------------------------

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected *Config
	}{
		{
			id: component.NewID(component.MustNewType("signing")),
			expected: &Config{
				Algorithm:      "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceFile,
					File: &FileKeyConfig{
						CertFile: "/etc/otelcol/signing-cert.pem",
						KeyFile:  "/etc/otelcol/signing-key.pem",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "sha512_full"),
			expected: &Config{
				Algorithm:      "RS512",
				CertificateRef: "full",
				KeySource: KeySourceConfig{
					Type: KeySourceFile,
					File: &FileKeyConfig{
						CertFile: "/etc/otelcol/signing-cert.pem",
						KeyFile:  "/etc/otelcol/signing-key.pem",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "env"),
			expected: &Config{
				Algorithm:      "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env: &EnvKeyConfig{
						CertEnvVar: "SIGNING_CERT_PEM",
						KeyEnvVar:  "SIGNING_KEY_PEM",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "k8s"),
			expected: &Config{
				Algorithm:      "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceK8sSecret,
					K8sSecret: &K8sSecretConfig{
						Name:      "signing-secret",
						Namespace: "default",
						CertKey:   "tls.crt",
						KeyKey:    "tls.key",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "bao"),
			expected: &Config{
				Algorithm:      "RS256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceBao,
					Bao: &BaoKeyConfig{
						Address:    "https://bao.example.com",
						SecretPath: "secret/data/signing",
						CertField:  "certificate",
						KeyField:   "private_key",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(component.MustNewType("signing"), "hmac"),
			expected: &Config{
				Algorithm:      "HMAC-SHA256",
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env:  &EnvKeyConfig{HMACKeyEnvVar: "SIGNING_HMAC_KEY"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_invalid.yaml"))
	require.NoError(t, err)

	invalidIDs := []string{
		"signing/missing_key_source",
		"signing/bad_hash",
		"signing/bad_cert_ref",
		"signing/file_missing_cert",
		"signing/env_missing_key",
		"signing/k8s_missing_name",
		"signing/bao_missing_path",
	}

	for _, id := range invalidIDs {
		t.Run(id, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(id)
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.Error(t, cfg.(*Config).Validate(), "expected Validate() to fail for %s", id)
		})
	}
}

// ---------------------------------------------------------------------------
// ConsumeLogs — resource attribute injection (consume_test.go)
// ---------------------------------------------------------------------------

func TestConsumeLogsResourceAttrs(t *testing.T) {
	prov := newTestProvider(t)
	sink := &logSink{}
	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		nextLogs:     sink,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:abc",
	}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr("test")
	lr.Attributes().PutStr("audit.actor.id", "u1")

	if err := p.ConsumeLogs(t.Context(), ld); err != nil {
		t.Fatalf("ConsumeLogs: %v", err)
	}

	if len(sink.logs) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(sink.logs))
	}
	res := sink.logs[0].ResourceLogs().At(0).Resource().Attributes()
	algo, ok := res.Get("audit.integrity.algorithm")
	if !ok || algo.Str() != "RS256" {
		t.Errorf("audit.integrity.algorithm: got %q, want RS256", algo.Str())
	}
	certRef, ok2 := res.Get("audit.integrity.certificate")
	if !ok2 || certRef.Str() != "sha256:abc" {
		t.Errorf("audit.integrity.certificate: got %q, want sha256:abc", certRef.Str())
	}
	t.Logf("✅ resource attrs: algorithm=%s certificate=%s", algo.Str(), certRef.Str())

	rec := sink.logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	if _, ok4 := rec.Attributes().Get("audit.integrity.value"); !ok4 {
		t.Error("audit.integrity.value missing from log record")
	}
	t.Logf("✅ record integrity attrs present")

	verifyRecord(t, rec, &prov.key.PublicKey)
	t.Logf("✅ signature verifies against public key")
}

// ---------------------------------------------------------------------------
// newKeyMaterialProvider dispatch (coverage2_test.go)
// ---------------------------------------------------------------------------

func TestNewKeyMaterialProviderEnv(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
	t.Setenv("NKM_CERT", string(certPEM))
	t.Setenv("NKM_KEY", string(keyPEM))

	cfg := &Config{
		Algorithm: "RS256",
		KeySource: KeySourceConfig{
			Type: KeySourceEnv,
			Env:  &EnvKeyConfig{CertEnvVar: "NKM_CERT", KeyEnvVar: "NKM_KEY"},
		},
	}
	prov, err := newKeyMaterialProvider(t.Context(), cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.GetPrivateKey() == nil || prov.GetCertificate() == nil {
		t.Error("provider returned nil key or cert")
	}
}

func TestNewKeyMaterialProviderK8sError(t *testing.T) {
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
	_, err := newKeyMaterialProvider(t.Context(), cfg, zap.NewNop())
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
				Address:    "http://127.0.0.1:19999",
				SecretPath: "secret/data/signing",
				CertField:  "certificate",
				KeyField:   "private_key",
			},
		},
	}
	_, err := newKeyMaterialProvider(t.Context(), cfg, zap.NewNop())
	if err == nil {
		t.Skip("bao client unexpectedly succeeded")
	}
}

func TestNewKeyMaterialProviderUnknownType(t *testing.T) {
	cfg := &Config{
		Algorithm: "RS256",
		KeySource: KeySourceConfig{Type: "unknown"},
	}
	_, err := newKeyMaterialProvider(t.Context(), cfg, zap.NewNop())
	if err == nil {
		t.Error("expected error for unknown key source type")
	}
}

// ---------------------------------------------------------------------------
// createLogsProcessor — invalid config type (coverage2_test.go)
// ---------------------------------------------------------------------------

func TestCreateLogsProcessorInvalidConfig(t *testing.T) {
	f := NewFactory()
	settings := processortest.NewNopSettings(f.Type())
	_, err := f.CreateLogs(t.Context(), settings, &struct{ x int }{}, &logSink{})
	if err == nil {
		t.Error("expected error for invalid config type")
	}
}

// ---------------------------------------------------------------------------
// parseCertificateData — PKCS8 non-RSA key (coverage2_test.go)
// ---------------------------------------------------------------------------

func TestParseCertificateDataPKCS8NonRSA(t *testing.T) {
	certPEM, _, _ := generateTestPEM(t)

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
// faultyProvider — returns nil key and cert
// ---------------------------------------------------------------------------

type faultyProvider struct{}

func (*faultyProvider) GetPrivateKey() crypto.Signer      { return nil }
func (*faultyProvider) GetCertificate() *x509.Certificate { return nil }
func (*faultyProvider) GetHMACKey() []byte                { return nil }

var _ KeyMaterialProvider = (*faultyProvider)(nil)

// ---------------------------------------------------------------------------
// ConsumeLogs error path — hash.Write failure (coverage2_test.go)
// ---------------------------------------------------------------------------

type alwaysErrHash struct{}

func (*alwaysErrHash) Write(_ []byte) (int, error) { return 0, errors.New("hash write error") }
func (*alwaysErrHash) Sum(b []byte) []byte         { return b }
func (*alwaysErrHash) Reset()                      {}
func (*alwaysErrHash) Size() int                   { return 32 }
func (*alwaysErrHash) BlockSize() int              { return 64 }

func TestConsumeLogsSignError(t *testing.T) {
	certPEM, keyPEM, _ := generateTestPEM(t)
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

	if err := p.ConsumeLogs(t.Context(), ld); err == nil {
		t.Error("expected error from ConsumeLogs when hash.Write fails")
	}
}

// ---------------------------------------------------------------------------
// buildCertificateRef — nil certificate (coverage2_test.go)
// ---------------------------------------------------------------------------

func TestBuildCertificateRefNilCert(t *testing.T) {
	_, err := buildCertificateRef(&faultyProvider{}, CertificateRefFingerprint)
	if err == nil {
		t.Error("expected error when provider returns nil certificate")
	}
}

// ---------------------------------------------------------------------------
// newProcessor — key file missing propagates error (coverage2_test.go)
// ---------------------------------------------------------------------------

func TestNewProcessorMissingKeyFiles(t *testing.T) {
	cfg := &Config{
		Algorithm:      "RS256",
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
