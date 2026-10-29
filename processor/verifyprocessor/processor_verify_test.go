// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
)

type staticProvider struct {
	cert    *x509.Certificate
	hmacKey []byte
}

func (s *staticProvider) GetCertificate() *x509.Certificate { return s.cert }
func (s *staticProvider) GetHMACKey() []byte                { return s.hmacKey }

type errLogsConsumer struct {
	err error
}

func (e errLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (e errLogsConsumer) ConsumeLogs(context.Context, plog.Logs) error {
	return e.err
}

func newTestRSA(t *testing.T) (*rsa.PrivateKey, *x509.Certificate, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "verify-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return key, cert, certPEM
}

func fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func newVerifyProc(t *testing.T, cfg *Config, prov KeyMaterialProvider) *verifyProcessor {
	t.Helper()
	return &verifyProcessor{
		config:       cfg,
		logger:       zap.NewNop(),
		nextConsumer: consumertest.NewNop(),
		provider:     prov,
	}
}

func signRSA(t *testing.T, key *rsa.PrivateKey, payload []byte, hash crypto.Hash) string {
	t.Helper()
	h := hash.New()
	_, err := h.Write(payload)
	require.NoError(t, err)
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, hash, h.Sum(nil))
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(sig)
}

func makeLogs(body string, attrs map[string]string, algorithm, certRef, sig string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	if algorithm != "" {
		rl.Resource().Attributes().PutStr(attrIntegrityAlgorithm, algorithm)
	}
	if certRef != "" {
		rl.Resource().Attributes().PutStr(attrIntegrityCertificate, certRef)
	}
	lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr(body)
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(1, 0)))
	for k, v := range attrs {
		lr.Attributes().PutStr(k, v)
	}
	if sig != "" {
		lr.Attributes().PutStr(attrIntegrityValue, sig)
	}
	return ld
}

func requireStatus(t *testing.T, attrs pcommon.Map, status string) {
	t.Helper()
	v, ok := attrs.Get(attrVerificationStatus)
	require.True(t, ok, "expected %s status", status)
	assert.Equal(t, status, v.Str())
}

func requireNoStatus(t *testing.T, attrs pcommon.Map) {
	t.Helper()
	_, ok := attrs.Get(attrVerificationStatus)
	assert.False(t, ok, "expected no verification status")
}

func TestVerifyRS256HappyPath(t *testing.T) {
	key, cert, _ := newTestRSA(t)
	p := newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{cert: cert})

	ld := makeLogs("hello", map[string]string{"k": "v"}, algorithmRS256, fingerprint(cert), "")
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	lr.Attributes().PutStr(attrIntegrityValue, signRSA(t, key, payload, crypto.SHA256))

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	requireNoStatus(t, ld.ResourceLogs().At(0).Resource().Attributes())
	requireNoStatus(t, lr.Attributes())
}

func TestVerifyRS512HappyPathAndTamper(t *testing.T) {
	key, cert, _ := newTestRSA(t)
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{cert: cert})

	ld := makeLogs("rs512-body", map[string]string{"k": "v"}, algorithmRS512, fingerprint(cert), "")
	rl := ld.ResourceLogs().At(0)
	lr := rl.ScopeLogs().At(0).LogRecords().At(0)
	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	lr.Attributes().PutStr(attrIntegrityValue, signRSA(t, key, payload, crypto.SHA512))

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	requireStatus(t, rl.Resource().Attributes(), statusVerified)
	requireNoStatus(t, lr.Attributes())

	ld2 := makeLogs("rs512-body", nil, algorithmRS512, fingerprint(cert), "")
	rl2 := ld2.ResourceLogs().At(0)
	lr2 := rl2.ScopeLogs().At(0).LogRecords().At(0)
	payload2, err := p.serializeLogRecord(lr2)
	require.NoError(t, err)
	lr2.Attributes().PutStr(attrIntegrityValue, signRSA(t, key, payload2, crypto.SHA256))

	require.NoError(t, p.ConsumeLogs(t.Context(), ld2))
	requireStatus(t, rl2.Resource().Attributes(), statusFailed)
	reason, ok := rl2.Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonIntegrityMismatch, reason.Str())
	requireNoStatus(t, lr2.Attributes())
}

