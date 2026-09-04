// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

// newRollingSpanLatencyProcessor builds the rolling_span_latency processor.
// This is a pass-through stub: the rolling EWMA baseline tracking and
// attribute-labeling logic lands in a follow-up PR once this component's
// structure has been reviewed and merged.
func newRollingSpanLatencyProcessor(
	ctx context.Context,
	cfg *Config,
	set processor.Settings,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		nextConsumer,
		func(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
			return td, nil
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
