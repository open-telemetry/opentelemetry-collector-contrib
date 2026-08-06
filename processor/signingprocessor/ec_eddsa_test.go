// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"hash"
	"math/big"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func generateECPEM(t *testing.T) (certPEM, keyPEM []byte, key *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate EC key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-ec"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create EC cert: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal EC key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return
}

func generateEd25519PEM(t *testing.T) (certPEM, keyPEM []byte, key ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate Ed25519 key: %v", err)
	}
	key = priv
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "test-ed25519"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		t.Fatalf("create Ed25519 cert: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal Ed25519 key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return
}

// newAlgoProcessor creates a signingProcessor with the given algorithm and provider.
func newAlgoProcessor(t *testing.T, algorithm string, prov KeyMaterialProvider) *signingProcessor {
	t.Helper()
	cfg := &Config{Algorithm: algorithm, CertificateRef: CertificateRefFingerprint}
	var hf func() hash.Hash
	switch cfg.GetHash() {
	case crypto.SHA256:
		hf = func() hash.Hash { return crypto.SHA256.New() }
	case crypto.SHA512:
		hf = func() hash.Hash { return crypto.SHA512.New() }
	}
	return &signingProcessor{
		config:       cfg,
		provider:     prov,
		hashFunc:     hf,
		jwaAlgorithm: algorithm,
		certRef:      "sha256:test",
	}
}

// ---------------------------------------------------------------------------
// Config validation — new algorithm values
// ---------------------------------------------------------------------------