func TestEmptyValueSerializedAsNull(t *testing.T) {
	p := newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{hmacKey: []byte("x")})
	lr := plog.NewLogRecord()
	lr.Attributes().PutEmpty("unset")
	lr.Attributes().PutEmptySlice("list").AppendEmpty()

	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	assert.JSONEq(t, `{"attributes":{"list":[null],"unset":null}}`, string(payload))
}

func TestVerifyHMACHappyPathAndTamper(t *testing.T) {
	hmacKey := []byte("super-secret-hmac-key")
	sink := &consumertest.LogsSink{}
	p := newVerifyProc(t, &Config{FailureMode: failureModeDrop, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})
	p.nextConsumer = sink

	ld := makeLogs("hmac-body", nil, algorithmHMACSHA256, "", "")
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, hmacKey)
	_, err = mac.Write(payload)
	require.NoError(t, err)
	lr.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	requireStatus(t, ld.ResourceLogs().At(0).Resource().Attributes(), statusVerified)
	requireNoStatus(t, lr.Attributes())
	assert.Equal(t, 1, sink.LogRecordCount())

	ld2 := makeLogs("hmac-body", nil, algorithmHMACSHA256, "", "")
	lr2 := ld2.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	payload2, err := p.serializeLogRecord(lr2)
	require.NoError(t, err)
	mac2 := hmac.New(sha256.New, hmacKey)
	_, err = mac2.Write(payload2)
	require.NoError(t, err)
	lr2.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(mac2.Sum(nil)))
	lr2.Attributes().PutStr(attrRecordID, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	lr2.Body().SetStr("tampered")

	err = p.ConsumeLogs(t.Context(), ld2)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.Contains(t, err.Error(), "verification failed for 1 log record(s)")
	assert.Contains(t, err.Error(), "id=a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	assert.NotContains(t, err.Error(), "tampered")
	assert.Equal(t, 0, ld2.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	assert.Equal(t, 1, sink.LogRecordCount())
}

func TestDropForwardsGoodAndReportsBadIDs(t *testing.T) {
	hmacKey := []byte("super-secret-hmac-key")
	sink := &consumertest.LogsSink{}
	p := newVerifyProc(t, &Config{FailureMode: failureModeDrop, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})
	p.nextConsumer = sink

	ld := makeLogs("ok-body", nil, algorithmHMACSHA256, "", "")
	records := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	okRec := records.At(0)
	payload, err := p.serializeLogRecord(okRec)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, hmacKey)
	_, err = mac.Write(payload)
	require.NoError(t, err)
	okRec.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	var traceID pcommon.TraceID
	copy(traceID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	badRec := records.AppendEmpty()
	badRec.SetTraceID(traceID)
	badRec.SetEventName("audit.bad")
	badRec.Body().SetStr("tampered")
	badRec.Attributes().PutStr(attrRecordID, "rec-bad-2")
	badRec.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString([]byte("not-the-mac")))

	err = p.ConsumeLogs(t.Context(), ld)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.Contains(t, err.Error(), "verification failed for 1 log record(s)")
	assert.Contains(t, err.Error(), "id=rec-bad-2")
	assert.NotContains(t, err.Error(), "record[")
	assert.NotContains(t, err.Error(), traceID.String())
	assert.NotContains(t, err.Error(), "audit.bad")
	assert.NotContains(t, err.Error(), "tampered")
	assert.NotContains(t, err.Error(), "ok-body")
	assert.Equal(t, 1, records.Len())
	assert.Equal(t, 1, sink.LogRecordCount())
	assert.Equal(t, "ok-body", sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
}

func TestStrictRejectsWholeBatch(t *testing.T) {
	hmacKey := []byte("super-secret-hmac-key")
	sink := &consumertest.LogsSink{}
	p := newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})
	p.nextConsumer = sink

	ld := makeLogs("ok-body", nil, algorithmHMACSHA256, "", "")
	records := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
	okRec := records.At(0)
	payload, err := p.serializeLogRecord(okRec)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, hmacKey)
	_, err = mac.Write(payload)
	require.NoError(t, err)
	okRec.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	badRec := records.AppendEmpty()
	badRec.Body().SetStr("tampered")
	badRec.Attributes().PutStr(attrRecordID, "rec-bad-3")
	badRec.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString([]byte("not-the-mac")))

	err = p.ConsumeLogs(t.Context(), ld)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.Contains(t, err.Error(), "id=rec-bad-3")
	assert.Equal(t, 2, records.Len())
	assert.Equal(t, 0, sink.LogRecordCount())
}

