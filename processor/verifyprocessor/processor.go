// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/verifyprocessor"

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gowebpki/jcs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const (
	jsonMaxDepth      = 128
	jsonMaxInputBytes = 1 << 21 // 2 MiB
	// maxFailureDetails caps how many record[r,s,i] indexes appear in a
	// permanent verification error; remaining failures are summarized as
	// "and N more" so large batches cannot inflate the error string.
	maxFailureDetails = 8

	attrIntegrityValue       = "audit.integrity.value"
	attrIntegrityAlgorithm   = "audit.integrity.algorithm"
	attrIntegrityCertificate = "audit.integrity.certificate"
	attrIntegrityPrefix      = "audit.integrity."
	attrRecordID             = "audit.record.id"

	attrVerificationStatus  = "audit.verification.status"
	attrVerificationReason  = "audit.verification.reason"
	attrVerificationProfile = "audit.verification.profile"
	attrVerificationPrefix  = "audit.verification."

	statusVerified = "verified"
	statusFailed   = "failed"

	reasonMissingIntegrityValue = "missing_integrity_value"
	reasonMissingAlgorithm      = "missing_algorithm"
	reasonUnsupportedAlgorithm  = "unsupported_algorithm"
	reasonCertificateMismatch   = "certificate_mismatch"
	reasonMissingCertificate    = "missing_certificate"
	reasonInvalidSignature      = "invalid_signature_encoding"
	reasonIntegrityMismatch     = "integrity_mismatch"
	reasonSerializationError    = "serialization_error"
	reasonKeyMaterialMismatch   = "key_material_mismatch"

	algorithmRS256      = "RS256"
	algorithmRS512      = "RS512"
	algorithmES256      = "ES256"
	algorithmEdDSA      = "EdDSA"
	algorithmHMACSHA256 = "HMAC-SHA256"
)

type verifyFailure struct {
	reason string
	err    error
}

func (f *verifyFailure) Error() string {
	if f.err != nil {
		return fmt.Sprintf("%s: %v", f.reason, f.err)
	}
	return f.reason
}

type logFailure struct {
	logID    string
	resource int
	scope    int
	record   int
}

type verificationBatchError struct {
	failures []logFailure
}

// consumeLogsError reports a downstream export failure together with the
// verification failures from the same batch. Unwrap returns only the export
// error so permanence follows the exporter.
type consumeLogsError struct {
	export error
	verify error
}

func (e *consumeLogsError) Error() string {
	return e.export.Error() + "; " + e.verify.Error()
}

func (e *consumeLogsError) Unwrap() error {
	return e.export
}

func (e *verificationBatchError) Error() string {
	n := len(e.failures)
	shown := min(n, maxFailureDetails)
	parts := make([]string, 0, shown+1)
	parts = append(parts, fmt.Sprintf("verification failed for %d log record(s)", n))
	for i := range shown {
		parts = append(parts, e.failures[i].String())
	}
	if n > shown {
		parts = append(parts, fmt.Sprintf("and %d more", n-shown))
	}
	return strings.Join(parts, "; ")
}

func (f logFailure) String() string {
	if f.logID != "" {
		return fmt.Sprintf("id=%s", f.logID)
	}
	return fmt.Sprintf("record[%d,%d,%d]", f.resource, f.scope, f.record)
}

func recordLogID(lr plog.LogRecord) string {
	if v, ok := lr.Attributes().Get(attrRecordID); ok {
		return v.Str()
	}
	return ""
}

type verifyProcessor struct {
	config       *Config
	logger       *zap.Logger
	nextConsumer consumer.Logs
	provider     KeyMaterialProvider
}

func newProcessor(ctx context.Context, cfg *Config, nextConsumer consumer.Logs, settings processor.Settings) (*verifyProcessor, error) {
	provider, err := newKeyMaterialProvider(ctx, cfg, settings.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize key material provider: %w", err)
	}

	if cfg.DeadLetter.Enabled {
		settings.Logger.Warn("dead_letter.enabled is set but dead-letter persistence is not yet implemented; verification failures will not be written to storage")
	}

	return &verifyProcessor{
		config:       cfg,
		logger:       settings.Logger,
		nextConsumer: nextConsumer,
		provider:     provider,
	}, nil
}

