// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotreloadprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/otelcol"
	otelpipeline "go.opentelemetry.io/collector/pipeline"
	otelprocessor "go.opentelemetry.io/collector/processor"
)

// HostWithFactories extends component.Host to provide access to processor factories.
type HostWithFactories interface {
	component.Host
	GetFactory(kind component.Kind, componentType component.Type) component.Factory
}

func loadSubprocessors[T any](
	ctx context.Context,
	config otelcol.Config,
	settings otelprocessor.Settings,
	host component.Host,
	nextConsumer T,
	createConsumer func(ctx context.Context, factory otelprocessor.Factory, settings otelprocessor.Settings, config component.Config, nextConsumer T, signal otelpipeline.Signal) (T, error),
) ([]T, error) {
	nc := nextConsumer

	if len(config.Service.Pipelines) == 0 {
		return nil, fmt.Errorf("no pipelines found")
	}

	if len(config.Service.Pipelines) > 1 {
		return nil, fmt.Errorf("only one pipeline is supported")
	}

	var subprocessors []T
	for id, pipeline := range config.Service.Pipelines {
		subprocessors = make([]T, len(pipeline.Processors))
		for i := len(pipeline.Processors) - 1; i >= 0; i-- {
			processor := pipeline.Processors[i]
			hostWithFactories, ok := host.(HostWithFactories)
			if !ok {
				return nil, fmt.Errorf("host does not implement HostWithFactories interface")
			}
			factory := hostWithFactories.GetFactory(component.KindProcessor, processor.Type())
			if factory == nil {
				return nil, fmt.Errorf("processor factory not found for type: %s", processor.Type())
			}
			processorFactory, ok := factory.(otelprocessor.Factory)
			if !ok {
				return nil, fmt.Errorf("factory for type %s is not a processor factory", processor.Type())
			}
			conf := processorFactory.CreateDefaultConfig()
			processorConfig := config.Processors[processor].(map[string]interface{})
			newConfMap := confmap.NewFromStringMap(processorConfig)
			err := newConfMap.Unmarshal(&conf)
			if err != nil {
				return nil, err
			}

			nc, err = createConsumer(ctx, processorFactory, otelprocessor.Settings{
				ID:                processor,
				BuildInfo:         settings.BuildInfo,
				TelemetrySettings: settings.TelemetrySettings,
			}, conf, nc, id.Signal())
			if err != nil {
				return nil, err
			}
			subprocessors[i] = nc
		}
	}

	return subprocessors, nil
}

func loadLogsSubprocessors(
	ctx context.Context,
	config otelcol.Config,
	settings otelprocessor.Settings,
	host component.Host,
	nextConsumer consumer.Logs,
) ([]consumer.Logs, error) {
	return loadSubprocessors(
		ctx,
		config,
		settings,
		host,
		nextConsumer,
		func(ctx context.Context, factory otelprocessor.Factory, settings otelprocessor.Settings, config component.Config, nextConsumer consumer.Logs, signal otelpipeline.Signal) (consumer.Logs, error) {
			if signal != otelpipeline.SignalLogs {
				return nil, fmt.Errorf("unsupported signal: %s", signal)
			}
			return factory.CreateLogs(ctx, settings, config, nextConsumer)
		},
	)
}

func loadMetricsSubprocessors(
	ctx context.Context,
	config otelcol.Config,
	settings otelprocessor.Settings,
	host component.Host,
	nextConsumer consumer.Metrics,
) ([]consumer.Metrics, error) {
	return loadSubprocessors(
		ctx,
		config,
		settings,
		host,
		nextConsumer,
		func(ctx context.Context, factory otelprocessor.Factory, settings otelprocessor.Settings, config component.Config, nextConsumer consumer.Metrics, signal otelpipeline.Signal) (consumer.Metrics, error) {
			if signal != otelpipeline.SignalMetrics {
				return nil, fmt.Errorf("unsupported signal: %s", signal)
			}
			return factory.CreateMetrics(ctx, settings, config, nextConsumer)
		},
	)
}

func loadTracesSubprocessors(
	ctx context.Context,
	config otelcol.Config,
	settings otelprocessor.Settings,
	host component.Host,
	nextConsumer consumer.Traces,
) ([]consumer.Traces, error) {
	return loadSubprocessors(
		ctx,
		config,
		settings,
		host,
		nextConsumer,
		func(ctx context.Context, factory otelprocessor.Factory, settings otelprocessor.Settings, config component.Config, nextConsumer consumer.Traces, signal otelpipeline.Signal) (consumer.Traces, error) {
			if signal != otelpipeline.SignalTraces {
				return nil, fmt.Errorf("unsupported signal: %s", signal)
			}
			return factory.CreateTraces(ctx, settings, config, nextConsumer)
		},
	)
}

