// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/spf13/cast"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

// runner starts and stops receiver instances.
type runner interface {
	// start a metrics receiver instance from its static config and discovered config.
	start(receiver receiverConfig, discoveredConfig userConfigMap, consumer *enhancingConsumer) (component.Component, error)
	// shutdown a receiver.
	shutdown(rcvr component.Component) error
}

// receiverRunner handles starting/stopping of a concrete subreceiver instance.
type receiverRunner struct {
	logger      *zap.Logger
	params      rcvr.CreateSettings
	idNamespace component.ID
	host        component.Host
	receivers   map[string]*wrappedReceiver
	lock        *sync.Mutex
}

func newReceiverRunner(params rcvr.CreateSettings, host component.Host) *receiverRunner {
	return &receiverRunner{
		logger:      params.Logger,
		params:      params,
		idNamespace: params.ID,
		host:        &loggingHost{host, params.Logger},
		receivers:   map[string]*wrappedReceiver{},
		lock:        &sync.Mutex{},
	}
}

var _ runner = (*receiverRunner)(nil)

func (run *receiverRunner) start(
	receiver receiverConfig,
	discoveredConfig userConfigMap,
	consumer *enhancingConsumer,
) (component.Component, error) {
	factory := run.host.GetFactory(component.KindReceiver, receiver.id.Type())

	if factory == nil {
		return nil, fmt.Errorf("unable to lookup factory for receiver %q", receiver.id.String())
	}

	receiverFactory := factory.(rcvr.Factory)

	cfg, targetEndpoint, err := run.loadRuntimeReceiverConfig(receiverFactory, receiver, discoveredConfig)
	if err != nil {
		return nil, err
	}

	// Sets dynamically created receiver to something like receiver_creator/1/redis{endpoint="localhost:6380"}/<EndpointID>.
	id := component.NewIDWithName(factory.Type(), fmt.Sprintf("%s/%s{endpoint=%q}/%s", receiver.id.Name(), run.idNamespace, targetEndpoint, receiver.endpointID))

	wr := &wrappedReceiver{}
	var createError error
	if consumer.logs != nil {
		if wr.logs, err = run.createLogsRuntimeReceiver(receiverFactory, id, cfg, consumer); err != nil {
			if errors.Is(err, component.ErrDataTypeIsNotSupported) {
				run.logger.Info("instantiated receiver doesn't support logs", zap.String("receiver", receiver.id.String()), zap.Error(err))
				wr.logs = nil
			} else {
				createError = multierr.Combine(createError, err)
			}
		}
	}
	if consumer.metrics != nil {
		if wr.metrics, err = run.createMetricsRuntimeReceiver(receiverFactory, id, cfg, consumer); err != nil {
			if errors.Is(err, component.ErrDataTypeIsNotSupported) {
				run.logger.Info("instantiated receiver doesn't support metrics", zap.String("receiver", receiver.id.String()), zap.Error(err))
				wr.metrics = nil
			} else {
				createError = multierr.Combine(createError, err)
			}
		}
	}
	if consumer.traces != nil {
		if wr.traces, err = run.createTracesRuntimeReceiver(receiverFactory, id, cfg, consumer); err != nil {
			if errors.Is(err, component.ErrDataTypeIsNotSupported) {
				run.logger.Info("instantiated receiver doesn't support traces", zap.String("receiver", receiver.id.String()), zap.Error(err))
				wr.traces = nil
			} else {
				createError = multierr.Combine(createError, err)
			}
		}
	}

	if createError != nil {
		return nil, fmt.Errorf("failed creating endpoint-derived receiver: %w", createError)
	}

	if err = wr.Start(context.Background(), run.host); err != nil {
		return nil, fmt.Errorf("failed starting endpoint-derived receiver: %w", createError)
	}

	return wr, nil
}

// shutdown the given receiver.
func (run *receiverRunner) shutdown(rcvr component.Component) error {
	return rcvr.Shutdown(context.Background())
}

// loadRuntimeReceiverConfig loads the given receiverTemplate merged with config values
// that may have been discovered at runtime.
func (run *receiverRunner) loadRuntimeReceiverConfig(
	factory rcvr.Factory,
	receiver receiverConfig,
	discoveredConfig userConfigMap,
) (component.Config, string, error) {
	// remove dynamically added "endpoint" field if not supported by receiver
	mergedConfig, targetEndpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, receiver.config, discoveredConfig)
	if err != nil {
		return nil, targetEndpoint, fmt.Errorf("failed to merge constituent template configs: %w", err)
	}

	receiverCfg := factory.CreateDefaultConfig()
	if err := component.UnmarshalConfig(mergedConfig, receiverCfg); err != nil {
		return nil, "", fmt.Errorf("failed to load %q template config: %w", receiver.id.String(), err)
	}
	return receiverCfg, targetEndpoint, nil
}

