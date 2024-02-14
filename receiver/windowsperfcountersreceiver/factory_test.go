// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsperfcountersreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

var creationParams = receivertest.NewNopCreateSettings()

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	cfg.(*Config).PerfCounters = []ObjectConfig{
		{
			Object:   "object",
			Counters: []CounterConfig{{Name: "counter", MetricRep: MetricRep{Name: "metric"}}},
		},
	}

	cfg.(*Config).MetricMetaData = map[string]MetricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       GaugeMetric{},
		},
	}

	assert.NoError(t, component.ValidateConfig(cfg))
}

func TestCreateTracesReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []ObjectConfig{
		{
			Object:   "object",
			Counters: []CounterConfig{{Name: "counter", MetricRep: MetricRep{Name: "metric"}}},
		},
	}

	cfg.(*Config).MetricMetaData = map[string]MetricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       GaugeMetric{},
		},
	}
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)
}

func TestCreateTracesReceiverNoMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []ObjectConfig{
		{
			Object:   "object",
			Counters: []CounterConfig{{Name: "counter"}},
		},
	}
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)
}

func TestCreateLogsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []ObjectConfig{
		{
			Object:   "object",
			Counters: []CounterConfig{{Name: "counter", MetricRep: MetricRep{Name: "metric"}}},
		},
	}

	cfg.(*Config).MetricMetaData = map[string]MetricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       GaugeMetric{},
		},
	}

	tReceiver, err := factory.CreateLogsReceiver(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, tReceiver)
}
