// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package redactionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/metadata"
)

// NewFactory creates a factory for the redaction processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

// createTracesProcessor creates an instance of redaction for processing traces
func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)

	redaction, err := newRedaction(ctx, oCfg, set.Logger)
	if err != nil {
		// TODO: Placeholder for an error metric in the next PR
		return nil, fmt.Errorf("error creating a redaction processor: %w", err)
	}

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		redaction.processTraces,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

// createLogsProcessor creates an instance of redaction for processing logs
func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	oCfg := cfg.(*Config)
	logCfg := *oCfg
	// Attributes are defined for metrics and traces:
	// https://opentelemetry.io/docs/specs/semconv/database/
	// For logs, we don't rely on the "db.system.name" attribute to
	// do the sanitization.
	logCfg.DBSanitizer.AllowFallbackWithoutSystem = true

	red, err := newRedaction(ctx, &logCfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("error creating a redaction processor: %w", err)
	}

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		next,
		red.processLogs,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}

// createMetricsProcessor creates an instance of redaction for processing metrics
func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)

	red, err := newRedaction(ctx, oCfg, set.Logger)
	if err != nil {
		return nil, fmt.Errorf("error creating a redaction processor: %w", err)
	}

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		next,
		red.processMetrics,
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}))
}
