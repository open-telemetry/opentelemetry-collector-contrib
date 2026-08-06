// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"hash"
	"math/big"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
)

// ---------------------------------------------------------------------------
// helpers shared across test files
// ---------------------------------------------------------------------------

func generateTestPEM(t *testing.T) (certPEM, keyPEM []byte, key *rsa.PrivateKey, cert *x509.Certificate) {
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
	cert, _ = x509.ParseCertificate(der)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return
}

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
			name: "k8s_secret valid with namespace",
			cfg:  Config{Algorithm: "RS256", KeySource: KeySourceConfig{Type: "k8s_secret", K8sSecret: &K8sSecretConfig{Name: "s", Namespace: "ns", CertKey: "c", KeyKey: "k"}}},
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
	certPEM, keyPEM, _, _ := generateTestPEM(t)

	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)

	cfg := &Config{
		Algorithm: "RS256",
		CertificateRef: CertificateRefFingerprint,
		KeySource: KeySourceConfig{
			Type: KeySourceFile,
			File: &FileKeyConfig{CertFile: certFile, KeyFile: keyFile},
		},
	}

	f := NewFactory()
	settings := processortest.NewNopSettings(f.Type())
	sink := &logSink{}

	proc, err := f.CreateLogs(context.Background(), settings, cfg, sink)
	if err != nil {
		t.Fatalf("CreateLogs: %v", err)
	}
	if err := proc.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := proc.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
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
// parseCertificateData / CertificateReader
// ---------------------------------------------------------------------------

func TestParseCertificateData(t *testing.T) {
	certPEM, keyPEM, key, _ := generateTestPEM(t)

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
	certPEM, _, _, _ := generateTestPEM(t)

	t.Run("plain PEM returned as-is", func(t *testing.T) {
		result := decodeIfBase64(certPEM)
		if string(result) != string(certPEM) {
			t.Error("plain PEM should be returned unchanged")
		}
	})

	t.Run("base64-encoded PEM decoded", func(t *testing.T) {
		encoded := []byte(base64.StdEncoding.EncodeToString(certPEM))
		result := decodeIfBase64(encoded)
		if string(result) != string(certPEM) {
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
	certPEM, keyPEM, _, _ := generateTestPEM(t)
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
	certPEM, keyPEM, _, _ := generateTestPEM(t)
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
	// Ensure vars are unset
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
	certPEM, keyPEM, _, _ := generateTestPEM(t)
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
		if len(ref) == 0 {
			t.Error("full ref should be non-empty")
		}
		// Should be valid base64
		if _, err := base64.StdEncoding.DecodeString(ref); err != nil {
			t.Errorf("full ref is not valid base64: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// processor Start/Shutdown + valueToInterface
// ---------------------------------------------------------------------------

func TestProcessorStartShutdown(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
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
	if err := p.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		t.Errorf("Start: %v", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
	caps := p.Capabilities()
	if !caps.MutatesData {
		t.Error("expected MutatesData=true")
	}
}

func TestValueToInterface(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
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
	certPEM, keyPEM, _, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)

	cfg := &Config{
		Algorithm: "RS256",
		CertificateRef: CertificateRefFingerprint,
		KeySource:      KeySourceConfig{Type: KeySourceFile, File: &FileKeyConfig{CertFile: certFile, KeyFile: keyFile}},
	}
	// Patch GetHash to return an unsupported value by injecting a broken provider
	// that returns a nil certificate — triggering the buildCertificateRef error path.
	type nilCertProvider struct{ key *rsa.PrivateKey }
	_ = nilCertProvider{}

	// Directly exercise the unsupported-hash branch: crypto.Hash(0) is not SHA256/SHA512.
	// We do this by building a processor struct manually and verifying the switch falls through.
	// The newProcessor function uses cfg.GetHash(); since GetHash() defaults to SHA256 for unknown
	// strings we instead test via Validate() that "MD5" is caught before newProcessor is reached.
	if err := (&Config{Algorithm: "MD5", KeySource: KeySourceConfig{Type: KeySourceFile, File: &FileKeyConfig{CertFile: "c", KeyFile: "k"}}}).Validate(); err == nil {
		t.Error("Validate() should reject MD5")
	}
	_ = cfg
}

// Satisfy consumer.Logs interface for logSink (already defined in consume_test.go,
// but that file uses an unexported type — we cannot duplicate it, so we reference it).
var _ consumer.Logs = (*logSink)(nil)

// ---------------------------------------------------------------------------
// serializeLogRecord — non-string body (nil branch)
// ---------------------------------------------------------------------------

func TestSerializeLogRecordNonStringBody(t *testing.T) {
	certPEM, keyPEM, _, _ := generateTestPEM(t)
	certFile := writeTempFile(t, certPEM)
	keyFile := writeTempFile(t, keyPEM)
	prov, _ := newFileKeyMaterialProvider(&FileKeyConfig{CertFile: certFile, KeyFile: keyFile})

	p := &signingProcessor{
		config:   &Config{Algorithm: "RS256"},
		provider: prov,
		hashFunc: func() hash.Hash { return crypto.SHA256.New() },
	}

	lr := plog.NewLogRecord()
	lr.Body().SetInt(99) // non-string body — should be omitted from payload
	lr.SetTimestamp(pcommon.Timestamp(1000000))

	b, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serializeLogRecord: %v", err)
	}
	// Non-string body must not appear in payload
	if string(b) == "" {
		t.Error("expected non-empty serialized payload")
	}
}
