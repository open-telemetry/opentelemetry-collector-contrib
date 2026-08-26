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
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"

	"github.com/gowebpki/jcs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const (
	// jcsMaxDepth caps nesting depth before calling jcs.Transform.
	// gowebpki/jcs has no depth limit: beyond ~1M levels the Go runtime kills
	// the process with a fatal stack overflow that recover() cannot catch, and
	// shallower depths cause quadratic CPU cost via string concatenation.
	jcsMaxDepth = 128
	// jcsMaxInputBytes caps the serialized JSON size before JCS canonicalization.
	jcsMaxInputBytes = 1 << 20 // 1 MiB
)

type signingProcessor struct {
	config       *Config
	logger       *zap.Logger
	nextLogs     consumer.Logs
	provider     KeyMaterialProvider
	hashFunc     func() hash.Hash
	jwaAlgorithm string // audit.integrity.algorithm value (e.g. "RS256")
	certRef      string // audit.integrity.certificate value (fingerprint or full DER)
}

func newProcessor(cfg *Config, nextLogs consumer.Logs, settings processor.Settings) (*signingProcessor, error) {
	ctx := context.Background()

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
		nextLogs:     nextLogs,
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
			resourceLog.Resource().Attributes().PutStr("audit.integrity.algorithm", p.jwaAlgorithm)
			if p.certRef != "" {
				resourceLog.Resource().Attributes().PutStr("audit.integrity.certificate", p.certRef)
			}
		}
	}

	return p.nextLogs.ConsumeLogs(ctx, ld)
}

// processLogRecord computes a canonical JSON hash (RFC 8785) of the log record
// (excluding audit.integrity.* attributes) and signs it with the configured algorithm.
// It adds one attribute:
//   - audit.integrity.value: base64-encoded signature
func (p *signingProcessor) processLogRecord(lr plog.LogRecord) error {
	logData, err := p.serializeLogRecord(lr)
	if err != nil {
		return fmt.Errorf("failed to serialize log record: %w", err)
	}

	signature, err := p.sign(logData)
	if err != nil {
		return fmt.Errorf("failed to sign log record: %w", err)
	}

	lr.Attributes().PutStr("audit.integrity.value", base64.StdEncoding.EncodeToString(signature))
	return nil
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

// serializeLogRecord produces a canonical JSON representation of the full log
// record. All audit.integrity.* attributes are excluded because they are added
// after serialization and must not be part of the signed payload.
func (p *signingProcessor) serializeLogRecord(lr plog.LogRecord) ([]byte, error) {
	data := make(map[string]any)

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

	attrs := make(map[string]any)
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		if !strings.HasPrefix(k, "audit.integrity.") {
			attrs[k] = p.valueToInterface(v)
		}
		return true
	})
	if len(attrs) > 0 {
		data["attributes"] = attrs
	}

	return p.marshalJCS(data)
}

// marshalJCS produces a RFC 8785 (JCS) canonical JSON byte slice.
// json.Marshal sorts map keys (Go ≥ 1.12), then jcs.Transform normalises
// number representation and validates the result per the JCS spec.
//
// Guards against gowebpki/jcs having no depth or size cap: deep nesting
// causes quadratic runtime and eventually a fatal stack overflow that
// recover() cannot catch.
func (*signingProcessor) marshalJCS(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(raw) > jcsMaxInputBytes {
		return nil, fmt.Errorf("serialized log record exceeds size limit (%d > %d bytes)", len(raw), jcsMaxInputBytes)
	}
	depth := 0
	for _, b := range raw {
		switch b {
		case '{', '[':
			depth++
			if depth > jcsMaxDepth {
				return nil, fmt.Errorf("serialized log record exceeds nesting depth limit (%d)", jcsMaxDepth)
			}
		case '}', ']':
			depth--
		}
	}
	return jcs.Transform(raw)
}

func (p *signingProcessor) valueToInterface(v pcommon.Value) any {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return v.Int()
	case pcommon.ValueTypeDouble:
		return v.Double()
	case pcommon.ValueTypeBool:
		return v.Bool()
	case pcommon.ValueTypeBytes:
		return base64.StdEncoding.EncodeToString(v.Bytes().AsRaw())
	case pcommon.ValueTypeSlice:
		slice := make([]any, v.Slice().Len())
		for i := 0; i < v.Slice().Len(); i++ {
			slice[i] = p.valueToInterface(v.Slice().At(i))
		}
		return slice
	case pcommon.ValueTypeMap:
		m := make(map[string]any)
		v.Map().Range(func(k string, val pcommon.Value) bool {
			m[k] = p.valueToInterface(val)
			return true
		})
		return m
	default:
		return nil
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
