// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1" // Endpoint is required, not going to be used here.

	mockLogsConsumer := consumertest.NewNop()
	lReceiver, err := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockLogsConsumer)
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	mockMetricsConsumer := consumertest.NewNop()
	mReceiver, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockMetricsConsumer)
	assert.Nil(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")
}

func TestFactoryType(t *testing.T) {
	assert.Equal(t, component.Type("splunk_hec"), NewFactory().Type())
}

func TestCreateNilNextConsumerMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"

	mReceiver, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	assert.EqualError(t, err, "nil metricsConsumer")
	assert.Nil(t, mReceiver, "receiver creation failed")
}

func TestCreateNilNextConsumerLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"

	mReceiver, err := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	assert.EqualError(t, err, "nil logsConsumer")
	assert.Nil(t, mReceiver, "receiver creation failed")
}

func TestMultipleLogsReceivers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"
	mockLogsConsumer := consumertest.NewNop()
	mReceiver, _ := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockLogsConsumer)
	mReceiver2, _ := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockLogsConsumer)
	assert.Equal(t, mReceiver, mReceiver2)
}

func TestMultipleMetricsReceivers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"
	mockMetricsConsumer := consumertest.NewNop()
	mReceiver, _ := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockMetricsConsumer)
	mReceiver2, _ := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockMetricsConsumer)
	assert.Equal(t, mReceiver, mReceiver2)
}

func TestReuseLogsAndMetricsReceivers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"
	mockMetricsConsumer := consumertest.NewNop()
	mReceiver, _ := createLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockMetricsConsumer)
	mReceiver2, _ := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, mockMetricsConsumer)
	assert.Equal(t, mReceiver, mReceiver2)
	assert.NotNil(t, mReceiver.(*sharedcomponent.SharedComponent).Component.(*splunkReceiver).metricsConsumer)
	assert.NotNil(t, mReceiver.(*sharedcomponent.SharedComponent).Component.(*splunkReceiver).logsConsumer)
}
