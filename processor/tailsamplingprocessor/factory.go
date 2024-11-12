// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

// NewFactory returns a new factory for the Tail Sampling processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		DecisionWait: 30 * time.Second,
		NumTraces:    50000,
	}
}

func createTracesProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	tCfg := cfg.(*Config)
	opts := []Option{}

	if telemetry.IsRecordPolicyEnabled() {
		opts = append(opts, withRecordPolicy())
	}
	return newTracesProcessor(ctx, params, nextConsumer, *tCfg, opts...)
}
