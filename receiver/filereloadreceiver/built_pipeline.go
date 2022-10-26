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

package filereloadereceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// baseConsumer redeclared here since not public in consumer package. May consider to make that public.
type baseConsumer interface {
	Capabilities() consumer.Capabilities
}

type builtComponent struct {
	id   config.ComponentID
	comp component.Component
}

type pipeline struct {
	logger       *zap.Logger
	processors   []builtComponent
	lastConsumer baseConsumer
}

type builtPipelines map[config.ComponentID]*pipeline

func buildPipelines(
	ctx context.Context,
	settings component.TelemetrySettings,
	buildInfo component.BuildInfo,
	host component.Host,
	cfg *runnerConfig,
	next baseConsumer,
) (builtPipelines, error) {
	pipelines := make(builtPipelines, len(cfg.PartialPipelines))

	for id, pl := range cfg.PartialPipelines {
		p := &pipeline{
			logger: settings.Logger.With(
				zap.String(ZapNameKey, ZapKindPipeline),
				zap.String(ZapNameKey, id.String()),
			),
			processors:   make([]builtComponent, len(pl.Processors)),
			lastConsumer: next,
		}

		mutatesConsumedData := next.Capabilities().MutatesData
		for i := len(pl.Processors) - 1; i >= 0; i-- {
			procID := pl.Processors[i]

			proc, err := buildProcessor(ctx, settings, buildInfo, cfg.Processors, host, procID, id, p.lastConsumer)
			if err != nil {
				return nil, err
			}
			p.processors[i] = builtComponent{id: procID, comp: proc}
			p.lastConsumer = proc.(baseConsumer)
			mutatesConsumedData = mutatesConsumedData || p.lastConsumer.Capabilities().MutatesData
		}

		// Some consumers may not correctly implement the Capabilities, and ignore the next consumer when calculated the Capabilities.
		// Because of this wrap the first consumer if any consumers in the pipeline mutate the data and the first says that it doesn't.
		switch id.Type() {
		case config.MetricsDataType:
			p.lastConsumer = capMetrics{Metrics: p.lastConsumer.(consumer.Metrics), cap: consumer.Capabilities{MutatesData: mutatesConsumedData}}
		case config.LogsDataType:
			p.lastConsumer = capLogs{Logs: p.lastConsumer.(consumer.Logs), cap: consumer.Capabilities{MutatesData: mutatesConsumedData}}
		default:
			return nil, fmt.Errorf("create cap consumer in pipeline %q, data type %q is not supported", id, id.Type())
		}

		pipelines[id] = p
	}

	return pipelines, nil
}

func (p *pipeline) StartProcessors(ctx context.Context, host component.Host) error {
	p.logger.Info("starting processors")
	for i := len(p.processors) - 1; i >= 0; i-- {
		if err := p.processors[i].comp.Start(ctx, host); err != nil {
			return err
		}
	}
	p.logger.Info("starting processors done")

	return nil
}

func (p *pipeline) StopProcessors(ctx context.Context) error {
	p.logger.Info("stopping processors")
	var errs error
	for _, p := range p.processors {
		errs = multierr.Append(errs, p.comp.Shutdown(ctx))
	}

	p.logger.Info("stopping done")

	return errs
}

func buildProcessor(ctx context.Context,
	settings component.TelemetrySettings,
	buildInfo component.BuildInfo,
	cfgs map[config.ComponentID]config.Processor,
	host component.Host,
	id config.ComponentID,
	pipelineID config.ComponentID,
	next baseConsumer,
) (component.Processor, error) {
	procCfg, existsCfg := cfgs[id]
	if !existsCfg {
		return nil, fmt.Errorf("processor %q is not configured", id)
	}

	factory := host.GetFactory(component.KindProcessor, id.Type())
	if factory == nil {
		return nil, fmt.Errorf("unable to lookup factory for processor %q", id.String())
	}

	set := component.ProcessorCreateSettings{
		TelemetrySettings: settings,
		BuildInfo:         buildInfo,
	}
	set.TelemetrySettings.Logger = processorLogger(settings.Logger, id)

	proc, err := createProcessor(ctx, set, procCfg, id, pipelineID, next, factory.(component.ProcessorFactory))
	if err != nil {
		return nil, fmt.Errorf("failed to create %q processor, in pipeline %q: %w", id, pipelineID, err)
	}
	return proc, nil
}

func createProcessor(ctx context.Context, set component.ProcessorCreateSettings, cfg config.Processor, id config.ComponentID, pipelineID config.ComponentID, next baseConsumer, factory component.ProcessorFactory) (component.Processor, error) {
	switch pipelineID.Type() {
	case config.TracesDataType:
		return factory.CreateTracesProcessor(ctx, set, cfg, next.(consumer.Traces))

	case config.MetricsDataType:
		return factory.CreateMetricsProcessor(ctx, set, cfg, next.(consumer.Metrics))

	case config.LogsDataType:
		return factory.CreateLogsProcessor(ctx, set, cfg, next.(consumer.Logs))
	}
	return nil, fmt.Errorf("error creating processor %q in pipeline %q, data type %q is not supported", id, pipelineID, pipelineID.Type())
}

func processorLogger(logger *zap.Logger, procID config.ComponentID) *zap.Logger {
	return logger.With(
		zap.String(ZapKindKey, ZapKindProcessor),
		zap.String(ZapNameKey, procID.String()),
	)
}
