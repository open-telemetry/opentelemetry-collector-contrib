// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

func newCardinalityProcessor(
	ctx context.Context,
	cfg *Config,
	set processor.Settings,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		func(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
			return md, nil
		},
		processorhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		processorhelper.WithStart(func(_ context.Context, _ component.Host) error {
			return nil
		}),
		processorhelper.WithShutdown(func(_ context.Context) error {
			return nil
		}),
	)
}