// mergeTemplateAndDiscoveredConfigs will unify the templated and discovered configs,
// setting the `endpoint` field from the discovered one if 1. not specified by the user
// and 2. determined to be supported (by trial and error of unmarshalling a temp intermediary).
func mergeTemplatedAndDiscoveredConfigs(factory rcvr.Factory, templated, discovered userConfigMap) (*confmap.Conf, string, error) {
	targetEndpoint := cast.ToString(templated[endpointConfigKey])
	if _, endpointSet := discovered[tmpSetEndpointConfigKey]; endpointSet {
		delete(discovered, tmpSetEndpointConfigKey)
		targetEndpoint = cast.ToString(discovered[endpointConfigKey])

		// confirm the endpoint we've added is supported, removing if not
		endpointConfig := confmap.NewFromStringMap(map[string]any{
			endpointConfigKey: targetEndpoint,
		})
		if err := endpointConfig.Unmarshal(factory.CreateDefaultConfig(), confmap.WithErrorUnused()); err != nil {
			// rather than attach to error content that can change over time,
			// confirm the error only arises w/ ErrorUnused mapstructure setting ("invalid keys")
			if err = endpointConfig.Unmarshal(factory.CreateDefaultConfig(), confmap.WithIgnoreUnused()); err == nil {
				delete(discovered, endpointConfigKey)
			}
		}
	}
	discoveredConfig := confmap.NewFromStringMap(discovered)
	templatedConfig := confmap.NewFromStringMap(templated)

	// Merge in discoveredConfig containing values discovered at runtime.
	if err := templatedConfig.Merge(discoveredConfig); err != nil {
		return nil, targetEndpoint, fmt.Errorf("failed to merge template config from discovered runtime values: %w", err)
	}
	return templatedConfig, targetEndpoint, nil
}

// createLogsRuntimeReceiver creates a receiver that is discovered at runtime.
func (run *receiverRunner) createLogsRuntimeReceiver(
	factory rcvr.Factory,
	id component.ID,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (rcvr.Logs, error) {
	runParams := run.params
	runParams.Logger = runParams.Logger.With(zap.String("name", id.String()))
	runParams.ID = id
	return factory.CreateLogsReceiver(context.Background(), runParams, cfg, nextConsumer)
}

// createMetricsRuntimeReceiver creates a receiver that is discovered at runtime.
func (run *receiverRunner) createMetricsRuntimeReceiver(
	factory rcvr.Factory,
	id component.ID,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (rcvr.Metrics, error) {
	runParams := run.params
	runParams.Logger = runParams.Logger.With(zap.String("name", id.String()))
	runParams.ID = id
	return factory.CreateMetricsReceiver(context.Background(), runParams, cfg, nextConsumer)
}

// createTracesRuntimeReceiver creates a receiver that is discovered at runtime.
func (run *receiverRunner) createTracesRuntimeReceiver(
	factory rcvr.Factory,
	id component.ID,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (rcvr.Traces, error) {
	runParams := run.params
	runParams.Logger = runParams.Logger.With(zap.String("name", id.String()))
	runParams.ID = id
	return factory.CreateTracesReceiver(context.Background(), runParams, cfg, nextConsumer)
}

var _ component.Component = (*wrappedReceiver)(nil)

type wrappedReceiver struct {
	logs    rcvr.Logs
	metrics rcvr.Metrics
	traces  rcvr.Traces
}

func (w *wrappedReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	for _, r := range []component.Component{w.logs, w.metrics, w.traces} {
		if r != nil {
			if e := r.Start(ctx, host); e != nil {
				err = multierr.Combine(err, e)
			}
		}
	}
	return err
}

func (w *wrappedReceiver) Shutdown(ctx context.Context) error {
	var err error
	for _, r := range []component.Component{w.logs, w.metrics, w.traces} {
		if r != nil {
			if e := r.Shutdown(ctx); e != nil {
				err = multierr.Combine(err, e)
			}
		}
	}
	return err
}
