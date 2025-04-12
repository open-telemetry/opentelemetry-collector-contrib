// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv2

import (
	"errors"
	"testing"

	zipkinmodel "github.com/openzipkin/zipkin-go/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

func TestInternalTracesToZipkinSpans(t *testing.T) {
	tests := []struct {
		name string
		td   ptrace.Traces
		zs   []*zipkinmodel.SpanModel
		err  error
	}{
		{
			name: "empty",
			td:   ptrace.NewTraces(),
			err:  nil,
		},
		{
			name: "oneEmpty",
			td:   testdata.GenerateTracesOneEmptyResourceSpans(),
			zs:   []*zipkinmodel.SpanModel{},
			err:  nil,
		},
		{
			name: "noLibs",
			td:   testdata.GenerateTracesNoLibraries(),
			zs:   []*zipkinmodel.SpanModel{},
			err:  nil,
		},
		{
			name: "oneEmptyLib",
			td:   testdata.GenerateTracesOneEmptyInstrumentationLibrary(),
			zs:   []*zipkinmodel.SpanModel{},
			err:  nil,
		},
		{
			name: "oneSpanNoResource",
			td:   testdata.GenerateTracesOneSpanNoResource(),
			zs:   []*zipkinmodel.SpanModel{},
			err:  errors.New("TraceID is invalid"),
		},
		{
			name: "oneSpanOk",
			td:   generateTraceOneSpanOneTraceID(ptrace.StatusCodeOk),
			zs:   []*zipkinmodel.SpanModel{zipkinOneSpan(ptrace.StatusCodeOk)},
			err:  nil,
		},
		{
			name: "oneSpanError",
			td:   generateTraceOneSpanOneTraceID(ptrace.StatusCodeError),
			zs:   []*zipkinmodel.SpanModel{zipkinOneSpan(ptrace.StatusCodeError)},
			err:  nil,
		},
		{
			name: "oneSpanUnset",
			td:   generateTraceOneSpanOneTraceID(ptrace.StatusCodeUnset),
			zs:   []*zipkinmodel.SpanModel{zipkinOneSpan(ptrace.StatusCodeUnset)},
			err:  nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spans, err := FromTranslator{}.FromTraces(test.td)
			assert.Equal(t, test.err, err)
			if test.name == "empty" {
				assert.Nil(t, spans)
			} else {
				assert.Len(t, spans, len(test.zs))
				assert.Equal(t, test.zs, spans)
			}
		})
	}
}

func TestInternalTracesToZipkinSpansAndBack(t *testing.T) {
	tds, err := goldendataset.GenerateTraces(
		"../../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_traces.txt",
		"../../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_spans.txt")
	assert.NoError(t, err)
	for _, td := range tds {
		zipkinSpans, err := FromTranslator{}.FromTraces(td)
		assert.NoError(t, err)
		assert.Len(t, zipkinSpans, td.SpanCount())
		tdFromZS, zErr := ToTranslator{}.ToTraces(zipkinSpans)
		assert.NoError(t, zErr, "%+v", zipkinSpans)
		assert.NotNil(t, tdFromZS)
		assert.Equal(t, td.SpanCount(), tdFromZS.SpanCount())

		// check that all timestamps converted back and forth without change
		for i := 0; i < td.ResourceSpans().Len(); i++ {
			instSpans := td.ResourceSpans().At(i).ScopeSpans()
			for j := 0; j < instSpans.Len(); j++ {
				spans := instSpans.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					span := spans.At(k)

					// search for the span with the same id to compare to
					spanFromZS := findSpanByID(tdFromZS.ResourceSpans(), span.SpanID())

					assert.Equal(t, span.StartTimestamp().AsTime().UnixNano(), spanFromZS.StartTimestamp().AsTime().UnixNano())
					assert.Equal(t, span.EndTimestamp().AsTime().UnixNano(), spanFromZS.EndTimestamp().AsTime().UnixNano())
				}
			}
		}
	}
}

func findSpanByID(rs ptrace.ResourceSpansSlice, spanID pcommon.SpanID) ptrace.Span {
	for i := 0; i < rs.Len(); i++ {
		instSpans := rs.At(i).ScopeSpans()
		for j := 0; j < instSpans.Len(); j++ {
			spans := instSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.SpanID() == spanID {
					return span
				}
			}
		}
	}
	return ptrace.Span{}
}

func generateTraceOneSpanOneTraceID(status ptrace.StatusCode) ptrace.Traces {
	td := testdata.GenerateTracesOneSpan()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	span.SetTraceID([16]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
	})
	span.SetSpanID([8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
	switch status {
	case ptrace.StatusCodeError:
		span.Status().SetCode(ptrace.StatusCodeError)
		span.Status().SetMessage("error message")
	case ptrace.StatusCodeOk:
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.Status().SetMessage("")
	default:
		span.Status().SetCode(ptrace.StatusCodeUnset)
		span.Status().SetMessage("")
	}
	return td
}

func zipkinOneSpan(status ptrace.StatusCode) *zipkinmodel.SpanModel {
	trueBool := true

	var spanErr error
	spanTags := map[string]string{
		"resource-attr": "resource-attr-val-1",
	}

	switch status {
	case ptrace.StatusCodeOk:
		spanTags[conventions.OtelStatusCode] = "STATUS_CODE_OK"
	case ptrace.StatusCodeError:
		spanTags[conventions.OtelStatusCode] = "STATUS_CODE_ERROR"
		spanTags[conventions.OtelStatusDescription] = "error message"
		spanTags[tracetranslator.TagError] = "true"
		spanErr = errors.New("error message")
	}

	return &zipkinmodel.SpanModel{
		SpanContext: zipkinmodel.SpanContext{
			TraceID:  zipkinmodel.TraceID{High: 72623859790382856, Low: 651345242494996240},
			ID:       72623859790382856,
			ParentID: nil,
			Debug:    false,
			Sampled:  &trueBool,
			Err:      spanErr,
		},
		LocalEndpoint: &zipkinmodel.Endpoint{
			ServiceName: "OTLPResourceNoServiceName",
		},
		RemoteEndpoint: nil,
		Annotations: []zipkinmodel.Annotation{
			{
				Timestamp: testdata.TestSpanEventTime,
				Value:     "event-with-attr|{\"span-event-attr\":\"span-event-attr-val\"}|2",
			},
			{
				Timestamp: testdata.TestSpanEventTime,
				Value:     "event|{}|2",
			},
		},
		Tags:      spanTags,
		Name:      "operationA",
		Timestamp: testdata.TestSpanStartTime,
		Duration:  1000000468,
		Shared:    false,
	}
}
