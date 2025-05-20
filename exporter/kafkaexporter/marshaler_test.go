// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
)

func TestGetLogsMarshaler(t *testing.T) {
	// Verify built-in marshalers.
	_ = mustGetLogsMarshaler(t, "otlp_proto", componenttest.NewNopHost())
	_ = mustGetLogsMarshaler(t, "otlp_json", componenttest.NewNopHost())
	_ = mustGetLogsMarshaler(t, "raw", componenttest.NewNopHost())

	// Verify extensions take precedence over built-in marshalers.
	m := mustGetLogsMarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): plogMarshalerFuncExtension(func(plog.Logs) ([]byte, error) {
			return []byte("overridden"), nil
		}),
	})
	messages, err := m.MarshalLogs(plog.NewLogs())
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "overridden", string(messages[0].Value))

	// Verify named extensions are supported.
	m = mustGetLogsMarshaler(t, "otlp_proto/alice", extensionsHost{
		component.MustNewIDWithName("otlp_proto", "alice"): plogMarshalerFuncExtension(func(plog.Logs) ([]byte, error) {
			return []byte("bob"), nil
		}),
	})
	messages, err = m.MarshalLogs(plog.NewLogs())
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "bob", string(messages[0].Value))

	// Specifying an extension for a different type should fail fast.
	m, err = getLogsMarshaler("otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): struct{ component.Component }{},
	})
	require.EqualError(t, err, `extension "otlp_proto" is not a logs marshaler`)
	assert.Nil(t, m)
}

func TestGetMetricsMarshaler(t *testing.T) {
	// Verify a built-in marshaler.
	_ = mustGetMetricsMarshaler(t, "otlp_proto", componenttest.NewNopHost())

	// Verify extensions take precedence over built-in marshalers.
	m := mustGetMetricsMarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): pmetricMarshalerFuncExtension(func(pmetric.Metrics) ([]byte, error) {
			return []byte("overridden"), nil
		}),
	})
	messages, err := m.MarshalMetrics(pmetric.NewMetrics())
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "overridden", string(messages[0].Value))

	// Specifying an extension for a different type should fail fast.
	m, err = getMetricsMarshaler("otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): struct{ component.Component }{},
	})
	require.EqualError(t, err, `extension "otlp_proto" is not a metrics marshaler`)
	assert.Nil(t, m)
}

func TestGetTracesMarshaler(t *testing.T) {
	// Verify a built-in marshaler.
	_ = mustGetTracesMarshaler(t, "otlp_proto", componenttest.NewNopHost())
	_ = mustGetTracesMarshaler(t, "jaeger_proto", componenttest.NewNopHost())
	_ = mustGetTracesMarshaler(t, "jaeger_json", componenttest.NewNopHost())
	_ = mustGetTracesMarshaler(t, "zipkin_proto", componenttest.NewNopHost())
	_ = mustGetTracesMarshaler(t, "zipkin_json", componenttest.NewNopHost())

	// Verify extensions take precedence over built-in marshalers.
	m := mustGetTracesMarshaler(t, "otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): ptraceMarshalerFuncExtension(func(ptrace.Traces) ([]byte, error) {
			return []byte("overridden"), nil
		}),
	})
	messages, err := m.MarshalTraces(ptrace.NewTraces())
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "overridden", string(messages[0].Value))

	// Specifying an extension for a different type should fail fast.
	m, err = getTracesMarshaler("otlp_proto", extensionsHost{
		component.MustNewID("otlp_proto"): struct{ component.Component }{},
	})
	require.EqualError(t, err, `extension "otlp_proto" is not a traces marshaler`)
	assert.Nil(t, m)
}

func mustGetLogsMarshaler(tb testing.TB, encoding string, host component.Host) marshaler.LogsMarshaler {
	tb.Helper()
	m, err := getLogsMarshaler(encoding, host)
	require.NoError(tb, err)
	return m
}

func mustGetMetricsMarshaler(tb testing.TB, encoding string, host component.Host) marshaler.MetricsMarshaler {
	tb.Helper()
	m, err := getMetricsMarshaler(encoding, host)
	require.NoError(tb, err)
	return m
}

func mustGetTracesMarshaler(tb testing.TB, encoding string, host component.Host) marshaler.TracesMarshaler {
	tb.Helper()
	m, err := getTracesMarshaler(encoding, host)
	require.NoError(tb, err)
	return m
}
