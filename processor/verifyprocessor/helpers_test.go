// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

func TestConfigValidate(t *testing.T) {
	t.Run("defaults filled", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceEnv, Env: &EnvKeyConfig{HMACKey: "k"}}}
		require.NoError(t, cfg.Validate())
		assert.Equal(t, modeSync, cfg.Mode)
		assert.Equal(t, failureModeStrict, cfg.FailureMode)
		assert.Equal(t, defaultProfile, cfg.VerificationProfile)
	})

	t.Run("invalid mode", func(t *testing.T) {
		cfg := &Config{Mode: "async", KeySource: KeySourceConfig{Type: keySourceEnv, Env: &EnvKeyConfig{HMACKey: "k"}}}
		assert.ErrorIs(t, cfg.Validate(), errInvalidMode)
	})

	t.Run("invalid failure mode", func(t *testing.T) {
		cfg := &Config{FailureMode: "ignore", KeySource: KeySourceConfig{Type: keySourceEnv, Env: &EnvKeyConfig{HMACKey: "k"}}}
		assert.ErrorIs(t, cfg.Validate(), errInvalidFailureMode)
	})

	t.Run("unknown key source", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: "vault"}}
		assert.ErrorIs(t, cfg.Validate(), errInvalidKeySourceType)
	})

	t.Run("env missing block", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceEnv}}
		assert.ErrorIs(t, cfg.Validate(), errMissingKeySourceConfig)
	})

	t.Run("file missing block", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceFile}}
		assert.ErrorIs(t, cfg.Validate(), errMissingKeySourceConfig)
	})

	t.Run("k8s missing block", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceK8sSecret}}
		assert.ErrorIs(t, cfg.Validate(), errMissingKeySourceConfig)
	})

	t.Run("k8s missing name", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceK8sSecret, K8sSecret: &K8sSecretConfig{SecretConfig: SecretConfig{HMACKey: "k"}}}}
		require.Error(t, cfg.Validate())
	})

	t.Run("k8s default namespace", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceK8sSecret, K8sSecret: &K8sSecretConfig{Name: "sec", SecretConfig: SecretConfig{HMACKey: "k"}}}}
		require.NoError(t, cfg.Validate())
		assert.Equal(t, "default", cfg.KeySource.K8sSecret.Namespace)
	})

	t.Run("bao missing block", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceBao}}
		assert.ErrorIs(t, cfg.Validate(), errMissingKeySourceConfig)
	})

	t.Run("bao missing secret path", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceBao, Bao: &BaoKeyConfig{SecretConfig: SecretConfig{HMACKey: "k"}}}}
		require.Error(t, cfg.Validate())
	})

	t.Run("bao default mount", func(t *testing.T) {
		cfg := &Config{KeySource: KeySourceConfig{Type: keySourceBao, Bao: &BaoKeyConfig{SecretPath: "verify", SecretConfig: SecretConfig{HMACKey: "k"}}}}
		require.NoError(t, cfg.Validate())
		assert.Equal(t, defaultBaoMountPath, cfg.KeySource.Bao.MountPath)
	})

	t.Run("dead letter needs storage", func(t *testing.T) {
		cfg := &Config{
			KeySource:  KeySourceConfig{Type: keySourceEnv, Env: &EnvKeyConfig{HMACKey: "k"}},
			DeadLetter: DeadLetterConfig{Enabled: true},
		}
		assert.ErrorIs(t, cfg.Validate(), errDeadLetterNeedsStorage)
	})

	t.Run("dead letter defaults and modes", func(t *testing.T) {
		cfg := &Config{
			KeySource: KeySourceConfig{Type: keySourceEnv, Env: &EnvKeyConfig{HMACKey: "k"}},
			DeadLetter: DeadLetterConfig{
				Enabled:      true,
				StorageID:    component.MustNewID("file"),
				FailureModes: []string{failureModeStrict, failureModeDrop},
			},
		}
		require.NoError(t, cfg.Validate())
		assert.Equal(t, defaultDeadLetterKeyPrefix, cfg.DeadLetter.KeyPrefix)
	})

	t.Run("dead letter invalid mode", func(t *testing.T) {
		cfg := &Config{
			KeySource: KeySourceConfig{Type: keySourceEnv, Env: &EnvKeyConfig{HMACKey: "k"}},
			DeadLetter: DeadLetterConfig{
				Enabled:      true,
				StorageID:    component.MustNewID("file"),
				FailureModes: []string{"pass"},
			},
		}
		assert.ErrorIs(t, cfg.Validate(), errInvalidDeadLetterMode)
	})
}

