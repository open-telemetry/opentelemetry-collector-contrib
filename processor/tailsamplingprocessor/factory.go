// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

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
		DecisionWait:          30 * time.Second,
		NumTraces:             50000,
		SampleOnFirstMatch:    false,
		BucketCount:           10,  // Divide decision_wait into 10 time slices
		TracesPerBucketFactor: 1.1, // Allow 10% overage before compaction
	}
}

func createTracesProcessor(
	ctx context.Context,
	params processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	tCfg := cfg.(*Config)

	// Validate bucket configuration
	if tCfg.BucketCount == 0 {
		params.Logger.Warn("Invalid bucket_count (must be > 0), using default", zap.Uint64("default", 10))
		tCfg.BucketCount = 10
	}
	if tCfg.TracesPerBucketFactor <= 1.0 {
		params.Logger.Warn("Invalid traces_per_bucket_factor (must be > 1.0), using default", zap.Float64("default", 1.1))
		tCfg.TracesPerBucketFactor = 1.1
	}

	// Policy recording enabled by default (feature gate finalized)
	return newTracesProcessor(ctx, params, nextConsumer, *tCfg)
}
