// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/signingprocessor"

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/canonicaljson"
)

type signingProcessor struct {
	config       *Config
	logger       *zap.Logger
	nextLogs     consumer.Logs
	nextTraces   consumer.Traces
	provider     KeyMaterialProvider
	hashFunc     func() hash.Hash
	jwaAlgorithm string // audit.integrity.algorithm value (e.g. "RS256")
	certRef      string // audit.integrity.certificate value (fingerprint or full DER)
}

func newProcessor(ctx context.Context, cfg *Config, settings processor.Settings) (*signingProcessor, error) {
	provider, err := newKeyMaterialProvider(ctx, cfg, settings.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize key material provider: %w", err)
	}

	var hashFunc func() hash.Hash
	switch cfg.GetHash() {
	case crypto.SHA256:
		hashFunc = func() hash.Hash { return crypto.SHA256.New() }
	case crypto.SHA512:
		hashFunc = func() hash.Hash { return crypto.SHA512.New() }
	default:
		// EdDSA (crypto.Hash(0)): no pre-hashing; hashFunc left nil
	}

	var certRef string
	if cfg.Algorithm != AlgorithmHMACSHA256 {
		certRef, err = buildCertificateRef(provider, cfg.CertificateRef)
		if err != nil {
			return nil, fmt.Errorf("failed to build certificate reference: %w", err)
		}
	}

	return &signingProcessor{
		config:       cfg,
		logger:       settings.Logger,
		provider:     provider,
		hashFunc:     hashFunc,
		jwaAlgorithm: cfg.Algorithm,
		certRef:      certRef,
	}, nil
}

func (*signingProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *signingProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resourceLog := resourceLogs.At(i)

		signed := 0
		scopeLogs := resourceLog.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			scopeLog := scopeLogs.At(j)
			logRecords := scopeLog.LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				logRecord := logRecords.At(k)
				if err := p.processLogRecord(logRecord); err != nil {
					return fmt.Errorf("failed to process log record: %w", err)
				}
				signed++
			}
		}

		// audit.integrity.algorithm and audit.integrity.certificate are Resource-level
		// attributes per the audit logging spec. Set them only after all records in
		// this ResourceLogs block have been signed successfully, and only when at
		// least one record was actually signed.
		if signed > 0 {
			p.putResourceIntegrity(resourceLog.Resource().Attributes())
		}
	}

	return p.nextLogs.ConsumeLogs(ctx, ld)
}

// processLogRecord computes a canonical JSON hash (RFC 8785) of the log record
// and signs it with the configured algorithm. It adds two attributes:
//   - audit.integrity.value:  base64-encoded signature
//   - audit.integrity.signer: "collector"
//
// Fields excluded from the signed payload (per spec or by design):
//   - audit.integrity.* attributes (carry the proof itself)
//   - SeverityNumber / SeverityText (SHOULD NOT be set on audit records per spec)
//   - Flags (W3C trace-context sampling bit; observability concept, not audit-relevant;
//     may be modified by intermediaries without breaking audit semantics)
func (p *signingProcessor) processLogRecord(lr plog.LogRecord) error {
	opts := canonicaljson.SerializeLogRecordOptions{
		ExcludePrefixes: []string{"audit.integrity."},
		// Uses default MaxDepth (128) and MaxInputBytes (2 MiB)
	}
	logData, err := canonicaljson.SerializeLogRecord(lr, opts)
	if err != nil {
		return fmt.Errorf("failed to serialize log record: %w", err)
	}

	signature, err := p.sign(logData)
	if err != nil {
		return fmt.Errorf("failed to sign log record: %w", err)
	}

	lr.Attributes().PutStr("audit.integrity.value", base64.StdEncoding.EncodeToString(signature))
	lr.Attributes().PutStr("audit.integrity.signer", "collector")
	return nil
}

// ConsumeTraces signs every span the same way ConsumeLogs signs log records.
func (p *signingProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		resourceSpan := resourceSpans.At(i)

		signed := 0
		scopeSpans := resourceSpan.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				if err := p.processSpan(spans.At(k)); err != nil {
					return fmt.Errorf("failed to process span: %w", err)
				}
				signed++
			}
		}

		if signed > 0 {
			p.putResourceIntegrity(resourceSpan.Resource().Attributes())
		}
	}

	return p.nextTraces.ConsumeTraces(ctx, td)
}