func (*verifyProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (*verifyProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*verifyProcessor) Shutdown(_ context.Context) error {
	return nil
}

type recordOutcome struct {
	fail *verifyFailure
	id   string
}

func (p *verifyProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	resourceLogs := ld.ResourceLogs()
	var failures []logFailure
	stamp := p.config.FailureMode == failureModeMark || p.config.FailureMode == failureModeDrop

	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		resAttrs := rl.Resource().Attributes()

		algorithm := ""
		if algoVal, hasAlgo := resAttrs.Get(attrIntegrityAlgorithm); hasAlgo {
			algorithm = algoVal.Str()
		}
		certRef := ""
		if v, ok := resAttrs.Get(attrIntegrityCertificate); ok {
			certRef = v.Str()
		}
		resourceFail := p.checkResourceKeyMaterial(algorithm, certRef)

		scopeLogs := rl.ScopeLogs()
		scopeOutcomes := make([][]recordOutcome, scopeLogs.Len())
		for j := 0; j < scopeLogs.Len(); j++ {
			records := scopeLogs.At(j).LogRecords()
			outcomes := make([]recordOutcome, records.Len())
			for k := 0; k < records.Len(); k++ {
				lr := records.At(k)
				fail := resourceFail
				if fail == nil {
					fail = p.verifyLogRecord(lr, algorithm)
				}
				outcomes[k] = recordOutcome{fail: fail, id: recordLogID(lr)}
				if fail != nil {
					p.logger.Debug("log record verification failed",
						zap.String("reason", fail.reason),
						zap.Error(fail.err),
					)
					if p.config.FailureMode != failureModeMark {
						failures = append(failures, logFailure{
							logID:    outcomes[k].id,
							resource: i,
							scope:    j,
							record:   k,
						})
					}
				}
			}
			scopeOutcomes[j] = outcomes
		}

		if stamp {
			p.applyHierarchicalStamps(rl, scopeOutcomes)
		}

		if p.config.FailureMode == failureModeDrop {
			for j := 0; j < scopeLogs.Len(); j++ {
				outcomes := scopeOutcomes[j]
				recordIdx := 0
				scopeLogs.At(j).LogRecords().RemoveIf(func(plog.LogRecord) bool {
					idx := recordIdx
					recordIdx++
					return outcomes[idx].fail != nil
				})
			}
		}
	}

	if p.config.FailureMode == failureModeStrict && len(failures) > 0 {
		return consumererror.NewPermanent(&verificationBatchError{failures: failures})
	}

	if err := p.nextConsumer.ConsumeLogs(ctx, ld); err != nil {
		if len(failures) == 0 {
			return err
		}
		return &consumeLogsError{
			export: err,
			verify: consumererror.NewPermanent(&verificationBatchError{failures: failures}),
		}
	}
	if len(failures) > 0 {
		return consumererror.NewPermanent(&verificationBatchError{failures: failures})
	}
	return nil
}

// applyHierarchicalStamps writes audit.verification.* at the highest uniform
// level: resource if every record agrees, else scope if that scope agrees,
// else each log record when a scope is mixed.
func (p *verifyProcessor) applyHierarchicalStamps(rl plog.ResourceLogs, scopeOutcomes [][]recordOutcome) {
	total, failed := 0, 0
	var firstFailReason string
	uniformFailReason := true
	for _, outcomes := range scopeOutcomes {
		for _, o := range outcomes {
			total++
			if o.fail == nil {
				continue
			}
			failed++
			if firstFailReason == "" {
				firstFailReason = o.fail.reason
			} else if o.fail.reason != firstFailReason {
				uniformFailReason = false
			}
		}
	}
	if total == 0 {
		return
	}

	resAttrs := rl.Resource().Attributes()
	scopeLogs := rl.ScopeLogs()

	if failed == 0 {
		p.stampSuccessAttrs(resAttrs)
		for j := 0; j < scopeLogs.Len(); j++ {
			p.clearVerificationAttrs(scopeLogs.At(j).Scope().Attributes())
			p.clearRecordVerificationAttrs(scopeLogs.At(j).LogRecords())
		}
		return
	}

	if failed == total {
		reason := firstFailReason
		if !uniformFailReason {
			reason = ""
		}
		p.stampFailureAttrs(resAttrs, reason)
		for j := 0; j < scopeLogs.Len(); j++ {
			p.clearVerificationAttrs(scopeLogs.At(j).Scope().Attributes())
			p.clearRecordVerificationAttrs(scopeLogs.At(j).LogRecords())
		}
		return
	}

	p.clearVerificationAttrs(resAttrs)
	for j := 0; j < scopeLogs.Len(); j++ {
		sl := scopeLogs.At(j)
		outcomes := scopeOutcomes[j]
		if len(outcomes) == 0 {
			continue
		}
		scopeFailed := 0
		scopeFirstReason := ""
		scopeUniform := true
		for _, o := range outcomes {
			if o.fail == nil {
				continue
			}
			scopeFailed++
			if scopeFirstReason == "" {
				scopeFirstReason = o.fail.reason
			} else if o.fail.reason != scopeFirstReason {
				scopeUniform = false
			}
		}
		scopeAttrs := sl.Scope().Attributes()
		records := sl.LogRecords()
		switch {
		case scopeFailed == 0:
			p.stampSuccessAttrs(scopeAttrs)
			p.clearRecordVerificationAttrs(records)
		case scopeFailed == len(outcomes):
			reason := scopeFirstReason
			if !scopeUniform {
				reason = ""
			}
			p.stampFailureAttrs(scopeAttrs, reason)
			p.clearRecordVerificationAttrs(records)
		default:
			p.clearVerificationAttrs(scopeAttrs)
			for k := 0; k < records.Len(); k++ {
				lr := records.At(k)
				if outcomes[k].fail == nil {
					p.stampSuccessAttrs(lr.Attributes())
				} else {
					p.stampFailureAttrs(lr.Attributes(), outcomes[k].fail.reason)
				}
			}
		}
	}
}

