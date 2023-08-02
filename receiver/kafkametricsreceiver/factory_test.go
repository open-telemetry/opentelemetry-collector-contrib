// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "default config not created")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsReceiver_errors(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	cfg.Scrapers = []string{"topics"}
	r, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	assert.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsReceiver(t *testing.T) {
	prev := newMetricsReceiver
	newMetricsReceiver = func(ctx context.Context, config Config, params receiver.CreateSettings, consumer consumer.Metrics) (receiver.Metrics, error) {
		return nil, nil
	}
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	cfg.Scrapers = []string{"topics"}
	_, err := createMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, nil)
	newMetricsReceiver = prev
	assert.Nil(t, err)
}
