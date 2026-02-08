// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var (
	_ exporter.Logs    = (*exporterCreator)(nil)
	_ exporter.Metrics = (*exporterCreator)(nil)
	_ exporter.Traces  = (*exporterCreator)(nil)
	_ consumer.Logs    = (*exporterCreator)(nil)
	_ consumer.Metrics = (*exporterCreator)(nil)
	_ consumer.Traces  = (*exporterCreator)(nil)
)

// exporterCreator implements the exporter that dynamically creates sub-exporters at runtime.
type exporterCreator struct {
	params          exporter.Settings
	cfg             *Config
	observerHandler *observerHandler
	observables     []observer.Observable
	router          *telemetryRouter
	telemetry       *metadata.TelemetryBuilder
}

func newExporterCreator(params exporter.Settings, cfg *Config) (*exporterCreator, error) {
	telemetry, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	ec := &exporterCreator{
		params:    params,
		cfg:       cfg,
		router:    newTelemetryRouter(cfg.Routing.Rules, telemetry),
		telemetry: telemetry,
	}
	return ec, nil
}

// Start exporter_creator.
func (ec *exporterCreator) Start(_ context.Context, h component.Host) error {
	// TODO: Implement in PR2
	// 1. Find observer extensions
	// 2. Create observer handler
	// 3. Subscribe to observers
	_ = h
	
	// Initialize the gauge metric to 0
	if ec.telemetry != nil {
		ec.telemetry.ExporterCreatorExportersCount.Record(context.Background(), 0)
	}
	return nil
}

// Shutdown stops the exporter_creator and all its exporters started at runtime.
func (ec *exporterCreator) Shutdown(context.Context) error {
	// TODO: Implement in PR2
	// 1. Unsubscribe from observers
	// 2. Shutdown all sub-exporters
	if ec.telemetry != nil {
		ec.telemetry.Shutdown()
	}
	return nil
}

// Capabilities implements consumer.Logs, consumer.Metrics, consumer.Traces.
func (ec *exporterCreator) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs routes logs to the appropriate sub-exporter based on resource attributes.
func (ec *exporterCreator) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	// TODO: Implement in PR2
	// 1. For each resource, extract attributes
	// 2. Route to matching sub-exporters
	// 3. Route unmatched to default exporters
	return nil
}

// ConsumeMetrics routes metrics to the appropriate sub-exporter based on resource attributes.
func (ec *exporterCreator) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// TODO: Implement in PR2
	return nil
}

// ConsumeTraces routes traces to the appropriate sub-exporter based on resource attributes.
func (ec *exporterCreator) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	// TODO: Implement in PR2
	return nil
}
