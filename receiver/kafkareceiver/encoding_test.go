// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger-idl/model/v1"
	"github.com/jaegertracing/jaeger-idl/thrift-gen/zipkincore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	zipkinthriftconverter "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinthriftconverter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

var (
	customLogsUnmarshalerExtension struct {
		component.Component
		plog.Unmarshaler
	}
	customMetricsUnmarshalerExtension struct {
		component.Component
		pmetric.Unmarshaler
	}
	customTracesUnmarshalerExtension struct {
		component.Component
		ptrace.Unmarshaler
	}
)

func TestNewLogsUnmarshaler(t *testing.T) {
	logs := plog.NewLogs()
	logRecord := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.Body().SetStr("hello world")

	otlpProtoLogs, err := (&plog.ProtoMarshaler{}).MarshalLogs(logs)
	require.NoError(t, err)
	otlpJSONLogs, err := (&plog.JSONMarshaler{}).MarshalLogs(logs)
	require.NoError(t, err)

	for _, tc := range []struct {
		encoding string
		input    []byte
		check    func(*testing.T, plog.Logs)
	}{
		{
			encoding: "otlp_proto",
			input:    otlpProtoLogs,
			check: func(t *testing.T, actual plog.Logs) {
				assert.NoError(t, plogtest.CompareLogs(logs, actual))
			},
		},
		{
			encoding: "otlp_json",
			input:    otlpJSONLogs,
			check: func(t *testing.T, actual plog.Logs) {
				assert.NoError(t, plogtest.CompareLogs(logs, actual))
			},
		},
		{
			encoding: "raw",
			input:    []byte("hello world"),
			check: func(t *testing.T, actual plog.Logs) {
				require.Equal(t, 1, actual.LogRecordCount())
				assert.Equal(t,
					[]byte("hello world"),
					actual.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsRaw(),
				)
			},
		},
		{
			encoding: "json",
			input:    []byte(`{"x": "y"}`),
			check: func(t *testing.T, actual plog.Logs) {
				require.Equal(t, 1, actual.LogRecordCount())
				assert.Equal(t,
					map[string]any{"x": "y"},
					actual.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw(),
				)
			},
		},
		{
			encoding: "azure_resource_logs",
			input: []byte(`{"records":[{
				"time": "2022-11-11T04:48:27.6767145Z",
				"resourceId": "/RESOURCE_ID",
				"operationName": "hello world"
			}]}`),
			check: func(t *testing.T, actual plog.Logs) {
				require.Equal(t, 1, actual.LogRecordCount())

				opName, _ := actual.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Attributes().Get(
					"azure.operation.name",
				)
				assert.Equal(t, "hello world", opName.Str())
			},
		},
		{
			encoding: "text",
			input:    []byte("hello world"),
			check: func(t *testing.T, actual plog.Logs) {
				assert.NoError(t, plogtest.CompareLogs(logs, actual, plogtest.IgnoreObservedTimestamp()))
			},
		},
		{
			encoding: "text_utf16",
			input: func() []byte {
				out, _ := unicode.UTF16(
					unicode.LittleEndian,
					unicode.IgnoreBOM,
				).NewEncoder().Bytes([]byte("hello world"))
				return out
			}(),
			check: func(t *testing.T, actual plog.Logs) {
				assert.NoError(t, plogtest.CompareLogs(logs, actual, plogtest.IgnoreObservedTimestamp()))
			},
		},
		{
			encoding: "text_utf-16", // hyphen in the name
			input: func() []byte {
				out, _ := unicode.UTF16(
					unicode.LittleEndian,
					unicode.IgnoreBOM,
				).NewEncoder().Bytes([]byte("hello world"))
				return out
			}(),
			check: func(t *testing.T, actual plog.Logs) {
				assert.NoError(t, plogtest.CompareLogs(logs, actual, plogtest.IgnoreObservedTimestamp()))
			},
		},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			u := mustNewLogsUnmarshaler(t, tc.encoding, componenttest.NewNopHost())
			out, err := u.UnmarshalLogs(tc.input)
			require.NoError(t, err)
			tc.check(t, out)
		})
	}
}

