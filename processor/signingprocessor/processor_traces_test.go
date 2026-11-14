// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signingprocessor

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"hash"
	"strconv"
	"strings"
	"testing"

	"github.com/gowebpki/jcs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newTestTracesProcessor(t *testing.T) (*signingProcessor, *staticProvider, *consumertest.TracesSink) {
	t.Helper()
	prov := newTestProvider(t)
	sink := &consumertest.TracesSink{}
	return &signingProcessor{
		config:       &Config{Algorithm: "RS256", CertificateRef: CertificateRefFingerprint},
		provider:     prov,
		nextTraces:   sink,
		hashFunc:     func() hash.Hash { return crypto.SHA256.New() },
		jwaAlgorithm: "RS256",
		certRef:      "sha256:test",
	}, prov, sink
}

// newTestSpan fills every span field that is part of the signed payload, plus
// the fields that are deliberately left out of it.
func newTestSpan(ss ptrace.ScopeSpans) ptrace.Span {
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetParentSpanID(pcommon.SpanID{8, 7, 6, 5, 4, 3, 2, 1})
	span.SetName("tool.call")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.Timestamp(1714041600000000000))
	span.SetEndTimestamp(pcommon.Timestamp(1714041600500000000))
	span.Attributes().PutStr("gen_ai.tool.name", "search")
	span.TraceState().FromRaw("vendor=value")
	span.SetFlags(1)
	span.SetDroppedAttributesCount(2)

	ev := span.Events().AppendEmpty()
	ev.SetName("decision")
	ev.SetTimestamp(pcommon.Timestamp(1714041600250000000))
	ev.Attributes().PutStr("choice", "a")

	link := span.Links().AppendEmpty()
	link.SetTraceID(pcommon.TraceID{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
	link.SetSpanID(pcommon.SpanID{2, 2, 2, 2, 2, 2, 2, 2})
	link.Attributes().PutStr("link.kind", "follows")

	span.Status().SetCode(ptrace.StatusCodeError)
	span.Status().SetMessage("tool failed")
	return span
}

// verifySpan rebuilds the documented span payload independently of
// serializeSpan, then checks the RSA signature over its JCS form.
func verifySpan(t *testing.T, span ptrace.Span, pubKey *rsa.PublicKey) error {
	t.Helper()
	sigVal, ok := span.Attributes().Get("audit.integrity.value")
	require.True(t, ok, "audit.integrity.value missing")

	nanos := func(ts pcommon.Timestamp) string { return strconv.FormatInt(ts.AsTime().UnixNano(), 10) }
	attrs := func(m pcommon.Map) map[string]any {
		out := map[string]any{}
		m.Range(func(k string, v pcommon.Value) bool {
			if !strings.HasPrefix(k, "audit.integrity.") {
				out[k] = map[string]any{"stringValue": v.Str()}
			}
			return true
		})
		return out
	}
	ev := span.Events().At(0)
	link := span.Links().At(0)
	data := map[string]any{
		"trace_id":       span.TraceID().String(),
		"span_id":        span.SpanID().String(),
		"parent_span_id": span.ParentSpanID().String(),
		"name":           span.Name(),
		"kind":           int32(span.Kind()),
		"start_time":     nanos(span.StartTimestamp()),
		"end_time":       nanos(span.EndTimestamp()),
		"attributes":     attrs(span.Attributes()),
		"events": []any{map[string]any{
			"name":       ev.Name(),
			"timestamp":  nanos(ev.Timestamp()),
			"attributes": attrs(ev.Attributes()),
		}},
		"links": []any{map[string]any{
			"trace_id":   link.TraceID().String(),
			"span_id":    link.SpanID().String(),
			"attributes": attrs(link.Attributes()),
		}},
		"status": map[string]any{"code": int32(span.Status().Code()), "message": span.Status().Message()},
	}

	raw, err := json.Marshal(data)
	require.NoError(t, err)
	canonical, err := jcs.Transform(raw)
	require.NoError(t, err)
	digest := sha256.Sum256(canonical)

	sig, err := base64.StdEncoding.DecodeString(sigVal.Str())
	require.NoError(t, err)
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, digest[:], sig)
}

