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
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metadata"
)

const (
	DefaultNamespace               = "traces.span.metrics"
	legacyMetricNamesFeatureGateID = "connector.spanmetrics.legacyMetricNames"
	includeServiceInstanceIDGateID = "connector.spanmetrics.includeServiceInstanceID"
)

var (
	legacyMetricNamesFeatureGate *featuregate.Gate
	includeServiceInstanceID     *featuregate.Gate
)

func init() {
	// TODO: Remove this feature gate when the legacy metric names are removed.
	legacyMetricNamesFeatureGate = featuregate.GlobalRegistry().MustRegister(
		legacyMetricNamesFeatureGateID,
		featuregate.StageAlpha, // Alpha because we want it disabled by default.
		featuregate.WithRegisterDescription("When enabled, connector uses legacy metric names."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33227"),
	)
	includeServiceInstanceID = featuregate.GlobalRegistry().MustRegister(
		includeServiceInstanceIDGateID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, connector add service.instance.id to default dimensions."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/40400"),
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
		AggregationTemporality:      "AGGREGATION_TEMPORALITY_CUMULATIVE",
		ResourceMetricsCacheSize:    defaultResourceMetricsCacheSize,
		MetricsFlushInterval:        60 * time.Second,
		Histogram:                   HistogramConfig{Disable: false, Unit: defaultUnit},
		Namespace:                   DefaultNamespace,
		AggregationCardinalityLimit: 0,
		Exemplars: ExemplarsConfig{
			MaxPerDataPoint: defaultMaxPerDatapoint,
		},
	}
}

func createTracesToMetricsConnector(ctx context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	instanceID, ok := params.Resource.Attributes().Get(string(conventions.ServiceInstanceIDKey))
	// this never happen, just for test
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
