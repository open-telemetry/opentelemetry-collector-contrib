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
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// typeStr is the value of "type" for this processor in the configuration.
	typeStr config.Type = "groupbytrace"

	defaultWaitDuration   = time.Second
	defaultNumTraces      = 1_000_000
	defaultNumWorkers     = 1
	defaultDiscardOrphans = false
	defaultStoreOnDisk    = false
)

var (
	errDiskStorageNotSupported    = fmt.Errorf("option 'disk storage' not supported in this release")
	errDiscardOrphansNotSupported = fmt.Errorf("option 'discard orphans' not supported in this release")
)

// NewFactory returns a new factory for the Filter processor.
func NewFactory() component.ProcessorFactory {
	// TODO: find a more appropriate way to get this done, as we are swallowing the error here
	_ = view.Register(MetricViews()...)

	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithTraces(createTracesProcessor))
}

// createDefaultConfig creates the default configuration for the processor.
func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
		NumTraces:         defaultNumTraces,
		NumWorkers:        defaultNumWorkers,
		WaitDuration:      defaultWaitDuration,

		// not supported for now
		DiscardOrphans: defaultDiscardOrphans,
		StoreOnDisk:    defaultStoreOnDisk,
	}
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	_ context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Traces) (component.TracesProcessor, error) {

	oCfg := cfg.(*Config)

	var st storage
	if oCfg.StoreOnDisk {
		return nil, errDiskStorageNotSupported
	}
	if oCfg.DiscardOrphans {
		return nil, errDiscardOrphansNotSupported
	}

	// the only supported storage for now
	st = newMemoryStorage()

	return newGroupByTraceProcessor(params.Logger, st, nextConsumer, *oCfg), nil
}
