// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor"

import (
	"context"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/servicegraphprocessor/internal/metadata"
)

const (
	// The value of "type" key in configuration.
	// The stability level of the processor.
	connectorStability       = component.StabilityLevelDevelopment
	virtualNodeFeatureGateID = "processor.servicegraph.virtualNode"
)

var virtualNodeFeatureGate *featuregate.Gate

func init() {
	virtualNodeFeatureGate = featuregate.GlobalRegistry().MustRegister(
		virtualNodeFeatureGateID,
		featuregate.StageAlpha,
		featuregate.WithRegisterDescription("When enabled, when the edge expires, processor checks if it has peer attributes(`db.name, net.sock.peer.addr, net.peer.name, rpc.service, http.url, http.target`), and then aggregate the metrics with virtual node."),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17196"),
	)
}

// NewFactory creates a factory for the servicegraph processor.
func NewFactory() processor.Factory {
	// TODO: Handle this err
	_ = view.Register(serviceGraphProcessorViews()...)

	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

// NewConnectorFactory creates a factory for the servicegraph connector.
func NewConnectorFactory() connector.Factory {
	// TODO: Handle this err
	_ = view.Register(serviceGraphProcessorViews()...)

	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToMetrics(createTracesToMetricsConnector, connectorStability),
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
	p := newProcessor(params.Logger, cfg)
	p.tracesConsumer = nextConsumer
	return p, nil
}

func createTracesToMetricsConnector(_ context.Context, params connector.CreateSettings, cfg component.Config, nextConsumer consumer.Metrics) (connector.Traces, error) {
	c := newProcessor(params.Logger, cfg)
	c.metricsConsumer = nextConsumer
	return c, nil
}