func TestFactoryAndProviderErrors(t *testing.T) {
	factory := NewFactory()
	settings := processortest.NewNopSettings(typ)

	_, err := factory.CreateLogs(t.Context(), settings, "not-config", consumertest.NewNop())
	require.Error(t, err)

	cfg := factory.CreateDefaultConfig().(*Config)
	_, err = factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.Error(t, err)

	_, err = newKeyMaterialProvider(t.Context(), &Config{KeySource: KeySourceConfig{Type: "unknown"}}, zap.NewNop())
	require.Error(t, err)

	_, err = newInlineKeyMaterialProvider(&EnvKeyConfig{})
	require.Error(t, err)

	_, err = newInlineKeyMaterialProvider(&EnvKeyConfig{Certificate: "not-pem"})
	require.Error(t, err)

	_, err = newFileKeyMaterialProvider(&FileKeyConfig{HMACKey: filepath.Join(t.TempDir(), "missing.key")})
	require.Error(t, err)

	cfg = factory.CreateDefaultConfig().(*Config)
	cfg.KeySource = KeySourceConfig{Type: keySourceFile, File: &FileKeyConfig{HMACKey: "testdata/dev_hmac_key.txt"}}
	cfg.DeadLetter.Enabled = true
	cfg.DeadLetter.StorageID = component.MustNewID("file")
	proc, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, proc.Start(t.Context(), nil))
	require.NoError(t, proc.Shutdown(t.Context()))
}

func TestCertificateHelpers(t *testing.T) {
	assert.Nil(t, decodeIfBase64(nil))
	assert.Equal(t, []byte("plain"), decodeIfBase64([]byte("plain")))

	_, _, certPEM := newTestRSA(t)
	encoded := []byte(base64.StdEncoding.EncodeToString(certPEM))
	decoded := decodeIfBase64(encoded)
	assert.Contains(t, string(decoded), "-----BEGIN")

	encodedNotPEM := []byte(base64.StdEncoding.EncodeToString([]byte("hello")))
	assert.Equal(t, encodedNotPEM, decodeIfBase64(encodedNotPEM))

	_, err := parseCertificatePEM(nil)
	require.Error(t, err)
	_, err = parseCertificatePEM([]byte("no pem here"))
	require.Error(t, err)
	_, err = parseCertificatePEM([]byte("-----BEGIN CERTIFICATE-----\nnot-a-cert\n-----END CERTIFICATE-----"))
	require.Error(t, err)

	assert.False(t, certificateRefMatches(nil, "x"))
	err = certificateMatchesAlgorithm(nil, algorithmRS256)
	require.Error(t, err)

	_, cert, _ := newTestRSA(t)
	full := base64.StdEncoding.EncodeToString(cert.Raw)
	assert.True(t, certificateRefMatches(cert, full))
}