func TestDropReportsVerificationWhenExportFails(t *testing.T) {
	hmacKey := []byte("super-secret-hmac-key")
	p := newVerifyProc(t, &Config{FailureMode: failureModeDrop, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})

	newBatch := func(t *testing.T) plog.Logs {
		t.Helper()
		ld := makeLogs("ok-body", nil, algorithmHMACSHA256, "", "")
		records := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
		okRec := records.At(0)
		payload, err := p.serializeLogRecord(okRec)
		require.NoError(t, err)
		okRec.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
		badRec := records.AppendEmpty()
		badRec.Body().SetStr("tampered")
		badRec.Attributes().PutStr(attrRecordID, "rec-bad-export")
		badRec.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString([]byte("not-the-mac")))
		return ld
	}

	t.Run("retryable export", func(t *testing.T) {
		exportErr := errors.New("export unavailable")
		p.nextConsumer = errLogsConsumer{err: exportErr}
		err := p.ConsumeLogs(t.Context(), newBatch(t))
		require.Error(t, err)
		assert.ErrorIs(t, err, exportErr)
		assert.False(t, consumererror.IsPermanent(err))
		assert.Contains(t, err.Error(), "verification failed for 1 log record(s)")
		assert.Contains(t, err.Error(), "id=rec-bad-export")
	})

	t.Run("permanent export", func(t *testing.T) {
		exportErr := consumererror.NewPermanent(errors.New("export rejected"))
		p.nextConsumer = errLogsConsumer{err: exportErr}
		err := p.ConsumeLogs(t.Context(), newBatch(t))
		require.Error(t, err)
		assert.ErrorIs(t, err, exportErr)
		assert.True(t, consumererror.IsPermanent(err))
		assert.Contains(t, err.Error(), "id=rec-bad-export")
	})
}

func TestVerifyMarkFailureMode(t *testing.T) {
	hmacKey := []byte("mark-key")
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})

	ld := makeLogs("body", nil, algorithmHMACSHA256, "", "not-valid-base64!!!")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	rl := ld.ResourceLogs().At(0)
	requireStatus(t, rl.Resource().Attributes(), statusFailed)
	reason, ok := rl.Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonInvalidSignature, reason.Str())
	requireNoStatus(t, rl.ScopeLogs().At(0).LogRecords().At(0).Attributes())
}

func TestVerifyCertificateMismatch(t *testing.T) {
	key, cert, _ := newTestRSA(t)
	_, otherCert, _ := newTestRSA(t)
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{cert: cert})

	ld := makeLogs("hello", nil, algorithmRS256, fingerprint(otherCert), "")
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	lr.Attributes().PutStr(attrIntegrityValue, signRSA(t, key, payload, crypto.SHA256))

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	reason, ok := ld.ResourceLogs().At(0).Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonCertificateMismatch, reason.Str())
}

func TestVerifyKeyMaterialMismatchWrongKeyType(t *testing.T) {
	_, cert, _ := newTestRSA(t)
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{cert: cert})

	ld := makeLogs("hello", nil, algorithmES256, fingerprint(cert), "YWJjZA==")
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	rl := ld.ResourceLogs().At(0)
	requireStatus(t, rl.Resource().Attributes(), statusFailed)
	reason, ok := rl.Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonKeyMaterialMismatch, reason.Str())
	requireNoStatus(t, rl.ScopeLogs().At(0).LogRecords().At(0).Attributes())
}

