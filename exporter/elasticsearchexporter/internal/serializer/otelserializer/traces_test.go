// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestSerializeTraces(t *testing.T) {
	tests := []struct {
		name           string
		spanCustomizer func(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span)
		wantErr        bool
		expected       any
	}{
		{
			name: "default",
			spanCustomizer: func(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) {
				span.SetStartTimestamp(1721314113467654123)
				span.SetEndTimestamp(1721314113469654123)
				span.SetKind(ptrace.SpanKindServer)
				span.Status().SetCode(ptrace.StatusCodeOk)
				span.Status().SetMessage("Hello")
				resource.Attributes().PutInt("foo", 123)
				scope.Attributes().PutStr("foo", "bar")
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp": "1721314113467.654123",
				"duration":   json.Number("2000000"),
				"kind":       "Server",
				"links":      []any{},
				"resource": map[string]any{
					"attributes": map[string]any{
						"foo": json.Number("123"),
					},
				},
				"scope": map[string]any{
					"attributes": map[string]any{
						"foo": "bar",
					},
				},
				"status": map[string]any{
					"code":    "Ok",
					"message": "Hello",
				},
			},
		},
		{
			name: "unset status code",
			spanCustomizer: func(_ pcommon.Resource, _ pcommon.InstrumentationScope, span ptrace.Span) {
				span.Status().SetCode(ptrace.StatusCodeUnset)
			},
			wantErr: false,
			expected: map[string]any{
				"@timestamp": "0.0",
				"duration":   json.Number("0"),
				"kind":       "Unspecified",
				"links":      []any{},
				"resource":   map[string]any{},
				"scope":      map[string]any{},
				"status":     map[string]any{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traces := ptrace.NewTraces()
			resourceSpans := traces.ResourceSpans().AppendEmpty()
			scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
			span := scopeSpans.Spans().AppendEmpty()
			tt.spanCustomizer(resourceSpans.Resource(), scopeSpans.Scope(), span)
			traces.MarkReadOnly()

			var buf bytes.Buffer
			ser, err := New()
			require.NoError(t, err)
			err = ser.SerializeSpan(resourceSpans.Resource(), "", scopeSpans.Scope(), "", span, elasticsearch.Index{}, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("SerializeSpan() error = %v, wantErr %v", err, tt.wantErr)
			}
			spanBytes := buf.Bytes()
			eventAsJSON := string(spanBytes)
			var result any
			decoder := json.NewDecoder(bytes.NewBuffer(spanBytes))
			decoder.UseNumber()
			if err := decoder.Decode(&result); err != nil {
				t.Error(err)
			}

			assert.Equal(t, tt.expected, result, eventAsJSON)
		})
	}
}