func TestValueToInterfaceAndSerializeFields(t *testing.T) {
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: []byte("k")})

	ld := plog.NewLogs()
	lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.SetEventName("evt")
	lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(2, 0)))
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1, 0)))
	var tid pcommon.TraceID
	tid[0] = 1
	lr.SetTraceID(tid)
	var sid pcommon.SpanID
	sid[0] = 2
	lr.SetSpanID(sid)
	lr.Body().SetInt(7)
	lr.Attributes().PutDouble("d", 1.5)
	lr.Attributes().PutBool("b", true)
	lr.Attributes().PutEmptyBytes("bin").FromRaw([]byte{1, 2})
	slice := lr.Attributes().PutEmptySlice("s")
	slice.AppendEmpty().SetStr("x")
	m := lr.Attributes().PutEmptyMap("m")
	m.PutStr("k", "v")
	lr.Attributes().PutEmpty("empty")

	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	s := string(payload)
	assert.Contains(t, s, "event_name")
	assert.Contains(t, s, "observed_timestamp")
	assert.Contains(t, s, "trace_id")
	assert.Contains(t, s, "span_id")
	assert.Contains(t, s, "intValue")
	assert.Contains(t, s, "doubleValue")
	assert.Contains(t, s, "boolValue")
	assert.Contains(t, s, "bytesValue")

	nested := pcommon.NewValueMap()
	cur := nested.Map()
	for range jsonMaxDepth + 1 {
		next := cur.PutEmptyMap("n")
		cur = next
	}
	_, err = p.valueToInterface(nested, 0)
	require.Error(t, err)

	bad := pcommon.NewValueStr(string([]byte{0xff, 0xfe}))
	_, err = p.valueToInterface(bad, 0)
	require.Error(t, err)

	lr.SetEventName(string([]byte{0xff}))
	_, err = p.serializeLogRecord(lr)
	require.Error(t, err)
}

func TestResourceKeyMaterialAndErrorFormatting(t *testing.T) {
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{})
	ld := makeLogs("a", nil, algorithmHMACSHA256, "", "YWJjZA==")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	reason, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonKeyMaterialMismatch, reason.Str())

	p = newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: []byte("k")})
	ld = makeLogs("a", nil, "RSA-PSS", "", "YWJjZA==")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	reason, ok = ld.ResourceLogs().At(0).Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonUnsupportedAlgorithm, reason.Str())

	fail := &verifyFailure{reason: reasonMissingAlgorithm}
	assert.Equal(t, reasonMissingAlgorithm, fail.Error())
	fail = &verifyFailure{reason: reasonKeyMaterialMismatch, err: errors.New("no key")}
	assert.Contains(t, fail.Error(), reasonKeyMaterialMismatch)
	assert.Contains(t, fail.Error(), "no key")

	p = newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{hmacKey: []byte("k")})
	ld = plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr(attrIntegrityAlgorithm, algorithmHMACSHA256)
	records := rl.ScopeLogs().AppendEmpty().LogRecords()
	for i := range maxFailureDetails + 1 {
		rec := records.AppendEmpty()
		rec.Body().SetStr("x")
		rec.Attributes().PutStr(attrIntegrityValue, "YmFk")
		_ = i
	}
	err := p.ConsumeLogs(t.Context(), ld)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "and 1 more")
}

func TestMixedFailureReasonsStampWithoutReason(t *testing.T) {
	hmacKey := []byte("mix-key")
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})
	ld := makeLogs("a", nil, algorithmHMACSHA256, "", "")
	rl := ld.ResourceLogs().At(0)
	records := rl.ScopeLogs().At(0).LogRecords()
	records.At(0).Attributes().PutStr(attrIntegrityValue, "YmFk")
	missing := records.AppendEmpty()
	missing.Body().SetStr("b")

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	requireStatus(t, rl.Resource().Attributes(), statusFailed)
	_, ok := rl.Resource().Attributes().Get(attrVerificationReason)
	assert.False(t, ok)
}

func TestEmptyResourceAndProfileWithoutAttr(t *testing.T) {
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: []byte("k")})
	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty()
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))

	ld = makeLogs("a", map[string]string{attrVerificationStatus: statusVerified}, algorithmHMACSHA256, "", "YmFk")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	reason, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonIntegrityMismatch, reason.Str())
}

func TestES256BadSignature(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(9),
		Subject:      pkix.Name{CommonName: "ec-bad"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{cert: cert})
	ld := makeLogs("ec-body", nil, algorithmES256, fingerprint(cert), "YWJjZA==")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	reason, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonIntegrityMismatch, reason.Str())
}

func TestFileCertificateInvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.pem")
	require.NoError(t, os.WriteFile(path, []byte("-----BEGIN CERTIFICATE-----\nxxxx\n-----END CERTIFICATE-----"), 0o600))
	_, err := newFileKeyMaterialProvider(&FileKeyConfig{Certificate: path})
	require.Error(t, err)
}
