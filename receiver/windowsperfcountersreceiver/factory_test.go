// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsperfcountersreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/metadata"
)

var creationParams = receivertest.NewNopSettings(metadata.Type)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	cfg.(*Config).PerfCounters = []objectConfig{
		{
			Object:   "object",
			Counters: []counterConfig{{Name: "counter", metricRep: metricRep{Name: "metric"}}},
		},
	}

	cfg.(*Config).MetricMetaData = map[string]metricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       gaugeMetric{},
		},
	}

	assert.NoError(t, xconfmap.Validate(cfg))
}

func TestCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []objectConfig{
		{
			Object:   "object",
			Counters: []counterConfig{{Name: "counter", metricRep: metricRep{Name: "metric"}}},
		},
	}

	cfg.(*Config).MetricMetaData = map[string]metricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       gaugeMetric{},
		},
	}
	tReceiver, err := factory.CreateTraces(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.ErrorIs(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tReceiver)
}

func TestCreateTracesNoMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []objectConfig{
		{
			Object:   "object",
			Counters: []counterConfig{{Name: "counter"}},
		},
	}
	tReceiver, err := factory.CreateTraces(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.ErrorIs(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tReceiver)
}

func TestCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).PerfCounters = []objectConfig{
		{
			Object:   "object",
			Counters: []counterConfig{{Name: "counter", metricRep: metricRep{Name: "metric"}}},
		},
	}

	cfg.(*Config).MetricMetaData = map[string]metricConfig{
		"metric": {
			Description: "desc",
			Unit:        "1",
			Gauge:       gaugeMetric{},
		},
	}

	tReceiver, err := factory.CreateLogs(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.ErrorIs(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, tReceiver)
}