func TestConsumeTracesSignsSpans(t *testing.T) {
	p, prov, sink := newTestTracesProcessor(t)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	newTestSpan(rs.ScopeSpans().AppendEmpty())
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	require.Len(t, sink.AllTraces(), 1)

	got := sink.AllTraces()[0].ResourceSpans()
	span := got.At(0).ScopeSpans().At(0).Spans().At(0)
	require.NoError(t, verifySpan(t, span, &prov.key.PublicKey))

	signer, _ := span.Attributes().Get("audit.integrity.signer")
	assert.Equal(t, "collector", signer.Str())
	alg, _ := got.At(0).Resource().Attributes().Get("audit.integrity.algorithm")
	assert.Equal(t, "RS256", alg.Str())
	cert, _ := got.At(0).Resource().Attributes().Get("audit.integrity.certificate")
	assert.Equal(t, "sha256:test", cert.Str())

	_, marked := got.At(1).Resource().Attributes().Get("audit.integrity.algorithm")
	assert.False(t, marked, "a resource with no spans must not get integrity attributes")
}

func TestSignedSpanDetectsTampering(t *testing.T) {
	tamper := map[string]func(ptrace.Span){
		"name":            func(s ptrace.Span) { s.SetName("other") },
		"parent span id":  func(s ptrace.Span) { s.SetParentSpanID(pcommon.SpanID{9, 9, 9, 9, 9, 9, 9, 9}) },
		"end time":        func(s ptrace.Span) { s.SetEndTimestamp(s.EndTimestamp() + 1) },
		"attribute":       func(s ptrace.Span) { s.Attributes().PutStr("gen_ai.tool.name", "delete") },
		"event attribute": func(s ptrace.Span) { s.Events().At(0).Attributes().PutStr("choice", "b") },
		"link span id":    func(s ptrace.Span) { s.Links().At(0).SetSpanID(pcommon.SpanID{3, 3, 3, 3, 3, 3, 3, 3}) },
		"status code":     func(s ptrace.Span) { s.Status().SetCode(ptrace.StatusCodeOk) },
	}
	for name, mutate := range tamper {
		t.Run(name, func(t *testing.T) {
			p, prov, _ := newTestTracesProcessor(t)
			span := newTestSpan(ptrace.NewScopeSpans())
			require.NoError(t, p.processSpan(span))

			mutate(span)
			assert.Error(t, verifySpan(t, span, &prov.key.PublicKey))
		})
	}
}

func TestSerializeSpanExcludedFields(t *testing.T) {
	p, _, _ := newTestTracesProcessor(t)
	span := newTestSpan(ptrace.NewScopeSpans())
	before, err := p.serializeSpan(span)
	require.NoError(t, err)

	span.TraceState().FromRaw("vendor=changed")
	span.SetFlags(0)
	span.SetDroppedAttributesCount(9)
	span.SetDroppedEventsCount(9)
	span.SetDroppedLinksCount(9)
	span.Links().At(0).TraceState().FromRaw("vendor=changed")
	span.Attributes().PutStr("audit.integrity.value", "x")
	span.Events().At(0).Attributes().PutStr("audit.integrity.value", "x")
	span.Links().At(0).Attributes().PutStr("audit.integrity.value", "x")

	after, err := p.serializeSpan(span)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

func TestSerializeSpanRejectsInvalidUTF8(t *testing.T) {
	p, _, _ := newTestTracesProcessor(t)
	span := ptrace.NewSpan()
	span.SetName("bad\xff")
	_, err := p.serializeSpan(span)
	assert.ErrorContains(t, err, "span name")
}
