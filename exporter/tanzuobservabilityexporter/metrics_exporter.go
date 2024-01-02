// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"

import (
	"context"
	"fmt"

	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricsExporter struct {
	consumer *metricsConsumer
}

func createMetricsConsumer(config MetricsConfig, settings component.TelemetrySettings, otelVersion string) (*metricsConsumer, error) {
	s, err := senders.NewSender(config.Endpoint,
		senders.FlushIntervalSeconds(60),
		senders.SDKMetricsTags(map[string]string{"otel.metrics.collector_version": otelVersion}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy sender: %w", err)
	}
	cumulative := newCumulativeHistogramDataPointConsumer(s)
	delta := newDeltaHistogramDataPointConsumer(s)
	return newMetricsConsumer(
		[]typedMetricConsumer{
			newGaugeConsumer(s, settings),
			newSumConsumer(s, settings),
			newHistogramConsumer(cumulative, delta, s, regularHistogram, settings),
			newHistogramConsumer(cumulative, delta, s, exponentialHistogram, settings),
			newSummaryConsumer(s, settings),
		},
		s,
		true, config), nil
}

type metricsConsumerCreator func(config MetricsConfig, settings component.TelemetrySettings, otelVersion string) (
	*metricsConsumer, error)

func newMetricsExporter(settings exporter.CreateSettings, c component.Config, creator metricsConsumerCreator) (*metricsExporter, error) {
	cfg, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", c)
	}
	if !cfg.hasMetricsEndpoint() {
		return nil, fmt.Errorf("metrics.endpoint required")
	}
	if _, _, err := cfg.parseMetricsEndpoint(); err != nil {
		return nil, fmt.Errorf("failed to parse metrics.endpoint: %w", err)
	}
	consumer, err := creator(cfg.Metrics, settings.TelemetrySettings, settings.BuildInfo.Version)
	if err != nil {
		return nil, err
	}
	return &metricsExporter{
		consumer: consumer,
	}, nil
}

func (e *metricsExporter) pushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	return e.consumer.Consume(ctx, md)
}

func (e *metricsExporter) shutdown(_ context.Context) error {
	e.consumer.Close()
	return nil
}
