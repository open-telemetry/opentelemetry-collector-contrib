// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

const (
	CollectorHealthStatusMetric = "supervisor.agent.health_status"
)

type Metrics struct {
	collectorHealthStatusMetric metric.Int64UpDownCounter
	healthStatus                bool
}

func NewMetrics(meterProvider metric.MeterProvider) (*Metrics, error) {
	meter := meterProvider.Meter("opamp-supervisor")

	healthStatus, err := meter.Int64UpDownCounter(
		CollectorHealthStatusMetric,
		metric.WithDescription("Current health status of the collector (1=healthy, 0=unhealthy)"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize metrics to 0 to ensure they are exported
	healthStatus.Add(context.Background(), 0)
	return &Metrics{
		collectorHealthStatusMetric: healthStatus,
		healthStatus:                false,
	}, nil
}

func (m *Metrics) SetCollectorHealthStatus(ctx context.Context, healthy bool) {
	// Only update the metric if the health status has changed
	if !m.healthStatus && healthy {
		m.collectorHealthStatusMetric.Add(ctx, 1)
		m.healthStatus = healthy
	} else if m.healthStatus && !healthy {
		m.collectorHealthStatusMetric.Add(ctx, -1)
		m.healthStatus = healthy
	}
}
