// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package spanmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"

import (
	"context"
	"time"

	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metadata"
)

// NewFactory creates a factory for the spanmetrics connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AggregationTemporality:   "AGGREGATION_TEMPORALITY_CUMULATIVE",
		DimensionsCacheSize:      defaultDimensionsCacheSize,
		ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
		MetricsFlushInterval:     60 * time.Second,
		Histogram:                HistogramConfig{Disable: false, Unit: defaultUnit},
	}
}

func createTracesToMetricsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	c, err := newConnector(params.Logger, cfg, clockwork.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = nextConsumer
	return c, nil
}