func (p *verifyProcessor) clearRecordVerificationAttrs(records plog.LogRecordSlice) {
	for i := 0; i < records.Len(); i++ {
		p.clearVerificationAttrs(records.At(i).Attributes())
	}
}

func (*verifyProcessor) clearVerificationAttrs(attrs pcommon.Map) {
	attrs.RemoveIf(func(k string, _ pcommon.Value) bool {
		return strings.HasPrefix(k, attrVerificationPrefix)
	})
}

func (p *verifyProcessor) stampSuccessAttrs(attrs pcommon.Map) {
	attrs.PutStr(attrVerificationStatus, statusVerified)
	attrs.PutStr(attrVerificationProfile, p.config.VerificationProfile)
	attrs.Remove(attrVerificationReason)
}

func (p *verifyProcessor) stampFailureAttrs(attrs pcommon.Map, reason string) {
	attrs.PutStr(attrVerificationStatus, statusFailed)
	attrs.PutStr(attrVerificationProfile, p.config.VerificationProfile)
	if reason == "" {
		attrs.Remove(attrVerificationReason)
	} else {
		attrs.PutStr(attrVerificationReason, reason)
	}
}

func (p *verifyProcessor) checkResourceKeyMaterial(algorithm, certRef string) *verifyFailure {
	if algorithm == "" {
		return &verifyFailure{reason: reasonMissingAlgorithm}
	}
	switch algorithm {
	case algorithmRS256, algorithmRS512, algorithmES256, algorithmEdDSA:
		if p.provider.GetCertificate() == nil {
			return &verifyFailure{reason: reasonKeyMaterialMismatch, err: errors.New("asymmetric algorithm requires a configured certificate")}
		}
		if certRef == "" {
			return &verifyFailure{reason: reasonMissingCertificate}
		}
		if !certificateRefMatches(p.provider.GetCertificate(), certRef) {
			return &verifyFailure{reason: reasonCertificateMismatch}
		}
		if err := certificateMatchesAlgorithm(p.provider.GetCertificate(), algorithm); err != nil {
			return &verifyFailure{reason: reasonKeyMaterialMismatch, err: err}
		}
	case algorithmHMACSHA256:
		if len(p.provider.GetHMACKey()) == 0 {
			return &verifyFailure{reason: reasonKeyMaterialMismatch, err: errors.New("HMAC-SHA256 requires a configured hmac_key")}
		}
	default:
		return &verifyFailure{reason: reasonUnsupportedAlgorithm, err: fmt.Errorf("algorithm %q", algorithm)}
	}
	return nil
}