func TestResourceKeyMaterialFailureAppliesToAllRecords(t *testing.T) {
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: []byte("k")})

	ld := makeLogs("a", map[string]string{attrRecordID: "a"}, "", "", "YWJjZA==")
	rl := ld.ResourceLogs().At(0)
	lr2 := rl.ScopeLogs().At(0).LogRecords().AppendEmpty()
	lr2.Body().SetStr("b")
	lr2.Attributes().PutStr(attrRecordID, "b")
	lr2.Attributes().PutStr(attrIntegrityValue, "YWJjZA==")

	require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	requireStatus(t, rl.Resource().Attributes(), statusFailed)
	reason, ok := rl.Resource().Attributes().Get(attrVerificationReason)
	require.True(t, ok)
	assert.Equal(t, reasonMissingAlgorithm, reason.Str())
	requireNoStatus(t, rl.ScopeLogs().At(0).LogRecords().At(0).Attributes())
	requireNoStatus(t, rl.ScopeLogs().At(0).LogRecords().At(1).Attributes())
}

func TestMissingCertificateFailsWholeResource(t *testing.T) {
	_, cert, _ := newTestRSA(t)
	p := newVerifyProc(t, &Config{FailureMode: failureModeDrop, VerificationProfile: "default"}, &staticProvider{cert: cert})

	ld := makeLogs("a", map[string]string{attrRecordID: "a"}, algorithmRS256, "", "YWJjZA==")
	rl := ld.ResourceLogs().At(0)
	lr2 := rl.ScopeLogs().At(0).LogRecords().AppendEmpty()
	lr2.Body().SetStr("b")
	lr2.Attributes().PutStr(attrRecordID, "b")
	lr2.Attributes().PutStr(attrIntegrityValue, "YWJjZA==")

	sink := &consumertest.LogsSink{}
	p.nextConsumer = sink

	err := p.ConsumeLogs(t.Context(), ld)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.Equal(t, 0, rl.ScopeLogs().At(0).LogRecords().Len())
	assert.Equal(t, 0, sink.LogRecordCount())
	assert.Contains(t, err.Error(), "verification failed for 2 log record(s)")
	assert.Contains(t, err.Error(), "id=a")
	assert.Contains(t, err.Error(), "id=b")
}

func TestStrictReportsEveryFailingResource(t *testing.T) {
	hmacKey := []byte("strict-key")
	p := newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})
	sink := &consumertest.LogsSink{}
	p.nextConsumer = sink

	ld := makeLogs("a", nil, "", "", "YWJjZA==")
	second := ld.ResourceLogs().AppendEmpty()
	second.Resource().Attributes().PutStr(attrIntegrityAlgorithm, algorithmHMACSHA256)
	second.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("b")

	err := p.ConsumeLogs(t.Context(), ld)
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.Contains(t, err.Error(), "verification failed for 2 log record(s)")
	assert.Contains(t, err.Error(), "record[0,0,0]")
	assert.Contains(t, err.Error(), "record[1,0,0]")
	assert.Equal(t, 0, sink.LogRecordCount())
}

func TestDropKeepsHealthyResourceWhenAnotherFails(t *testing.T) {
	hmacKey := []byte("drop-key")
	p := newVerifyProc(t, &Config{FailureMode: failureModeDrop, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})
	sink := &consumertest.LogsSink{}
	p.nextConsumer = sink

	ld := makeLogs("bad", nil, "", "", "YWJjZA==")
	good := ld.ResourceLogs().AppendEmpty()
	good.Resource().Attributes().PutStr(attrIntegrityAlgorithm, algorithmHMACSHA256)
	lr := good.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr("good")
	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	mac := hmac.New(sha256.New, hmacKey)
	_, err = mac.Write(payload)
	require.NoError(t, err)
	lr.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	err = p.ConsumeLogs(t.Context(), ld)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "record[0,0,0]")
	assert.Equal(t, 1, sink.LogRecordCount())
	exported := sink.AllLogs()[0].ResourceLogs()
	require.Equal(t, 2, exported.Len())
	assert.Equal(t, 0, exported.At(0).ScopeLogs().At(0).LogRecords().Len())
	assert.Equal(t, 1, exported.At(1).ScopeLogs().At(0).LogRecords().Len())
}

