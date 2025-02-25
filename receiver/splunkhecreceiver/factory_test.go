// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
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
	lReceiver, err := createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockLogsConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, lReceiver, "receiver creation failed")

	mockMetricsConsumer := consumertest.NewNop()
	mReceiver, err := createMetricsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockMetricsConsumer)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, mReceiver, "receiver creation failed")
}

func TestFactoryType(t *testing.T) {
	assert.Equal(t, metadata.Type, NewFactory().Type())
}

func TestMultipleLogsReceivers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"
	mockLogsConsumer := consumertest.NewNop()
	mReceiver, _ := createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockLogsConsumer)
	mReceiver2, _ := createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockLogsConsumer)
	assert.Equal(t, mReceiver, mReceiver2)
}

func TestMultipleMetricsReceivers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"
	mockMetricsConsumer := consumertest.NewNop()
	mReceiver, _ := createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockMetricsConsumer)
	mReceiver2, _ := createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockMetricsConsumer)
	assert.Equal(t, mReceiver, mReceiver2)
}

func TestReuseLogsAndMetricsReceivers(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:1"
	mockMetricsConsumer := consumertest.NewNop()
	mReceiver, _ := createLogsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockMetricsConsumer)
	mReceiver2, _ := createMetricsReceiver(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, mockMetricsConsumer)
	assert.Equal(t, mReceiver, mReceiver2)
	assert.NotNil(t, mReceiver.(*sharedcomponent.SharedComponent).Component.(*splunkReceiver).metricsConsumer)
	assert.NotNil(t, mReceiver.(*sharedcomponent.SharedComponent).Component.(*splunkReceiver).logsConsumer)
}
