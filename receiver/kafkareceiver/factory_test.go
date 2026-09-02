// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

type trackingMeterProvider struct {
	metric.MeterProvider
	dropped []string
}

func (p *trackingMeterProvider) DropInjectedAttributes(attrs ...string) metric.MeterProvider {
	p.dropped = append(p.dropped, attrs...)
	return p.MeterProvider
}

type trackingTracerProvider struct {
	trace.TracerProvider
	dropped     []string
	tracerCalls int
}

func (p *trackingTracerProvider) DropInjectedAttributes(attrs ...string) trace.TracerProvider {
	p.dropped = append(p.dropped, attrs...)
	return p.TracerProvider
}

func (p *trackingTracerProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	p.tracerCalls++
	return p.TracerProvider.Tracer(name, options...)
}

type trackingCore struct {
	zapcore.Core
	dropped []string
}

func (c *trackingCore) DropInjectedAttributes(attrs ...string) zapcore.Core {
	c.dropped = append(c.dropped, attrs...)
	return c.Core
}

func encodingFromReceiver(tb testing.TB, r any, section string) string {
	tb.Helper()

	if rc, ok := r.(*franzConsumer); ok {
		switch section {
		case "Traces":
			return rc.config.Traces.Encoding
		case "Metrics":
			return rc.config.Metrics.Encoding
		case "Logs":
			return rc.config.Logs.Encoding
		case "Profiles":
			return rc.config.Profiles.Encoding
		}
	}

	tb.Fatalf("unsupported receiver type %T or section %q", r, section)
	return ""
}

func signalHeaderConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.SignalHeader = true
	return cfg
}

func createAllSignals(t *testing.T, cfg *Config) (traces, metrics, logs, profiles any) {
	t.Helper()
	factory := NewFactory()
	settings := receivertest.NewNopSettings(metadata.Type)
	var err error
	traces, err = factory.CreateTraces(t.Context(), settings, cfg, new(consumertest.TracesSink))
	require.NoError(t, err)
	metrics, err = factory.CreateMetrics(t.Context(), settings, cfg, new(consumertest.MetricsSink))
	require.NoError(t, err)
	logs, err = factory.CreateLogs(t.Context(), settings, cfg, new(consumertest.LogsSink))
	require.NoError(t, err)
	profiles, err = factory.(xreceiver.Factory).CreateProfiles(t.Context(), settings, cfg, new(consumertest.ProfilesSink))
	require.NoError(t, err)
	return traces, metrics, logs, profiles
}

func unwrapMux(r any) *muxReceiver {
	return r.(*sharedcomponent.SharedComponent).Unwrap().(*muxReceiver)
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, configkafka.NewDefaultClientConfig(), cfg.ClientConfig)
	assert.Equal(t, configkafka.NewDefaultConsumerConfig(), cfg.ConsumerConfig)
}

func TestSignalHeaderReceiverIsShared(t *testing.T) {
	cfg := signalHeaderConfig()
	cfg.Traces.Topics = []string{"^shared$"}
	cfg.Metrics.Topics = []string{"^shared$"}
	cfg.Logs.Topics = []string{"^logs$"}
	cfg.Profiles.Topics = []string{"^shared$"}
	cfg.Traces.ExcludeTopics = []string{"^debug$"}
	cfg.Metrics.ExcludeTopics = []string{"^debug$"}
	cfg.Logs.ExcludeTopics = []string{"^debug$"}
	cfg.Profiles.ExcludeTopics = []string{"^debug$"}

	traces, metrics, logs, profiles := createAllSignals(t, cfg)
	assert.Same(t, traces, metrics)
	assert.Same(t, traces, logs)
	assert.Same(t, traces, profiles)
	mux := unwrapMux(traces)
	assert.Equal(t, []string{"^shared$", "^logs$"}, mux.topics)
	assert.Equal(t, []string{"^debug$"}, mux.excludeTopics)
}

func TestSignalHeaderReceiverRejectsDifferentExclusions(t *testing.T) {
	cfg := signalHeaderConfig()
	cfg.Traces.Topics = []string{"^otel-.*"}
	cfg.Traces.ExcludeTopics = []string{"^otel-metrics$"}
	cfg.Metrics.Topics = []string{"otel-metrics"}
	factory := NewFactory()
	settings := receivertest.NewNopSettings(metadata.Type)

	_, err := factory.CreateTraces(t.Context(), settings, cfg, new(consumertest.TracesSink))
	require.NoError(t, err)
	_, err = factory.CreateMetrics(t.Context(), settings, cfg, new(consumertest.MetricsSink))
	assert.ErrorContains(t, err, "signal_header requires identical exclude_topics")
}

func TestSignalHeaderReceiverPreservesLiteralTopicsWithRegex(t *testing.T) {
	cfg := signalHeaderConfig()
	cfg.Traces.Topics = []string{"^traces-.*"}
	cfg.Logs.Topics = []string{"otlp.logs"}
	factory := NewFactory()
	settings := receivertest.NewNopSettings(metadata.Type)

	traces, err := factory.CreateTraces(t.Context(), settings, cfg, new(consumertest.TracesSink))
	require.NoError(t, err)
	_, err = factory.CreateLogs(t.Context(), settings, cfg, new(consumertest.LogsSink))
	require.NoError(t, err)
	assert.Equal(t, []string{"^traces-.*", `^otlp\.logs$`}, unwrapMux(traces).topics)
}

func TestSignalHeaderReceiverDropsSignalFromSharedTelemetry(t *testing.T) {
	meter := &trackingMeterProvider{MeterProvider: metricnoop.NewMeterProvider()}
	tracer := &trackingTracerProvider{TracerProvider: tracenoop.NewTracerProvider()}
	core := &trackingCore{Core: zap.NewNop().Core()}
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.MeterProvider = meter
	settings.TracerProvider = tracer
	settings.Logger = zap.New(core)

	_, err := newMuxReceiver(signalHeaderConfig(), settings)
	require.NoError(t, err)
	want := []string{kafka.SignalHeaderKey}
	assert.Equal(t, want, meter.dropped)
	assert.Equal(t, want, tracer.dropped)
	assert.Equal(t, want, core.dropped)
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Brokers = []string{"localhost:9092"}
	cfg.ClientConfig.ProtocolVersion = "2.0.0"
	r, err := createTracesReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithTracesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Traces.Encoding = "custom"
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Traces"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultTracesEncoding, encodingFromReceiver(t, receiver, "Traces"))
	})
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Brokers = []string{"localhost:9092"}
	cfg.ClientConfig.ProtocolVersion = "2.0.0"
	r, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Metrics.Encoding = "custom"
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Metrics"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultMetricsEncoding, encodingFromReceiver(t, receiver, "Metrics"))
	})
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Brokers = []string{"localhost:9092"}
	cfg.ClientConfig.ProtocolVersion = "2.0.0"
	r, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithLogsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Logs.Encoding = "custom"
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Logs"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultLogsEncoding, encodingFromReceiver(t, receiver, "Logs"))
	})
}

func TestCreateProfiles(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Brokers = []string{"localhost:9092"}
	cfg.ClientConfig.ProtocolVersion = "2.0.0"
	r, err := createProfilesReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithProfilesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Profiles.Encoding = "custom"
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Profiles"))
	})

	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultProfilesEncoding, encodingFromReceiver(t, receiver, "Profiles"))
	})
}
