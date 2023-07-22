// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windowsperfcountersreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateMetricsReceiver(t *testing.T) {
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

	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)
}
