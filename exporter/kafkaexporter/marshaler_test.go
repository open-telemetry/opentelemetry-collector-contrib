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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, err := createTracesMarshaler(Config{
				Encoding: e,
			})
			require.NoError(t, err)
			assert.NotNil(t, m)
		})
	}
}

func TestDefaultMetricsMarshalers(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
	}
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, err := createMetricMarshaler(Config{
				Encoding: e,
			})
			require.NoError(t, err)
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
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, err := createLogMarshaler(Config{
				Encoding: e,
			})
			require.NoError(t, err)
			assert.NotNil(t, m)
		})
	}
}

func TestOTLPMetricsJsonMarshaling(t *testing.T) {
	tests := []struct {
		name                 string
		partitionByResources bool
		messagePartitionKeys []sarama.Encoder
	}{
		{
			name:                 "partitioning_disabled",
			partitionByResources: false,
			messagePartitionKeys: []sarama.Encoder{nil},
		},
		{
			name:                 "partitioning_enabled",
			partitionByResources: true,
			messagePartitionKeys: []sarama.Encoder{
				sarama.ByteEncoder{0x62, 0x7f, 0x20, 0x34, 0x85, 0x49, 0x55, 0x2e, 0xfa, 0x93, 0xae, 0xd7, 0xde, 0x91, 0xd7, 0x16},
				sarama.ByteEncoder{0x75, 0x6b, 0xb4, 0xd6, 0xff, 0xeb, 0x92, 0x22, 0xa, 0x68, 0x65, 0x48, 0xe0, 0xd3, 0x94, 0x44},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetrics()
			r := pcommon.NewResource()
			r.Attributes().PutStr("service.name", "my_service_name")
			r.Attributes().PutStr("service.instance.id", "kek_x_1")
			r.CopyTo(metric.ResourceMetrics().AppendEmpty().Resource())

			rm := metric.ResourceMetrics().At(0)
			rm.SetSchemaUrl(conventions.SchemaURL)

			sm := rm.ScopeMetrics().AppendEmpty()
			pmetric.NewScopeMetrics()
			m := sm.Metrics().AppendEmpty()
			m.SetEmptyGauge()
			m.Gauge().DataPoints().AppendEmpty().SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(1, 0)))
			m.Gauge().DataPoints().At(0).Attributes().PutStr("gauage_attribute", "attr")
			m.Gauge().DataPoints().At(0).SetDoubleValue(1.0)

			r1 := pcommon.NewResource()
			r1.Attributes().PutStr("service.instance.id", "kek_x_2")
			r1.Attributes().PutStr("service.name", "my_service_name")
			r1.CopyTo(metric.ResourceMetrics().AppendEmpty().Resource())

			marshaler, err := createMetricMarshaler(
				Config{
					Encoding:                             "otlp_json",
					PartitionMetricsByResourceAttributes: tt.partitionByResources,
				})
			require.NoError(t, err)

			msgs, err := marshaler.Marshal(metric, "KafkaTopicX")
			require.NoError(t, err, "Must have marshaled the data without error")
			require.Len(t, msgs, len(tt.messagePartitionKeys), "Number of messages must be %d, but was %d", len(tt.messagePartitionKeys), len(msgs))

			for i := 0; i < len(tt.messagePartitionKeys); i++ {
				require.Equal(t, tt.messagePartitionKeys[i], msgs[i].Key, "message %d has incorrect key", i)
			}
		})
	}
}

