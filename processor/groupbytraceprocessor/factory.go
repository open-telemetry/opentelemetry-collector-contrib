// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

const (
	defaultWaitDuration   = time.Second
	defaultNumTraces      = 1_000_000
	defaultNumWorkers     = 1
	defaultDiscardOrphans = false
	defaultStoreOnDisk    = false
)

// NewFactory returns a new factory for the Filter processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

// createDefaultConfig creates the default configuration for the processor.
func createDefaultConfig() component.Config {
	return &Config{
		NumTraces:    defaultNumTraces,
		NumWorkers:   defaultNumWorkers,
		WaitDuration: defaultWaitDuration,

		// not supported for now
		DiscardOrphans: defaultDiscardOrphans,
		StoreOnDisk:    defaultStoreOnDisk,

		EmitStrategy: EmitStrategyTrace,
	}
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	_ context.Context,
	params processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	oCfg := cfg.(*Config)

	processor := newGroupByTraceProcessor(params, nextConsumer, *oCfg)
	// the only supported storage for now
	processor.st = newMemoryStorage(processor.telemetryBuilder)

	if oCfg.EmitStrategy == EmitStrategyService {
		processor.eventMachine.onSubtraceTick = processor.onSubtraceTick

		// Each worker gets its own ring buffer, deadlines and span storage. Traces
		// are routed to a worker by trace ID, so nothing is shared between them.
		for _, w := range processor.eventMachine.workers {
			w.subtraceBuffer = newSubtraceRingBuffer(max(1, oCfg.NumTraces/oCfg.NumWorkers))
			w.deadlines = newSubtraceDeadlines()
			w.subSt = newSubtraceMemoryStorage(processor.telemetryBuilder)
		}
	}

	return processor, nil
}
