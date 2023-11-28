// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats/view"
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

var (
	errDiskStorageNotSupported    = fmt.Errorf("option 'disk storage' not supported in this release")
	errDiscardOrphansNotSupported = fmt.Errorf("option 'discard orphans' not supported in this release")
)

// NewFactory returns a new factory for the Filter processor.
func NewFactory() processor.Factory {
	// TODO: find a more appropriate way to get this done, as we are swallowing the error here
	_ = view.Register(metricViews()...)

	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability))
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
	}
}

// createTracesProcessor creates a trace processor based on this config.
func createTracesProcessor(
	_ context.Context,
	params processor.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces) (processor.Traces, error) {

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
