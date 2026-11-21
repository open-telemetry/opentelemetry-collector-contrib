// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deadband // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deadband"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

var typeID = component.MustNewType("deadband")

// NewFactory creates the deadband processor factory.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		typeID,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Mode:            ModeAbsolute,
		Threshold:       1.0,
		StaleAfter:      5 * time.Minute,
		RepeatAttribute: defaultRepeatAttribute,
		State: StateConfig{
			MaxSeries: 100_000,
		},
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (processor.Metrics, error) {
	c := cfg.(*Config)
	return newDeadbandProcessor(ctx, c, set, next)
}
