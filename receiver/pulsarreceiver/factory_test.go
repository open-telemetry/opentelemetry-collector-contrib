// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		Topic:          "",
		Encoding:       defaultEncoding,
		ConsumerName:   defaultConsumerName,
		Subscription:   defaultSubscription,
		Endpoint:       defaultServiceURL,
		Authentication: Authentication{},
	}, cfg)
}

// trace
func TestCreateTracesReceiver_err_addr(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "invalid:6650"

	f := pulsarReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}
	r, err := f.createTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateTracesReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL

	f := pulsarReceiverFactory{tracesUnmarshalers: make(map[string]TracesUnmarshaler)}
	r, err := f.createTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateTraceReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	f := pulsarReceiverFactory{tracesUnmarshalers: defaultTracesUnmarshalers()}
	recv, err := f.createTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}

func TestWithTracesUnmarshalers(t *testing.T) {
	unmarshaler := &customTracesUnmarshaler{}
	f := NewFactory(withTracesUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

// metrics
func TestCreateMetricsReceiver_err_addr(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "invalid:6650"

	f := pulsarReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}
	r, err := f.createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL

	f := pulsarReceiverFactory{metricsUnmarshalers: make(map[string]MetricsUnmarshaler)}
	r, err := f.createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateMetricsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	f := pulsarReceiverFactory{metricsUnmarshalers: defaultMetricsUnmarshalers()}

	recv, err := f.createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	unmarshaler := &customMetricsUnmarshaler{}
	f := NewFactory(withMetricsUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		receiver, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		receiver, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, receiver)
	})
}

// logs
func TestCreateLogsReceiver_err_addr(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "invalid:6650"

	f := pulsarReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	r, err := f.createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateLogsReceiver_err_marshallers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL

	f := pulsarReceiverFactory{logsUnmarshalers: make(map[string]LogsUnmarshaler)}
	r, err := f.createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.Error(t, err)
	assert.Nil(t, r)
}

func Test_CreateLogsReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = defaultServiceURL

	f := pulsarReceiverFactory{logsUnmarshalers: defaultLogsUnmarshalers()}
	recv, err := f.createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	require.NoError(t, err)
	assert.NotNil(t, recv)
}

func TestWithLogsUnmarshalers(t *testing.T) {
	unmarshaler := &customLogsUnmarshaler{}
	f := NewFactory(withLogsUnmarshalers(unmarshaler))
	cfg := createDefaultConfig().(*Config)

	t.Run("custom_encoding", func(t *testing.T) {
		cfg.Encoding = unmarshaler.Encoding()
		exporter, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, exporter)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg.Encoding = defaultEncoding
		exporter, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, exporter)
	})
}

type customTracesUnmarshaler struct {
}

type customMetricsUnmarshaler struct {
}

type customLogsUnmarshaler struct {
}

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
