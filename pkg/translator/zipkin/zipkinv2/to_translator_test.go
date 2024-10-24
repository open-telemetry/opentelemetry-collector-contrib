// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinv2

import (
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	zipkinmodel "github.com/openzipkin/zipkin-go/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/internal/zipkin"
)

func TestZipkinSpansToInternalTraces(t *testing.T) {
	tests := []struct {
		name string
		zs   []*zipkinmodel.SpanModel
		td   ptrace.Traces
		err  error
	}{
		{
			name: "empty",
			td:   ptrace.NewTraces(),
			err:  nil,
		},
		{
			name: "minimalSpan",
			zs:   generateSpanNoEndpoints(),
			td:   generateTraceSingleSpanNoResourceOrInstrLibrary(),
			err:  nil,
		},
		{
			name: "onlyLocalEndpointSpan",
			zs:   generateSpanNoTags(),
			td:   generateTraceSingleSpanMinmalResource(),
			err:  nil,
		},
		{
			name: "errorTag",
			zs:   generateSpanErrorTags(),
			td:   generateTraceSingleSpanErrorStatus(),
			err:  nil,
		},
		{
			name: "span ok status",
			zs: []*zipkinmodel.SpanModel{
				{
					SpanContext: zipkinmodel.SpanContext{
						TraceID: convertTraceID(
							[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
						ID: convertSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}),
					},
					Name: "min.data",
					Tags: map[string]string{
						"otel.status_code": "Ok",
					},
					Timestamp: time.Unix(1596911098, 294000000),
					Duration:  1000000,
					Shared:    false,
				},
			},
			td: func() ptrace.Traces {
				td := ptrace.NewTraces()
				span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetTraceID(
					[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
				span.SetSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})
				span.SetName("min.data")
				span.Status().SetCode(ptrace.StatusCodeOk)
				span.SetStartTimestamp(1596911098294000000)
				span.SetEndTimestamp(1596911098295000000)
				return td
			}(),
		},
		{
			name: "span unset status",
			zs: []*zipkinmodel.SpanModel{
				{
					SpanContext: zipkinmodel.SpanContext{
						TraceID: convertTraceID(
							[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
						ID: convertSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}),
					},
					Name: "min.data",
					Tags: map[string]string{
						"otel.status_code": "Unset",
					},
					Timestamp: time.Unix(1596911098, 294000000),
					Duration:  1000000,
					Shared:    false,
				},
			},
			td: func() ptrace.Traces {
				td := ptrace.NewTraces()
				span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetTraceID(
					[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
				span.SetSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})
				span.SetName("min.data")
				span.Status().SetCode(ptrace.StatusCodeUnset)
				span.SetStartTimestamp(1596911098294000000)
				span.SetEndTimestamp(1596911098295000000)
				return td
			}(),
		},
		{
			name: "span error status",
			zs: []*zipkinmodel.SpanModel{
				{
					SpanContext: zipkinmodel.SpanContext{
						TraceID: convertTraceID(
							[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
						ID: convertSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}),
					},
					Name: "min.data",
					Tags: map[string]string{
						"otel.status_code": "Error",
					},
					Timestamp: time.Unix(1596911098, 294000000),
					Duration:  1000000,
					Shared:    false,
				},
			},
			td: func() ptrace.Traces {
				td := ptrace.NewTraces()
				span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetTraceID(
					[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
				span.SetSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})
				span.SetName("min.data")
				span.Status().SetCode(ptrace.StatusCodeError)
				span.SetStartTimestamp(1596911098294000000)
				span.SetEndTimestamp(1596911098295000000)
				return td
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			td, err := ToTranslator{}.ToTraces(test.zs)
			assert.EqualValues(t, test.err, err)
			if test.name != "nilSpan" {
				assert.Equal(t, len(test.zs), td.SpanCount())
			}
			assert.EqualValues(t, test.td, td)
		})
	}
}

func generateSpanNoEndpoints() []*zipkinmodel.SpanModel {
	spans := make([]*zipkinmodel.SpanModel, 1)
	spans[0] = &zipkinmodel.SpanModel{
		SpanContext: zipkinmodel.SpanContext{
			TraceID: convertTraceID(
				[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
			ID: convertSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}),
		},
		Name:           "MinimalData",
		Kind:           zipkinmodel.Client,
		Timestamp:      time.Unix(1596911098, 294000000),
		Duration:       1000000,
		Shared:         false,
		LocalEndpoint:  nil,
		RemoteEndpoint: nil,
		Annotations:    nil,
		Tags:           nil,
	}
	return spans
}

