// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"hash"
	"os"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// ---------------------------------------------------------------------------
// Config validation — HMAC-SHA256
// ---------------------------------------------------------------------------

func TestConfigValidateHMAC(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid env",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env:  &EnvKeyConfig{HMACKeyEnvVar: "MY_HMAC_KEY"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid file",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type: KeySourceFile,
					File: &FileKeyConfig{HMACKeyFile: "/etc/signing/hmac.key"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid k8s",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type:      KeySourceK8sSecret,
					K8sSecret: &K8sSecretConfig{Name: "s", HMACKey: "hmac.key"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid bao",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type: KeySourceBao,
					Bao:  &BaoKeyConfig{SecretPath: "s", HMACKeyField: "hmac"},
				},
			},
			wantErr: false,
		},
		{
			name: "env missing hmac_key_env_var",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env:  &EnvKeyConfig{},
				},
			},
			wantErr: true,
		},
		{
			name: "file missing hmac_key_file",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type: KeySourceFile,
					File: &FileKeyConfig{},
				},
			},
			wantErr: true,
		},
		{
			name: "k8s missing hmac_key",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type:      KeySourceK8sSecret,
					K8sSecret: &K8sSecretConfig{Name: "s"},
				},
			},
			wantErr: true,
		},
		{
			name: "bao missing hmac_key_field",
			cfg: Config{
				Algorithm: AlgorithmHMACSHA256,
				KeySource: KeySourceConfig{
					Type: KeySourceBao,
					Bao:  &BaoKeyConfig{SecretPath: "s"},
				},
			},
			wantErr: true,
		},
		{
			name: "certificate_ref 'full' — rejected",
			cfg: Config{
				Algorithm:      AlgorithmHMACSHA256,
				CertificateRef: "full",
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env:  &EnvKeyConfig{HMACKeyEnvVar: "K"},
				},
			},
			wantErr: true,
		},
		{
			name: "certificate_ref 'fingerprint' — also rejected (any non-empty value invalid for HMAC)",
			cfg: Config{
				Algorithm:      AlgorithmHMACSHA256,
				CertificateRef: "fingerprint",
				KeySource: KeySourceConfig{
					Type: KeySourceEnv,
					Env:  &EnvKeyConfig{HMACKeyEnvVar: "K"},
				},
			},
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

func TestConfigGetHashHMAC(t *testing.T) {
	if (&Config{Algorithm: AlgorithmHMACSHA256}).GetHash() != crypto.SHA256 {
		t.Error("HMAC-SHA256 GetHash() should return crypto.SHA256")
	}
}

// ---------------------------------------------------------------------------
// HMAC loading via env and file providers
// ---------------------------------------------------------------------------

func TestHMACProviderFromEnv(t *testing.T) {
	t.Setenv("TEST_HMAC_KEY", "super-secret-key")
	prov, err := newEnvKeyMaterialProvider(&EnvKeyConfig{HMACKeyEnvVar: "TEST_HMAC_KEY"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(prov.GetHMACKey()) != "super-secret-key" {
		t.Errorf("unexpected HMAC key: %q", string(prov.GetHMACKey()))
	}
	if prov.GetPrivateKey() != nil {
		t.Error("GetPrivateKey() should return nil for HMAC mode")
	}
	if prov.GetCertificate() != nil {
		t.Error("GetCertificate() should return nil for HMAC mode")
	}
}

func TestHMACProviderFromFile(t *testing.T) {
	f := writeTempFile(t, []byte("file-secret-key"))
	prov, err := newFileKeyMaterialProvider(&FileKeyConfig{HMACKeyFile: f})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(prov.GetHMACKey()) != "file-secret-key" {
		t.Errorf("unexpected HMAC key from file: %q", string(prov.GetHMACKey()))
	}
}

func TestHMACProviderMissingEnv(t *testing.T) {
	os.Unsetenv("MISSING_HMAC_KEY")
	_, err := newEnvKeyMaterialProvider(&EnvKeyConfig{HMACKeyEnvVar: "MISSING_HMAC_KEY"})
	if err == nil {
		t.Error("expected error for missing env var")
	}
}

func TestHMACProviderMissingFile(t *testing.T) {
	_, err := newFileKeyMaterialProvider(&FileKeyConfig{HMACKeyFile: "/no/such/file.key"})
	if err == nil {
		t.Error("expected error for missing key file")
	}
}

// ---------------------------------------------------------------------------
// HMAC-SHA256 sign + verify round-trip
// ---------------------------------------------------------------------------

func TestSignVerifyHMACSHA256(t *testing.T) {
	secret := "test-hmac-secret-32-bytes-padded!"
	t.Setenv("HMAC_TEST_KEY", secret)
	prov, err := newEnvKeyMaterialProvider(&EnvKeyConfig{HMACKeyEnvVar: "HMAC_TEST_KEY"})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	p := &signingProcessor{
		config:       &Config{Algorithm: AlgorithmHMACSHA256},
		provider:     prov,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: AlgorithmHMACSHA256,
		certRef:      "",
	}

	lr := plog.NewLogRecord()
	lr.SetEventName("user.login.success")
	lr.SetTimestamp(pcommon.Timestamp(1714041600000000000))
	lr.Attributes().PutStr("audit.actor.id", "u1")

	if err := p.processLogRecord(lr); err != nil {
		t.Fatalf("processLogRecord: %v", err)
	}

	sigVal, ok := lr.Attributes().Get("audit.integrity.value")
	if !ok {
		t.Fatal("audit.integrity.value missing")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigVal.Str())
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}

	payload, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	if !hmac.Equal(sigBytes, mac.Sum(nil)) {
		t.Error("HMAC verification failed")
	}
	t.Logf("✅ HMAC-SHA256 MAC verifies")
}

// ---------------------------------------------------------------------------
// ConsumeLogs — no audit.integrity.certificate for HMAC
// ---------------------------------------------------------------------------

func TestConsumeLogsHMACNoCertAttribute(t *testing.T) {
	t.Setenv("HMAC_NO_CERT_KEY", "secret")
	prov, _ := newEnvKeyMaterialProvider(&EnvKeyConfig{HMACKeyEnvVar: "HMAC_NO_CERT_KEY"})
	sink := &logSink{}

	p := &signingProcessor{
		config:       &Config{Algorithm: AlgorithmHMACSHA256},
		provider:     prov,
		nextLogs:     sink,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: AlgorithmHMACSHA256,
		certRef:      "",
	}

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.Timestamp(1000))

	if err := p.ConsumeLogs(context.Background(), ld); err != nil {
		t.Fatalf("ConsumeLogs: %v", err)
	}

	res := sink.logs[0].ResourceLogs().At(0).Resource().Attributes()
	algo, ok := res.Get("audit.integrity.algorithm")
	if !ok || algo.Str() != AlgorithmHMACSHA256 {
		t.Errorf("audit.integrity.algorithm: got %q, want %q", algo.Str(), AlgorithmHMACSHA256)
	}
	if _, exists := res.Get("audit.integrity.certificate"); exists {
		t.Error("audit.integrity.certificate should not be set for HMAC-SHA256")
	}
	t.Logf("✅ HMAC ConsumeLogs: algorithm set, certificate absent")
}

// ---------------------------------------------------------------------------
// Tamper detection
// ---------------------------------------------------------------------------

func TestHMACTamperedPayloadDetected(t *testing.T) {
	t.Setenv("HMAC_TAMPER_KEY", "tamper-test-secret")
	prov, _ := newEnvKeyMaterialProvider(&EnvKeyConfig{HMACKeyEnvVar: "HMAC_TAMPER_KEY"})

	p := &signingProcessor{
		config:       &Config{Algorithm: AlgorithmHMACSHA256},
		provider:     prov,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: AlgorithmHMACSHA256,
	}

	lr := plog.NewLogRecord()
	lr.SetEventName("original.event")
	lr.SetTimestamp(pcommon.Timestamp(2000000))

	if err := p.processLogRecord(lr); err != nil {
		t.Fatalf("processLogRecord: %v", err)
	}

	sigVal, _ := lr.Attributes().Get("audit.integrity.value")
	storedMAC, _ := base64.StdEncoding.DecodeString(sigVal.Str())

	lr.SetEventName("tampered.event")
	tamperedPayload, _ := p.serializeLogRecord(lr)

	mac := hmac.New(sha256.New, []byte("tamper-test-secret"))
	mac.Write(tamperedPayload)

	if hmac.Equal(storedMAC, mac.Sum(nil)) {
		t.Error("❌ tampered EventName did not change HMAC")
	} else {
		t.Logf("🔍 tampered EventName correctly produces different HMAC")
	}
}
