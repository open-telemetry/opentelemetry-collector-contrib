// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exportercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/exportercreator"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter"
	exp "go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// runner starts and stops exporter instances.
type runner interface {
	// start an exporter instance from its static config and discovered config.
	start(exporter exporterConfig, discoveredConfig userConfigMap, signals exporterSignals) (component.Component, error)
	// shutdown an exporter.
	shutdown(exp component.Component) error
}

// exporterRunner handles starting/stopping of a concrete subexporter instance.
type exporterRunner struct {
	logger      *zap.Logger
	params      exporter.Settings
	idNamespace component.ID
	host        host
	exporters   map[string]*wrappedExporter
	lock        *sync.Mutex
}

func newExporterRunner(params exporter.Settings, host host) *exporterRunner {
	return &exporterRunner{
		logger:      params.Logger,
		params:      params,
		idNamespace: params.ID,
		host:        host,
		exporters:   map[string]*wrappedExporter{},
		lock:        &sync.Mutex{},
	}
}

var _ runner = (*exporterRunner)(nil)

func (run *exporterRunner) start(
	exporterCfg exporterConfig,
	discoveredConfig userConfigMap,
	signals exporterSignals,
) (component.Component, error) {
	factory := run.host.GetFactory(component.KindExporter, exporterCfg.id.Type())

	if factory == nil {
		return nil, fmt.Errorf("unable to lookup factory for exporter %q", exporterCfg.id.String())
	}

	exporterFactory := factory.(exp.Factory)

	cfg, targetEndpoint, err := run.loadRuntimeExporterConfig(exporterFactory, exporterCfg, discoveredConfig)
	if err != nil {
		return nil, err
	}

	// Sets dynamically created exporter to something like exporter_creator/1/otlp{endpoint="localhost:4317"}/<EndpointID>.
	id := component.NewIDWithName(factory.Type(), fmt.Sprintf("%s/%s{endpoint=%q}/%s", exporterCfg.id.Name(), run.idNamespace, targetEndpoint, exporterCfg.endpointID))

	we := &wrappedExporter{}
	var createError error

	// Create exporter instances only for enabled signals
	if signals.logs {
		if we.logs, err = run.createLogsRuntimeExporter(exporterFactory, id, cfg); err != nil {
			if errors.Is(err, pipeline.ErrSignalNotSupported) {
				run.logger.Debug("instantiated exporter doesn't support logs", zap.String("exporter", exporterCfg.id.String()))
				we.logs = nil
			} else {
				createError = multierr.Combine(createError, err)
			}
		}
	}
	if signals.metrics {
		if we.metrics, err = run.createMetricsRuntimeExporter(exporterFactory, id, cfg); err != nil {
			if errors.Is(err, pipeline.ErrSignalNotSupported) {
				run.logger.Debug("instantiated exporter doesn't support metrics", zap.String("exporter", exporterCfg.id.String()))
				we.metrics = nil
			} else {
				createError = multierr.Combine(createError, err)
			}
		}
	}
	if signals.traces {
		if we.traces, err = run.createTracesRuntimeExporter(exporterFactory, id, cfg); err != nil {
			if errors.Is(err, pipeline.ErrSignalNotSupported) {
				run.logger.Debug("instantiated exporter doesn't support traces", zap.String("exporter", exporterCfg.id.String()))
				we.traces = nil
			} else {
				createError = multierr.Combine(createError, err)
			}
		}
	}

	if createError != nil {
		return nil, fmt.Errorf("failed creating endpoint-derived exporter: %w", createError)
	}

	// Check if at least one signal is supported
	if we.logs == nil && we.metrics == nil && we.traces == nil {
		return nil, fmt.Errorf("exporter %q does not support any signal type", exporterCfg.id.String())
	}

	if err = we.Start(context.Background(), run.host); err != nil {
		return nil, fmt.Errorf("failed starting endpoint-derived exporter: %w", err)
	}

	return we, nil
}

// shutdown the given exporter.
func (*exporterRunner) shutdown(exp component.Component) error {
	return exp.Shutdown(context.Background())
}

// loadRuntimeExporterConfig loads the given exporterTemplate merged with config values
// that may have been discovered at runtime.
func (*exporterRunner) loadRuntimeExporterConfig(
	factory exp.Factory,
	exporterCfg exporterConfig,
	discoveredConfig userConfigMap,
) (component.Config, string, error) {
	// Merge templated and discovered configs
	mergedConfig, targetEndpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, exporterCfg.config, discoveredConfig)
	if err != nil {
		return nil, targetEndpoint, fmt.Errorf("failed to merge constituent template configs: %w", err)
	}

	exporterCfgMap := factory.CreateDefaultConfig()
	if err := mergedConfig.Unmarshal(exporterCfgMap); err != nil {
		return nil, "", fmt.Errorf("failed to load %q template config: %w", exporterCfg.id.String(), err)
	}
	if err := xconfmap.Validate(exporterCfgMap); err != nil {
		return nil, "", fmt.Errorf("invalid runtime exporter config: exporters::%s: %w", exporterCfg.id, err)
	}
	return exporterCfgMap, targetEndpoint, nil
}

