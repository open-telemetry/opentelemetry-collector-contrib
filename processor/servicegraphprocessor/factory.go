// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor"
)

const (
	// The value of "type" key in configuration.
	typeStr = "servicegraph"
	// The stability level of the processor.
	connectorStability                    = component.StabilityLevelDevelopment
	virtualNodeFeatureGateID              = "processor.servicegraph.virtualNode"
	legacyLatencyMetricNamesFeatureGateID = "processor.servicegraph.legacyLatencyMetricNames"
	legacyLatencyUnitMs                   = "processor.servicegraph.legacyLatencyUnitMs"
)

var virtualNodeFeatureGate, legacyMetricNamesFeatureGate, legacyLatencyUnitMsFeatureGate *featuregate.Gate

func init() {
	virtualNodeFeatureGate = featuregate.GlobalRegistry().MustRegister(
		virtualNodeFeatureGateID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, when the edge expires, processor checks if it has peer attributes(`db.name, net.sock.peer.addr, net.peer.name, rpc.service, http.url, http.target`), and then aggregate the metrics with virtual node."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17196"),
	)
	// TODO: Remove this feature gate when the legacy metric names are removed.
	legacyMetricNamesFeatureGate = featuregate.GlobalRegistry().MustRegister(
		legacyLatencyMetricNamesFeatureGateID,
		featuregate.StageAlpha, // Alpha because we want it disabled by default.
		featuregate.WithRegisterDescription("When enabled, processor uses legacy latency metric names."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18743,https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16578"),
	)
	legacyLatencyUnitMsFeatureGate = featuregate.GlobalRegistry().MustRegister(
		legacyLatencyUnitMs,
		featuregate.StageAlpha, // Alpha because we want it disabled by default.
		featuregate.WithRegisterDescription("When enabled, processor reports latency in milliseconds, instead of seconds."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27488"),
	)
}

// NewFactory creates a factory for the servicegraph processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, connectorStability),
	)
}

// NewConnectorFactoryFunc creates a function that returns a factory for the servicegraph connector.
var NewConnectorFactoryFunc = func(cfgType component.Type, tracesToMetricsStability component.StabilityLevel) connector.Factory {
	return connector.NewFactory(
		cfgType,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, tracesToMetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Store: StoreConfig{
			TTL:      2 * time.Second,
			MaxItems: 1000,
		},
		CacheLoop:           time.Minute,
		StoreExpirationLoop: 2 * time.Second,
	}
}

func createTracesProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	p := newProcessor(params.TelemetrySettings, cfg)
	p.tracesConsumer = nextConsumer
	return p, nil
}

func createTracesToMetricsConnector(_ context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	c := newProcessor(params.TelemetrySettings, cfg)
	c.metricsConsumer = nextConsumer
	return c, nil
}