func TestOTLPLogsJsonMarshaling(t *testing.T) {
	tests := []struct {
		name                 string
		partitionByResources bool
		messagePartitionKeys []sarama.Encoder
	}{
		{
			name:                 "partitioning_disabled",
			partitionByResources: false,
			messagePartitionKeys: []sarama.Encoder{nil},
		},
		{
			name:                 "partitioning_enabled",
			partitionByResources: true,
			messagePartitionKeys: []sarama.Encoder{
				sarama.ByteEncoder{0x62, 0x7f, 0x20, 0x34, 0x85, 0x49, 0x55, 0x2e, 0xfa, 0x93, 0xae, 0xd7, 0xde, 0x91, 0xd7, 0x16},
				sarama.ByteEncoder{0x75, 0x6b, 0xb4, 0xd6, 0xff, 0xeb, 0x92, 0x22, 0xa, 0x68, 0x65, 0x48, 0xe0, 0xd3, 0x94, 0x44},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := plog.NewLogs()
			r := pcommon.NewResource()
			r.Attributes().PutStr("service.name", "my_service_name")
			r.Attributes().PutStr("service.instance.id", "kek_x_1")
			r.CopyTo(log.ResourceLogs().AppendEmpty().Resource())

			rm := log.ResourceLogs().At(0)
			rm.SetSchemaUrl(conventions.SchemaURL)

			sl := rm.ScopeLogs().AppendEmpty()
			plog.NewScopeLogs()
			l := sl.LogRecords().AppendEmpty()
			l.SetSeverityText("INFO")
			l.SetTimestamp(pcommon.Timestamp(1))
			l.Body().SetStr("Simple log message")

			r1 := pcommon.NewResource()
			r1.Attributes().PutStr("service.instance.id", "kek_x_2")
			r1.Attributes().PutStr("service.name", "my_service_name")
			r1.CopyTo(log.ResourceLogs().AppendEmpty().Resource())

			marshaler, err := createLogMarshaler(
				Config{
					Encoding:                          "otlp_json",
					PartitionLogsByResourceAttributes: tt.partitionByResources,
				})
			require.NoError(t, err)

			msgs, err := marshaler.Marshal(log, "KafkaTopicX")
			require.NoError(t, err, "Must have marshaled the data without error")
			require.Len(t, msgs, len(tt.messagePartitionKeys), "Number of messages must be %d, but was %d", len(tt.messagePartitionKeys), len(msgs))

			for i := 0; i < len(tt.messagePartitionKeys); i++ {
				require.Equal(t, tt.messagePartitionKeys[i], msgs[i].Key, "message %d has incorrect key", i)
			}
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

	unkeyedMessageKey := []sarama.ByteEncoder{nil}

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

	keyedMessageKey := []sarama.ByteEncoder{sarama.ByteEncoder("0102030405060708090a0b0c0d0e0f10"), sarama.ByteEncoder("1112131415161718191a1b1c1d1e1f20")}

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
		partitionTracesByID bool
		numExpectedMessages int
		expectedJSON        []any
		expectedMessageKey  []sarama.ByteEncoder
		unmarshaled         any
	}{
		{encoding: "otlp_json", numExpectedMessages: 1, expectedJSON: unkeyedOtlpJSONResult, expectedMessageKey: unkeyedMessageKey, unmarshaled: map[string]any{}},
		{encoding: "otlp_json", partitionTracesByID: true, numExpectedMessages: 2, expectedJSON: keyedOtlpJSONResult, expectedMessageKey: keyedMessageKey, unmarshaled: map[string]any{}},
		{encoding: "zipkin_json", numExpectedMessages: 1, expectedJSON: unkeyedZipkinJSONResult, expectedMessageKey: unkeyedMessageKey, unmarshaled: []map[string]any{}},
		{encoding: "zipkin_json", partitionTracesByID: true, numExpectedMessages: 2, expectedJSON: keyedZipkinJSONResult, expectedMessageKey: keyedMessageKey, unmarshaled: []map[string]any{}},
	}

	for _, test := range tests {
		marshaler, err := createTracesMarshaler(Config{
			Encoding:            test.encoding,
			PartitionTracesByID: test.partitionTracesByID,
			Producer:            Producer{MaxMessageBytes: defaultProducerMaxMessageBytes},
		})
		require.NoErrorf(t, err, "Must have %s marshaler", test.encoding)

		msg, err := marshaler.Marshal(traces, t.Name())
		var messages []*sarama.ProducerMessage
		if len(msg) > 0 {
			messages = msg[0].Messages
		}
		require.NoError(t, err, "Must have marshaled the data without error")
		require.Len(t, messages, test.numExpectedMessages, "Expected number of messages in the message")

		for idx, singleMsg := range messages {
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

func TestTracesEncodingMarshaler(t *testing.T) {
	m := &tracesEncodingMarshaler{
		marshaler: &nopComponent{},
		encoding:  "trace_encoding",
	}
	assert.Equal(t, "trace_encoding", m.Encoding())
	data, err := m.Marshal(ptrace.NewTraces(), "topic")
	assert.NoError(t, err)
	assert.Len(t, data, 1)
}

type encodingTracesErrorMarshaler struct {
	err error
}

func (m *encodingTracesErrorMarshaler) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return nil, m.err
}

func TestTracesEncodingMarshaler_error(t *testing.T) {
	expErr := fmt.Errorf("failed to marshal")
	m := &tracesEncodingMarshaler{
		marshaler: &encodingTracesErrorMarshaler{err: expErr},
		encoding:  "trace_encoding",
	}
	data, err := m.Marshal(ptrace.NewTraces(), "topic")
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestMetricsEncodingMarshaler(t *testing.T) {
	m := &metricsEncodingMarshaler{
		marshaler: &nopComponent{},
		encoding:  "metric_encoding",
	}
	assert.Equal(t, "metric_encoding", m.Encoding())
	data, err := m.Marshal(pmetric.NewMetrics(), "topic")
	assert.NoError(t, err)
	assert.Len(t, data, 1)
}

type encodingMetricsErrorMarshaler struct {
	err error
}

func (m *encodingMetricsErrorMarshaler) MarshalMetrics(_ pmetric.Metrics) ([]byte, error) {
	return nil, m.err
}

func TestMetricsEncodingMarshaler_error(t *testing.T) {
	expErr := fmt.Errorf("failed to marshal")
	m := &metricsEncodingMarshaler{
		marshaler: &encodingMetricsErrorMarshaler{err: expErr},
		encoding:  "metrics_encoding",
	}
	data, err := m.Marshal(pmetric.NewMetrics(), "topic")
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestLogsEncodingMarshaler(t *testing.T) {
	m := &logsEncodingMarshaler{
		marshaler: &nopComponent{},
		encoding:  "log_encoding",
	}
	assert.Equal(t, "log_encoding", m.Encoding())
	data, err := m.Marshal(plog.NewLogs(), "topic")
	assert.NoError(t, err)
	assert.Len(t, data, 1)
}

type encodingLogsErrorMarshaler struct {
	err error
}

func (m *encodingLogsErrorMarshaler) MarshalLogs(_ plog.Logs) ([]byte, error) {
	return nil, m.err
}

func TestLogsEncodingMarshaler_error(t *testing.T) {
	expErr := fmt.Errorf("failed to marshal")
	m := &logsEncodingMarshaler{
		marshaler: &encodingLogsErrorMarshaler{err: expErr},
		encoding:  "logs_encoding",
	}
	data, err := m.Marshal(plog.NewLogs(), "topic")
	assert.Error(t, err)
	assert.Nil(t, data)
}
