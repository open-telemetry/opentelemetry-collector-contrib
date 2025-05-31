// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/metric"
)

const (
	CollectorStartupAttemptsMetric = "opamp.supervisor.collector.startup_attempts"
	CollectorStartupErrorsMetric   = "opamp.supervisor.collector.startup_errors"
	CollectorConfigErrorsMetric    = "opamp.supervisor.collector.config_errors"
	CollectorHealthStatusMetric    = "opamp.supervisor.collector.health_status"
)

type Metrics struct {
	CollectorStartupAttempts    metric.Int64Counter
	CollectorStartupErrors      metric.Int64Counter
	CollectorConfigErrors       metric.Int64Counter
	collectorHealthStatusMetric metric.Int64UpDownCounter
	healthStatus                bool
}

func NewMetrics(meterProvider metric.MeterProvider) (*Metrics, error) {
	meter := meterProvider.Meter("opamp-supervisor")

	startupAttempts, err := meter.Int64Counter(
		CollectorStartupAttemptsMetric,
		metric.WithDescription("Number of collector startup attempts"),
	)
	if err != nil {
		return nil, err
	}

	startupErrors, err := meter.Int64Counter(
		CollectorStartupErrorsMetric,
		metric.WithDescription("Number of collector startup errors"),
	)
	if err != nil {
		return nil, err
	}

	configErrors, err := meter.Int64Counter(
		CollectorConfigErrorsMetric,
		metric.WithDescription("Number of configuration errors"),
	)
	if err != nil {
		return nil, err
	}

	healthStatus, err := meter.Int64UpDownCounter(
		CollectorHealthStatusMetric,
		metric.WithDescription("Current health status of the collector (1=healthy, 0=unhealthy)"),
	)
	if err != nil {
		return nil, err
	}

	// Initialize metrics to 0 to ensure they are exported
	healthStatus.Add(context.Background(), 0)
	configErrors.Add(context.Background(), 0)
	startupAttempts.Add(context.Background(), 0)
	startupErrors.Add(context.Background(), 0)
	return &Metrics{
		CollectorStartupAttempts:    startupAttempts,
		CollectorConfigErrors:       configErrors,
		CollectorStartupErrors:      startupErrors,
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