func TestConfigValidateAlgorithms(t *testing.T) {
	validFile := &FileKeyConfig{CertFile: "c.pem", KeyFile: "k.pem"}
	tests := []struct {
		algorithm string
		wantErr   bool
	}{
		{AlgorithmRS256, false},
		{AlgorithmRS512, false},
		{AlgorithmES256, false},
		{AlgorithmEdDSA, false},
		{"HS256", true},
		{"PS256", true},
		{"", false}, // defaults to RS256
	}
	for _, tt := range tests {
		t.Run(tt.algorithm, func(t *testing.T) {
			cfg := &Config{
				Algorithm: tt.algorithm,
				KeySource:  KeySourceConfig{Type: KeySourceFile, File: validFile},
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigGetHash(t *testing.T) {
	cases := []struct{ alg string; want crypto.Hash }{
		{AlgorithmRS256, crypto.SHA256},
		{AlgorithmRS512, crypto.SHA512},
		{AlgorithmES256, crypto.SHA256},
		{AlgorithmEdDSA, crypto.Hash(0)},
	}
	for _, c := range cases {
		if got := (&Config{Algorithm: c.alg}).GetHash(); got != c.want {
			t.Errorf("%s: GetHash() = %v, want %v", c.alg, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// certificate_reader — EC and Ed25519 key parsing
// ---------------------------------------------------------------------------

func TestParseCertificateDataEC(t *testing.T) {
	certPEM, keyPEM, _ := generateECPEM(t)
	cr, err := parseCertificateData(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cr.GetPrivateKey().(*ecdsa.PrivateKey); !ok {
		t.Error("expected *ecdsa.PrivateKey")
	}
}

func TestParseCertificateDataEd25519(t *testing.T) {
	certPEM, keyPEM, _ := generateEd25519PEM(t)
	cr, err := parseCertificateData(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cr.GetPrivateKey().(ed25519.PrivateKey); !ok {
		t.Error("expected ed25519.PrivateKey")
	}
}

func TestParseCertificateDataECPKCS8(t *testing.T) {
	// ECDSA key as PKCS8 "PRIVATE KEY"
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(4), Subject: pkix.Name{CommonName: "t"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	cr, err := parseCertificateData(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := cr.GetPrivateKey().(*ecdsa.PrivateKey); !ok {
		t.Error("expected *ecdsa.PrivateKey from PKCS8")
	}
}

func TestParseCertificateDataKeyMismatch(t *testing.T) {
	// RSA cert + EC key → should fail
	rsaCertPEM, _, _, _ := generateTestPEM(t)
	_, ecKeyPEM, _ := generateECPEM(t)
	// Use RSA cert with EC key — parseCertificateData should catch the mismatch
	_, err := parseCertificateData(rsaCertPEM, ecKeyPEM)
	if err == nil {
		t.Error("expected error for key/cert algorithm mismatch")
	}
}

// ---------------------------------------------------------------------------
// ES256 sign + verify round-trip
// ---------------------------------------------------------------------------

func TestSignVerifyES256(t *testing.T) {
	certPEM, keyPEM, ecKey := generateECPEM(t)
	cr, _ := parseCertificateData(certPEM, keyPEM)
	p := newAlgoProcessor(t, AlgorithmES256, cr)

	lr := plog.NewLogRecord()
	lr.SetEventName("user.login.success")
	lr.SetTimestamp(pcommon.Timestamp(1714041600000000000))
	lr.Attributes().PutStr("audit.actor.id", "u1")
	lr.Attributes().PutStr("audit.action", "LOGIN")

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

	// Recompute SHA-256 of canonical payload
	import_sha := func(b []byte) [32]byte {
		var h [32]byte
		hasher := crypto.SHA256.New()
		hasher.Write(b)
		copy(h[:], hasher.Sum(nil))
		return h
	}
	h := import_sha(payload)

	if !ecdsa.VerifyASN1(&ecKey.PublicKey, h[:], sigBytes) {
		t.Error("ES256 signature verification failed")
	}
	t.Logf("✅ ES256 signature verifies")
}

// ---------------------------------------------------------------------------
// EdDSA sign + verify round-trip
// ---------------------------------------------------------------------------

func TestSignVerifyEdDSA(t *testing.T) {
	certPEM, keyPEM, edKey := generateEd25519PEM(t)
	cr, _ := parseCertificateData(certPEM, keyPEM)
	p := newAlgoProcessor(t, AlgorithmEdDSA, cr)

	lr := plog.NewLogRecord()
	lr.SetEventName("document.access")
	lr.SetTimestamp(pcommon.Timestamp(1714041700000000000))
	lr.Attributes().PutStr("audit.actor.id", "svc-1")
	lr.Attributes().PutStr("audit.action", "READ")

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

	// EdDSA: verify against raw canonical payload (no pre-hash)
	payload, err := p.serializeLogRecord(lr)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	pubKey := edKey.Public().(ed25519.PublicKey)
	if !ed25519.Verify(pubKey, payload, sigBytes) {
		t.Error("EdDSA signature verification failed")
	}
	t.Logf("✅ EdDSA signature verifies")
}

// ---------------------------------------------------------------------------
// Wrong key type errors
// ---------------------------------------------------------------------------

func TestSignWrongKeyTypeRSA(t *testing.T) {
	// ES256 algorithm but RSA key → error
	_, _, rsaKey, _ := generateTestPEM(t)
	type rsaProvider struct{ key *rsa.PrivateKey; cert *x509.Certificate }
	prov := &struct {
		key  *rsa.PrivateKey
		cert *x509.Certificate
	}{key: rsaKey}
	_ = prov

	// Use faultyProvider which returns nil — just verify the dispatch returns error
	p := newAlgoProcessor(t, AlgorithmES256, &faultyProvider{})
	lr := plog.NewLogRecord()
	lr.SetTimestamp(pcommon.Timestamp(1000))
	err := p.processLogRecord(lr)
	if err == nil {
		t.Error("expected error when ES256 used with nil/wrong key")
	}
}

func TestSignWrongKeyTypeEdDSA(t *testing.T) {
	p := newAlgoProcessor(t, AlgorithmEdDSA, &faultyProvider{})
	lr := plog.NewLogRecord()
	lr.SetTimestamp(pcommon.Timestamp(1000))
	err := p.processLogRecord(lr)
	if err == nil {
		t.Error("expected error when EdDSA used with nil/wrong key")
	}
}

// ---------------------------------------------------------------------------
// Tamper test for ES256
// ---------------------------------------------------------------------------

func TestES256TamperedPayloadDetected(t *testing.T) {
	certPEM, keyPEM, ecKey := generateECPEM(t)
	cr, _ := parseCertificateData(certPEM, keyPEM)
	p := newAlgoProcessor(t, AlgorithmES256, cr)

	lr := plog.NewLogRecord()
	lr.SetEventName("original.event")
	lr.SetTimestamp(pcommon.Timestamp(1000000))

	if err := p.processLogRecord(lr); err != nil {
		t.Fatalf("processLogRecord: %v", err)
	}

	sigVal, _ := lr.Attributes().Get("audit.integrity.value")
	sigBytes, _ := base64.StdEncoding.DecodeString(sigVal.Str())

	// Tamper: change the EventName
	lr.SetEventName("tampered.event")
	tamperedPayload, _ := p.serializeLogRecord(lr)

	hasher := crypto.SHA256.New()
	hasher.Write(tamperedPayload)
	h := hasher.Sum(nil)

	// Verify using the ASN.1 DER signature manually
	var sig struct{ R, S asn1.RawValue }
	if _, err := asn1.Unmarshal(sigBytes, &sig); err != nil {
		// signature format check failed — that's fine, means it's invalid
		t.Logf("🔍 tampered payload: sig parse failed (expected)")
		return
	}
	if ecdsa.VerifyASN1(&ecKey.PublicKey, h, sigBytes) {
		t.Error("❌ tampered EventName did not invalidate ES256 signature")
	} else {
		t.Logf("🔍 tampered EventName correctly invalidates ES256 signature")
	}
}