func TestNewLogsUnmarshalerExtension(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)

	// Verify extensions take precedence over built-in unmarshalers.
	u := mustNewLogsUnmarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): &customLogsUnmarshalerExtension,
	})
	assert.Equal(t, &customLogsUnmarshalerExtension, u)

	// Specifying an extension for a different type should fail fast.
	u, err := newLogsUnmarshaler("not_logs", settings, extensionsHost{
		component.MustNewID("not_logs"): &customTracesUnmarshalerExtension,
	})
	require.EqualError(t, err, `extension "not_logs" is not a logs unmarshaler`)
	assert.Nil(t, u)
}

func TestNewLogsUnmarshalerTextEncoding(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)
	u, err := newLogsUnmarshaler("text_invalid", settings, componenttest.NewNopHost())
	require.EqualError(t, err, `invalid text encoding: unsupported encoding 'invalid'`)
	assert.Nil(t, u)
}

func TestNewMetricsUnmarshaler(t *testing.T) {
	metrics := pmetric.NewMetrics()
	metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("a_gauge")
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(123)

	otlpProtoMetrics, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(metrics)
	require.NoError(t, err)
	otlpJSONMetrics, err := (&pmetric.JSONMarshaler{}).MarshalMetrics(metrics)
	require.NoError(t, err)

	for _, tc := range []struct {
		encoding string
		input    []byte
		check    func(*testing.T, pmetric.Metrics)
	}{
		{
			encoding: "otlp_proto",
			input:    otlpProtoMetrics,
			check: func(t *testing.T, actual pmetric.Metrics) {
				assert.NoError(t, pmetrictest.CompareMetrics(metrics, actual))
			},
		},
		{
			encoding: "otlp_json",
			input:    otlpJSONMetrics,
			check: func(t *testing.T, actual pmetric.Metrics) {
				assert.NoError(t, pmetrictest.CompareMetrics(metrics, actual))
			},
		},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			u := mustNewMetricsUnmarshaler(t, tc.encoding, componenttest.NewNopHost())
			out, err := u.UnmarshalMetrics(tc.input)
			require.NoError(t, err)
			tc.check(t, out)
		})
	}
}

func TestNewMetricsUnmarshalerExtension(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)

	// Verify extensions take precedence over built-in unmarshalers.
	u := mustNewMetricsUnmarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): &customMetricsUnmarshalerExtension,
	})
	assert.Equal(t, &customMetricsUnmarshalerExtension, u)

	// Specifying an extension for a different type should fail fast.
	u, err := newMetricsUnmarshaler("not_metrics", settings, extensionsHost{
		component.MustNewID("not_metrics"): &customLogsUnmarshalerExtension,
	})
	require.EqualError(t, err, `extension "not_metrics" is not a metrics unmarshaler`)
	assert.Nil(t, u)
}

