// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
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
	ils.Spans().AppendEmpty()
	ils.Spans().AppendEmpty()

	span := ils.Spans().At(0)
	span.SetKind(ptrace.SpanKindServer)
	span.SetName("foo")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Second)))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})
	span.SetParentSpanID([8]byte{8, 9, 10, 11, 12, 13, 14})

	span = ils.Spans().At(1)
	span.SetKind(ptrace.SpanKindClient)
	span.SetName("bar")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Second)))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{15, 16, 17, 18, 19, 20, 21})
	span.SetParentSpanID([8]byte{0, 1, 2, 3, 4, 5, 6, 7})

	span = ils.Spans().At(2)
	span.SetKind(ptrace.SpanKindServer)
	span.SetName("baz")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(now))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Second)))
	span.SetTraceID([16]byte{17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})
	span.SetSpanID([8]byte{22, 23, 24, 25, 26, 27, 28})
	span.SetParentSpanID([8]byte{29, 30, 31, 32, 33, 34, 35, 36})

	// Since marshaling json is not guaranteed to be in order
	// within a string, using a map to compare that the expected values are there
	unkeyedOtlpJSON := map[string]any{
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
							map[string]any{
								"traceId":           "0102030405060708090a0b0c0d0e0f10",
								"spanId":            "0f10111213141500",
								"parentSpanId":      "0001020304050607",
								"name":              "bar",
								"kind":              float64(ptrace.SpanKindClient),
								"startTimeUnixNano": fmt.Sprint(now.UnixNano()),
								"endTimeUnixNano":   fmt.Sprint(now.Add(time.Second).UnixNano()),
								"status":            map[string]any{},
							},
							map[string]any{
								"traceId":           "1112131415161718191a1b1c1d1e1f20",
								"spanId":            "161718191a1b1c00",
								"parentSpanId":      "1d1e1f2021222324",
								"name":              "baz",
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

	unkeyedOtlpJSONResult := make([]any, 1)
	unkeyedOtlpJSONResult[0] = unkeyedOtlpJSON

	keyedOtlpJSON1 := map[string]any{
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
							map[string]any{
								"traceId":           "0102030405060708090a0b0c0d0e0f10",
								"spanId":            "0f10111213141500",
								"parentSpanId":      "0001020304050607",
								"name":              "bar",
								"kind":              float64(ptrace.SpanKindClient),
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

	unkeyedMessageKey := []sarama.Encoder{nil}

	keyedOtlpJSON2 := map[string]any{
		"resourceSpans": []any{
			map[string]any{
				"resource": map[string]any{},
				"scopeSpans": []any{
					map[string]any{
						"scope": map[string]any{},
						"spans": []any{
							map[string]any{
								"traceId":           "1112131415161718191a1b1c1d1e1f20",
								"spanId":            "161718191a1b1c00",
								"parentSpanId":      "1d1e1f2021222324",
								"name":              "baz",
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

	keyedOtlpJSONResult := make([]any, 2)
	keyedOtlpJSONResult[0] = keyedOtlpJSON1
	keyedOtlpJSONResult[1] = keyedOtlpJSON2

	keyedMessageKey := []sarama.Encoder{sarama.ByteEncoder("0102030405060708090a0b0c0d0e0f10"), sarama.ByteEncoder("1112131415161718191a1b1c1d1e1f20")}

	unkeyedZipkinJSON := []any{
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
		map[string]any{
			"traceId":       "0102030405060708090a0b0c0d0e0f10",
			"id":            "0f10111213141500",
			"parentId":      "0001020304050607",
			"name":          "bar",
			"timestamp":     float64(time.Second.Microseconds()),
			"duration":      float64(time.Second.Microseconds()),
			"kind":          string(zipkin.Client),
			"localEndpoint": map[string]any{"serviceName": "otlpresourcenoservicename"},
		},
		map[string]any{
			"traceId":       "1112131415161718191a1b1c1d1e1f20",
			"id":            "161718191a1b1c00",
			"parentId":      "1d1e1f2021222324",
			"name":          "baz",
			"timestamp":     float64(time.Second.Microseconds()),
			"duration":      float64(time.Second.Microseconds()),
			"kind":          string(zipkin.Server),
			"localEndpoint": map[string]any{"serviceName": "otlpresourcenoservicename"},
		},
	}

	unkeyedZipkinJSONResult := make([]any, 1)
	unkeyedZipkinJSONResult[0] = unkeyedZipkinJSON

	keyedZipkinJSON1 := []any{
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
		map[string]any{
			"traceId":       "0102030405060708090a0b0c0d0e0f10",
			"id":            "0f10111213141500",
			"parentId":      "0001020304050607",
			"name":          "bar",
			"timestamp":     float64(time.Second.Microseconds()),
			"duration":      float64(time.Second.Microseconds()),
			"kind":          string(zipkin.Client),
			"localEndpoint": map[string]any{"serviceName": "otlpresourcenoservicename"},
		},
	}

	keyedZipkinJSON2 := []any{
		map[string]any{
			"traceId":       "1112131415161718191a1b1c1d1e1f20",
			"id":            "161718191a1b1c00",
			"parentId":      "1d1e1f2021222324",
			"name":          "baz",
			"timestamp":     float64(time.Second.Microseconds()),
			"duration":      float64(time.Second.Microseconds()),
			"kind":          string(zipkin.Server),
			"localEndpoint": map[string]any{"serviceName": "otlpresourcenoservicename"},
		},
	}

	keyedZipkinJSONResult := make([]any, 2)
	keyedZipkinJSONResult[0] = keyedZipkinJSON1
	keyedZipkinJSONResult[1] = keyedZipkinJSON2

	tests := []struct {
		encoding            string
		keyed               bool
		numExpectedMessages int
		expectedJSON        []any
		expectedMessageKey  []sarama.Encoder
		unmarshaled         any
	}{
		{encoding: "otlp_json", numExpectedMessages: 1, expectedJSON: unkeyedOtlpJSONResult, expectedMessageKey: unkeyedMessageKey, unmarshaled: map[string]any{}},
		{encoding: "otlp_json", keyed: true, numExpectedMessages: 2, expectedJSON: keyedOtlpJSONResult, expectedMessageKey: keyedMessageKey, unmarshaled: map[string]any{}},
		{encoding: "zipkin_json", numExpectedMessages: 1, expectedJSON: unkeyedZipkinJSONResult, expectedMessageKey: unkeyedMessageKey, unmarshaled: []map[string]any{}},
		{encoding: "zipkin_json", keyed: true, numExpectedMessages: 2, expectedJSON: keyedZipkinJSONResult, expectedMessageKey: keyedMessageKey, unmarshaled: []map[string]any{}},
	}

	for _, test := range tests {

		marshaler, ok := tracesMarshalers()[test.encoding]
		require.True(t, ok, fmt.Sprintf("Must have %s marshaller", test.encoding))

		if test.keyed {
			keyableMarshaler, ok := marshaler.(KeyableTracesMarshaler)
			require.True(t, ok, "Must be a KeyableTracesMarshaler")
			keyableMarshaler.Key()
		}

		msg, err := marshaler.Marshal(traces, t.Name())
		require.NoError(t, err, "Must have marshaled the data without error")
		require.Len(t, msg, test.numExpectedMessages, "Expected number of messages in the message")

		for idx, singleMsg := range msg {
			data, err := singleMsg.Value.Encode()
			require.NoError(t, err, "Must not error when encoding value")
			require.NotNil(t, data, "Must have valid data to test")

			unmarshaled := test.unmarshaled
			err = json.Unmarshal(data, &unmarshaled)
			require.NoError(t, err, "Must not error marshaling expected data")

			assert.Equal(t, test.expectedJSON[idx], unmarshaled, "Must match the expected value")
			assert.Equal(t, test.expectedMessageKey[idx], singleMsg.Key)
		}
	}
}
