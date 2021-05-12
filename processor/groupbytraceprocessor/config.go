// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbytraceprocessor

import (
	"time"

	"go.opentelemetry.io/collector/config"
)

// Config is the configuration for the processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// NumTraces is the max number of traces to keep in memory waiting for the duration.
	// Default: 1_000_000.
	NumTraces int `mapstructure:"num_traces"`

	// NumWorkers is a number of workers processing event queue. Should be equal to physical processors number.
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
}
