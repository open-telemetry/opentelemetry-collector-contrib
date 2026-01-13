// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector/internal/metadata"
)

const (
	virtualNodeFeatureGateID              = "connector.servicegraph.virtualNode"
	legacyLatencyMetricNamesFeatureGateID = "connector.servicegraph.legacyLatencyMetricNames"
	legacyLatencyUnitMs                   = "connector.servicegraph.legacyLatencyUnitMs"
)

var virtualNodeFeatureGate, legacyMetricNamesFeatureGate, legacyLatencyUnitMsFeatureGate *featuregate.Gate

func init() {
	virtualNodeFeatureGate = featuregate.GlobalRegistry().MustRegister(
		virtualNodeFeatureGateID,
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("When enabled and setting `virtual_node_peer_attributes` is not empty, the connector looks for the presence of these attributes in span to create virtual server nodes."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17196"),
	)
	// TODO: Remove this feature gate when the legacy metric names are removed.
	legacyMetricNamesFeatureGate = featuregate.GlobalRegistry().MustRegister(
		legacyLatencyMetricNamesFeatureGateID,
		featuregate.StageAlpha, // Alpha because we want it disabled by default.
		featuregate.WithRegisterDescription("When enabled, connector uses legacy latency metric names."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/18743,https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/16578"),
	)
	legacyLatencyUnitMsFeatureGate = featuregate.GlobalRegistry().MustRegister(
		legacyLatencyUnitMs,
		featuregate.StageAlpha, // Alpha because we want it disabled by default.
		featuregate.WithRegisterDescription("When enabled, connector reports latency in milliseconds, instead of seconds."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/27488"),
	)
}

// NewFactory returns a ConnectorFactory.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, metadata.TracesToMetricsStability),
	)
}

func createDefaultConfig() component.Config {
	bounds := []time.Duration{
		2 * time.Millisecond,
		4 * time.Millisecond,
		6 * time.Millisecond,
		8 * time.Millisecond,
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		1 * time.Second,
		1400 * time.Millisecond,
		2 * time.Second,
		5 * time.Second,
		10 * time.Second,
		15 * time.Second,
	}

	return &Config{
		Store: StoreConfig{
			TTL:      2 * time.Second,
			MaxItems: 1000,
		},
		CacheLoop:              time.Minute,
		StoreExpirationLoop:    2 * time.Second,
		MetricsTimestampOffset: 0,
		VirtualNodePeerAttributes: []string{
			string(semconv.PeerServiceKey), string(semconv.DBNameKey), string(semconv.DBSystemKey),
		},
		DatabaseNameAttributes:  []string{string(semconv.DBNameKey)},
		MetricsFlushInterval:    60 * time.Second, // 1 DPM
		LatencyHistogramBuckets: bounds,
	}
}

func createTracesToMetricsConnector(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	return newConnector(params.TelemetrySettings, cfg, nextConsumer)
}