func generateSpanNoTags() []*zipkinmodel.SpanModel {
	spans := generateSpanNoEndpoints()
	spans[0].LocalEndpoint = &zipkinmodel.Endpoint{ServiceName: "SoleAttr"}
	return spans
}

func generateSpanErrorTags() []*zipkinmodel.SpanModel {
	errorTags := make(map[string]string)
	errorTags["error"] = "true"

	spans := generateSpanNoEndpoints()
	spans[0].Tags = errorTags
	return spans
}

func generateTraceSingleSpanNoResourceOrInstrLibrary() ptrace.Traces {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(
		[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
	span.SetSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})
	span.SetName("MinimalData")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(1596911098294000000)
	span.SetEndTimestamp(1596911098295000000)
	return td
}

func generateTraceSingleSpanMinmalResource() ptrace.Traces {
	td := generateTraceSingleSpanNoResourceOrInstrLibrary()
	rs := td.ResourceSpans().At(0)
	rsc := rs.Resource()
	rsc.Attributes().PutStr(conventions.AttributeServiceName, "SoleAttr")
	return td
}

func generateTraceSingleSpanErrorStatus() ptrace.Traces {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(
		[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80})
	span.SetSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8})
	span.SetName("MinimalData")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(1596911098294000000)
	span.SetEndTimestamp(1596911098295000000)
	span.Status().SetCode(ptrace.StatusCodeError)
	return td
}

func TestV2SpanWithoutTimestampGetsTag(t *testing.T) {
	duration := int64(2948533333)
	spans := make([]*zipkinmodel.SpanModel, 1)
	spans[0] = &zipkinmodel.SpanModel{
		SpanContext: zipkinmodel.SpanContext{
			TraceID: convertTraceID(
				[16]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8, 0xF9, 0xFA, 0xFB, 0xFC, 0xFD, 0xFE, 0xFF, 0x80}),
			ID: convertSpanID([8]byte{0xAF, 0xAE, 0xAD, 0xAC, 0xAB, 0xAA, 0xA9, 0xA8}),
		},
		Name:           "NoTimestamps",
		Kind:           zipkinmodel.Client,
		Duration:       time.Duration(duration),
		Shared:         false,
		LocalEndpoint:  nil,
		RemoteEndpoint: nil,
		Annotations:    nil,
		Tags:           nil,
	}

	gb, err := ToTranslator{}.ToTraces(spans)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	gs := gb.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.NotNil(t, gs.StartTimestamp)
	assert.NotNil(t, gs.EndTimestamp)

	// expect starttime to be set to zero (unix time)
	unixTime := gs.StartTimestamp().AsTime().Unix()
	assert.Equal(t, int64(0), unixTime)

	// expect end time to be zero (unix time) plus the duration
	assert.Equal(t, duration, gs.EndTimestamp().AsTime().UnixNano())

	wasAbsent, mapContainedKey := gs.Attributes().Get(zipkin.StartTimeAbsent)
	assert.True(t, mapContainedKey)
	assert.True(t, wasAbsent.Bool())
}