func (p *verifyProcessor) verifyLogRecord(lr plog.LogRecord, algorithm string) *verifyFailure {
	sigVal, ok := lr.Attributes().Get(attrIntegrityValue)
	if !ok || sigVal.Str() == "" {
		return &verifyFailure{reason: reasonMissingIntegrityValue}
	}

	payload, err := p.serializeLogRecord(lr)
	if err != nil {
		return &verifyFailure{reason: reasonSerializationError, err: err}
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sigVal.Str())
	if err != nil {
		return &verifyFailure{reason: reasonInvalidSignature, err: err}
	}

	err = p.verifySignature(algorithm, payload, sigBytes)
	if err == nil {
		return nil
	}
	if !hasVerificationAttrs(lr) || !p.verificationProfileMatches(lr) {
		return &verifyFailure{reason: reasonIntegrityMismatch, err: err}
	}

	stampedPayload, serErr := p.serializePayload(lr, true)
	if serErr != nil {
		return &verifyFailure{reason: reasonSerializationError, err: serErr}
	}
	if retryErr := p.verifySignature(algorithm, stampedPayload, sigBytes); retryErr != nil {
		return &verifyFailure{reason: reasonIntegrityMismatch, err: err}
	}
	return nil
}

// hasVerificationAttrs reports whether a previous verifyprocessor stamped the
// record. A signer placed after that verifier signs over those stamps, so the
// payload must be rebuilt with them included to match its signature.
func (p *verifyProcessor) verificationProfileMatches(lr plog.LogRecord) bool {
	v, ok := lr.Attributes().Get(attrVerificationProfile)
	if !ok {
		return false
	}
	return v.Str() == p.config.VerificationProfile
}

func hasVerificationAttrs(lr plog.LogRecord) bool {
	found := false
	lr.Attributes().Range(func(k string, _ pcommon.Value) bool {
		if strings.HasPrefix(k, attrVerificationPrefix) {
			found = true
			return false
		}
		return true
	})
	return found
}

func (p *verifyProcessor) verifySignature(algorithm string, payload, sig []byte) error {
	switch algorithm {
	case algorithmRS256:
		pub, ok := p.provider.GetCertificate().PublicKey.(*rsa.PublicKey)
		if !ok {
			return errors.New("RS256 requires an RSA certificate public key")
		}
		sum := sha256.Sum256(payload)
		return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig)

	case algorithmRS512:
		pub, ok := p.provider.GetCertificate().PublicKey.(*rsa.PublicKey)
		if !ok {
			return errors.New("RS512 requires an RSA certificate public key")
		}
		sum := sha512.Sum512(payload)
		return rsa.VerifyPKCS1v15(pub, crypto.SHA512, sum[:], sig)

	case algorithmES256:
		pub, ok := p.provider.GetCertificate().PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return errors.New("ES256 requires an ECDSA certificate public key")
		}
		sum := sha256.Sum256(payload)
		if !ecdsa.VerifyASN1(pub, sum[:], sig) {
			return errors.New("ECDSA signature verification failed")
		}
		return nil

	case algorithmEdDSA:
		pub, ok := p.provider.GetCertificate().PublicKey.(ed25519.PublicKey)
		if !ok {
			return errors.New("EdDSA requires an Ed25519 certificate public key")
		}
		if !ed25519.Verify(pub, payload, sig) {
			return errors.New("Ed25519 signature verification failed")
		}
		return nil

	case algorithmHMACSHA256:
		mac := hmac.New(sha256.New, p.provider.GetHMACKey())
		if _, err := mac.Write(payload); err != nil {
			return err
		}
		if !hmac.Equal(mac.Sum(nil), sig) {
			return errors.New("HMAC verification failed")
		}
		return nil

	default:
		return fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
}

// TODO: support additional certificate reference methods (Key ID, Subject Key
// Identifier, Issuer+Serial) once signingprocessor can emit them.
func certificateRefMatches(cert *x509.Certificate, ref string) bool {
	if cert == nil {
		return false
	}
	fingerprint := "sha256:" + hex.EncodeToString(sha256Sum(cert.Raw))
	if ref == fingerprint {
		return true
	}
	full := base64.StdEncoding.EncodeToString(cert.Raw)
	return ref == full
}

func certificateMatchesAlgorithm(cert *x509.Certificate, algorithm string) error {
	if cert == nil {
		return errors.New("certificate is nil")
	}
	switch algorithm {
	case algorithmRS256, algorithmRS512:
		if _, ok := cert.PublicKey.(*rsa.PublicKey); !ok {
			return fmt.Errorf("%s requires an RSA certificate public key", algorithm)
		}
	case algorithmES256:
		if _, ok := cert.PublicKey.(*ecdsa.PublicKey); !ok {
			return errors.New("ES256 requires an ECDSA certificate public key")
		}
	case algorithmEdDSA:
		if _, ok := cert.PublicKey.(ed25519.PublicKey); !ok {
			return errors.New("EdDSA requires an Ed25519 certificate public key")
		}
	}
	return nil
}