func (p *signingProcessor) processSpan(span ptrace.Span) error {
	opts := canonicaljson.SerializeSpanOptions{
		ExcludePrefixes: []string{"audit.integrity."},
	}
	spanData, err := canonicaljson.SerializeSpan(span, opts)
	if err != nil {
		return fmt.Errorf("failed to serialize span: %w", err)
	}

	signature, err := p.sign(spanData)
	if err != nil {
		return fmt.Errorf("failed to sign span: %w", err)
	}

	span.Attributes().PutStr("audit.integrity.value", base64.StdEncoding.EncodeToString(signature))
	span.Attributes().PutStr("audit.integrity.signer", "collector")
	return nil
}

// putResourceIntegrity sets the Resource-level audit.integrity.algorithm and
// audit.integrity.certificate attributes.
func (p *signingProcessor) putResourceIntegrity(attrs pcommon.Map) {
	attrs.PutStr("audit.integrity.algorithm", p.jwaAlgorithm)
	if p.certRef != "" {
		attrs.PutStr("audit.integrity.certificate", p.certRef)
	}
}

// serializeLogRecord is a test helper that wraps canonicaljson.SerializeLogRecord
// with the signing processor's default options.
func (*signingProcessor) serializeLogRecord(lr plog.LogRecord) ([]byte, error) {
	opts := canonicaljson.SerializeLogRecordOptions{
		ExcludePrefixes: []string{"audit.integrity."},
	}
	return canonicaljson.SerializeLogRecord(lr, opts)
}

// sign produces a signature over payload using the configured JWA algorithm.
// For RS256/RS512/ES256 it hashes payload first; for EdDSA it passes the raw
// message because ed25519 hashes internally (SHA-512); for HMAC-SHA256 it
// computes an HMAC-SHA256 MAC.
func (p *signingProcessor) sign(payload []byte) ([]byte, error) {
	switch p.config.Algorithm {
	case AlgorithmRS256, AlgorithmRS512:
		h := p.hashFunc()
		if _, err := h.Write(payload); err != nil {
			return nil, fmt.Errorf("failed to compute hash: %w", err)
		}
		hashBytes := h.Sum(nil)
		privateKey := p.provider.GetPrivateKey()
		rsaKey, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("algorithm %s requires an RSA private key", p.config.Algorithm)
		}
		return rsa.SignPKCS1v15(rand.Reader, rsaKey, p.config.GetHash(), hashBytes)

	case AlgorithmES256:
		h := p.hashFunc()
		if _, err := h.Write(payload); err != nil {
			return nil, fmt.Errorf("failed to compute hash: %w", err)
		}
		hashBytes := h.Sum(nil)
		privateKey := p.provider.GetPrivateKey()
		ecKey, ok := privateKey.(*ecdsa.PrivateKey)
		if !ok {
			return nil, errors.New("algorithm ES256 requires an ECDSA private key")
		}
		return ecdsa.SignASN1(rand.Reader, ecKey, hashBytes)

	case AlgorithmEdDSA:
		privateKey := p.provider.GetPrivateKey()
		edKey, ok := privateKey.(ed25519.PrivateKey)
		if !ok {
			return nil, errors.New("algorithm EdDSA requires an Ed25519 private key")
		}
		// Ed25519 signs the raw message; no pre-hashing.
		return ed25519.Sign(edKey, payload), nil

	case AlgorithmHMACSHA256:
		key := p.provider.GetHMACKey()
		if len(key) == 0 {
			return nil, errors.New("algorithm HMAC-SHA256 requires a non-empty HMAC key")
		}
		mac := hmac.New(sha256.New, key)
		if _, err := mac.Write(payload); err != nil {
			return nil, fmt.Errorf("failed to compute HMAC: %w", err)
		}
		return mac.Sum(nil), nil

	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", p.config.Algorithm)
	}
}

func (*signingProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*signingProcessor) Shutdown(_ context.Context) error {
	return nil
}

// buildCertificateRef computes the audit.integrity.certificate attribute value.
// "fingerprint" produces "sha256:<hex>" of the DER-encoded certificate.
// "full" produces the base64 (standard, no line wrapping) of the DER-encoded certificate.
func buildCertificateRef(provider KeyMaterialProvider, mode string) (string, error) {
	cert := provider.GetCertificate()
	if cert == nil {
		return "", errors.New("key material provider returned nil certificate")
	}
	der := cert.Raw
	switch mode {
	case CertificateRefFull:
		return base64.StdEncoding.EncodeToString(der), nil
	default: // CertificateRefFingerprint
		sum := sha256.Sum256(der)
		return "sha256:" + hex.EncodeToString(sum[:]), nil
	}
}