func TestNewTracesUnmarshaler(t *testing.T) {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	resourceSpans.Resource().Attributes().PutStr("service.name", "test-service")
	span := resourceSpans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("otel.zipkin.absentField.startTime", "*") // required for zipkin_json
	span.SetName("do_thing")
	span.SetTraceID(pcommon.TraceID{1})
	span.SetSpanID(pcommon.SpanID{1})

	otlpProtoTraces, err := (&ptrace.ProtoMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)
	otlpJSONTraces, err := (&ptrace.JSONMarshaler{}).MarshalTraces(traces)
	require.NoError(t, err)

	jaegerzipkinSpans := []*zipkincore.Span{zipkincore.NewSpan()}
	jaegerzipkinSpans[0].Name = "do_thing"

	jaegerSpan := &jaegerproto.Span{}
	jaegerSpan.OperationName = "do_thing"

	for _, tc := range []struct {
		encoding string
		input    []byte
		check    func(*testing.T, ptrace.Traces)
	}{
		{
			encoding: "otlp_proto",
			input:    otlpProtoTraces,
			check: func(t *testing.T, actual ptrace.Traces) {
				assert.NoError(t, ptracetest.CompareTraces(traces, actual))
			},
		},
		{
			encoding: "otlp_json",
			input:    otlpJSONTraces,
			check: func(t *testing.T, actual ptrace.Traces) {
				assert.NoError(t, ptracetest.CompareTraces(traces, actual))
			},
		},
		{
			encoding: "jaeger_proto",
			input: func() []byte {
				encoded, _ := jaegerSpan.Marshal()
				return encoded
			}(),
			check: func(t *testing.T, actual ptrace.Traces) {
				require.Equal(t, 1, actual.SpanCount())
				assert.Equal(t,
					"do_thing",
					actual.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name(),
				)
			},
		},
		{
			encoding: "jaeger_json",
			input: func() []byte {
				encoded, _ := (&jsonpb.Marshaler{}).MarshalToString(jaegerSpan)
				return []byte(encoded)
			}(),
			check: func(t *testing.T, actual ptrace.Traces) {
				require.Equal(t, 1, actual.SpanCount())
				assert.Equal(t,
					"do_thing",
					actual.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name(),
				)
			},
		},
		{
			encoding: "zipkin_proto",
			input: func() []byte {
				encoded, err := zipkinv2.NewProtobufTracesMarshaler().MarshalTraces(traces)
				require.NoError(t, err)
				return encoded
			}(),
			check: func(t *testing.T, actual ptrace.Traces) {
				assert.NoError(t, ptracetest.CompareTraces(traces, actual))
			},
		},
		{
			encoding: "zipkin_json",
			input: func() []byte {
				encoded, err := zipkinv2.NewJSONTracesMarshaler().MarshalTraces(traces)
				require.NoError(t, err)
				return encoded
			}(),
			check: func(t *testing.T, actual ptrace.Traces) {
				assert.NoError(t, ptracetest.CompareTraces(
					traces, actual,
					ptracetest.IgnoreSpanAttributeValue("otel.zipkin.absentField.startTime"),
				))
			},
		},
		{
			encoding: "zipkin_thrift",
			input: func() []byte {
				encoded, err := zipkinthriftconverter.SerializeThrift(context.Background(), jaegerzipkinSpans)
				require.NoError(t, err)
				return encoded
			}(),
			check: func(t *testing.T, actual ptrace.Traces) {
				require.Equal(t, 1, actual.SpanCount())
				assert.Equal(t,
					"do_thing",
					actual.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name(),
				)
			},
		},
	} {
		t.Run(tc.encoding, func(t *testing.T) {
			u := mustNewTracesUnmarshaler(t, tc.encoding, componenttest.NewNopHost())
			out, err := u.UnmarshalTraces(tc.input)
			require.NoError(t, err)
			tc.check(t, out)
		})
	}
}

func TestNewTracesUnmarshalerExtension(t *testing.T) {
	settings := receivertest.NewNopSettings(metadata.Type)

	// Verify extensions take precedence over built-in unmarshalers.
	u := mustNewTracesUnmarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): &customTracesUnmarshalerExtension,
	})
	assert.Equal(t, &customTracesUnmarshalerExtension, u)

	// Specifying an extension for a different type should fail fast.
	u, err := newTracesUnmarshaler("not_traces", settings, extensionsHost{
		component.MustNewID("not_traces"): &customLogsUnmarshalerExtension,
	})
	require.EqualError(t, err, `extension "not_traces" is not a traces unmarshaler`)
	assert.Nil(t, u)
}

func mustNewLogsUnmarshaler(tb testing.TB, encoding string, host component.Host) plog.Unmarshaler {
	settings := receivertest.NewNopSettings(metadata.Type)
	u, err := newLogsUnmarshaler(encoding, settings, host)
	require.NoError(tb, err)
	return u
}

func mustNewMetricsUnmarshaler(tb testing.TB, encoding string, host component.Host) pmetric.Unmarshaler {
	settings := receivertest.NewNopSettings(metadata.Type)
	u, err := newMetricsUnmarshaler(encoding, settings, host)
	require.NoError(tb, err)
	return u
}

func mustNewTracesUnmarshaler(tb testing.TB, encoding string, host component.Host) ptrace.Unmarshaler {
	settings := receivertest.NewNopSettings(metadata.Type)
	u, err := newTracesUnmarshaler(encoding, settings, host)
	require.NoError(tb, err)
	return u
}

type extensionsHost map[component.ID]component.Component

func (h extensionsHost) GetExtensions() map[component.ID]component.Component {
	return h
}
