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
	"encoding/json"
	"hash"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/gowebpki/jcs"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// staticProvider wraps an in-memory RSA key + cert for test use.
type staticProvider struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
}

func (s *staticProvider) GetPrivateKey() crypto.Signer { return s.key }
func (s *staticProvider) GetCertificate() *x509.Certificate { return s.cert }
func (s *staticProvider) GetHMACKey() []byte                   { return nil }

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

func newTestProcessor(t *testing.T, provider KeyMaterialProvider) *signingProcessor {
	t.Helper()
	return &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     provider,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}
}

// verifyRecord replicates what verify-signed-log.go does:
// strip audit.integrity.* attrs, re-serialize with JCS, hash, verify sig.
func verifyRecord(t *testing.T, lr plog.LogRecord, pubKey *rsa.PublicKey) {
	t.Helper()

	sigVal, ok := lr.Attributes().Get("audit.integrity.value")
	if !ok {
		t.Fatal("audit.integrity.value missing")
	}

	// Reconstruct the payload the processor signed
	data := make(map[string]interface{})
	if lr.EventName() != "" {
		data["event_name"] = lr.EventName()
	}
	if lr.Body().Type() == pcommon.ValueTypeStr {
		data["body"] = lr.Body().Str()
	}
	if lr.Timestamp() != 0 {
		data["timestamp"] = lr.Timestamp().AsTime().UnixNano()
	}
	if lr.ObservedTimestamp() != 0 {
		data["observed_timestamp"] = lr.ObservedTimestamp().AsTime().UnixNano()
	}
	if lr.SeverityNumber() != 0 {
		data["severity_number"] = lr.SeverityNumber()
	}
	if lr.SeverityText() != "" {
		data["severity_text"] = lr.SeverityText()
	}
	if !lr.TraceID().IsEmpty() {
		data["trace_id"] = lr.TraceID().String()
	}
	if !lr.SpanID().IsEmpty() {
		data["span_id"] = lr.SpanID().String()
	}
	attrs := make(map[string]interface{})
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		if !strings.HasPrefix(k, "audit.integrity.") {
			attrs[k] = v.Str()
		}
		return true
	})
	if len(attrs) > 0 {
		data["attributes"] = attrs
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		t.Fatalf("jcs: %v", err)
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

// TestSignVerifyWithSeverityAndTrace covers the "sign complete log record" commit:
// severity_number, severity_text, trace_id, span_id must be part of the signed payload.
func TestSignVerifyWithSeverityAndTrace(t *testing.T) {
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
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("WARN")
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

// TestJCSCanonicalization verifies that two records with the same data but
// attributes inserted in different order produce identical canonical bytes.
func TestJCSCanonicalization(t *testing.T) {
	prov := newTestProvider(t)
	p := &signingProcessor{
		config:   &Config{Algorithm: "RS256"},
		provider: prov,
		hashFunc: func() hash.Hash { return crypto.SHA256.New() },
	}

	makeRecord := func(order []string) plog.LogRecord {
		lr := plog.NewLogRecord()
		lr.SetTimestamp(pcommon.Timestamp(1000000000))
		for _, k := range order {
			lr.Attributes().PutStr(k, "v")
		}
		return lr
	}

	r1 := makeRecord([]string{"z.attr", "a.attr", "m.attr"})
	r2 := makeRecord([]string{"a.attr", "m.attr", "z.attr"})

	b1, err := p.serializeLogRecord(r1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := p.serializeLogRecord(r2)
	if err != nil {
		t.Fatal(err)
	}

	if string(b1) != string(b2) {
		t.Errorf("JCS produced different output for same data in different insertion order:\n  order1: %s\n  order2: %s", b1, b2)
	}
}

// TestIntegrityAttrsExcludedFromPayload ensures audit.integrity.* attrs added
// by the processor are not included in the hash input (would cause verify failure).
func TestIntegrityAttrsExcludedFromPayload(t *testing.T) {
	prov := newTestProvider(t)
	p := &signingProcessor{
		config:   &Config{Algorithm: "RS256"},
		provider: prov,
		hashFunc: func() hash.Hash { return crypto.SHA256.New() },
	}

	lr := plog.NewLogRecord()
	lr.Attributes().PutStr("audit.actor.id", "u1")
	// Pre-populate an integrity attr to ensure it's stripped from payload
	lr.Attributes().PutStr("audit.integrity.stale", "should-be-excluded")

	b, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "audit.integrity") {
		t.Errorf("serialized payload contains audit.integrity.* attr: %s", string(b))
	}
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
	tamperedPayload, err := p.serializeLogRecord(lr)
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
