// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"context"
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/source/noop"
	yamlsource "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/source/yaml"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

var Type = metadata.Type

type FactoryOption func(*lookupProcessorFactory)

// WithSources adds custom source factories to the processor.
// This REPLACES the default sources on first call, then MERGES on subsequent calls.
// (Same pattern as transform processor's WithXxxFunctions)
//
// Example:
//
//	lookupprocessor.NewFactoryWithOptions(
//	    lookupprocessor.WithSources(httplookup.NewFactory()),
//	)
func WithSources(factories ...lookupsource.SourceFactory) FactoryOption {
	return func(f *lookupProcessorFactory) {
		if !f.defaultSourcesOverridden {
			f.sources = make(map[string]lookupsource.SourceFactory)
			f.defaultSourcesOverridden = true
		}
		for _, factory := range factories {
			f.sources[factory.Type()] = factory
		}
	}
}

type lookupProcessorFactory struct {
	sources                  map[string]lookupsource.SourceFactory
	defaultSourcesOverridden bool
}

func defaultSources() map[string]lookupsource.SourceFactory {
	return map[string]lookupsource.SourceFactory{
		"noop": noop.NewFactory(),
		"yaml": yamlsource.NewFactory(),
	}
}

func NewFactory() processor.Factory {
	return NewFactoryWithOptions()
}

// NewFactoryWithOptions creates a lookup processor factory with custom sources.
//
// Example (third-party HTTP source):
//
//	import (
//	    "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"
//	    "github.com/user/otel-lookup-http/httplookup"
//	)
//
//	factories.Processors[lookupprocessor.Type] = lookupprocessor.NewFactoryWithOptions(
//	    lookupprocessor.WithSources(httplookup.NewFactory()),
//	)
func NewFactoryWithOptions(options ...FactoryOption) processor.Factory {
	f := &lookupProcessorFactory{
		sources: defaultSources(),
	}
	for _, opt := range options {
		opt(f)
	}

	return processor.NewFactory(
		metadata.Type,
		f.createDefaultConfig,
		processor.WithLogs(f.createLogsProcessor, metadata.LogsStability),
	)
}

func (*lookupProcessorFactory) createDefaultConfig() component.Config {
	return &Config{
		Source: SourceConfig{
			Type: "noop",
		},
	}
}

func (f *lookupProcessorFactory) createLogsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	next consumer.Logs,
) (processor.Logs, error) {
	processorCfg := cfg.(*Config)

	source, err := f.createSource(ctx, set, processorCfg)
	if err != nil {
		return nil, err
	}

	proc := newLookupProcessor(source, processorCfg, set.Logger)

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

func (f *lookupProcessorFactory) createSource(
	ctx context.Context,
	set processor.Settings,
	cfg *Config,
) (lookupsource.Source, error) {
	sourceType := cfg.Source.Type
	if sourceType == "" {
		sourceType = "noop"
	}

	factory, ok := f.sources[sourceType]
	if !ok {
		return nil, fmt.Errorf("unknown source type %q", sourceType)
	}

	sourceCfg := factory.CreateDefaultConfig()
	if len(cfg.Source.Config) > 0 {
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			TagName:          "mapstructure",
			Result:           sourceCfg,
			WeaklyTypedInput: true,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
			),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create decoder for source %q: %w", sourceType, err)
		}
		if err := decoder.Decode(cfg.Source.Config); err != nil {
			return nil, fmt.Errorf("failed to decode config for source %q: %w", sourceType, err)
		}
	}

	if err := sourceCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config for source %q: %w", sourceType, err)
	}

	createSettings := lookupsource.CreateSettings{
		TelemetrySettings: set.TelemetrySettings,
		BuildInfo:         set.BuildInfo,
	}

	return factory.CreateSource(ctx, createSettings, sourceCfg)
}
