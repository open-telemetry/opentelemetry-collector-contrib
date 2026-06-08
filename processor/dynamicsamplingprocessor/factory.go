// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/metadata"
)

// NewFactory returns a new factory for the dynamic sampling processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TraceTimeout:  30 * time.Second,
		DecisionDelay: 2 * time.Second,
		NumTraces:     50_000,
		DecisionCache: DecisionCacheConfig{
			SampledCacheSize:    10_000,
			NonSampledCacheSize: 10_000,
		},
	}
}

func createTracesProcessor(_ context.Context, set processor.Settings, cfg component.Config, next consumer.Traces) (processor.Traces, error) {
	c := cfg.(*Config)
	return newProcessor(set, c, next)
}
