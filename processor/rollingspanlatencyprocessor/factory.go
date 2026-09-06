// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package rollingspanlatencyprocessor is documented in doc.go.
package rollingspanlatencyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor/internal/metadata"
)

// NewFactory creates and registers the rolling_span_latency processor
// factory with the OpenTelemetry Collector.
//
// The component is registered at StabilityLevelDevelopment, which signals to
// operators that the API and configuration schema may change between
// releases.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

// createDefaultConfig returns a *Config populated with conservative defaults.
func createDefaultConfig() component.Config {
	return &Config{
		AttributeKey:          "latency.category",
		ResourceKeyAttributes: []string{"service.namespace", "service.name", "deployment.environment.name"},
		HalfLife:              2 * time.Hour,
		IdleTimeout:           8 * time.Hour,
		EvictionInterval:      10 * time.Minute,
		SlowThreshold:         3.0,
		VerySlowThreshold:     4.0,
		ChurnWarningRatio:     0.5,
		MinStddev:             time.Millisecond,
		MaxBaselines:          0,
		WarmupCount:           30,
	}
}

// createTracesProcessor is the constructor wired into the factory by
// processor.WithTraces. The Collector calls it once per pipeline that
// references this component, passing the merged (default + user)
// configuration as a component.Config interface.
func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: expected *Config, got %T", cfg)
	}
	return newRollingSpanLatencyProcessor(ctx, oCfg, set, nextConsumer)
}
