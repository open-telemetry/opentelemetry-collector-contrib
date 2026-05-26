// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package spanmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
)

const (
	DefaultNamespace = "traces.span.metrics"
)

// NewFactory creates a factory for the spanmetrics connector.
func NewFactory() connector.Factory {
	return xconnector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xconnector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
		xconnector.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AggregationTemporality:   "AGGREGATION_TEMPORALITY_CUMULATIVE",
		ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
		MetricsFlushInterval:     60 * time.Second,
		Histogram: HistogramConfig{Disable: false, Unit: func() metrics.Unit {
			if metadata.ConnectorSpanmetricsUseSecondAsDefaultMetricsUnitFeatureGate.IsEnabled() {
				return metrics.Seconds
			}

			return metrics.Milliseconds
		}()},
		Namespace:                   DefaultNamespace,
		AggregationCardinalityLimit: 0,
		Exemplars: ExemplarsConfig{
			MaxPerDataPoint: defaultMaxPerDatapoint,
		},
	}
}

func createTracesToMetricsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	instanceID, ok := params.Resource.Attributes().Get(collectorInstanceKey)
	// This never happens: the OpenTelemetry Collector automatically adds this attribute.
	// See: https://github.com/open-telemetry/opentelemetry-collector/blob/main/service/internal/resource/config.go#L31
	//
	// The fallback logic below exists solely for lifecycle tests in generated_component_test.go,
	// where the mocked telemetry setting does not include the service.instance.id attribute.
	if !ok {
		instanceUUID, _ := uuid.NewRandom()
		instanceID = pcommon.NewValueStr(instanceUUID.String())
	}

	c, err := newConnector(params.Logger, cfg, clockwork.FromContext(ctx), instanceID.AsString())
	if err != nil {
		return nil, err
	}
	c.metricsConsumer = nextConsumer
	return c, nil
}
