// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory builds the processor factory wired for functional composition sources.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Traces,
) (processor.Traces, error) {
	source, err := createLookupSource(set, cfg)
	if err != nil {
		return nil, err
	}
	proc := newLookupProcessor(source, set.Logger)

	return processorhelper.NewTraces(
		ctx,
		set,
		cfg,
		next,
		proc.processTraces,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(proc.Start),
		processorhelper.WithShutdown(proc.Shutdown),
	)
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Metrics,
) (processor.Metrics, error) {
	source, err := createLookupSource(set, cfg)
	if err != nil {
		return nil, err
	}
	proc := newLookupProcessor(source, set.Logger)

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		next,
		proc.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(proc.Start),
		processorhelper.WithShutdown(proc.Shutdown),
	)
}

func createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	source, err := createLookupSource(set, cfg)
	if err != nil {
		return nil, err
	}
	proc := newLookupProcessor(source, set.Logger)

	return processorhelper.NewLogs(
		ctx,
		set,
		cfg,
		next,
		proc.processLogs,
		processorhelper.WithCapabilities(processorCapabilities),
		processorhelper.WithStart(proc.Start),
		processorhelper.WithShutdown(proc.Shutdown),
	)
}

func createLookupSource(set processor.Settings, cfg component.Config) (lookup.Source, error) {
	processorCfg := cfg.(*Config)

	if processorCfg.Source.Extension == (component.ID{}) {
		return nil, errors.New("lookup extension must be configured")
	}

	return newHostedExtensionSource(processorCfg.Source.Extension, set.Logger), nil
}

func newHostedExtensionSource(extensionID component.ID, logger *zap.Logger) lookup.Source {
	es := &extensionSource{
		extensionID: extensionID,
		logger:      logger,
	}

	return lookup.NewSource(
		lookup.LookupFunc(es.lookup),
		lookup.TypeFunc(es.sourceType),
		lookup.StartFunc(es.start),
		lookup.ShutdownFunc(es.shutdown),
	)
}

type extensionSource struct {
	extensionID component.ID
	extension   lookup.LookupExtension
	logger      *zap.Logger
}

func (e *extensionSource) lookup(ctx context.Context, key string) (any, bool, error) {
	if e.extension == nil {
		return nil, false, fmt.Errorf("lookup extension %q not initialized", e.extensionID)
	}
	return e.extension.Lookup(ctx, key)
}

func (e *extensionSource) sourceType() string {
	if e.extension != nil {
		return e.extension.Type()
	}
	return e.extensionID.String()
}

func (e *extensionSource) start(ctx context.Context, host component.Host) error {
	_ = ctx
	if host == nil {
		return fmt.Errorf("no host provided to resolve lookup extension %q", e.extensionID)
	}

	inst := host.GetExtensions()[e.extensionID]
	if inst == nil {
		return fmt.Errorf("extension %q not found", e.extensionID)
	}

	lookupExt, ok := inst.(lookup.LookupExtension)
	if !ok {
		return fmt.Errorf("extension %q is not a lookup extension", e.extensionID)
	}

	e.extension = lookupExt
	if e.logger != nil {
		e.logger.Info("using lookup extension", zap.String("extension", e.extensionID.String()))
	}

	return nil
}

func (e *extensionSource) shutdown(ctx context.Context) error {
	_ = ctx
	e.extension = nil
	return nil
}