func TestTagsToSpanLinks(t *testing.T) {
	tests := []struct {
		name          string
		tags          map[string]string
		expectedLinks []ptrace.SpanLink
		expectedError bool
	}{
		{
			name: "Single valid link",
			tags: map[string]string{
				"otlp.link.0": "0102030405060708090a0b0c0d0e0f10|0102030405060708|state|{\"attr1\":\"value1\",\"attr2\":123}|0",
			},
			expectedLinks: []ptrace.SpanLink{
				func() ptrace.SpanLink {
					link := ptrace.NewSpanLink()
					traceID, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f10")
					link.SetTraceID(pcommon.TraceID(traceID))
					spanID, _ := hex.DecodeString("0102030405060708")
					link.SetSpanID(pcommon.SpanID(spanID))
					link.TraceState().FromRaw("state")
					attrs := link.Attributes()
					attrs.PutStr("attr1", "value1")
					attrs.PutInt("attr2", 123)
					link.SetDroppedAttributesCount(0)
					return link
				}(),
			},
			expectedError: false,
		},
		{
			name: "Multiple valid links",
			tags: map[string]string{
				"otlp.link.0": "0102030405060708090a0b0c0d0e0f10|0102030405060708|state1|{\"attr1\":\"value1\"}|0",
				"otlp.link.1": "1112131415161718191a1b1c1d1e1f20|1112131415161718|state2|{\"attr2\":456}|1",
			},
			expectedLinks: []ptrace.SpanLink{
				func() ptrace.SpanLink {
					link := ptrace.NewSpanLink()
					traceID, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f10")
					link.SetTraceID(pcommon.TraceID(traceID))
					spanID, _ := hex.DecodeString("0102030405060708")
					link.SetSpanID(pcommon.SpanID(spanID))
					link.TraceState().FromRaw("state1")
					attrs := link.Attributes()
					attrs.PutStr("attr1", "value1")
					link.SetDroppedAttributesCount(0)
					return link
				}(),
				func() ptrace.SpanLink {
					link := ptrace.NewSpanLink()
					traceID, _ := hex.DecodeString("1112131415161718191a1b1c1d1e1f20")
					link.SetTraceID(pcommon.TraceID(traceID))
					spanID, _ := hex.DecodeString("1112131415161718")
					link.SetSpanID(pcommon.SpanID(spanID))
					link.TraceState().FromRaw("state2")
					attrs := link.Attributes()
					attrs.PutInt("attr2", 456)
					link.SetDroppedAttributesCount(1)
					return link
				}(),
			},
			expectedError: false,
		},
		{
			name: "Invalid format (too few parts)",
			tags: map[string]string{
				"otlp.link.0": "invalid|value",
			},
			expectedLinks: []ptrace.SpanLink{},
			expectedError: false,
		},
		{
			name: "Invalid hex in trace ID",
			tags: map[string]string{
				"otlp.link.0": "nothex|0102030405060708|state|{\"attr1\":\"value1\"}|0",
			},
			expectedLinks: nil,
			expectedError: true,
		},
		{
			name: "Invalid dropped attributes count",
			tags: map[string]string{
				"otlp.link.0": "0102030405060708090a0b0c0d0e0f10|0102030405060708|state|{\"attr1\":\"value1\"}|notanumber",
			},
			expectedLinks: nil,
			expectedError: true,
		},
		{
			name: "Attributes with pipe character",
			tags: map[string]string{
				"otlp.link.0": "0102030405060708090a0b0c0d0e0f10|0102030405060708|state|{\"attr1\":\"value1|with|pipes\"}|0",
			},
			expectedLinks: []ptrace.SpanLink{
				func() ptrace.SpanLink {
					link := ptrace.NewSpanLink()
					traceID, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f10")
					link.SetTraceID(pcommon.TraceID(traceID))
					spanID, _ := hex.DecodeString("0102030405060708")
					link.SetSpanID(pcommon.SpanID(spanID))
					link.TraceState().FromRaw("state")
					attrs := link.Attributes()
					attrs.PutStr("attr1", "value1|with|pipes")
					link.SetDroppedAttributesCount(0)
					return link
				}(),
			},
			expectedError: false,
		},
		{
			name:          "No links in tags",
			tags:          map[string]string{},
			expectedLinks: []ptrace.SpanLink{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := ptrace.NewSpanLinkSlice()
			err := TagsToSpanLinks(tt.tags, dest)
			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, len(tt.expectedLinks), dest.Len())

			for i, expected := range tt.expectedLinks {
				link := dest.At(i)

				assert.Equal(t, expected.TraceID(), link.TraceID(), "TraceID mismatch")
				assert.Equal(t, expected.SpanID(), link.SpanID(), "SpanID mismatch")
				assert.Equal(t, expected.TraceState().AsRaw(), link.TraceState().AsRaw())

				assert.Equal(t, expected.DroppedAttributesCount(), link.DroppedAttributesCount())

				// Verify attributes
				expectedAttrs := expected.Attributes()
				actualAttrs := link.Attributes()
				assert.Equal(t, expectedAttrs.Len(), actualAttrs.Len(), "Attributes length mismatch")

				expectedAttrs.Range(func(k string, v pcommon.Value) bool {
					actualVal, ok := actualAttrs.Get(k)
					assert.True(t, ok, "Attribute %s not found in link", k)
					assert.Equal(t, v, actualVal, "Attribute %s value mismatch", k)
					return true
				})
			}

			// Verify that link tags are removed
			for i := 0; i < 128; i++ {
				key := "otlp.link." + strconv.Itoa(i)
				_, ok := tt.tags[key]
				assert.False(t, ok, "Expected tag %s to be deleted", key)
			}
		})
	}
}
