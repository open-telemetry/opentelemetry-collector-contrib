// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package isolationforestprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor/internal/metadata"
)

// NewFactory returns a new factory for the Isolation Forest processor
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(createMetrics, metadata.MetricsStability),
	)
}

// createDefaultConfig creates the default configuration for the processor
func createDefaultConfig() component.Config {
	return &Config{
		NumTrees:              100,
		SubsampleSize:         256,
		WindowSize:           1000,
		AnomalyThreshold:     0.6,
		TrainingInterval:     5 * time.Minute,
		MetricsToAnalyze:     []string{},
		Features:             []string{},
		AddAnomalyScore:      true,
		DropAnomalousMetrics: false,
	}
}

// createMetrics creates a metrics processor based on the config
func createMetrics(
	_ context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)
	return newIsolationForestProcessor(set, oCfg, nextConsumer)
}
