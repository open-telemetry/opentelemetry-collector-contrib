// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windowsperfcountersreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateMetrics(t *testing.T) {
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

	mReceiver, err := factory.CreateMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())

	assert.NoError(t, err)
	assert.NotNil(t, mReceiver)
}
