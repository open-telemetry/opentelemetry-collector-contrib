// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"time"
)

// EmitStrategy controls how the processor groups spans before releasing them to
// the next consumer.
type EmitStrategy string

const (
	// EmitStrategyTrace (default) buffers all spans for a trace together and
	// releases them as one batch after wait_duration.
	EmitStrategyTrace EmitStrategy = "trace"

	// EmitStrategyService buffers spans per service subtree within a trace and
	// releases each subtree separately after wait_duration.
	EmitStrategyService EmitStrategy = "service"
)

// Config is the configuration for the processor.
type Config struct {
	// NumTraces is the max number of traces to keep in memory waiting for the duration.
	// Default: 1_000_000.
	NumTraces int `mapstructure:"num_traces"`

	// NumWorkers is a number of workers processing event queue.
	// Default: 1.
	NumWorkers int `mapstructure:"num_workers"`

	// WaitDuration tells the processor to wait for the specified duration for the trace to be complete.
	// Default: 1s.
	WaitDuration time.Duration `mapstructure:"wait_duration"`

	// DiscardOrphans instructs the processor to discard traces without the root span.
	// This typically indicates that the trace is incomplete.
	// Default: false.
	// Not yet implemented, and an error will be returned when this option is used.
	DiscardOrphans bool `mapstructure:"discard_orphans"`

	// StoreOnDisk tells the processor to keep only the trace ID in memory, serializing the trace spans to disk.
	// Useful when the duration to wait for traces to complete is high.
	// Default: false.
	// Not yet implemented, and an error will be returned when this option is used.
	StoreOnDisk bool `mapstructure:"store_on_disk"`

	// EmitStrategy controls the span-emit granularity.
	// Valid values: "trace" (default), "service".
	EmitStrategy EmitStrategy `mapstructure:"emit_strategy"`
}