func TestVerifyES256AndEdDSA(t *testing.T) {
	t.Run("ES256", func(t *testing.T) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: "ec"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		require.NoError(t, err)
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)

		p := newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{cert: cert})
		ld := makeLogs("ec-body", nil, algorithmES256, fingerprint(cert), "")
		lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		payload, err := p.serializeLogRecord(lr)
		require.NoError(t, err)
		sum := sha256.Sum256(payload)
		sig, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
		require.NoError(t, err)
		lr.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(sig))
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	})

	t.Run("EdDSA", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(3),
			Subject:      pkix.Name{CommonName: "ed"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
		require.NoError(t, err)
		cert, err := x509.ParseCertificate(der)
		require.NoError(t, err)

		p := newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{cert: cert})
		ld := makeLogs("ed-body", nil, algorithmEdDSA, fingerprint(cert), "")
		lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		payload, err := p.serializeLogRecord(lr)
		require.NoError(t, err)
		sig := ed25519.Sign(priv, payload)
		lr.Attributes().PutStr(attrIntegrityValue, base64.StdEncoding.EncodeToString(sig))
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
	})
}

func TestFileAndEnvProviders(t *testing.T) {
	_, cert, certPEM := newTestRSA(t)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))

	hmacPath := filepath.Join(dir, "hmac.key")
	rawHMAC := []byte("provider-hmac-secret")
	require.NoError(t, os.WriteFile(hmacPath, []byte(base64.StdEncoding.EncodeToString(rawHMAC)), 0o600))

	t.Run("file certificate", func(t *testing.T) {
		prov, err := newFileKeyMaterialProvider(&FileKeyConfig{Certificate: certPath})
		require.NoError(t, err)
		require.NotNil(t, prov.GetCertificate())
		assert.Equal(t, cert.Raw, prov.GetCertificate().Raw)
	})

	t.Run("file hmac", func(t *testing.T) {
		prov, err := newFileKeyMaterialProvider(&FileKeyConfig{HMACKey: hmacPath})
		require.NoError(t, err)
		assert.Equal(t, rawHMAC, prov.GetHMACKey())
	})

	t.Run("env certificate", func(t *testing.T) {
		prov, err := newInlineKeyMaterialProvider(&EnvKeyConfig{Certificate: string(certPEM)})
		require.NoError(t, err)
		require.NotNil(t, prov.GetCertificate())
	})

	t.Run("env hmac", func(t *testing.T) {
		prov, err := newInlineKeyMaterialProvider(&EnvKeyConfig{HMACKey: base64.StdEncoding.EncodeToString(rawHMAC)})
		require.NoError(t, err)
		assert.Equal(t, rawHMAC, prov.GetHMACKey())
	})

	t.Run("file hmac raw", func(t *testing.T) {
		rawPath := filepath.Join(dir, "hmac-raw.key")
		raw := []byte("raw-hmac-secret!")
		require.NoError(t, os.WriteFile(rawPath, raw, 0o600))
		prov, err := newFileKeyMaterialProvider(&FileKeyConfig{HMACKey: rawPath})
		require.NoError(t, err)
		assert.Equal(t, raw, prov.GetHMACKey())
	})

	t.Run("env hmac raw", func(t *testing.T) {
		raw := "raw-hmac-secret!"
		prov, err := newInlineKeyMaterialProvider(&EnvKeyConfig{HMACKey: raw})
		require.NoError(t, err)
		assert.Equal(t, []byte(raw), prov.GetHMACKey())
	})

	t.Run("env hmac padded base64", func(t *testing.T) {
		prov, err := newInlineKeyMaterialProvider(&EnvKeyConfig{HMACKey: "\n" + base64.StdEncoding.EncodeToString(rawHMAC) + "\n"})
		require.NoError(t, err)
		assert.Equal(t, rawHMAC, prov.GetHMACKey())
	})

	t.Run("file hmac empty", func(t *testing.T) {
		emptyPath := filepath.Join(dir, "hmac-empty.key")
		require.NoError(t, os.WriteFile(emptyPath, []byte(" \n"), 0o600))
		_, err := newFileKeyMaterialProvider(&FileKeyConfig{HMACKey: emptyPath})
		require.Error(t, err)
	})

	t.Run("env hmac empty", func(t *testing.T) {
		_, err := newInlineKeyMaterialProvider(&EnvKeyConfig{HMACKey: "  "})
		require.Error(t, err)
	})

	t.Run("file missing", func(t *testing.T) {
		_, err := newFileKeyMaterialProvider(&FileKeyConfig{Certificate: filepath.Join(dir, "missing.pem")})
		require.Error(t, err)
	})
}

