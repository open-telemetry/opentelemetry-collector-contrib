// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/natstest"
)

func testSettings() exporter.Settings {
	return exportertest.NewNopSettings(metadata.Type)
}

func generateTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("test-span")
	return td
}

func generateMetrics() pmetric.Metrics {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test-metric")
	m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	return md
}

func generateLogs() plog.Logs {
	ld := plog.NewLogs()
	lr := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr.Body().SetStr("test-log")
	return ld
}

// subscribeSync connects a bare client to the server and returns a synchronous
// subscription on the subject, flushed so the interest is registered before the
// exporter publishes.
func subscribeSync(t *testing.T, url, subject string) *nats.Subscription {
	t.Helper()
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	sub, err := nc.SubscribeSync(subject)
	require.NoError(t, err)
	require.NoError(t, nc.Flush())
	return sub
}

func TestExportTraces(t *testing.T) {
	tests := []struct {
		encoding  string
		marshaler ptrace.Marshaler
	}{
		{"otlp_proto", &ptrace.ProtoMarshaler{}},
		{"otlp_json", &ptrace.JSONMarshaler{}},
	}
	for _, tt := range tests {
		t.Run(tt.encoding, func(t *testing.T) {
			url := natstest.NewServer(t)
			cfg := createDefaultConfig().(*Config)
			cfg.URLs = []string{url}
			cfg.Traces.Encoding = tt.encoding

			sub := subscribeSync(t, url, cfg.Traces.Subject)

			exp := newTracesExporter(*cfg, testSettings())
			require.NoError(t, exp.start(t.Context(), componenttest.NewNopHost()))
			t.Cleanup(func() { _ = exp.close(t.Context()) })

			td := generateTraces()
			require.NoError(t, exp.export(t.Context(), td))

			msg, err := sub.NextMsg(5 * time.Second)
			require.NoError(t, err)
			assert.Equal(t, cfg.Traces.Subject, msg.Subject)

			expected, err := tt.marshaler.MarshalTraces(td)
			require.NoError(t, err)
			assert.Equal(t, expected, msg.Data)
		})
	}
}

func TestExportMetrics(t *testing.T) {
	url := natstest.NewServer(t)
	cfg := createDefaultConfig().(*Config)
	cfg.URLs = []string{url}

	sub := subscribeSync(t, url, cfg.Metrics.Subject)

	exp := newMetricsExporter(*cfg, testSettings())
	require.NoError(t, exp.start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = exp.close(t.Context()) })

	md := generateMetrics()
	require.NoError(t, exp.export(t.Context(), md))

	msg, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)

	expected, err := (&pmetric.ProtoMarshaler{}).MarshalMetrics(md)
	require.NoError(t, err)
	assert.Equal(t, expected, msg.Data)
}

func TestExportLogs(t *testing.T) {
	url := natstest.NewServer(t)
	cfg := createDefaultConfig().(*Config)
	cfg.URLs = []string{url}

	sub := subscribeSync(t, url, cfg.Logs.Subject)

	exp := newLogsExporter(*cfg, testSettings())
	require.NoError(t, exp.start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { _ = exp.close(t.Context()) })

	ld := generateLogs()
	require.NoError(t, exp.export(t.Context(), ld))

	msg, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)

	expected, err := (&plog.ProtoMarshaler{}).MarshalLogs(ld)
	require.NoError(t, err)
	assert.Equal(t, expected, msg.Data)
}

func TestExportUnknownEncoding(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.URLs = []string{"nats://localhost:4222"}
	cfg.Traces.Encoding = "unknownencoding"

	exp := newTracesExporter(*cfg, testSettings())
	err := exp.start(t.Context(), componenttest.NewNopHost())
	require.ErrorContains(t, err, "unrecognized traces encoding")
}

func TestExportTracesJetStream(t *testing.T) {
	url := natstest.NewJetStreamServer(t)
	ctx := t.Context()

	// Pre-create the stream bound to the traces subject; the exporter does not
	// create streams itself.
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	require.NoError(t, err)
	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "OTEL",
		Subjects: []string{defaultTracesSubject},
	})
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.URLs = []string{url}
	cfg.JetStream = configoptional.Some(JetStreamConfig{PublishTimeout: 5 * time.Second})

	exp := newTracesExporter(*cfg, testSettings())
	require.NoError(t, exp.start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { _ = exp.close(ctx) })

	require.NoError(t, exp.export(ctx, generateTraces()))

	stream, err := js.Stream(ctx, "OTEL")
	require.NoError(t, err)
	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, info.State.Msgs)
}
