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
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
)

const (
	DefaultNamespace               = "traces.span.metrics"
	legacyMetricNamesFeatureGateID = "connector.spanmetrics.legacyMetricNames"
)

var instances = sharedcomponent.NewSharedComponents()

var legacyMetricNamesFeatureGate *featuregate.Gate

func init() {
	// TODO: Remove this feature gate when the legacy metric names are removed.
	legacyMetricNamesFeatureGate = featuregate.GlobalRegistry().MustRegister(
		legacyMetricNamesFeatureGateID,
		featuregate.StageAlpha, // Alpha because we want it disabled by default.
		featuregate.WithRegisterDescription("When enabled, connector uses legacy metric names."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33227"),
	)
}

// NewFactory creates a factory for the spanmetrics connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
		connector.WithTracesToTraces(createTracesToTracesConnector, metadata.TracesToTracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AggregationTemporality:      "AGGREGATION_TEMPORALITY_CUMULATIVE",
		ResourceMetricsCacheSize:    defaultResourceMetricsCacheSize,
		MetricsFlushInterval:        60 * time.Second,
		Histogram:                   HistogramConfig{Disable: false, Unit: defaultUnit},
		Namespace:                   DefaultNamespace,
		AggregationCardinalityLimit: 0,
	}
}

func createTracesToMetricsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	c, err := getOrCreateConnector(ctx, cfg, params.Logger)
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = nextConsumer
	return c, nil
}

func createTracesToTracesConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Traces) (connector.Traces, error) {
	c, err := getOrCreateConnector(ctx, cfg, params.Logger)
	if err != nil {
		return nil, err
	}
	c.traceConsumer = nextConsumer
	return c, nil
}

// By using a shared component, we ensure that we can forward the traces with exemplars marked as exemplars
func getOrCreateConnector(ctx context.Context, cfg component.Config, logger *zap.Logger) (*connectorImp, error) {
	var err error
	inst := instances.GetOrAdd(cfg, func() component.Component {
		var c *connectorImp
		c, err = newConnector(logger, cfg, clockwork.FromContext(ctx))
		if err != nil {
			logger.Error("Failed to create spanmetrics connector", zap.Error(err))
			return nil
		}
		return c
	})

	if err != nil {
		return nil, err
	}

	return inst.Unwrap().(*connectorImp), nil
}