func TestStubProvidersNotImplemented(t *testing.T) {
	_, err := newKeyMaterialProvider(t.Context(), &Config{
		KeySource: KeySourceConfig{Type: keySourceK8sSecret, K8sSecret: &K8sSecretConfig{Name: "n", SecretConfig: SecretConfig{HMACKey: "k"}}},
	}, zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")

	_, err = newKeyMaterialProvider(t.Context(), &Config{
		KeySource: KeySourceConfig{Type: keySourceBao, Bao: &BaoKeyConfig{SecretPath: "p", SecretConfig: SecretConfig{HMACKey: "k"}}},
	}, zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestFactoryCreateWithFileHMAC(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.KeySource = KeySourceConfig{
		Type: keySourceFile,
		File: &FileKeyConfig{HMACKey: "testdata/dev_hmac_key.txt"},
	}
	proc, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(typ), cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.True(t, proc.Capabilities().MutatesData)
}

func TestIntegrityAttrsExcludedFromPayload(t *testing.T) {
	p := newVerifyProc(t, &Config{FailureMode: failureModeStrict, VerificationProfile: "default"}, &staticProvider{hmacKey: []byte("x")})
	ld := makeLogs("body", map[string]string{
		"keep":                 "yes",
		attrIntegrityValue:     "sig",
		attrVerificationStatus: "stale",
	}, algorithmHMACSHA256, "", "sig")
	lr := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	payload, err := p.serializeLogRecord(lr)
	require.NoError(t, err)
	assert.NotContains(t, string(payload), "audit.integrity")
	assert.NotContains(t, string(payload), "audit.verification")
	assert.Contains(t, string(payload), "keep")
}

func hmacSign(t *testing.T, key, payload []byte) string {
	t.Helper()
	mac := hmac.New(sha256.New, key)
	_, err := mac.Write(payload)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestHierarchicalStamping(t *testing.T) {
	hmacKey := []byte("hier-key")
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})

	signOK := func(t *testing.T, lr plog.LogRecord) {
		t.Helper()
		payload, err := p.serializeLogRecord(lr)
		require.NoError(t, err)
		lr.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
	}

	t.Run("all resource success stamps resource only", func(t *testing.T) {
		ld := makeLogs("a", nil, algorithmHMACSHA256, "", "")
		rl := ld.ResourceLogs().At(0)
		records := rl.ScopeLogs().At(0).LogRecords()
		signOK(t, records.At(0))
		bad := records.AppendEmpty()
		bad.Body().SetStr("b")
		signOK(t, bad)

		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, rl.Resource().Attributes(), statusVerified)
		requireNoStatus(t, rl.ScopeLogs().At(0).Scope().Attributes())
		requireNoStatus(t, records.At(0).Attributes())
		requireNoStatus(t, records.At(1).Attributes())
	})

	t.Run("all resource fail stamps resource only", func(t *testing.T) {
		ld := makeLogs("a", nil, algorithmHMACSHA256, "", "YmFk")
		rl := ld.ResourceLogs().At(0)
		rl.ScopeLogs().At(0).LogRecords().AppendEmpty().Attributes().PutStr(attrIntegrityValue, "YmFk")

		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, rl.Resource().Attributes(), statusFailed)
		requireNoStatus(t, rl.ScopeLogs().At(0).Scope().Attributes())
		requireNoStatus(t, rl.ScopeLogs().At(0).LogRecords().At(0).Attributes())
	})

	t.Run("per scope when resource is mixed", func(t *testing.T) {
		ld := makeLogs("ok", nil, algorithmHMACSHA256, "", "")
		rl := ld.ResourceLogs().At(0)
		okScope := rl.ScopeLogs().At(0)
		signOK(t, okScope.LogRecords().At(0))

		failScope := rl.ScopeLogs().AppendEmpty()
		failRec := failScope.LogRecords().AppendEmpty()
		failRec.Body().SetStr("bad")
		failRec.Attributes().PutStr(attrIntegrityValue, "YmFk")

		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireNoStatus(t, rl.Resource().Attributes())
		requireStatus(t, okScope.Scope().Attributes(), statusVerified)
		requireNoStatus(t, okScope.LogRecords().At(0).Attributes())
		requireStatus(t, failScope.Scope().Attributes(), statusFailed)
		requireNoStatus(t, failRec.Attributes())
	})

	t.Run("mixed scope stamps each log", func(t *testing.T) {
		ld := makeLogs("ok", nil, algorithmHMACSHA256, "", "")
		rl := ld.ResourceLogs().At(0)
		records := rl.ScopeLogs().At(0).LogRecords()
		signOK(t, records.At(0))
		bad := records.AppendEmpty()
		bad.Body().SetStr("bad")
		bad.Attributes().PutStr(attrIntegrityValue, "YmFk")

		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireNoStatus(t, rl.Resource().Attributes())
		requireNoStatus(t, rl.ScopeLogs().At(0).Scope().Attributes())
		requireStatus(t, records.At(0).Attributes(), statusVerified)
		requireStatus(t, bad.Attributes(), statusFailed)
	})
}

