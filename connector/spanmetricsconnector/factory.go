// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package spanmetricsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metrics"
)

const (
	DefaultNamespace                           = "traces.span.metrics"
	legacyMetricNamesFeatureGateID             = "connector.spanmetrics.legacyMetricNames"
	includeCollectorInstanceIDFeatureGateID    = "connector.spanmetrics.includeCollectorInstanceID"
	useSecondAsDefaultMetricsUnitFeatureGateID = "connector.spanmetrics.useSecondAsDefaultMetricsUnit"
	excludeResourceMetricsFeatureGate          = "connector.spanmetrics.excludeResourceMetrics"
)

var (
	legacyMetricNamesFeatureGate  *featuregate.Gate
	includeCollectorInstanceID    *featuregate.Gate
	useSecondAsDefaultMetricsUnit *featuregate.Gate
	excludeResourceMetrics        *featuregate.Gate
)

func init() {
	// TODO: Remove this feature gate when the legacy metric names are removed.
	legacyMetricNamesFeatureGate = featuregate.GlobalRegistry().MustRegister(
		legacyMetricNamesFeatureGateID,
		featuregate.StageAlpha, // Alpha because we want it disabled by default.
		featuregate.WithRegisterDescription("When enabled, connector uses legacy metric names."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33227"),
	)
	includeCollectorInstanceID = featuregate.GlobalRegistry().MustRegister(
		includeCollectorInstanceIDFeatureGateID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, connector add collector.instance.id to default dimensions."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40400"),
	)
	useSecondAsDefaultMetricsUnit = featuregate.GlobalRegistry().MustRegister(
		useSecondAsDefaultMetricsUnitFeatureGateID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, connector use second as default unit for duration metrics."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42462"),
	)
	excludeResourceMetrics = featuregate.GlobalRegistry().MustRegister(
		excludeResourceMetricsFeatureGate,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, connector will exclude all resource attributes."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42103"),
	)
}

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
		ResourceMetricsCacheSize: defaultResourceMetricsCacheSize,
		MetricsFlushInterval:     60 * time.Second,
		Histogram: HistogramConfig{Disable: false, Unit: func() metrics.Unit {
			if useSecondAsDefaultMetricsUnit.IsEnabled() {
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