// mergeTemplatedAndDiscoveredConfigs will unify the templated and discovered configs,
// setting the `endpoint` field from the discovered one if 1. not specified by the user
// and 2. determined to be supported (by trial and error of unmarshalling a temp intermediary).
func mergeTemplatedAndDiscoveredConfigs(factory exp.Factory, templated, discovered userConfigMap) (*confmap.Conf, string, error) {
	const endpointConfigKey = "endpoint"
	const tmpSetEndpointConfigKey = "<tmp.exporter.creator.automatically.set.endpoint.field>"

	targetEndpoint := cast.ToString(templated[endpointConfigKey])

	// Check if template has an endpoint configured
	templateHasEndpoint := targetEndpoint != "" && targetEndpoint != "0"

	if _, endpointSet := discovered[tmpSetEndpointConfigKey]; endpointSet {
		delete(discovered, tmpSetEndpointConfigKey)

		// If template already has an endpoint, don't override it with discovered endpoint
		if !templateHasEndpoint {
			// Template doesn't have endpoint, use discovered one
			targetEndpoint = cast.ToString(discovered[endpointConfigKey])

			// confirm the endpoint we've added is supported, removing if not
			endpointConfig := confmap.NewFromStringMap(map[string]any{
				endpointConfigKey: targetEndpoint,
			})
			if err := endpointConfig.Unmarshal(factory.CreateDefaultConfig()); err != nil {
				// we assume that the error is due to unused keys in the config, so we need to remove endpoint key
				delete(discovered, endpointConfigKey)
			}
		} else {
			// Template has endpoint, remove discovered endpoint to prevent override during merge
			delete(discovered, endpointConfigKey)
		}
	}

	discoveredConfig := confmap.NewFromStringMap(discovered)
	templatedConfig := confmap.NewFromStringMap(templated)

	// Merge in discoveredConfig containing values discovered at runtime.
	// confmap.Merge merges discovered into templated, so discovered values override templated.
	// We've already removed endpoint from discovered if template has one, so template endpoint will be preserved.
	if err := templatedConfig.Merge(discoveredConfig); err != nil {
		return nil, targetEndpoint, fmt.Errorf("failed to merge template config from discovered runtime values: %w", err)
	}

	return templatedConfig, targetEndpoint, nil
}

// createLogsRuntimeExporter creates an exporter that is discovered at runtime.
func (run *exporterRunner) createLogsRuntimeExporter(
	factory exp.Factory,
	id component.ID,
	cfg component.Config,
) (exp.Logs, error) {
	runParams := run.params
	runParams.Logger = runParams.Logger.With(zap.String("name", id.String()))
	runParams.ID = id
	return factory.CreateLogs(context.Background(), runParams, cfg)
}

// createMetricsRuntimeExporter creates an exporter that is discovered at runtime.
func (run *exporterRunner) createMetricsRuntimeExporter(
	factory exp.Factory,
	id component.ID,
	cfg component.Config,
) (exp.Metrics, error) {
	runParams := run.params
	runParams.Logger = runParams.Logger.With(zap.String("name", id.String()))
	runParams.ID = id
	return factory.CreateMetrics(context.Background(), runParams, cfg)
}

// createTracesRuntimeExporter creates an exporter that is discovered at runtime.
func (run *exporterRunner) createTracesRuntimeExporter(
	factory exp.Factory,
	id component.ID,
	cfg component.Config,
) (exp.Traces, error) {
	runParams := run.params
	runParams.Logger = runParams.Logger.With(zap.String("name", id.String()))
	runParams.ID = id
	return factory.CreateTraces(context.Background(), runParams, cfg)
}

var _ component.Component = (*wrappedExporter)(nil)

type wrappedExporter struct {
	logs    exp.Logs
	metrics exp.Metrics
	traces  exp.Traces
}

func (w *wrappedExporter) Start(ctx context.Context, host component.Host) error {
	var err error
	for _, e := range []component.Component{w.logs, w.metrics, w.traces} {
		if e != nil {
			if e := e.Start(ctx, host); e != nil {
				err = multierr.Combine(err, e)
			}
		}
	}
	return err
}

func (w *wrappedExporter) Shutdown(ctx context.Context) error {
	var err error
	for _, e := range []component.Component{w.logs, w.metrics, w.traces} {
		if e != nil {
			if e := e.Shutdown(ctx); e != nil {
				err = multierr.Combine(err, e)
			}
		}
	}
	return err
}
