// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, []string{defaultBroker}, cfg.Brokers)
	assert.Equal(t, defaultGroupID, cfg.GroupID)
	assert.Equal(t, defaultClientID, cfg.ClientID)
	assert.Equal(t, defaultInitialOffset, cfg.InitialOffset)
	assert.Equal(t, defaultSessionTimeout, cfg.SessionTimeout)
	assert.Equal(t, defaultHeartbeatInterval, cfg.HeartbeatInterval)
}

func TestCreateTracesReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	f := kafkaReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}
	r, err := f.createTracesReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
}

func TestWithTracesUnmarshalers(t *testing.T) {
	unmarshaler := &customTracesUnmarshaler{}
	f := NewFactory(withTracesUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker
	cfg.Metadata.Full = false
	cfg.ProtocolVersion = "2.0.0"

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
		tracesConsumer, ok := receiver.(*kafkaTracesConsumer)
		require.True(t, ok)
		require.Equal(t, defaultTracesTopic, tracesConsumer.config.Topic)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
		tracesConsumer, ok := receiver.(*kafkaTracesConsumer)
		require.True(t, ok)
		require.Equal(t, defaultTracesTopic, tracesConsumer.config.Topic)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

func TestCreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	f := kafkaReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}
	r, err := f.createMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	unmarshaler := &customMetricsUnmarshaler{}
	f := NewFactory(withMetricsUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker
	cfg.Metadata.Full = false
	cfg.ProtocolVersion = "2.0.0"

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
		metricsConsumer, ok := receiver.(*kafkaMetricsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultMetricsTopic, metricsConsumer.config.Topic)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
		metricsConsumer, ok := receiver.(*kafkaMetricsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultMetricsTopic, metricsConsumer.config.Topic)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

func TestCreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	f := kafkaReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers("Test Version", zap.NewNop())}
	r, err := f.createLogsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
	require.NoError(t, err)
	// no available broker
	require.Error(t, r.Start(context.Background(), componenttest.NewNopHost()))
}

func TestGetLogsUnmarshaler_encoding_text_error(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
	}{
		{
			name:     "text encoding has typo",
			encoding: "text_uft-8",
		},
		{
			name:     "text encoding is a random string",
			encoding: "text_vnbqgoba156",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := getLogsUnmarshaler(test.encoding, defaultLogsUnmarshalers("Test Version", zap.NewNop()))
			assert.ErrorContains(t, err, fmt.Sprintf("unsupported encoding '%v'", test.encoding[5:]))
		})
	}
}

func TestWithLogsUnmarshalers(t *testing.T) {
	unmarshaler := &customLogsUnmarshaler{}
	f := NewFactory(withLogsUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)
	// disable contacting broker
	cfg.Metadata.Full = false
	cfg.ProtocolVersion = "2.0.0"

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
		logsConsumer, ok := receiver.(*kafkaLogsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultLogsTopic, logsConsumer.config.Topic)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, nil)
		logsConsumer, ok := receiver.(*kafkaLogsConsumer)
		require.True(t, ok)
		require.Equal(t, defaultLogsTopic, logsConsumer.config.Topic)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

type customTracesUnmarshaler struct{}

type customMetricsUnmarshaler struct{}

type customLogsUnmarshaler struct{}

var _ TracesUnmarshaler = (*customTracesUnmarshaler)(nil)

func (c customTracesUnmarshaler) Unmarshal([]byte) (ptrace.Traces, error) {
	panic("implement me")
}

func (c customTracesUnmarshaler) Encoding() string {
	return "custom"
}

func (c customMetricsUnmarshaler) Unmarshal([]byte) (pmetric.Metrics, error) {
	panic("implement me")
}

func (c customMetricsUnmarshaler) Encoding() string {
	return "custom"
}

func (c customLogsUnmarshaler) Unmarshal([]byte) (plog.Logs, error) {
	panic("implement me")
}

func (c customLogsUnmarshaler) Encoding() string {
	return "custom"
}
