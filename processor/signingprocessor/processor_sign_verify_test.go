// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"hash"
	"math/big"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/canonicaljson"
)

// staticProvider wraps an in-memory RSA key + cert for test use.
type staticProvider struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
}

func (s *staticProvider) GetPrivateKey() crypto.Signer      { return s.key }
func (s *staticProvider) GetCertificate() *x509.Certificate { return s.cert }
func (*staticProvider) GetHMACKey() []byte                  { return nil }

func newTestProvider(t *testing.T) *staticProvider {
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
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return &staticProvider{key: key, cert: cert}
}

// verifyRecord replicates what the verify processor does:
// strip audit.integrity.* attrs, re-serialize with JCS, hash, verify sig.
//
// This implicitly tests that ExcludePrefixes works correctly: if
// audit.integrity.* attributes were not excluded from the canonical payload,
// the computed hash would differ from the signed hash and verification would fail.
// Every test using verifyRecord() thus validates the ExcludePrefixes feature.
func verifyRecord(t *testing.T, lr plog.LogRecord, pubKey *rsa.PublicKey) {
	t.Helper()

	sigVal, ok := lr.Attributes().Get("audit.integrity.value")
	if !ok {
		t.Fatal("audit.integrity.value missing")
	}

	// Use canonicaljson to serialize (same as the processor does)
	canonical, err := canonicaljson.SerializeLogRecord(lr, canonicaljson.SerializeLogRecordOptions{
		ExcludePrefixes: []string{"audit.integrity."},
	})
	if err != nil {
		t.Fatalf("canonicaljson: %v", err)
	}

	computedHash := sha256.Sum256(canonical)

	sigBytes, err := base64.StdEncoding.DecodeString(sigVal.Str())
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, computedHash[:], sigBytes); err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

// TestSignVerifyBasic covers the happy-path: body + timestamp + attributes.
func TestSignVerifyBasic(t *testing.T) {
	prov := newTestProvider(t)
	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}

	lr := plog.NewLogRecord()
	lr.Body().SetStr("user.login.success")
	lr.SetTimestamp(pcommon.Timestamp(1714041600000000000))
	lr.SetObservedTimestamp(pcommon.Timestamp(1714041600001000000))
	lr.Attributes().PutStr("audit.record.id", "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	lr.Attributes().PutStr("audit.actor.id", "u8472")
	lr.Attributes().PutStr("audit.actor.type", "user")
	lr.Attributes().PutStr("audit.action", "LOGIN")
	lr.Attributes().PutStr("audit.outcome", "success")

	if err := p.processLogRecord(lr); err != nil {
		t.Fatalf("processLogRecord: %v", err)
	}

	verifyRecord(t, lr, &prov.key.PublicKey)
}

// TestSignVerifyWithTraceCorrelation covers trace_id and span_id being part of
// the signed payload. SeverityNumber/SeverityText are intentionally excluded
// (SHOULD NOT be set on audit records per spec).
func TestSignVerifyWithTraceCorrelation(t *testing.T) {
	prov := newTestProvider(t)
	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}

	lr := plog.NewLogRecord()
	lr.Body().SetStr("database.query")
	lr.SetTimestamp(pcommon.Timestamp(1714041700000000000))
	var traceID pcommon.TraceID
	copy(traceID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	lr.SetTraceID(traceID)
	var spanID pcommon.SpanID
	copy(spanID[:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	lr.SetSpanID(spanID)
	lr.Attributes().PutStr("audit.actor.id", "svc-1")
	lr.Attributes().PutStr("audit.action", "READ")

	if err := p.processLogRecord(lr); err != nil {
		t.Fatalf("processLogRecord: %v", err)
	}

	verifyRecord(t, lr, &prov.key.PublicKey)
}

// TestSignVerifyEventName confirms that EventName is part of the signed payload:
// changing it after signing must cause signature verification to fail.
func TestSignVerifyEventName(t *testing.T) {
	prov := newTestProvider(t)
	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}

	lr := plog.NewLogRecord()
	lr.SetEventName("user.login.success")
	lr.SetTimestamp(pcommon.Timestamp(1714041600000000000))
	lr.Attributes().PutStr("audit.actor.id", "u1")

	if err := p.processLogRecord(lr); err != nil {
		t.Fatalf("processLogRecord: %v", err)
	}

	// Happy path: EventName in payload → signature valid
	verifyRecord(t, lr, &prov.key.PublicKey)

	// Tamper: change EventName → signature must no longer verify
	sigVal, _ := lr.Attributes().Get("audit.integrity.value")
	sigBytes, _ := base64.StdEncoding.DecodeString(sigVal.Str())

	// Re-serialize with tampered EventName
	lr.SetEventName("admin.delete.all")
	tamperedPayload, err := serializeLogRecordWithDefaults(lr)
	if err != nil {
		t.Fatalf("serialize tampered: %v", err)
	}
	h := sha256.Sum256(tamperedPayload)
	err = rsa.VerifyPKCS1v15(&prov.key.PublicKey, crypto.SHA256, h[:], sigBytes)
	if err == nil {
		t.Error("❌ tampered EventName did not invalidate the signature")
	} else {
		t.Logf("🔍 tampered EventName correctly invalidates signature: %v", err)
	}
}

// TestSignVerifyStructuredBody drives a non-string body through the full
// sign-then-verify path.
func TestSignVerifyStructuredBody(t *testing.T) {
	prov := newTestProvider(t)
	p := &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}

	lr := plog.NewLogRecord()
	body := lr.Body().SetEmptyMap()
	body.PutStr("action", "delete-all")
	body.PutInt("count", 3)
	body.PutBool("dry_run", false)
	body.PutEmptyMap("who").PutStr("id", "u8472")
	lr.SetTimestamp(pcommon.Timestamp(1714041600000000000))
	lr.Attributes().PutStr("audit.action", "DELETE")

	if err := p.processLogRecord(lr); err != nil {
		t.Fatalf("processLogRecord: %v", err)
	}

	verifyRecord(t, lr, &prov.key.PublicKey)
}