func TestVerificationStampsAcrossResignChain(t *testing.T) {
	hmacKey := []byte("chain-key")
	p := newVerifyProc(t, &Config{FailureMode: failureModeMark, VerificationProfile: "default"}, &staticProvider{hmacKey: hmacKey})

	newSignedLogs := func(t *testing.T) (plog.Logs, plog.ResourceLogs, plog.LogRecord) {
		ld := makeLogs("chain-body", map[string]string{"user": "alice"}, algorithmHMACSHA256, "", "")
		rl := ld.ResourceLogs().At(0)
		lr := rl.ScopeLogs().At(0).LogRecords().At(0)
		payload, err := p.serializeLogRecord(lr)
		require.NoError(t, err)
		lr.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, rl.Resource().Attributes(), statusVerified)
		requireNoStatus(t, lr.Attributes())
		return ld, rl, lr
	}

	t.Run("verify then verify", func(t *testing.T) {
		ld, rl, lr := newSignedLogs(t)
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, rl.Resource().Attributes(), statusVerified)
		requireNoStatus(t, lr.Attributes())
	})

	t.Run("sign verify resign verify", func(t *testing.T) {
		ld, rl, lr := newSignedLogs(t)
		payload, err := p.serializePayload(lr, true)
		require.NoError(t, err)
		lr.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, rl.Resource().Attributes(), statusVerified)
		requireNoStatus(t, lr.Attributes())
	})

	t.Run("log stamp altered after resign of mixed scope", func(t *testing.T) {
		ld := makeLogs("ok", map[string]string{"user": "alice"}, algorithmHMACSHA256, "", "")
		rl := ld.ResourceLogs().At(0)
		records := rl.ScopeLogs().At(0).LogRecords()
		ok := records.At(0)
		payload, err := p.serializeLogRecord(ok)
		require.NoError(t, err)
		ok.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
		ok2 := records.AppendEmpty()
		ok2.Body().SetStr("ok2")
		payload2, err := p.serializeLogRecord(ok2)
		require.NoError(t, err)
		ok2.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload2))
		bad := records.AppendEmpty()
		bad.Body().SetStr("bad")
		bad.Attributes().PutStr(attrIntegrityValue, "YmFk")
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, ok.Attributes(), statusVerified)

		stampedPayload, err := p.serializePayload(ok, true)
		require.NoError(t, err)
		ok.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, stampedPayload))
		ok.Attributes().PutStr(attrVerificationProfile, "forged")

		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, ok.Attributes(), statusFailed)
		reason, _ := ok.Attributes().Get(attrVerificationReason)
		assert.Equal(t, reasonIntegrityMismatch, reason.Str())
		requireStatus(t, ok2.Attributes(), statusVerified)
	})

	t.Run("resign over matching profile verifies", func(t *testing.T) {
		ld := makeLogs("ok", map[string]string{"user": "alice"}, algorithmHMACSHA256, "", "")
		rl := ld.ResourceLogs().At(0)
		records := rl.ScopeLogs().At(0).LogRecords()
		ok := records.At(0)
		payload, err := p.serializeLogRecord(ok)
		require.NoError(t, err)
		ok.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
		bad := records.AppendEmpty()
		bad.Body().SetStr("bad")
		bad.Attributes().PutStr(attrIntegrityValue, "YmFk")
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, ok.Attributes(), statusVerified)

		stampedPayload, err := p.serializePayload(ok, true)
		require.NoError(t, err)
		ok.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, stampedPayload))
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, ok.Attributes(), statusVerified)
		profile, found := ok.Attributes().Get(attrVerificationProfile)
		require.True(t, found)
		assert.Equal(t, "default", profile.Str())
	})

	t.Run("resign over foreign profile verifies", func(t *testing.T) {
		ld := makeLogs("ok", nil, algorithmHMACSHA256, "", "")
		records := ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords()
		ok := records.At(0)
		payload, err := p.serializeLogRecord(ok)
		require.NoError(t, err)
		ok.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
		kept := records.AppendEmpty()
		kept.Body().SetStr("kept")
		keptPayload, err := p.serializeLogRecord(kept)
		require.NoError(t, err)
		kept.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, keptPayload))
		bad := records.AppendEmpty()
		bad.Body().SetStr("bad")
		bad.Attributes().PutStr(attrIntegrityValue, "YmFk")
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))

		ok.Attributes().PutStr(attrVerificationProfile, "other-profile")
		stampedPayload, err := p.serializePayload(ok, true)
		require.NoError(t, err)
		ok.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, stampedPayload))
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, ok.Attributes(), statusVerified)
		profile, found := ok.Attributes().Get(attrVerificationProfile)
		require.True(t, found)
		assert.Equal(t, "default", profile.Str())
		requireStatus(t, kept.Attributes(), statusVerified)
	})

	t.Run("body tampered after resign", func(t *testing.T) {
		ld, rl, lr := newSignedLogs(t)
		payload, err := p.serializePayload(lr, true)
		require.NoError(t, err)
		lr.Attributes().PutStr(attrIntegrityValue, hmacSign(t, hmacKey, payload))
		lr.Body().SetStr("tampered")
		require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		requireStatus(t, rl.Resource().Attributes(), statusFailed)
	})
}

func TestValidateSecretMaterialExactlyOne(t *testing.T) {
	assert.ErrorIs(t, validateSecretMaterial(SecretConfig{}), errKeySourceNeedsMaterial)
	assert.ErrorIs(t, validateSecretMaterial(SecretConfig{Certificate: "c", HMACKey: "h"}), errKeySourceBothMaterials)
	assert.NoError(t, validateSecretMaterial(SecretConfig{Certificate: "c"}))
	assert.NoError(t, validateSecretMaterial(SecretConfig{HMACKey: "h"}))
}
