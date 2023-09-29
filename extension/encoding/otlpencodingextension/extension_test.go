// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func Test_Marshal_Unmarshal_Logs(t *testing.T) {
	factory := NewFactory()
	e, err := factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), createDefaultConfig())
	require.NoError(t, err)
	err = e.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("foo")
	buf, err := e.(plog.Marshaler).MarshalLogs(ld)
	require.NoError(t, err)
	require.True(t, len(buf) > 0)
	ld2, err := e.(plog.Unmarshaler).UnmarshalLogs(buf)
	require.NoError(t, err)
	require.Equal(t, 1, ld2.LogRecordCount())
	require.NoError(t, e.Shutdown(context.Background()))
}

func Test_Marshal_Unmarshal_Metrics(t *testing.T) {
	factory := NewFactory()
	e, err := factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), createDefaultConfig())
	require.NoError(t, err)
	err = e.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("foo")
	m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	buf, err := e.(pmetric.Marshaler).MarshalMetrics(md)
	require.NoError(t, err)
	require.True(t, len(buf) > 0)
	md2, err := e.(pmetric.Unmarshaler).UnmarshalMetrics(buf)
	require.NoError(t, err)
	require.Equal(t, 1, md2.MetricCount())
	require.NoError(t, e.Shutdown(context.Background()))
}

func Test_Marshal_Unmarshal_Traces(t *testing.T) {
	factory := NewFactory()
	e, err := factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), createDefaultConfig())
	require.NoError(t, err)
	err = e.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("foo")
	buf, err := e.(ptrace.Marshaler).MarshalTraces(td)
	require.NoError(t, err)
	require.True(t, len(buf) > 0)
	td2, err := e.(ptrace.Unmarshaler).UnmarshalTraces(buf)
	require.NoError(t, err)
	require.Equal(t, 1, td2.SpanCount())
	require.NoError(t, e.Shutdown(context.Background()))
}

func Test_Bad_Protocol(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		Protocol: "foo",
	}
	require.ErrorContains(t, cfg.Validate(), `invalid protocol "foo"`)
	e, err := factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)
	require.ErrorContains(t, e.Start(context.Background(), componenttest.NewNopHost()), `unsupported protocol: "foo"`)

}
