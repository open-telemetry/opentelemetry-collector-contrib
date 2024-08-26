// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestOTLPJSONTracesUnmarshalerEncoding(t *testing.T) {
	unmarshaler := newOTLPJSONTracesUnmarshaler()
	assert.Equal(t, "otlp_json", unmarshaler.Encoding())
}

func TestOTLPJSONTracesUnmarshal(t *testing.T) {
	unmarshaler := newOTLPJSONTracesUnmarshaler()

	tests := []struct {
		name     string
		json     string
		wantErr  bool
		validate func(t *testing.T, traces ptrace.Traces)
	}{
		{
			name: "valid_single_span",
			json: `{
				"resourceSpans": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": {"stringValue": "test_service"}
						}]
					},
					"scopeSpans": [{
						"scope": {
							"name": "test_scope",
							"version": "1.0.0"
						},
						"spans": [{
							"traceId": "5B8EFFF798038103D269B633813FC60C",
							"spanId": "EEE19B7EC3C1B173",
							"parentSpanId": "FFFFFFFFFFFFFFFF",
							"name": "test_span",
							"kind": 1,
							"startTimeUnixNano": "1544712660300000000",
							"endTimeUnixNano": "1544712661300000000",
							"attributes": [{
								"key": "attr1",
								"value": {"stringValue": "val1"}
							}]
						}]
					}]
				}]
			}`,
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				assert.Equal(t, 1, traces.ResourceSpans().Len())
				rs := traces.ResourceSpans().At(0)
				v, ok := rs.Resource().Attributes().Get("service.name")
				assert.True(t, ok)
				assert.Equal(t, "test_service", v.Str())
				assert.Equal(t, 1, rs.ScopeSpans().Len())
				ss := rs.ScopeSpans().At(0)
				assert.Equal(t, "test_scope", ss.Scope().Name())
				assert.Equal(t, "1.0.0", ss.Scope().Version())
				assert.Equal(t, 1, ss.Spans().Len())
				span := ss.Spans().At(0)
				assert.Equal(t, "test_span", span.Name())
				assert.Equal(t, pcommon.TraceID([16]byte{0x5B, 0x8E, 0xFF, 0xF7, 0x98, 0x03, 0x81, 0x03, 0xD2, 0x69, 0xB6, 0x33, 0x81, 0x3F, 0xC6, 0x0C}), span.TraceID())
				assert.Equal(t, pcommon.SpanID([8]byte{0xEE, 0xE1, 0x9B, 0x7E, 0xC3, 0xC1, 0xB1, 0x73}), span.SpanID())
				v, ok = span.Attributes().Get("attr1")
				assert.True(t, ok)
				assert.Equal(t, "val1", v.Str())
			},
		},
		{
			name:    "invalid_json",
			json:    `{invalid_json`,
			wantErr: true,
		},
		{
			name:    "empty_json",
			json:    `{}`,
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				assert.Equal(t, 0, traces.ResourceSpans().Len())
			},
		},
		{
			name:    "missing_resource_spans",
			json:    `{"someOtherField": []}`,
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				assert.Equal(t, 0, traces.ResourceSpans().Len())
			},
		},
		{
			name:    "empty_resource_spans",
			json:    `{"resourceSpans": []}`,
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				assert.Equal(t, 0, traces.ResourceSpans().Len())
			},
		},
		{
			name: "multiple_resource_spans",
			json: `{
				"resourceSpans": [
					{
						"resource": {
							"attributes": [{
								"key": "service.name",
								"value": {"stringValue": "service1"}
							}]
						},
						"scopeSpans": [{
							"scope": {"name": "scope1"},
							"spans": [{"name": "span1"}]
						}]
					},
					{
						"resource": {
							"attributes": [{
								"key": "service.name",
								"value": {"stringValue": "service2"}
							}]
						},
						"scopeSpans": [{
							"scope": {"name": "scope2"},
							"spans": [{"name": "span2"}]
						}]
					}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, traces ptrace.Traces) {
				assert.Equal(t, 2, traces.ResourceSpans().Len())
				v, ok := traces.ResourceSpans().At(0).Resource().Attributes().Get("service.name")
				assert.True(t, ok)
				assert.Equal(t, "service1", v.Str())
				v, ok = traces.ResourceSpans().At(1).Resource().Attributes().Get("service.name")
				assert.True(t, ok)
				assert.Equal(t, "service2", v.Str())
				assert.Equal(t, "span1", traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
				assert.Equal(t, "span2", traces.ResourceSpans().At(1).ScopeSpans().At(0).Spans().At(0).Name())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces, err := unmarshaler.Unmarshal([]byte(tt.json))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			tt.validate(t, traces)
		})
	}
}

func TestParseAttributes(t *testing.T) {
	attrs := []interface{}{
		map[string]interface{}{
			"key":   "stringAttr",
			"value": map[string]interface{}{"stringValue": "test"},
		},
		map[string]interface{}{
			"key":   "intAttr",
			"value": map[string]interface{}{"intValue": float64(123)}, // JSON numbers are float64
		},
		map[string]interface{}{
			"key":   "doubleAttr",
			"value": map[string]interface{}{"doubleValue": 123.456},
		},
		map[string]interface{}{
			"key":   "boolAttr",
			"value": map[string]interface{}{"boolValue": true},
		},
	}

	dest := pcommon.NewMap()
	parseAttributes(attrs, dest)

	v, ok := dest.Get("stringAttr")
	assert.True(t, ok)
	assert.Equal(t, "test", v.Str())

	v, ok = dest.Get("intAttr")
	assert.True(t, ok)
	assert.Equal(t, int64(123), v.Int())

	v, ok = dest.Get("doubleAttr")
	assert.True(t, ok)
	assert.Equal(t, 123.456, v.Double())

	v, ok = dest.Get("boolAttr")
	assert.True(t, ok)
	assert.True(t, v.Bool())
}

func TestParseSpan(t *testing.T) {
	spanMap := map[string]interface{}{
		"name":               "test_span",
		"traceId":            "5B8EFFF798038103D269B633813FC60C",
		"spanId":             "EEE19B7EC3C1B173",
		"parentSpanId":       "FFFFFFFFFFFFFFFF",
		"kind":               2,
		"startTimeUnixNano":  "1544712660300000000",
		"endTimeUnixNano":    "1544712661300000000",
		"status":             map[string]interface{}{"code": 1, "message": "OK"},
		"attributes": []interface{}{
			map[string]interface{}{
				"key":   "attr1",
				"value": map[string]interface{}{"stringValue": "val1"},
			},
		},
	}

	span := ptrace.NewSpan()
	parseSpan(spanMap, span)

	assert.Equal(t, "test_span", span.Name())
	assert.Equal(t, pcommon.TraceID([16]byte{0x5B, 0x8E, 0xFF, 0xF7, 0x98, 0x03, 0x81, 0x03, 0xD2, 0x69, 0xB6, 0x33, 0x81, 0x3F, 0xC6, 0x0C}), span.TraceID())
	assert.Equal(t, pcommon.SpanID([8]byte{0xEE, 0xE1, 0x9B, 0x7E, 0xC3, 0xC1, 0xB1, 0x73}), span.SpanID())
	assert.Equal(t, pcommon.SpanID([8]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}), span.ParentSpanID())
	assert.Equal(t, ptrace.SpanKindServer, span.Kind())
	assert.Equal(t, uint64(1544712660300000000), span.StartTimestamp())
	assert.Equal(t, uint64(1544712661300000000), span.EndTimestamp())
	assert.Equal(t, ptrace.StatusCodeOk, span.Status().Code())
	assert.Equal(t, "OK", span.Status().Message())
	v, ok := span.Attributes().Get("attr1")
	assert.True(t, ok)
	assert.Equal(t, "val1", v.Str())
}