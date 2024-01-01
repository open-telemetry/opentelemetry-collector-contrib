// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	zipkin "github.com/openzipkin/zipkin-go/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func TestDefaultTracesMarshalers(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
		"zipkin_proto",
		"zipkin_json",
		"jaeger_proto",
		"jaeger_json",
	}
	marshalers := tracesMarshalers()
	assert.Equal(t, len(expectedEncodings), len(marshalers))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestDefaultMetricsMarshalers(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
	}
	marshalers := metricsMarshalers()
	assert.Equal(t, len(expectedEncodings), len(marshalers))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestDefaultLogsMarshalers(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
		"raw",
	}
	marshalers := logsMarshalers()
	assert.Equal(t, len(expectedEncodings), len(marshalers))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestOTLPTracesJsonMarshaling(t *testing.T) {
	t.Parallel()

	now := time.Unix(1, 0)

	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty()

	rs := traces.ResourceSpans().At(0)
	rs.SetSchemaUrl(conventions.SchemaURL)
	rs.ScopeSpans().AppendEmpty()

	ils := rs.ScopeSpans().At(0)
	ils.SetSchemaUrl(conventions.SchemaURL)
	ils.Spans().AppendEmpty()

	span := ils.Spans().At(0)
	span.SetKind(ptrace.SpanKindServer)
	span.SetName("foo")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Second)))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
	span.SetParentSpanID([8]byte{8, 9, 10, 11, 12, 13, 14})

	// Since marshaling json is not guaranteed to be in order
	// within a string, using a map to compare that the expected values are there
	otlpJSON := map[string]any{
		"resourceSpans": []any{
			map[string]any{
				"resource": map[string]any{},
				"scopeSpans": []any{
					map[string]any{
						"scope": map[string]any{},
						"spans": []any{
							map[string]any{
								"traceId":           "0102030405060708090a0b0c0d0e0f10",
								"spanId":            "0001020304050607",
								"parentSpanId":      "08090a0b0c0d0e00",
								"name":              "foo",
								"kind":              float64(ptrace.SpanKindServer),
								"startTimeUnixNano": fmt.Sprint(now.UnixNano()),
								"endTimeUnixNano":   fmt.Sprint(now.Add(time.Second).UnixNano()),
								"status":            map[string]any{},
							},
						},
						"schemaUrl": conventions.SchemaURL,
					},
				},
				"schemaUrl": conventions.SchemaURL,
			},
		},
	}

	zipkinJSON := []any{
		map[string]any{
			"traceId":       "0102030405060708090a0b0c0d0e0f10",
			"id":            "0001020304050607",
			"parentId":      "08090a0b0c0d0e00",
			"name":          "foo",
			"timestamp":     float64(time.Second.Microseconds()),
			"duration":      float64(time.Second.Microseconds()),
			"kind":          string(zipkin.Server),
			"localEndpoint": map[string]any{"serviceName": "otlpresourcenoservicename"},
		},
	}

	tests := []struct {
		encoding     string
		expectedJSON any
		unmarshaled  any
	}{
		{encoding: "otlp_json", expectedJSON: otlpJSON, unmarshaled: map[string]any{}},
		{encoding: "zipkin_json", expectedJSON: zipkinJSON, unmarshaled: []map[string]any{}},
	}

	for _, test := range tests {

		marshaler, ok := tracesMarshalers()[test.encoding]
		require.True(t, ok, fmt.Sprintf("Must have %s marshaller", test.encoding))

		msg, err := marshaler.Marshal(traces, t.Name())
		require.NoError(t, err, "Must have marshaled the data without error")
		require.Len(t, msg, 1, "Must have one entry in the message")

		data, err := msg[0].Value.Encode()
		require.NoError(t, err, "Must not error when encoding value")
		require.NotNil(t, data, "Must have valid data to test")

		unmarshaled := test.unmarshaled
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err, "Must not error marshaling expected data")

		assert.Equal(t, test.expectedJSON, unmarshaled, "Must match the expected value")

	}
}
