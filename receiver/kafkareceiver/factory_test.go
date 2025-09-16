// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, configkafka.NewDefaultClientConfig(), cfg.ClientConfig)
	assert.Equal(t, configkafka.NewDefaultConsumerConfig(), cfg.ConsumerConfig)
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createTracesReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(t.Context(), componenttest.NewNopHost()))
}

func TestWithTracesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		cfg.Traces.Encoding = "custom"
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		tracesConsumer, ok := receiver.(*kafkaTracesConsumer)
		require.True(t, ok)
		require.Equal(t, "custom", tracesConsumer.config.Traces.Encoding)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		tracesConsumer, ok := receiver.(*kafkaTracesConsumer)
		require.True(t, ok)
		require.Equal(t, defaultTracesEncoding, tracesConsumer.config.Traces.Encoding)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(t.Context(), componenttest.NewNopHost()))
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		cfg.Metrics.Encoding = "custom"
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		metricsConsumer, ok := receiver.(*kafkaMetricsConsumer)
		require.True(t, ok)
		require.Equal(t, "custom", metricsConsumer.config.Metrics.Encoding)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		metricsConsumer, ok := receiver.(*kafkaMetricsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultMetricsEncoding, metricsConsumer.config.Metrics.Encoding)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(t.Context(), componenttest.NewNopHost()))
}

func TestWithLogsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		cfg.Logs.Encoding = "custom"
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		logsConsumer, ok := receiver.(*kafkaLogsConsumer)
		require.True(t, ok)
		require.Equal(t, "custom", logsConsumer.config.Logs.Encoding)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig()
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		logsConsumer, ok := receiver.(*kafkaLogsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultLogsEncoding, logsConsumer.config.Logs.Encoding)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}