func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

func (p *verifyProcessor) serializeLogRecord(lr plog.LogRecord) ([]byte, error) {
	return p.serializePayload(lr, false)
}

func (p *verifyProcessor) serializePayload(lr plog.LogRecord, includeVerification bool) ([]byte, error) {
	data := make(map[string]any)

	if lr.EventName() != "" {
		if err := checkUTF8(lr.EventName(), "event name"); err != nil {
			return nil, err
		}
		data["event_name"] = lr.EventName()
	}

	if lr.Body().Type() != pcommon.ValueTypeEmpty {
		body, err := p.valueToInterface(lr.Body(), 0)
		if err != nil {
			return nil, fmt.Errorf("log record body: %w", err)
		}
		data["body"] = body
	}

	if lr.Timestamp() != 0 {
		data["timestamp"] = strconv.FormatInt(lr.Timestamp().AsTime().UnixNano(), 10)
	}

	if lr.ObservedTimestamp() != 0 {
		data["observed_timestamp"] = strconv.FormatInt(lr.ObservedTimestamp().AsTime().UnixNano(), 10)
	}

	if !lr.TraceID().IsEmpty() {
		data["trace_id"] = lr.TraceID().String()
	}

	if !lr.SpanID().IsEmpty() {
		data["span_id"] = lr.SpanID().String()
	}

	attrs := make(map[string]any)
	var attrErr error
	lr.Attributes().Range(func(k string, v pcommon.Value) bool {
		if strings.HasPrefix(k, attrIntegrityPrefix) {
			return true
		}
		if !includeVerification && strings.HasPrefix(k, attrVerificationPrefix) {
			return true
		}
		if err := checkUTF8(k, "attribute key"); err != nil {
			attrErr = err
			return false
		}
		val, err := p.valueToInterface(v, 0)
		if err != nil {
			attrErr = err
			return false
		}
		attrs[k] = val
		return true
	})
	if attrErr != nil {
		return nil, attrErr
	}
	if len(attrs) > 0 {
		data["attributes"] = attrs
	}

	return p.marshalJCS(data)
}

func (*verifyProcessor) marshalJCS(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(raw) > jsonMaxInputBytes {
		return nil, fmt.Errorf("serialized log record exceeds size limit (%d > %d bytes)", len(raw), jsonMaxInputBytes)
	}
	return jcs.Transform(raw)
}

func (p *verifyProcessor) valueToInterface(v pcommon.Value, depth int) (any, error) {
	if depth > jsonMaxDepth {
		return nil, fmt.Errorf("value exceeds nesting depth limit (%d)", jsonMaxDepth)
	}
	switch v.Type() {
	case pcommon.ValueTypeStr:
		s := v.Str()
		if err := checkUTF8(s, "string value"); err != nil {
			return nil, err
		}
		return map[string]any{"stringValue": s}, nil
	case pcommon.ValueTypeInt:
		return map[string]any{"intValue": strconv.FormatInt(v.Int(), 10)}, nil
	case pcommon.ValueTypeDouble:
		return map[string]any{"doubleValue": v.Double()}, nil
	case pcommon.ValueTypeBool:
		return map[string]any{"boolValue": v.Bool()}, nil
	case pcommon.ValueTypeBytes:
		return map[string]any{"bytesValue": base64.StdEncoding.EncodeToString(v.Bytes().AsRaw())}, nil
	case pcommon.ValueTypeSlice:
		slice := make([]any, v.Slice().Len())
		for i := 0; i < v.Slice().Len(); i++ {
			val, err := p.valueToInterface(v.Slice().At(i), depth+1)
			if err != nil {
				return nil, err
			}
			slice[i] = val
		}
		return slice, nil
	case pcommon.ValueTypeMap:
		m := make(map[string]any)
		var mapErr error
		v.Map().Range(func(k string, val pcommon.Value) bool {
			if err := checkUTF8(k, "map key"); err != nil {
				mapErr = err
				return false
			}
			converted, err := p.valueToInterface(val, depth+1)
			if err != nil {
				mapErr = err
				return false
			}
			m[k] = converted
			return true
		})
		if mapErr != nil {
			return nil, mapErr
		}
		return m, nil
	case pcommon.ValueTypeEmpty:
		return map[string]any{}, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %s", v.Type())
	}
}

func checkUTF8(s, what string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%s contains invalid UTF-8", what)
	}
	return nil
}
