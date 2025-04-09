// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, configkafka.NewDefaultClientConfig(), cfg.ClientConfig)
	assert.Equal(t, configkafka.NewDefaultConsumerConfig(), cfg.ConsumerConfig)
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createTracesReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
}

func TestWithTracesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Encoding = "custom"
		receiver, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		tracesConsumer, ok := receiver.(*kafkaTracesConsumer)
		require.True(t, ok)
		require.Equal(t, defaultTracesTopic, tracesConsumer.config.Topic)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		receiver, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		tracesConsumer, ok := receiver.(*kafkaTracesConsumer)
		require.True(t, ok)
		require.Equal(t, defaultTracesTopic, tracesConsumer.config.Topic)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createMetricsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Encoding = "custom"
		receiver, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		metricsConsumer, ok := receiver.(*kafkaMetricsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultMetricsTopic, metricsConsumer.config.Topic)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		receiver, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		metricsConsumer, ok := receiver.(*kafkaMetricsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultMetricsTopic, metricsConsumer.config.Topic)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
}

func TestWithLogsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.Encoding = "custom"
		receiver, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		logsConsumer, ok := receiver.(*kafkaLogsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultLogsTopic, logsConsumer.config.Topic)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		receiver, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		logsConsumer, ok := receiver.(*kafkaLogsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultLogsTopic, logsConsumer.config.Topic)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}
