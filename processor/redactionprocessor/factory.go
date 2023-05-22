// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "redaction"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the redaction processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

// createTracesProcessor creates an instance of redaction for processing traces
func createTracesProcessor(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)

	redaction, err := newRedaction(ctx, oCfg, set.Logger)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("error creating a redaction processor: %w", err)
	}

	return processorhelper.NewTracesProcessor(
		ctx,
		set,
		cfg,
		next,
		redaction.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}
